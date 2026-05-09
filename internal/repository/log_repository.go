package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type LogRepository struct {
	db *gorm.DB
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db}
}

func (r *LogRepository) GetTableName(featureID int, t time.Time) string {
	return fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))
}

func (r *LogRepository) CreateTableIfNotExists(featureID int, month time.Time) error {
	tableName := r.GetTableName(featureID, month)
	return r.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			feature_id INT NOT NULL,
			log_time BIGINT NOT NULL,
			idc VARCHAR(32),
			enc_data TEXT,
			enc_key TEXT,
			raw_json TEXT,
			parsed_json JSON,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_log_time (log_time),
			INDEX idx_feature_logtime (feature_id, log_time)
		)
	`, tableName)).Error
}

func (r *LogRepository) Save(featureID int, entry *model.LogEntry) error {
	month := time.Unix(entry.LogTime, 0)
	if err := r.CreateTableIfNotExists(featureID, month); err != nil {
		return err
	}

	tableName := r.GetTableName(featureID, month)
	return r.db.Exec(fmt.Sprintf(`
		INSERT INTO %s (feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, tableName), entry.FeatureID, entry.LogTime, entry.IDC, entry.EncData, entry.EncKey, entry.RawJSON, entry.ParsedJSON).Error
}

func (r *LogRepository) BatchSave(featureID int, entries []model.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	month := time.Unix(entries[0].LogTime, 0)
	if err := r.CreateTableIfNotExists(featureID, month); err != nil {
		return err
	}

	tableName := r.GetTableName(featureID, month)

	tx := r.db.Exec("START TRANSACTION")
	for _, entry := range entries {
		err := tx.Exec(fmt.Sprintf(`
			INSERT INTO %s (feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, tableName), entry.FeatureID, entry.LogTime, entry.IDC, entry.EncData, entry.EncKey, entry.RawJSON, entry.ParsedJSON).Error
		if err != nil {
			tx.Exec("ROLLBACK")
			return err
		}
	}
	return tx.Exec("COMMIT").Error
}

func (r *LogRepository) Query(featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error) {
	tableName := r.GetTableName(featureID, time.Unix(startTime, 0))

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE log_time >= ? AND log_time <= ?", tableName)
	if err := r.db.Raw(countSQL, startTime, endTime).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf(`
		SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
		FROM %s WHERE log_time >= ? AND log_time <= ?
		ORDER BY log_time DESC
		LIMIT ? OFFSET ?
	`, tableName)

	var entries []model.LogEntry
	if err := r.db.Raw(querySQL, startTime, endTime, pageSize, offset).Scan(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}