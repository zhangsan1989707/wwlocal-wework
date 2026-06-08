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

// GetInactiveUserCount 返回未使用人数（active_days==0），用于 Overview KPI
func (r *LogRepository) GetInactiveUserCount(featureIDs []int, startTime int64) (int, error) {
	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	startDate := time.Unix(startTime, 0).Format("2006-01-02")

	sql := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM contacts c
		LEFT JOIN (
			SELECT mobile
			FROM user_daily_stats
			WHERE feature_id IN (%s) AND stat_date >= ?
			GROUP BY mobile
		) stats ON stats.mobile = c.mobile
		WHERE stats.mobile IS NULL AND c.status = 1
	`, strings.Join(fidPlaceholders, ","))

	args := make([]interface{}, 0, len(fidArgs)+1)
	args = append(args, fidArgs...)
	args = append(args, startDate)

	var count int64
	if err := r.DB.Raw(sql, args...).Scan(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *LogRepository) GetUsersWithDayStats(featureIDs []int, startTime int64, deptID int, totalDays int, minInactiveDays int, limit, offset int) ([]InactiveUser, int64, error) {
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

	baseQuery := fmt.Sprintf(`
		FROM contacts c
		LEFT JOIN (
			SELECT mobile, COUNT(DISTINCT stat_date) AS active_days
			FROM user_daily_stats
			WHERE feature_id IN (%s) %s
			GROUP BY mobile
		) stats ON stats.mobile = c.mobile
		WHERE c.status = 1 %s
	`, strings.Join(fidPlaceholders, ","), timeFilter, deptFilter)

	// 先查 total
	var args []interface{}
	args = append(args, fidArgs...)
	args = append(args, timeArgs...)
	if deptID > 0 {
		args = append(args, deptID)
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (SELECT %d - COALESCE(stats.active_days, 0) AS inactive_days %s HAVING inactive_days >= ?) t", totalDays, baseQuery)
	countArgs := append([]interface{}{}, args...)
	countArgs = append(countArgs, minInactiveDays)

	var total int64
	if err := r.DB.Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	// 再查分页数据
	selectSQL := fmt.Sprintf(`
		SELECT c.name, c.mobile, c.position, c.department, c.user_id,
		       COALESCE(stats.active_days, 0) AS active_days,
		       %d - COALESCE(stats.active_days, 0) AS inactive_days
	`, totalDays) + baseQuery

	dataArgs := append([]interface{}{}, args...)
	dataArgs = append(dataArgs, minInactiveDays)
	selectSQL += " HAVING inactive_days >= ? ORDER BY inactive_days DESC, c.name ASC LIMIT ? OFFSET ?"
	dataArgs = append(dataArgs, limit, offset)

	var users []InactiveUser
	if err := r.DB.Raw(selectSQL, dataArgs...).Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetDeptInactiveStats 按部门统计未使用人数（SQL 侧聚合，替代 Go 遍历）
func (r *LogRepository) GetDeptInactiveStats(featureIDs []int, startTime int64, totalDays int, minInactiveDays int) (map[int]int, error) {
	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	startDate := time.Unix(startTime, 0).Format("2006-01-02")

	sql := fmt.Sprintf(`
		SELECT cd.department AS dept_id, COUNT(*) AS cnt
		FROM contacts c
		INNER JOIN contact_departments cd ON cd.user_id = c.user_id
		LEFT JOIN (
			SELECT mobile, COUNT(DISTINCT stat_date) AS active_days
			FROM user_daily_stats
			WHERE feature_id IN (%s) AND stat_date >= ?
			GROUP BY mobile
		) stats ON stats.mobile = c.mobile
		WHERE c.status = 1 AND (%d - COALESCE(stats.active_days, 0)) >= ?
		GROUP BY cd.department
	`, strings.Join(fidPlaceholders, ","), totalDays)

	args := make([]interface{}, 0, len(fidArgs)+2)
	args = append(args, fidArgs...)
	args = append(args, startDate, minInactiveDays)

	var rows []struct {
		DeptID int
		Cnt    int
	}
	if err := r.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[int]int, len(rows))
	for _, row := range rows {
		result[row.DeptID] = row.Cnt
	}
	return result, nil
}

// TrendPoint 趋势数据点
type TrendPoint struct {
	Period      string `json:"period"`
	ActiveUsers int    `json:"active_users"`
}

// DeptTrendStat 部门趋势统计
type DeptTrendStat struct {
	DeptID        int     `json:"id"`
	DeptName      string  `json:"name"`
	TotalContacts int64   `json:"total"`
	ActiveCount   int64   `json:"active"`
	InactiveCount int64   `json:"inactive"`
	ActiveRate    float64 `json:"active_rate"`
	AvgActiveDays float64 `json:"avg_active_days"`
}

// GetTrendStats 按时间粒度聚合活跃人数
func (r *LogRepository) GetTrendStats(featureIDs []int, startDate, endDate string, granularity string) ([]TrendPoint, error) {
	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	var groupExpr, orderExpr string
	switch granularity {
	case "week":
		groupExpr = "DATE_FORMAT(stat_date, '%x-W%v')"
		orderExpr = "MIN(stat_date)"
	case "month":
		groupExpr = "DATE_FORMAT(stat_date, '%Y-%m')"
		orderExpr = "MIN(stat_date)"
	case "quarter":
		groupExpr = "CONCAT(YEAR(stat_date), '-Q', QUARTER(stat_date))"
		orderExpr = "MIN(stat_date)"
	default: // day
		groupExpr = "stat_date"
		orderExpr = "stat_date"
	}

	sql := fmt.Sprintf(`
		SELECT %s AS period, COUNT(DISTINCT mobile) AS active_users
		FROM user_daily_stats
		WHERE feature_id IN (%s) AND stat_date >= ? AND stat_date <= ?
		GROUP BY %s
		ORDER BY %s
	`, groupExpr, strings.Join(fidPlaceholders, ","), groupExpr, orderExpr)

	args := make([]interface{}, 0, len(fidArgs)+2)
	args = append(args, fidArgs...)
	args = append(args, startDate, endDate)

	var points []TrendPoint
	if err := r.DB.Raw(sql, args...).Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// GetDataCoverage 计算数据覆盖率
func (r *LogRepository) GetDataCoverage(featureIDs []int, startDate, endDate string) (int, int, map[int]int, error) {
	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	// 总覆盖天数
	totalSQL := fmt.Sprintf(`
		SELECT COUNT(DISTINCT stat_date) FROM user_daily_stats
		WHERE feature_id IN (%s) AND stat_date >= ? AND stat_date <= ?
	`, strings.Join(fidPlaceholders, ","))

	totalArgs := make([]interface{}, 0, len(fidArgs)+2)
	totalArgs = append(totalArgs, fidArgs...)
	totalArgs = append(totalArgs, startDate, endDate)

	var coveredDays int64
	if err := r.DB.Raw(totalSQL, totalArgs...).Scan(&coveredDays).Error; err != nil {
		return 0, 0, nil, err
	}

	// 按 feature 分组覆盖天数
	featureSQL := fmt.Sprintf(`
		SELECT feature_id, COUNT(DISTINCT stat_date) AS days
		FROM user_daily_stats
		WHERE feature_id IN (%s) AND stat_date >= ? AND stat_date <= ?
		GROUP BY feature_id
	`, strings.Join(fidPlaceholders, ","))

	var featureRows []struct {
		FeatureID int
		Days      int
	}
	if err := r.DB.Raw(featureSQL, totalArgs...).Scan(&featureRows).Error; err != nil {
		return 0, 0, nil, err
	}

	byFeature := make(map[int]int, len(featureRows))
	for _, row := range featureRows {
		byFeature[row.FeatureID] = row.Days
	}

	// 计算期望天数
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	expectedDays := int(end.Sub(start).Hours()/24) + 1

	return expectedDays, int(coveredDays), byFeature, nil
}

// GetTrendByDept 部门维度活跃统计
func (r *LogRepository) GetTrendByDept(featureIDs []int, startDate, endDate string) ([]DeptTrendStat, error) {
	fidPlaceholders := make([]string, len(featureIDs))
	fidArgs := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		fidPlaceholders[i] = "?"
		fidArgs[i] = fid
	}

	sql := fmt.Sprintf(`
		SELECT
			cd.department AS dept_id,
			COUNT(DISTINCT c.mobile) AS total_contacts,
			COUNT(DISTINCT CASE WHEN uds.mobile IS NOT NULL THEN c.mobile END) AS active_count,
			COALESCE(AVG(CASE WHEN uds.mobile IS NOT NULL THEN uds.active_days END), 0) AS avg_active_days
		FROM contacts c
		INNER JOIN contact_departments cd ON cd.user_id = c.user_id
		LEFT JOIN (
			SELECT mobile, COUNT(DISTINCT stat_date) AS active_days
			FROM user_daily_stats
			WHERE feature_id IN (%s) AND stat_date >= ? AND stat_date <= ?
			GROUP BY mobile
		) uds ON uds.mobile = c.mobile
		WHERE c.status = 1
		GROUP BY cd.department
		ORDER BY active_count DESC
	`, strings.Join(fidPlaceholders, ","))

	args := make([]interface{}, 0, len(fidArgs)+2)
	args = append(args, fidArgs...)
	args = append(args, startDate, endDate)

	var rows []struct {
		DeptID        int
		TotalContacts int64
		ActiveCount   int64
		AvgActiveDays float64
	}
	if err := r.DB.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	// 获取部门名称
	deptNames := make(map[int]string)
	var depts []struct {
		ID   int
		Name string
	}
	r.DB.Raw("SELECT id, name FROM departments").Scan(&depts)
	for _, d := range depts {
		deptNames[d.ID] = d.Name
	}

	result := make([]DeptTrendStat, 0, len(rows))
	for _, row := range rows {
		inactive := row.TotalContacts - row.ActiveCount
		var rate float64
		if row.TotalContacts > 0 {
			rate = float64(row.ActiveCount) / float64(row.TotalContacts)
		}
		result = append(result, DeptTrendStat{
			DeptID:        row.DeptID,
			DeptName:      deptNames[row.DeptID],
			TotalContacts: row.TotalContacts,
			ActiveCount:   row.ActiveCount,
			InactiveCount: inactive,
			ActiveRate:    rate,
			AvgActiveDays: row.AvgActiveDays,
		})
	}
	return result, nil
}
