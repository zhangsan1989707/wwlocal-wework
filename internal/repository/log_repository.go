package repository

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type LogRepository struct {
	DB           *gorm.DB
	tableCreated map[string]bool // 缓存已确认存在的表
	tableMu      sync.RWMutex
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{DB: db, tableCreated: make(map[string]bool)}
}

func (r *LogRepository) GetTableName(featureID int, t time.Time) string {
	return fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))
}

func (r *LogRepository) TableExists(tableName string) bool {
	var result int
	r.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&result)
	return result > 0
}

func encDataHash(encData string) string {
	h := md5.Sum([]byte(encData))
	return hex.EncodeToString(h[:])
}

func (r *LogRepository) CreateTableIfNotExists(featureID int, month time.Time) error {
	tableName := r.GetTableName(featureID, month)
	r.tableMu.RLock()
	cached := r.tableCreated[tableName]
	r.tableMu.RUnlock()
	if cached {
		return nil
	}
	err := r.DB.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			feature_id INT NOT NULL,
			log_time BIGINT NOT NULL,
			idc VARCHAR(32),
			enc_data TEXT,
			enc_key TEXT,
			raw_json TEXT,
			parsed_json JSON,
			enc_data_hash CHAR(32),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_log_time (log_time),
			INDEX idx_feature_logtime (feature_id, log_time),
			UNIQUE KEY uk_dedup (feature_id, log_time, enc_data_hash)
		)
	`, tableName)).Error
	if err != nil {
		return err
	}
	r.MigrateLogTable(tableName)
	r.tableMu.Lock()
	r.tableCreated[tableName] = true
	r.tableMu.Unlock()
	return nil
}

func (r *LogRepository) MigrateLogTable(tableName string) {
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN enc_data_hash CHAR(32) AFTER parsed_json", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE INDEX uk_dedup (feature_id, log_time, enc_data_hash)", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN login_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.login_user.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_login_openid (login_openid)", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN sender_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.sender.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_sender_openid (sender_openid)", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN root_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_root_openid (root_openid)", tableName))
}

func (r *LogRepository) Save(featureID int, entry *model.LogEntry) error {
	month := time.Unix(entry.LogTime, 0)
	if err := r.CreateTableIfNotExists(featureID, month); err != nil {
		return err
	}

	tableName := r.GetTableName(featureID, month)
	hash := encDataHash(entry.EncData)
	return r.DB.Exec(fmt.Sprintf(`
		INSERT IGNORE INTO %s (feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, enc_data_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, tableName), entry.FeatureID, entry.LogTime, entry.IDC, entry.EncData, entry.EncKey, entry.RawJSON, entry.ParsedJSON, hash).Error
}

func (r *LogRepository) BatchSave(featureID int, entries []model.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	monthGroups := make(map[time.Time][]model.LogEntry)
	for _, entry := range entries {
		month := time.Unix(entry.LogTime, 0)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		monthGroups[month] = append(monthGroups[month], entry)
	}

	for month, group := range monthGroups {
		if err := r.CreateTableIfNotExists(featureID, month); err != nil {
			return err
		}

		tableName := r.GetTableName(featureID, month)
		tx := r.DB.Begin()
		for i := 0; i < len(group); i += 500 {
			end := i + 500
			if end > len(group) {
				end = len(group)
			}
			batch := group[i:end]

			sql := fmt.Sprintf("INSERT IGNORE INTO %s (feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, enc_data_hash) VALUES ", tableName)
			var args []interface{}
			for j, entry := range batch {
				hash := encDataHash(entry.EncData)
				if j > 0 {
					sql += ","
				}
				sql += "(?, ?, ?, ?, ?, ?, ?, ?)"
				args = append(args, entry.FeatureID, entry.LogTime, entry.IDC, entry.EncData, entry.EncKey, entry.RawJSON, entry.ParsedJSON, hash)
			}
			if err := tx.Exec(sql, args...).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
		if err := tx.Commit().Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *LogRepository) BatchSaveWithTx(tx *gorm.DB, featureID int, entries []model.LogEntry) (map[string]bool, int, error) {
	if len(entries) == 0 {
		return nil, 0, nil
	}

	savedMobiles := make(map[string]bool)
	savedCount := 0

	monthGroups := make(map[time.Time][]model.LogEntry)
	for _, entry := range entries {
		month := time.Unix(entry.LogTime, 0)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		monthGroups[month] = append(monthGroups[month], entry)
	}

	for month, group := range monthGroups {
		if err := r.CreateTableIfNotExists(featureID, month); err != nil {
			return nil, 0, err
		}

		tableName := r.GetTableName(featureID, month)
		for i := 0; i < len(group); i += 500 {
			end := i + 500
			if end > len(group) {
				end = len(group)
			}
			batch := group[i:end]

			sql := fmt.Sprintf("INSERT IGNORE INTO %s (feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, enc_data_hash) VALUES ", tableName)
			var args []interface{}
			for j, entry := range batch {
				hash := encDataHash(entry.EncData)
				if j > 0 {
					sql += ","
				}
				sql += "(?, ?, ?, ?, ?, ?, ?, ?)"
				args = append(args, entry.FeatureID, entry.LogTime, entry.IDC, entry.EncData, entry.EncKey, entry.RawJSON, entry.ParsedJSON, hash)
			}
			if err := tx.Exec(sql, args...).Error; err != nil {
				return nil, 0, err
			}
			savedCount += len(batch)

			for _, e := range batch {
				if e.ParsedJSON != "" {
					var parsed map[string]interface{}
					if json.Unmarshal([]byte(e.ParsedJSON), &parsed) == nil {
						if lu, ok := parsed["login_user"].(map[string]interface{}); ok {
							if openid, ok := lu["openid"].(string); ok && openid != "" {
								savedMobiles[openid] = true
							}
						}
					}
				}
			}
		}
	}
	return savedMobiles, savedCount, nil
}

func (r *LogRepository) GetExistingLogTables(featureIDs []int, months []time.Time) map[string]bool {
	existing := make(map[string]bool)
	if len(featureIDs) == 0 || len(months) == 0 {
		return existing
	}

	var tableNames []string
	for _, fid := range featureIDs {
		for _, m := range months {
			tableNames = append(tableNames, r.GetTableName(fid, m))
		}
	}

	placeholders := make([]string, len(tableNames))
	args := make([]interface{}, len(tableNames))
	for i, name := range tableNames {
		placeholders[i] = "?"
		args[i] = name
	}

	sql := fmt.Sprintf(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN (%s)",
		strings.Join(placeholders, ","),
	)

	var found []string
	r.DB.Raw(sql, args...).Scan(&found)
	for _, name := range found {
		existing[name] = true
	}
	return existing
}
