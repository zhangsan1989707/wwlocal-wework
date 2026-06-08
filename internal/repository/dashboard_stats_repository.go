package repository

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type DashboardStatsRepository struct {
	DB *gorm.DB
}

func NewDashboardStatsRepository(db *gorm.DB) *DashboardStatsRepository {
	return &DashboardStatsRepository{DB: db}
}

func (r *DashboardStatsRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.DashboardDailyStat{}, &model.DashboardDailyUserList{})
}

// UpsertStat 插入或更新指标值
func (r *DashboardStatsRepository) UpsertStat(statDate, metricType, dimensionKey string, value int64) error {
	sql := `INSERT INTO dashboard_daily_stats (stat_date, metric_type, dimension_key, metric_value, created_at)
		VALUES (?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE metric_value = VALUES(metric_value), created_at = NOW()`
	return r.DB.Exec(sql, statDate, metricType, dimensionKey, value).Error
}

// BatchUpsertStats 批量插入或更新指标
func (r *DashboardStatsRepository) BatchUpsertStats(stats []model.DashboardDailyStat) error {
	if len(stats) == 0 {
		return nil
	}
	const batchSize = 500
	for i := 0; i < len(stats); i += batchSize {
		end := i + batchSize
		if end > len(stats) {
			end = len(stats)
		}
		batch := stats[i:end]

		sql := `INSERT INTO dashboard_daily_stats (stat_date, metric_type, dimension_key, metric_value, created_at) VALUES `
		var args []interface{}
		for j, s := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?,?, NOW())"
			args = append(args, s.StatDate, s.MetricType, s.DimensionKey, s.MetricValue)
		}
		sql += ` ON DUPLICATE KEY UPDATE metric_value = VALUES(metric_value), created_at = NOW()`
		if err := r.DB.Exec(sql, args...).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetStats 获取指定日期的指标
func (r *DashboardStatsRepository) GetStats(statDate string) ([]model.DashboardDailyStat, error) {
	var stats []model.DashboardDailyStat
	err := r.DB.Where("stat_date = ?", statDate).Find(&stats).Error
	return stats, err
}

// GetStatsByType 获取指定类型在日期范围内的指标
func (r *DashboardStatsRepository) GetStatsByType(metricType, startDate, endDate string) ([]model.DashboardDailyStat, error) {
	var stats []model.DashboardDailyStat
	err := r.DB.Where("metric_type = ? AND stat_date >= ? AND stat_date <= ?", metricType, startDate, endDate).
		Order("stat_date ASC").Find(&stats).Error
	return stats, err
}

// GetStatsByTypes 获取多个类型在日期范围内的指标
func (r *DashboardStatsRepository) GetStatsByTypes(metricTypes []string, startDate, endDate string) ([]model.DashboardDailyStat, error) {
	var stats []model.DashboardDailyStat
	err := r.DB.Where("metric_type IN ? AND stat_date >= ? AND stat_date <= ?", metricTypes, startDate, endDate).
		Order("stat_date ASC, metric_type ASC").Find(&stats).Error
	return stats, err
}

// GetLatestDate 获取最新的预计算日期
func (r *DashboardStatsRepository) GetLatestDate() (string, error) {
	var date string
	err := r.DB.Raw("SELECT COALESCE(MAX(stat_date), '') FROM dashboard_daily_stats").Scan(&date).Error
	return date, err
}

// GetStatsWithAggregation 按时间粒度聚合指标（日/周/月/季）
func (r *DashboardStatsRepository) GetStatsWithAggregation(metricType, startDate, endDate, granularity string, dimensionKey string) ([]AggregatedStat, error) {
	dimFilter := ""
	var args []interface{}
	if dimensionKey != "" {
		dimFilter = "AND dimension_key = ?"
		args = append(args, dimensionKey)
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
		SELECT %s AS period, SUM(metric_value) AS value
		FROM dashboard_daily_stats
		WHERE metric_type = ? AND stat_date >= ? AND stat_date <= ? %s
		GROUP BY %s
		ORDER BY %s
	`, groupExpr, dimFilter, groupExpr, orderExpr)

	args = append([]interface{}{metricType, startDate, endDate}, args...)
	var results []AggregatedStat
	if err := r.DB.Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

type AggregatedStat struct {
	Period string `json:"period"`
	Value  int64  `json:"value"`
}

// UpsertUserList 插入或更新用户明细
func (r *DashboardStatsRepository) UpsertUserList(users []model.DashboardDailyUserList) error {
	if len(users) == 0 {
		return nil
	}
	const batchSize = 500
	for i := 0; i < len(users); i += batchSize {
		end := i + batchSize
		if end > len(users) {
			end = len(users)
		}
		batch := users[i:end]

		sql := `INSERT INTO dashboard_daily_user_list (stat_date, list_type, mobile, user_id, name, department, extra, created_at) VALUES `
		var args []interface{}
		for j, u := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?,?,?,?,?, NOW())"
			args = append(args, u.StatDate, u.ListType, u.Mobile, u.UserID, u.Name, u.Department, u.Extra)
		}
		sql += ` ON DUPLICATE KEY UPDATE user_id=VALUES(user_id), name=VALUES(name), department=VALUES(department), extra=VALUES(extra), created_at=NOW()`
		if err := r.DB.Exec(sql, args...).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetUserList 获取指定日期和类型的用户明细
func (r *DashboardStatsRepository) GetUserList(statDate, listType string, limit, offset int) ([]model.DashboardDailyUserList, int64, error) {
	query := r.DB.Where("stat_date = ? AND list_type = ?", statDate, listType)

	var total int64
	query.Model(&model.DashboardDailyUserList{}).Count(&total)

	var users []model.DashboardDailyUserList
	err := query.Order("name ASC").Limit(limit).Offset(offset).Find(&users).Error
	return users, total, err
}

// ExportUserList 导出用户明细（不分页）
func (r *DashboardStatsRepository) ExportUserList(statDate, listType string) ([]model.DashboardDailyUserList, error) {
	var users []model.DashboardDailyUserList
	err := r.DB.Where("stat_date = ? AND list_type = ?", statDate, listType).
		Order("name ASC").Find(&users).Error
	return users, err
}

// DeleteByDate 删除指定日期的预计算数据
func (r *DashboardStatsRepository) DeleteByDate(statDate string) error {
	if err := r.DB.Where("stat_date = ?", statDate).Delete(&model.DashboardDailyStat{}).Error; err != nil {
		return fmt.Errorf("delete daily stats: %w", err)
	}
	if err := r.DB.Where("stat_date = ?", statDate).Delete(&model.DashboardDailyUserList{}).Error; err != nil {
		return fmt.Errorf("delete user list: %w", err)
	}
	return nil
}

// GetMetricTypeStats 获取指定日期的所有指标，返回 map[metric_type][dimension_key]value
func (r *DashboardStatsRepository) GetMetricTypeStats(statDate string) (map[string]map[string]int64, error) {
	var stats []model.DashboardDailyStat
	if err := r.DB.Where("stat_date = ?", statDate).Find(&stats).Error; err != nil {
		return nil, err
	}
	result := make(map[string]map[string]int64)
	for _, s := range stats {
		if result[s.MetricType] == nil {
			result[s.MetricType] = make(map[string]int64)
		}
		result[s.MetricType][s.DimensionKey] = s.MetricValue
	}
	return result, nil
}

// CountFromLogTable 从分表中统计指定日期的记录数
func (r *DashboardStatsRepository) CountFromLogTable(featureID int, statDate string) (int64, error) {
	t, err := time.Parse("2006-01-02", statDate)
	if err != nil {
		return 0, err
	}
	tableName := fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))

	var exists int
	r.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&exists)
	if exists == 0 {
		return 0, nil
	}

	var count int64
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE DATE(FROM_UNIXTIME(log_time)) = ?", tableName)
	err = r.DB.Raw(sql, statDate).Scan(&count).Error
	return count, err
}

// CountDistinctFromLogTable 从分表中统计指定日期的去重字段数
func (r *DashboardStatsRepository) CountDistinctFromLogTable(featureID int, statDate string, field string) (int64, error) {
	t, err := time.Parse("2006-01-02", statDate)
	if err != nil {
		return 0, err
	}
	tableName := fmt.Sprintf("log_%d_%s", featureID, t.Format("200601"))

	var exists int
	r.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&exists)
	if exists == 0 {
		return 0, nil
	}

	var count int64
	sql := fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM %s WHERE DATE(FROM_UNIXTIME(log_time)) = ? AND %s IS NOT NULL AND %s != ''", field, tableName, field, field)
	err = r.DB.Raw(sql, statDate).Scan(&count).Error
	return count, err
}

// SumFromLogTables 从多张分表中统计指定日期的记录总数
func (r *DashboardStatsRepository) SumFromLogTables(featureIDs []int, statDate string) (int64, error) {
	var total int64
	for _, fid := range featureIDs {
		count, err := r.CountFromLogTable(fid, statDate)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// CountDistinctMultiTable 从多张分表中统计指定日期的去重字段数
func (r *DashboardStatsRepository) CountDistinctMultiTable(featureIDs []int, statDate string, field string) (int64, error) {
	// 收集所有去重值
	seen := make(map[string]bool)
	for _, fid := range featureIDs {
		t, err := time.Parse("2006-01-02", statDate)
		if err != nil {
			return 0, err
		}
		tableName := fmt.Sprintf("log_%d_%s", fid, t.Format("200601"))

		var exists int
		r.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&exists)
		if exists == 0 {
			continue
		}

		sql := fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE DATE(FROM_UNIXTIME(log_time)) = ? AND %s IS NOT NULL AND %s != ''", field, tableName, field, field)
		var values []string
		r.DB.Raw(sql, statDate).Scan(&values)
		for _, v := range values {
			seen[v] = true
		}
	}
	return int64(len(seen)), nil
}

// GetDeviceStats 从 90000054 日志中统计设备类型分布
func (r *DashboardStatsRepository) GetDeviceStats(statDate string) (map[int]int64, error) {
	t, err := time.Parse("2006-01-02", statDate)
	if err != nil {
		return nil, err
	}
	tableName := fmt.Sprintf("log_90000054_%s", t.Format("200601"))

	var exists int
	r.DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&exists)
	if exists == 0 {
		return nil, nil
	}

	sql := fmt.Sprintf(`
		SELECT
			CAST(JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '$.devtype')) AS UNSIGNED) AS devtype,
			COUNT(DISTINCT login_openid) AS cnt
		FROM %s
		WHERE DATE(FROM_UNIXTIME(log_time)) = ?
			AND parsed_json IS NOT NULL
			AND JSON_EXTRACT(parsed_json, '$.devtype') IS NOT NULL
		GROUP BY devtype
	`, tableName)

	var rows []struct {
		Devtype int
		Cnt     int64
	}
	if err := r.DB.Raw(sql, statDate).Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[int]int64)
	for _, row := range rows {
		result[row.Devtype] = row.Cnt
	}
	return result, nil
}

// GetActiveUsersFromDailyStats 从 user_daily_stats 获取指定日期的活跃用户
func (r *DashboardStatsRepository) GetActiveUsersFromDailyStats(featureIDs []int, statDate string) (int64, error) {
	placeholders := make([]string, len(featureIDs))
	args := make([]interface{}, len(featureIDs))
	for i, fid := range featureIDs {
		placeholders[i] = "?"
		args[i] = fid
	}
	args = append(args, statDate)

	var count int64
	sql := fmt.Sprintf(`
		SELECT COUNT(DISTINCT mobile) FROM user_daily_stats
		WHERE feature_id IN (%s) AND stat_date = ?
	`, strings.Join(placeholders, ","))
	err := r.DB.Raw(sql, args...).Scan(&count).Error
	return count, err
}

// GetRegisteredUserCount 获取注册用户总数
func (r *DashboardStatsRepository) GetRegisteredUserCount() (int64, error) {
	var count int64
	err := r.DB.Raw("SELECT COUNT(*) FROM contacts WHERE status = 1").Scan(&count).Error
	return count, err
}

// GetActivatedUserCount 获取激活用户总数（有 90000048 记录的用户）
func (r *DashboardStatsRepository) GetActivatedUserCount() (int64, error) {
	var count int64
	err := r.DB.Raw("SELECT COUNT(DISTINCT mobile) FROM user_daily_stats WHERE feature_id = 90000048").Scan(&count).Error
	return count, err
}
