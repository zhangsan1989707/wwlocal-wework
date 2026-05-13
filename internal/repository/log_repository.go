package repository

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type LogRepository struct {
	db           *gorm.DB
	tableCreated map[string]bool // 缓存已确认存在的表
}

func NewLogRepository(db *gorm.DB) *LogRepository {
	return &LogRepository{db: db, tableCreated: make(map[string]bool)}
}

func (r *LogRepository) GetTableName(featureID int, t time.Time) string {
	return fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))
}

func (r *LogRepository) TableExists(tableName string) bool {
	var result int
	sql := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'", tableName)
	r.db.Raw(sql).Scan(&result)
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
	err := r.db.Exec(fmt.Sprintf(`
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

// MigrateLogTable 为已存在的表添加 enc_data_hash 列和去重索引
func (r *LogRepository) MigrateLogTable(tableName string) {
	// MySQL 不支持 ADD COLUMN IF NOT EXISTS，直接忽略已存在的错误
	r.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN enc_data_hash CHAR(32) AFTER parsed_json", tableName))
	r.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE INDEX uk_dedup (feature_id, log_time, enc_data_hash)", tableName))
}

func (r *LogRepository) Save(featureID int, entry *model.LogEntry) error {
	month := time.Unix(entry.LogTime, 0)
	if err := r.CreateTableIfNotExists(featureID, month); err != nil {
		return err
	}

	tableName := r.GetTableName(featureID, month)
	hash := encDataHash(entry.EncData)
	return r.db.Exec(fmt.Sprintf(`
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
		tx := r.db.Begin()
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
		r.db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE log_time >= ? AND log_time <= ?", tableName), startTime, endTime).Scan(&count)
		total += count

		var times []int64
		r.db.Raw(fmt.Sprintf("SELECT log_time FROM %s WHERE log_time >= ? AND log_time <= ? ORDER BY log_time DESC", tableName), startTime, endTime).Scan(&times)
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
		r.db.Raw(querySQL, pageMinTime, pageMaxTime).Scan(&entries)
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
		r.db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, where), args...).Scan(&count)
		total += count

		var times []int64
		r.db.Raw(fmt.Sprintf("SELECT log_time FROM %s WHERE %s ORDER BY log_time DESC", tableName, where), args...).Scan(&times)
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
		r.db.Raw(querySQL, args...).Scan(&entries)
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
		// "sender.openid" -> "$.sender.openid"
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
		parts = append(parts, "JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.openid')) = ?")
		args = append(args, mobile)
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
		if err := r.db.Raw(sql).Scan(&results).Error; err != nil {
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
	r.db.Raw(sql, args...).Scan(&found)
	for _, name := range found {
		existing[name] = true
	}
	return existing
}

// GetInactiveUsers 查询近 N 个月没有任何 login/wake 日志的联系人
// startTime > 0 时，对当前月表额外加 log_time 过滤（周维度）
// deptID > 0 时，按部门过滤
func (r *LogRepository) GetInactiveUsers(featureIDs []int, months []time.Time, existingTables map[string]bool, startTime int64, deptID int) ([]InactiveUser, error) {
	currentMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())

	var notExistsClauses []string
	for _, fid := range featureIDs {
		for _, m := range months {
			tableName := r.GetTableName(fid, m)
			if !existingTables[tableName] {
				continue
			}
			cond := fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(parsed_json,'$.login_user.openid')) = c.mobile")
			if startTime > 0 && m.Equal(currentMonth) {
				cond += fmt.Sprintf(" AND log_time >= %d", startTime)
			}
			notExistsClauses = append(notExistsClauses,
				fmt.Sprintf("NOT EXISTS (SELECT 1 FROM %s WHERE %s)", tableName, cond),
			)
		}
	}

	where := "c.status = 1"
	for _, clause := range notExistsClauses {
		where += "\nAND " + clause
	}
	if deptID > 0 {
		where += fmt.Sprintf("\nAND JSON_CONTAINS(c.department, '%d')", deptID)
	}

	sql := fmt.Sprintf(`
		SELECT c.name, c.mobile, c.position, c.department, c.user_id
		FROM contacts c
		WHERE %s
	`, where)

	var users []InactiveUser
	if err := r.db.Raw(sql).Scan(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// GetUsersWithDayStats 查询联系人的活跃/未使用天数，支持阈值过滤
// totalDays: 统计周期总天数
// startTime > 0 时，对当前月表额外加 log_time 过滤
// minInactiveDays: 最少未使用天数（0 = 不过滤）
func (r *LogRepository) GetUsersWithDayStats(featureIDs []int, months []time.Time, existingTables map[string]bool, startTime int64, deptID int, totalDays int, minInactiveDays int) ([]InactiveUser, error) {
	currentMonth := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())

	// 构建 UNION ALL 子查询：每个活跃表中提取 openid + 日期
	var unionParts []string
	for _, fid := range featureIDs {
		for _, m := range months {
			tableName := r.GetTableName(fid, m)
			if !existingTables[tableName] {
				continue
			}
			timeCond := ""
			if startTime > 0 && m.Equal(currentMonth) {
				timeCond = fmt.Sprintf(" AND log_time >= %d", startTime)
			}
			unionParts = append(unionParts,
				fmt.Sprintf("SELECT DISTINCT JSON_UNQUOTE(JSON_EXTRACT(parsed_json,'$.login_user.openid')) AS mobile, DATE(FROM_UNIXTIME(log_time)) AS day FROM %s WHERE 1=1%s", tableName, timeCond),
			)
		}
	}

	deptFilter := ""
	if deptID > 0 {
		deptFilter = fmt.Sprintf("AND JSON_CONTAINS(c.department, '%d')", deptID)
	}

	var sql string
	if len(unionParts) == 0 {
		// 没有任何日志表，所有人 active_days = 0
		sql = fmt.Sprintf(`
			SELECT c.name, c.mobile, c.position, c.department, c.user_id,
			       0 AS active_days, %d AS inactive_days
			FROM contacts c
			WHERE 1=1 %s
		`, totalDays, deptFilter)
	} else {
		unionSQL := strings.Join(unionParts, " UNION ALL ")
		sql = fmt.Sprintf(`
			SELECT c.name, c.mobile, c.position, c.department, c.user_id,
			       COALESCE(stats.active_days, 0) AS active_days,
			       %d - COALESCE(stats.active_days, 0) AS inactive_days
			FROM contacts c
			LEFT JOIN (
				SELECT mobile, COUNT(DISTINCT day) AS active_days
				FROM (%s) AS all_days
				GROUP BY mobile
			) stats ON stats.mobile = c.mobile
			WHERE 1=1 %s
		`, totalDays, unionSQL, deptFilter)
	}

	if minInactiveDays > 0 {
		sql += fmt.Sprintf(" HAVING inactive_days >= %d", minInactiveDays)
	}
	sql += " ORDER BY inactive_days DESC, c.name ASC"

	var users []InactiveUser
	if err := r.db.Raw(sql).Scan(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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
		if err := r.db.Raw(sql).Scan(&t).Error; err != nil {
			continue
		}
		if t > maxLogTime {
			maxLogTime = t
		}
	}
	return maxLogTime
}
