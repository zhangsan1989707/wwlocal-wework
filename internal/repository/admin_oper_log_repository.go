package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
)

type AdminOperLogRepository struct {
	DB *gorm.DB
}

func NewAdminOperLogRepository(db *gorm.DB) *AdminOperLogRepository {
	return &AdminOperLogRepository{DB: db}
}

func (r *AdminOperLogRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.AdminOperLog{})
}

func (r *AdminOperLogRepository) BatchSave(logs []model.AdminOperLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.DB.CreateInBatches(logs, 500).Error
}

func (r *AdminOperLogRepository) BatchSaveWithTx(tx *gorm.DB, logs []model.AdminOperLog) error {
	if len(logs) == 0 {
		return nil
	}
	return tx.CreateInBatches(logs, 500).Error
}

func (r *AdminOperLogRepository) Count(operType string, operUserID string, startTime int64, endTime int64) (int64, error) {
	query := r.DB.Model(&model.AdminOperLog{})
	if operType != "" {
		query = query.Where("oper_type = ?", operType)
	}
	if operUserID != "" {
		query = query.Where("oper_userid = ?", operUserID)
	}
	if startTime > 0 {
		query = query.Where("oper_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("oper_time <= ?", endTime)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func (r *AdminOperLogRepository) List(operType string, operUserID string, startTime int64, endTime int64, page int, pageSize int) ([]model.AdminOperLog, int64, error) {
	var logs []model.AdminOperLog
	var total int64
	query := r.DB.Model(&model.AdminOperLog{})
	if operType != "" {
		query = query.Where("oper_type = ?", operType)
	}
	if operUserID != "" {
		query = query.Where("oper_userid = ?", operUserID)
	}
	if startTime > 0 {
		query = query.Where("oper_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("oper_time <= ?", endTime)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Order("oper_time DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *AdminOperLogRepository) GetDistinctOperTypes() ([]string, error) {
	var types []string
	err := r.DB.Model(&model.AdminOperLog{}).Distinct("oper_type").Order("oper_type").Pluck("oper_type", &types).Error
	return types, err
}

func (r *AdminOperLogRepository) GetDistinctOperUsers() ([]string, error) {
	var users []string
	err := r.DB.Model(&model.AdminOperLog{}).Distinct("oper_userid").Order("oper_userid").Pluck("oper_userid", &users).Error
	return users, err
}

func (r *AdminOperLogRepository) GetLatestOperTime() int64 {
	var log model.AdminOperLog
	if err := r.DB.Order("oper_time DESC").First(&log).Error; err != nil {
		return 0
	}
	return log.OperTime
}

func (r *AdminOperLogRepository) GetOperStatsByType(startTime int64, endTime int64) (map[string]int64, error) {
	type Result struct {
		OperType string
		Count    int64
	}
	var results []Result
	query := r.DB.Model(&model.AdminOperLog{}).Select("oper_type, COUNT(*) as count")
	if startTime > 0 {
		query = query.Where("oper_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("oper_time <= ?", endTime)
	}
	if err := query.Group("oper_type").Scan(&results).Error; err != nil {
		return nil, err
	}
	stats := make(map[string]int64)
	for _, r := range results {
		stats[r.OperType] = r.Count
	}
	return stats, nil
}

func (r *AdminOperLogRepository) GetOperStatsByUser(startTime int64, endTime int64) (map[string]int64, error) {
	type Result struct {
		OperUserID string
		Count      int64
	}
	var results []Result
	query := r.DB.Model(&model.AdminOperLog{}).Select("oper_userid, COUNT(*) as count")
	if startTime > 0 {
		query = query.Where("oper_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("oper_time <= ?", endTime)
	}
	if err := query.Group("oper_userid").Scan(&results).Error; err != nil {
		return nil, err
	}
	stats := make(map[string]int64)
	for _, r := range results {
		stats[r.OperUserID] = r.Count
	}
	return stats, nil
}

func (r *AdminOperLogRepository) DeleteBefore(timeUnix int64) (int64, error) {
	result := r.DB.Where("oper_time < ?", timeUnix).Delete(&model.AdminOperLog{})
	return result.RowsAffected, result.Error
}

func (r *AdminOperLogRepository) ExistsByOperTimeAndUserID(operTime int64, operUserID string) (bool, error) {
	var count int64
	err := r.DB.Model(&model.AdminOperLog{}).Where("oper_time = ? AND oper_userid = ?", operTime, operUserID).Count(&count).Error
	return count > 0, err
}

// BatchExistByOperTimeAndUserIDs 批量检查 (oper_time, oper_userid) 对是否已存在。
// 返回已存在的 (time,userid) 组合的集合。
func (r *AdminOperLogRepository) BatchExistByOperTimeAndUserIDs(pairs []model.AdminOperLogAPI) (map[[2]string]bool, error) {
	existing := make(map[[2]string]bool)
	if len(pairs) == 0 {
		return existing, nil
	}

	// 收集所有 oper_time
	timeSet := make(map[int64]bool)
	for _, p := range pairs {
		timeSet[p.OperTime] = true
	}
	var times []int64
	for t := range timeSet {
		times = append(times, t)
	}

	// 用 IN 查询批量获取这些时间点的所有记录
	type pair struct {
		OperTime   int64
		OperUserID string
	}
	var results []pair
	err := r.DB.Model(&model.AdminOperLog{}).
		Select("oper_time, oper_userid").
		Where("oper_time IN ?", times).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	for _, r := range results {
		existing[[2]string{fmt.Sprint(r.OperTime), r.OperUserID}] = true
	}
	return existing, nil
}

type AdminOperLogDailyStat struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (r *AdminOperLogRepository) GetDailyStats(startTime int64, endTime int64) ([]AdminOperLogDailyStat, error) {
	type Result struct {
		Date  time.Time
		Count int64
	}
	var results []Result
	query := r.DB.Model(&model.AdminOperLog{}).
		Select("DATE(FROM_UNIXTIME(oper_time)) as date, COUNT(*) as count")
	if startTime > 0 {
		query = query.Where("oper_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("oper_time <= ?", endTime)
	}
	if err := query.Group("date").Order("date ASC").Scan(&results).Error; err != nil {
		return nil, err
	}
	stats := make([]AdminOperLogDailyStat, len(results))
	for i, r := range results {
		stats[i] = AdminOperLogDailyStat{
			Date:  r.Date.Format("2006-01-02"),
			Count: r.Count,
		}
	}
	return stats, nil
}
