package repository

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type InactiveUser struct {
	Name         string `json:"name"`
	Mobile       string `json:"mobile"`
	Position     string `json:"position"`
	Department   string `json:"department"`
	UserID       string `json:"user_id"`
	ActiveDays   int    `json:"active_days"`
	InactiveDays int    `json:"inactive_days"`
}

type TableSizeInfo struct {
	TableName string `gorm:"column:TABLE_NAME"`
	RowCount  int64  `gorm:"column:TABLE_ROWS"`
	DataSize  string `gorm:"column:DATA_LENGTH"`
	IndexSize string `gorm:"column:INDEX_LENGTH"`
}

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
		for _, row := range results {
			samples = append(samples, row.ParsedJSON)
		}
	}
	return samples
}

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

func (r *LogRepository) CleanupOldData(featureID int, beforeMonth time.Time) (int64, error) {
	var totalDeleted int64
	var monthsToCheck []time.Time
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
		sql := fmt.Sprintf("DELETE FROM %s WHERE log_time < ?", tableName)
		result := r.DB.Exec(sql, time.Date(beforeMonth.Year(), beforeMonth.Month(), 1, 0, 0, 0, 0, beforeMonth.Location()).Unix())
		if result.Error != nil {
			slog.Info(fmt.Sprintf("cleanup table %s failed: %v", tableName, result.Error))
			continue
		}
		totalDeleted += result.RowsAffected
		slog.Info(fmt.Sprintf("cleanup table %s, deleted %d rows", tableName, result.RowsAffected))
	}
	return totalDeleted, nil
}

func (r *LogRepository) DropOldTables(featureID int, beforeMonth time.Time) ([]string, error) {
	var droppedTables []string
	var allTables []string
	if err := r.DB.Raw("SELECT TABLE_NAME FROM information_schema.tables WHERE table_schema = DATABASE() AND TABLE_NAME LIKE 'log_%'").Scan(&allTables).Error; err != nil {
		return nil, err
	}

	for _, tableName := range allTables {
		parts := strings.Split(tableName, "_")
		if len(parts) < 3 {
			continue
		}
		if parts[0] != "log" {
			continue
		}
		fid, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		if featureID > 0 && fid != featureID {
			continue
		}
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
		if tableMonth.Before(beforeMonth) || tableMonth.Equal(beforeMonth) {
			if err := r.DB.Exec(fmt.Sprintf("DROP TABLE %s", tableName)).Error; err != nil {
				slog.Info(fmt.Sprintf("drop table %s failed: %v", tableName, err))
				continue
			}
			droppedTables = append(droppedTables, tableName)
			slog.Info(fmt.Sprintf("dropped table %s", tableName))
		}
	}
	return droppedTables, nil
}

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
