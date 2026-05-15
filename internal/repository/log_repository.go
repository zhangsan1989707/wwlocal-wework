package repository

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type LogRepository struct {
	DB           *gorm.DB
	tableCreated map[string]bool // 缓存已确认存在的表
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{DB: db, tableCreated: make(map[string]bool)}
}

func (r *LogRepository) GetTableName(featureID int, t time.Time) string {
	return fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))
}

func (r *LogRepository) TableExists(tableName string) bool {
	var result int
	sql := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'", tableName)
	r.DB.Raw(sql).Scan(&result)
	return result > 0
}

func encDataHash(encData string) string {
	h := md5.Sum([]byte(encData))
	return hex.EncodeToString(h[:])
}

func (r *LogRepository) CreateTableIfNotExists(featureID int, month time.Time) error {
	tableName := r.GetTableName(featureID, month)
	if r.tableCreated[tableName] {
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
	// 对已有表补列和索引（CREATE TABLE IF NOT EXISTS 不会修改已有表结构）
	r.MigrateLogTable(tableName)
	r.tableCreated[tableName] = true
	return nil
}

// MigrateLogTable 为已存在的表添加缺失的列和索引
func (r *LogRepository) MigrateLogTable(tableName string) {
	// MySQL 不支持 ADD COLUMN IF NOT EXISTS，直接忽略已存在的错误
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN enc_data_hash CHAR(32) AFTER parsed_json", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE INDEX uk_dedup (feature_id, log_time, enc_data_hash)", tableName))
	
	// JSON 路径虚拟列 + 索引，加速常见查询
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN login_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.login_user.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_login_openid (login_openid)", tableName))
	
	// 预建其他高频路径索引
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN sender_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.sender.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_sender_openid (sender_openid)", tableName))
	
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN root_openid VARCHAR(64) AS (JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.openid'))) VIRTUAL", tableName))
	r.DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD INDEX idx_root_openid (root_openid)", tableName))
}

// CreateUserDailyStatsTable 创建用户日活跃汇总表
func (r *LogRepository) CreateUserDailyStatsTable() error {
	return r.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_daily_stats (
			mobile VARCHAR(32) NOT NULL,
			feature_id INT NOT NULL,
			stat_date DATE NOT NULL,
			PRIMARY KEY (mobile, feature_id, stat_date)
		) ENGINE=InnoDB
	`).Error
}

// BatchUpsertDailyStats 将 distinct 手机号写入日活跃汇总表，每批 2000 条
func (r *LogRepository) BatchUpsertDailyStats(featureID int, mobiles map[string]bool, logTime int64) {
	if len(mobiles) == 0 {
		return
	}
	statDate := time.Unix(logTime, 0).Format("2006-01-02")

	const batchSize = 2000
	mobileList := make([]string, 0, len(mobiles))
	for m := range mobiles {
		mobileList = append(mobileList, m)
	}

	for i := 0; i < len(mobileList); i += batchSize {
		end := i + batchSize
		if end > len(mobileList) {
			end = len(mobileList)
		}
		batch := mobileList[i:end]

		sql := "INSERT IGNORE INTO user_daily_stats (mobile, feature_id, stat_date) VALUES "
		var args []interface{}
		for j, mobile := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?)"
			args = append(args, mobile, featureID, statDate)
		}
		if err := r.DB.Exec(sql, args...).Error; err != nil {
			log.Printf("upsert daily stats failed (batch %d-%d): %v", i, end, err)
		}
	}
}

func (r *LogRepository) BatchUpsertDailyStatsWithTx(tx *gorm.DB, featureID int, mobiles map[string]bool, logTime int64) error {
	if len(mobiles) == 0 {
		return nil
	}
	statDate := time.Unix(logTime, 0).Format("2006-01-02")

	const batchSize = 2000
	mobileList := make([]string, 0, len(mobiles))
	for m := range mobiles {
		mobileList = append(mobileList, m)
	}

	for i := 0; i < len(mobileList); i += batchSize {
		end := i + batchSize
		if end > len(mobileList) {
			end = len(mobileList)
		}
		batch := mobileList[i:end]

		sql := "INSERT IGNORE INTO user_daily_stats (mobile, feature_id, stat_date) VALUES "
		var args []interface{}
		for j, mobile := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?)"
			args = append(args, mobile, featureID, statDate)
		}
		if err := tx.Exec(sql, args...).Error; err != nil {
			return err
		}
	}
	return nil
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

	// 按月分组
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
		// 每批 500 条，单表事务
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

	// 按月分组
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

			// 提取已保存条目中的手机号
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

// QueryAcrossMonths 查询指定 feature 在 startTime~endTime 跨越的所有月份表，SQL 层分页
func (r *LogRepository) QueryAcrossMonths(featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error) {
	months := r.monthsBetween(startTime, endTime)

	// Phase 1: 各表 COUNT + 收集 log_time（仅 int64，内存极小）
	var total int64
	type timeRef struct{ logTime int64 }
	var allTimes []timeRef

	for _, month := range months {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}

		var count int64
		r.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE log_time >= ? AND log_time <= ?", tableName), startTime, endTime).Scan(&count)
		total += count

		var times []int64
		r.DB.Raw(fmt.Sprintf("SELECT log_time FROM %s WHERE log_time >= ? AND log_time <= ? ORDER BY log_time DESC", tableName), startTime, endTime).Scan(&times)
		for _, t := range times {
			allTimes = append(allTimes, timeRef{logTime: t})
		}
	}

	if total == 0 {
		return []model.LogEntry{}, 0, nil
	}

	// 排序并确定分页时间范围
	sort.Slice(allTimes, func(i, j int) bool { return allTimes[i].logTime > allTimes[j].logTime })
	start := (page - 1) * pageSize
	if start >= len(allTimes) {
		return []model.LogEntry{}, total, nil
	}
	end := start + pageSize
	if end > len(allTimes) {
		end = len(allTimes)
	}
	pageMaxTime := allTimes[start].logTime
	pageMinTime := allTimes[end-1].logTime

	// Phase 2: 只拉取分页范围内的时间段数据
	var allEntries []model.LogEntry
	for _, month := range months {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		querySQL := fmt.Sprintf(`
			SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
			FROM %s WHERE log_time >= ? AND log_time <= ?
			ORDER BY log_time DESC
		`, tableName)
		var entries []model.LogEntry
		r.DB.Raw(querySQL, pageMinTime, pageMaxTime).Scan(&entries)
		allEntries = append(allEntries, entries...)
	}

	// 最终排序并截取精确分页
	sort.Slice(allEntries, func(i, j int) bool { return allEntries[i].LogTime > allEntries[j].LogTime })
	if len(allTimes) > 0 {
		// 重新按全局索引截取
		globalIdx := 0
		var result []model.LogEntry
		for _, entry := range allEntries {
			if globalIdx >= start && globalIdx < end {
				result = append(result, entry)
			}
			globalIdx++
		}
		if len(result) > pageSize {
			result = result[:pageSize]
		}
		return result, total, nil
	}
	return allEntries, total, nil
}

// QueryAcrossMonthsWithConditions 在 SQL 层通过 JSON path 过滤 parsed_json 字段，SQL 层分页
func (r *LogRepository) QueryAcrossMonthsWithConditions(featureID int, startTime, endTime int64, conditions map[string]interface{}, mobile string, page, pageSize int) ([]model.LogEntry, int64, error) {
	months := r.monthsBetween(startTime, endTime)
	jsonWhere, jsonArgs := r.buildJSONConditions(conditions, mobile)

	// Phase 1: COUNT + 收集 log_time
	var total int64
	type timeRef struct{ logTime int64 }
	var allTimes []timeRef

	for _, month := range months {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, endTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}

		var count int64
		r.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, where), args...).Scan(&count)
		total += count

		var times []int64
		r.DB.Raw(fmt.Sprintf("SELECT log_time FROM %s WHERE %s ORDER BY log_time DESC", tableName, where), args...).Scan(&times)
		for _, t := range times {
			allTimes = append(allTimes, timeRef{logTime: t})
		}
	}

	if total == 0 {
		return []model.LogEntry{}, 0, nil
	}

	sort.Slice(allTimes, func(i, j int) bool { return allTimes[i].logTime > allTimes[j].logTime })
	start := (page - 1) * pageSize
	if start >= len(allTimes) {
		return []model.LogEntry{}, total, nil
	}
	end := start + pageSize
	if end > len(allTimes) {
		end = len(allTimes)
	}
	pageMaxTime := allTimes[start].logTime
	pageMinTime := allTimes[end-1].logTime

	// Phase 2: 拉取分页数据
	var allEntries []model.LogEntry
	for _, month := range months {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{pageMinTime, pageMaxTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}
		querySQL := fmt.Sprintf(`
			SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
			FROM %s WHERE %s ORDER BY log_time DESC
		`, tableName, where)
		var entries []model.LogEntry
		r.DB.Raw(querySQL, args...).Scan(&entries)
		allEntries = append(allEntries, entries...)
	}

	sort.Slice(allEntries, func(i, j int) bool { return allEntries[i].LogTime > allEntries[j].LogTime })
	globalIdx := 0
	var result []model.LogEntry
	for _, entry := range allEntries {
		if globalIdx >= start && globalIdx < end {
			result = append(result, entry)
		}
		globalIdx++
	}
	if len(result) > pageSize {
		result = result[:pageSize]
	}
	return result, total, nil
}

// validJSONPathKey 校验 JSON path key 只含字母、数字、下划线、点
func validJSONPathKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}

// buildJSONConditions 将前端 conditions 转为 MySQL JSON path 查询条件
func (r *LogRepository) buildJSONConditions(conditions map[string]interface{}, mobile string) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for key, expected := range conditions {
		if !validJSONPathKey(key) {
			continue
		}
		// 优先使用预建的虚拟列
		if key == "login_user.openid" {
			matchValue := expected
			operator := "LIKE"
			if condMap, isMap := expected.(map[string]interface{}); isMap {
				if v, exists := condMap["value"]; exists {
					matchValue = v
				}
				if op, exists := condMap["operator"]; exists {
					if opStr, ok := op.(string); ok && opStr == "=" {
						operator = "="
					}
				}
			}
			if operator == "LIKE" {
				parts = append(parts, "LOWER(login_openid) LIKE ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, "%"+strings.ToLower(s)+"%")
				} else {
					args = append(args, fmt.Sprintf("%%%v%%", strings.ToLower(fmt.Sprintf("%v", matchValue))))
				}
			} else {
				parts = append(parts, "LOWER(login_openid) = ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, strings.ToLower(s))
				} else {
					args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
				}
			}
			continue
		}
		if key == "sender.openid" {
			matchValue := expected
			operator := "LIKE"
			if condMap, isMap := expected.(map[string]interface{}); isMap {
				if v, exists := condMap["value"]; exists {
					matchValue = v
				}
				if op, exists := condMap["operator"]; exists {
					if opStr, ok := op.(string); ok && opStr == "=" {
						operator = "="
					}
				}
			}
			if operator == "LIKE" {
				parts = append(parts, "LOWER(sender_openid) LIKE ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, "%"+strings.ToLower(s)+"%")
				} else {
					args = append(args, fmt.Sprintf("%%%v%%", strings.ToLower(fmt.Sprintf("%v", matchValue))))
				}
			} else {
				parts = append(parts, "LOWER(sender_openid) = ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, strings.ToLower(s))
				} else {
					args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
				}
			}
			continue
		}
		if key == "openid" {
			matchValue := expected
			operator := "LIKE"
			if condMap, isMap := expected.(map[string]interface{}); isMap {
				if v, exists := condMap["value"]; exists {
					matchValue = v
				}
				if op, exists := condMap["operator"]; exists {
					if opStr, ok := op.(string); ok && opStr == "=" {
						operator = "="
					}
				}
			}
			if operator == "LIKE" {
				parts = append(parts, "LOWER(root_openid) LIKE ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, "%"+strings.ToLower(s)+"%")
				} else {
					args = append(args, fmt.Sprintf("%%%v%%", strings.ToLower(fmt.Sprintf("%v", matchValue))))
				}
			} else {
				parts = append(parts, "LOWER(root_openid) = ?")
				if s, ok := matchValue.(string); ok {
					args = append(args, strings.ToLower(s))
				} else {
					args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
				}
			}
			continue
		}

		// 其他路径继续使用 JSON 提取
		jsonPath := "$." + key
		jsonExtract := fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '%s'))", jsonPath)

		matchValue := expected
		operator := "LIKE"
		if condMap, isMap := expected.(map[string]interface{}); isMap {
			if v, exists := condMap["value"]; exists {
				matchValue = v
			}
			if op, exists := condMap["operator"]; exists {
				if opStr, ok := op.(string); ok && opStr == "=" {
					operator = "="
				}
			}
		}

		switch v := matchValue.(type) {
		case string:
			if operator == "LIKE" {
				parts = append(parts, fmt.Sprintf("LOWER(%s) LIKE ?", jsonExtract))
				args = append(args, "%"+strings.ToLower(v)+"%")
			} else {
				parts = append(parts, fmt.Sprintf("LOWER(%s) = ?", jsonExtract))
				args = append(args, strings.ToLower(v))
			}
		case float64:
			// JSON 中的数字
			parts = append(parts, fmt.Sprintf("%s = ?", jsonExtract))
			args = append(args, v)
		default:
			parts = append(parts, fmt.Sprintf("%s = ?", jsonExtract))
			args = append(args, fmt.Sprintf("%v", v))
		}
	}

	if mobile != "" {
		// 使用预建的虚拟列
		parts = append(parts, "(login_openid = ? OR root_openid = ? OR sender_openid = ?)")
		args = append(args, mobile, mobile, mobile)
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " AND "), args
}

// SampleParsedJSON 从每个 feature 的最近月份表中采样 parsed_json 用于提取字段路径
func (r *LogRepository) SampleParsedJSON(featureIDs []int, limit int) []string {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var samples []string
	for _, featureID := range featureIDs {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		sql := fmt.Sprintf("SELECT parsed_json FROM %s WHERE parsed_json IS NOT NULL AND parsed_json != '' LIMIT %d", tableName, limit)
		var results []struct {
			ParsedJSON string
		}
		if err := r.DB.Raw(sql).Scan(&results).Error; err != nil {
			continue
		}
		for _, r := range results {
			samples = append(samples, r.ParsedJSON)
		}
	}
	return samples
}

// monthsBetween 返回 startTime~endTime 跨越的所有月份（月初时间点）
func (r *LogRepository) monthsBetween(startTime, endTime int64) []time.Time {
	start := time.Unix(startTime, 0)
	end := time.Unix(endTime, 0)

	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	endMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())

	var months []time.Time
	for !cur.After(endMonth) {
		months = append(months, cur)
		cur = cur.AddDate(0, 1, 0)
	}
	return months
}

// InactiveUser 未使用人员
type InactiveUser struct {
	Name         string `json:"name"`
	Mobile       string `json:"mobile"`
	Position     string `json:"position"`
	Department   string `json:"department"`
	UserID       string `json:"user_id"`
	ActiveDays   int    `json:"active_days"`
	InactiveDays int    `json:"inactive_days"`
}

// GetExistingLogTables 查询 INFORMATION_SCHEMA 返回存在的日志表名
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

// GetInactiveUsers 从预聚合表查询完全没有活跃记录的联系人
func (r *LogRepository) GetInactiveUsers(featureIDs []int, startTime int64, deptID int) ([]InactiveUser, error) {
	deptFilter := ""
	if deptID > 0 {
		deptFilter = "AND EXISTS (SELECT 1 FROM contact_departments cd WHERE cd.user_id = c.user_id AND cd.department = ?)"
	}

	timeFilter := ""
	var timeArgs []interface{}
	if startTime > 0 {
		startDate := time.Unix(startTime, 0).Format("2006-01-02")
		timeFilter = "AND uds.stat_date >= ?"
		timeArgs = append(timeArgs, startDate)
	}

	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	sql := fmt.Sprintf(`
		SELECT c.name, c.mobile, c.position, c.department, c.user_id,
		       0 AS active_days, 0 AS inactive_days
		FROM contacts c
		LEFT JOIN (
			SELECT mobile
			FROM user_daily_stats
			WHERE feature_id IN (%s) %s
			GROUP BY mobile
		) uds ON uds.mobile = c.mobile
		WHERE uds.mobile IS NULL AND c.status = 1 %s
	`, strings.Join(fidPlaceholders, ","), timeFilter, deptFilter)

	var args []interface{}
	args = append(args, fidArgs...)
	args = append(args, timeArgs...)
	if deptID > 0 {
		args = append(args, deptID)
	}

	var users []InactiveUser
	if err := r.DB.Raw(sql, args...).Scan(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// GetUsersWithDayStats 从预聚合表查询联系人的活跃/未使用天数
func (r *LogRepository) GetUsersWithDayStats(featureIDs []int, startTime int64, deptID int, totalDays int, minInactiveDays int) ([]InactiveUser, error) {
	deptFilter := ""
	if deptID > 0 {
		deptFilter = "AND EXISTS (SELECT 1 FROM contact_departments cd WHERE cd.user_id = c.user_id AND cd.department = ?)"
	}

	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	timeFilter := ""
	var timeArgs []interface{}
	if startTime > 0 {
		startDate := time.Unix(startTime, 0).Format("2006-01-02")
		timeFilter = "AND stat_date >= ?"
		timeArgs = append(timeArgs, startDate)
	}

	sql := fmt.Sprintf(`
		SELECT c.name, c.mobile, c.position, c.department, c.user_id,
		       COALESCE(stats.active_days, 0) AS active_days,
		       %d - COALESCE(stats.active_days, 0) AS inactive_days
		FROM contacts c
		LEFT JOIN (
			SELECT mobile, COUNT(DISTINCT stat_date) AS active_days
			FROM user_daily_stats
			WHERE feature_id IN (%s) %s
			GROUP BY mobile
		) stats ON stats.mobile = c.mobile
		WHERE 1=1 %s
	`, totalDays, strings.Join(fidPlaceholders, ","), timeFilter, deptFilter)

	var args []interface{}
	args = append(args, fidArgs...)
	args = append(args, timeArgs...)
	if deptID > 0 {
		args = append(args, deptID)
	}

	if minInactiveDays > 0 {
		sql += fmt.Sprintf(" HAVING inactive_days >= %d", minInactiveDays)
	}
	sql += " ORDER BY inactive_days DESC, c.name ASC"

	var users []InactiveUser
	if err := r.DB.Raw(sql, args...).Scan(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

type TableSizeInfo struct {
	TableName  string `gorm:"column:TABLE_NAME"`
	RowCount   int64  `gorm:"column:TABLE_ROWS"`
	DataSize   string `gorm:"column:DATA_LENGTH"`
	IndexSize  string `gorm:"column:INDEX_LENGTH"`
}

func (r *LogRepository) GetTableSizes(limit int) ([]TableSizeInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	var tables []TableSizeInfo
	err := r.DB.Raw(`
		SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME LIKE 'log_%'
		ORDER BY TABLE_ROWS DESC
		LIMIT ?
	`, limit).Scan(&tables).Error
	return tables, err
}

// GetActualMaxLogTime 查询指定 feature 在数据库中实际存储的最大 log_time
func (r *LogRepository) GetActualMaxLogTime(featureID int) int64 {
	now := time.Now()
	var maxLogTime int64
	for i := 0; i < 12; i++ {
		month := now.AddDate(0, -i, 0)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		var t int64
		sql := fmt.Sprintf("SELECT COALESCE(MAX(log_time), 0) FROM %s", tableName)
		if err := r.DB.Raw(sql).Scan(&t).Error; err != nil {
			continue
		}
		if t > maxLogTime {
			maxLogTime = t
		}
	}
	return maxLogTime
}

// CleanupOldData 清理指定月份之前的旧数据
func (r *LogRepository) CleanupOldData(featureID int, beforeMonth time.Time) (int64, error) {
	var totalDeleted int64
	// 生成需要检查的月份列表
	var monthsToCheck []time.Time
	// 从 beforeMonth 往前推 5 年，检查所有可能存在的表
	for i := 0; i < 60; i++ {
		month := beforeMonth.AddDate(0, -i, 0)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		monthsToCheck = append(monthsToCheck, month)
	}

	for _, month := range monthsToCheck {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		// 删除该表中符合条件的数据
		sql := fmt.Sprintf("DELETE FROM %s WHERE log_time < ?", tableName)
		result := r.DB.Exec(sql, time.Date(beforeMonth.Year(), beforeMonth.Month(), 1, 0, 0, 0, 0, beforeMonth.Location()).Unix())
		if result.Error != nil {
			log.Printf("cleanup table %s failed: %v", tableName, result.Error)
			continue
		}
		totalDeleted += result.RowsAffected
		log.Printf("cleanup table %s, deleted %d rows", tableName, result.RowsAffected)
	}
	return totalDeleted, nil
}

// QueryByCursor 使用游标分页查询，减少内存消耗
func (r *LogRepository) QueryByCursor(featureID int, startTime, endTime int64, cursor int64, pageSize int, conditions map[string]interface{}, mobile string) ([]model.LogEntry, int64, int64, error) {
	months := r.monthsBetween(startTime, endTime)
	jsonWhere, jsonArgs := r.buildJSONConditions(conditions, mobile)
	
	// 计算总记录数
	var total int64
	for _, month := range months {
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, endTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}
		var count int64
		if err := r.DB.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, where), args...).Scan(&count).Error; err != nil {
			log.Printf("count table %s failed: %v", tableName, err)
		}
		total += count
	}
	
	// 如果有 cursor，只查询 cursor 之后的记录
	queryEndTime := endTime
	if cursor > 0 {
		queryEndTime = cursor
	}
	
	// 按时间从新到旧查询，获取下一页
	var allEntries []model.LogEntry
	remaining := pageSize
	// 从最新月份开始查询
	for i := len(months) - 1; i >= 0 && remaining > 0; i-- {
		month := months[i]
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, queryEndTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}
		// 如果有 cursor，排除等于 cursor 的记录，避免重复
		if cursor > 0 {
			where += " AND log_time < ?"
			args = append(args, cursor)
		}
		
		querySQL := fmt.Sprintf(`
			SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
			FROM %s WHERE %s ORDER BY log_time DESC LIMIT ?
		`, tableName, where)
		
		var entries []model.LogEntry
		queryArgs := append(args, remaining)
		if err := r.DB.Raw(querySQL, queryArgs...).Scan(&entries).Error; err != nil {
			log.Printf("query table %s failed: %v", tableName, err)
			continue
		}
		allEntries = append(allEntries, entries...)
		remaining -= len(entries)
	}
	
	// 获取下一页的 cursor
	var nextCursor int64 = 0
	if len(allEntries) >= pageSize {
		nextCursor = allEntries[len(allEntries)-1].LogTime
		// 截断到 pageSize
		if len(allEntries) > pageSize {
			allEntries = allEntries[:pageSize]
		}
	}
	
	return allEntries, total, nextCursor, nil
}

// DropOldTables 删除指定月份之前的整表
func (r *LogRepository) DropOldTables(featureID int, beforeMonth time.Time) ([]string, error) {
	var droppedTables []string
	// 获取所有 log_ 开头的表
	var allTables []string
	if err := r.DB.Raw("SELECT TABLE_NAME FROM information_schema.tables WHERE table_schema = DATABASE() AND TABLE_NAME LIKE 'log_%'").Scan(&allTables).Error; err != nil {
		return nil, err
	}

	for _, tableName := range allTables {
		// 解析表名中的 featureID 和月份
		parts := strings.Split(tableName, "_")
		if len(parts) < 3 {
			continue
		}
		if parts[0] != "log" {
			continue
		}
		// 解析 featureID
		fid, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		// 如果指定了 featureID 且不匹配，跳过
		if featureID > 0 && fid != featureID {
			continue
		}
		// 解析月份
		monthStr := parts[2]
		if len(monthStr) < 6 {
			continue
		}
		year, err := strconv.Atoi(monthStr[:4])
		if err != nil {
			continue
		}
		monthNum, err := strconv.Atoi(monthStr[4:6])
		if err != nil {
			continue
		}
		tableMonth := time.Date(year, time.Month(monthNum), 1, 0, 0, 0, 0, beforeMonth.Location())
		// 如果表的月份早于 beforeMonth，删除表
		if tableMonth.Before(beforeMonth) || tableMonth.Equal(beforeMonth) {
			if err := r.DB.Exec(fmt.Sprintf("DROP TABLE %s", tableName)).Error; err != nil {
				log.Printf("drop table %s failed: %v", tableName, err)
				continue
			}
			droppedTables = append(droppedTables, tableName)
			log.Printf("dropped table %s", tableName)
		}
	}
	return droppedTables, nil
}
