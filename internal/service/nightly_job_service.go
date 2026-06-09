package service

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

// 活跃用户统计的 feature ID 列表
var activeFeatureIDs = []int{90000031, 90000032, 90000033, 90000035, 90000036, 90000037}

// 消息相关的 feature ID 列表
var msgFeatureIDs = []int{90000035, 90000036, 90000037}

// NightlyJobService 定时预计算看板统计数据
type NightlyJobService struct {
	syncSvc        *SyncService
	contactSyncSvc *ContactSyncService
	statsRepo      *repository.DashboardStatsRepository
	contactRepo    *repository.ContactRepository
	logRepo        nightlyLogRepository
	cfg            *config.Config
	running        bool
	jobRunning     bool // 当前任务是否正在执行
	mu             sync.Mutex
	stopCh         chan struct{}
	timer          *time.Timer
}

type NightlyJobStatus struct {
	Enabled            bool   `json:"enabled"`
	ScheduleTime       string `json:"schedule_time"`
	LookbackDays       int    `json:"lookback_days"`
	Running            bool   `json:"running"`
	JobRunning         bool   `json:"job_running"`
	LatestStatDate     string `json:"latest_stat_date"`
	LatestUserListDate string `json:"latest_user_list_date"`
}

type nightlyLogRepository interface {
	BackfillDailyStatsFromLogs(featureIDs []int, statDate string) error
	GetTableName(featureID int, t time.Time) string
	TableExists(tableName string) bool
}

func NewNightlyJobService(
	syncSvc *SyncService,
	contactSyncSvc *ContactSyncService,
	statsRepo *repository.DashboardStatsRepository,
	contactRepo *repository.ContactRepository,
	logRepo *repository.LogRepository,
	cfg *config.Config,
) *NightlyJobService {
	return &NightlyJobService{
		syncSvc:        syncSvc,
		contactSyncSvc: contactSyncSvc,
		statsRepo:      statsRepo,
		contactRepo:    contactRepo,
		logRepo:        logRepo,
		cfg:            cfg,
	}
}

// Start 启动定时任务调度
func (s *NightlyJobService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.scheduleNext()
	slog.Info(fmt.Sprintf("nightly job started, will run at %02d:%02d daily",
		s.cfg.Nightly.Hour, s.cfg.Nightly.Minute))
}

// Stop 停止定时任务
func (s *NightlyJobService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	close(s.stopCh)
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.running = false
	slog.Info("nightly job stopped")
}

// IsRunning 返回定时任务是否已启动
func (s *NightlyJobService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// IsJobRunning 返回当前是否有任务正在执行
func (s *NightlyJobService) IsJobRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.jobRunning
}

func (s *NightlyJobService) Status() NightlyJobStatus {
	s.mu.Lock()
	running := s.running
	jobRunning := s.jobRunning
	s.mu.Unlock()

	lookback := s.cfg.Nightly.LookbackDays
	if lookback <= 0 {
		lookback = 1
	}

	latestStatDate, _ := s.statsRepo.GetLatestDate()
	latestUserListDate, _ := s.statsRepo.GetLatestUserListDate()

	return NightlyJobStatus{
		Enabled:            s.cfg.Nightly.Enabled,
		ScheduleTime:       fmt.Sprintf("%02d:%02d", s.cfg.Nightly.Hour, s.cfg.Nightly.Minute),
		LookbackDays:       lookback,
		Running:            running,
		JobRunning:         jobRunning,
		LatestStatDate:     latestStatDate,
		LatestUserListDate: latestUserListDate,
	}
}

// scheduleNext 计算距下次目标时间的时长并设置定时器。调用者须持有 s.mu
func (s *NightlyJobService) scheduleNext() {
	d := s.durationToNext()
	s.timer = time.AfterFunc(d, func() {
		s.run()
		s.mu.Lock()
		if s.running {
			s.scheduleNext()
		}
		s.mu.Unlock()
	})
	slog.Info(fmt.Sprintf("nightly job next run in %v", d))
}

// durationToNext 计算距下一个目标时刻的时长。调用者须持有 s.mu
func (s *NightlyJobService) durationToNext() time.Duration {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	now := time.Now().In(loc)
	target := time.Date(now.Year(), now.Month(), now.Day(),
		s.cfg.Nightly.Hour, s.cfg.Nightly.Minute, 0, 0, loc)
	if !now.Before(target) {
		target = target.AddDate(0, 0, 1)
	}
	return target.Sub(now)
}

// RunOnce 手动触发一次夜间任务，statDate 格式为 "2006-01-02"
func (s *NightlyJobService) RunOnce(statDate string) {
	s.mu.Lock()
	if s.jobRunning {
		s.mu.Unlock()
		return
	}
	s.jobRunning = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.jobRunning = false
			s.mu.Unlock()
			if r := recover(); r != nil {
				slog.Error(fmt.Sprintf("nightly job RunOnce panic: %v\n%s", r, debug.Stack()))
			}
		}()
		s.runForDate(statDate)
	}()
}

// run 按配置的 lookback_days 计算目标日期并执行
func (s *NightlyJobService) run() {
	s.mu.Lock()
	if s.jobRunning {
		s.mu.Unlock()
		slog.Warn("nightly job: skipped, previous job still running")
		return
	}
	s.jobRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.jobRunning = false
		s.mu.Unlock()
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("nightly job panic: %v\n%s", r, debug.Stack()))
		}
	}()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	if loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	lookback := s.cfg.Nightly.LookbackDays
	if lookback <= 0 {
		lookback = 1
	}
	targetDate := time.Now().In(loc).AddDate(0, 0, -lookback)
	statDate := targetDate.Format("2006-01-02")

	s.runForDate(statDate)
}

// runForDate 执行完整流程：增量同步 + 预计算统计
func (s *NightlyJobService) runForDate(statDate string) {
	startTime := time.Now()
	slog.Info(fmt.Sprintf("nightly job started for %s", statDate))

	// 1. 增量同步日志
	slog.Info("nightly job: step 1/3 - incremental log sync")
	syncResults := s.syncSvc.SyncAllFeaturesIncremental()
	var failedFeatures []int
	for fid, count := range syncResults {
		if count < 0 {
			failedFeatures = append(failedFeatures, fid)
		}
	}
	if len(failedFeatures) > 0 {
		slog.Warn(fmt.Sprintf("nightly job: %d features failed sync: %v, stats may be incomplete", len(failedFeatures), failedFeatures))
	}

	// 2. 增量同步通讯录
	slog.Info("nightly job: step 2/3 - incremental contact sync")
	s.contactSyncSvc.SyncContactsIncremental()

	// 3. 预计算统计指标
	slog.Info(fmt.Sprintf("nightly job: step 3/3 - compute stats for %s", statDate))
	if err := s.computeStats(statDate); err != nil {
		slog.Error(fmt.Sprintf("nightly job: compute stats failed: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("nightly job completed for %s, took %.1f seconds",
		statDate, time.Since(startTime).Seconds()))
}

// computeStats 计算指定日期的全部看板指标并写入数据库
func (s *NightlyJobService) computeStats(statDate string) error {
	if err := s.logRepo.BackfillDailyStatsFromLogs(s.cfg.Features.IDs, statDate); err != nil {
		return fmt.Errorf("backfill daily stats: %w", err)
	}

	// 删除旧数据，保证幂等
	if err := s.statsRepo.DeleteByDate(statDate); err != nil {
		return fmt.Errorf("delete old stats: %w", err)
	}

	// 收集全局指标
	stats, inactiveUsers, err := s.computeGlobalStats(statDate)
	if err != nil {
		return fmt.Errorf("compute global stats: %w", err)
	}

	// 收集部门维度指标
	deptStats, err := s.computeDeptStats(statDate)
	if err != nil {
		slog.Warn(fmt.Sprintf("nightly job: dept stats failed (continuing): %v", err))
	} else {
		stats = append(stats, deptStats...)
	}

	// 批量写入
	if err := s.statsRepo.BatchUpsertStats(stats); err != nil {
		return fmt.Errorf("batch upsert stats: %w", err)
	}
	slog.Info(fmt.Sprintf("nightly job: wrote %d stat rows for %s", len(stats), statDate))

	// 写入不活跃用户列表
	if len(inactiveUsers) > 0 {
		if err := s.statsRepo.UpsertUserList(inactiveUsers); err != nil {
			return fmt.Errorf("upsert user list: %w", err)
		}
		slog.Info(fmt.Sprintf("nightly job: wrote %d inactive users for %s", len(inactiveUsers), statDate))
	}

	return nil
}

// computeGlobalStats 计算全局指标（dimension_key="*"）
func (s *NightlyJobService) computeGlobalStats(statDate string) ([]model.DashboardDailyStat, []model.DashboardDailyUserList, error) {
	var stats []model.DashboardDailyStat

	// -- 注册用户数 --
	registered, err := s.statsRepo.GetRegisteredUserCount()
	if err != nil {
		return nil, nil, fmt.Errorf("get registered count: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricUserRegistered, "*", registered))

	// -- 激活用户数 --
	activated, err := s.statsRepo.GetActivatedUserCount()
	if err != nil {
		return nil, nil, fmt.Errorf("get activated count: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricUserActivated, "*", activated))
	stats = append(stats, s.stat(statDate, model.MetricUserNotActivated, "*", registered-activated))

	// -- 活跃用户数 --
	active, err := s.statsRepo.GetActiveUsersFromDailyStats(activeFeatureIDs, statDate)
	if err != nil {
		return nil, nil, fmt.Errorf("get active count: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricUserActive, "*", active))
	stats = append(stats, s.stat(statDate, model.MetricUserInactive, "*", registered-active))

	// -- 激活率 / 活跃率 (permille) --
	if registered > 0 {
		stats = append(stats, s.stat(statDate, model.MetricRateActivation, "*", activated*1000/registered))
		stats = append(stats, s.stat(statDate, model.MetricRateActive, "*", active*1000/registered))
	}

	// -- 消息量 & 发送人数 --
	msgCount, err := s.statsRepo.SumFromLogTables(msgFeatureIDs, statDate)
	if err != nil {
		return nil, nil, fmt.Errorf("sum msg count: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricMsgCount, "*", msgCount))

	msgSender, err := s.statsRepo.CountDistinctMultiTable(msgFeatureIDs, statDate, "sender_openid")
	if err != nil {
		return nil, nil, fmt.Errorf("count msg sender: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricMsgSender, "*", msgSender))

	// -- 创建群数 --
	groupCreated, err := s.statsRepo.CountFromLogTable(90000038, statDate)
	if err != nil {
		return nil, nil, fmt.Errorf("count group created: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricGroupCreated, "*", groupCreated))

	// -- 活跃群数 --
	groupActive, err := s.statsRepo.CountDistinctFromLogTable(90000037, statDate, "root_openid")
	if err != nil {
		return nil, nil, fmt.Errorf("count group active: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricGroupActive, "*", groupActive))

	// -- 群活跃率（基于累计创建群数） --
	groupCreatedCumul, err := s.cumulativeGroupCreated(statDate)
	if err != nil {
		slog.Warn(fmt.Sprintf("nightly job: get cumulative group created failed: %v", err))
		groupCreatedCumul = 0
	}
	denomGroup := groupCreatedCumul
	if denomGroup < 1 {
		denomGroup = 1
	}
	stats = append(stats, s.stat(statDate, model.MetricRateGroupActive, "*", groupActive*1000/denomGroup))

	// -- 设备分布 --
	deviceStats, err := s.statsRepo.GetDeviceStats(statDate)
	if err != nil {
		slog.Warn(fmt.Sprintf("nightly job: get device stats failed: %v", err))
	} else {
		var deviceTotal int64
		for devtype, metricType := range model.DeviceTypeMap {
			count := deviceStats[devtype]
			stats = append(stats, s.stat(statDate, metricType, "*", count))
			deviceTotal += count
		}
		stats = append(stats, s.stat(statDate, model.MetricDeviceTotal, "*", deviceTotal))
	}

	// -- 应用访问 --
	appAccessUser, err := s.statsRepo.GetActiveUsersFromDailyStats([]int{90000033}, statDate)
	if err != nil {
		return nil, nil, fmt.Errorf("count app access user: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricAppAccessUser, "*", appAccessUser))

	appAccessCount, err := s.statsRepo.CountFromLogTable(90000033, statDate)
	if err != nil {
		return nil, nil, fmt.Errorf("count app access count: %w", err)
	}
	stats = append(stats, s.stat(statDate, model.MetricAppAccessCount, "*", appAccessCount))

	// -- 不活跃用户列表 --
	inactiveUsers, err := s.buildInactiveUserList(statDate)
	if err != nil {
		slog.Warn(fmt.Sprintf("nightly job: build inactive user list failed: %v", err))
	}

	return stats, inactiveUsers, nil
}

// computeDeptStats 计算部门维度的指标
func (s *NightlyJobService) computeDeptStats(statDate string) ([]model.DashboardDailyStat, error) {
	depts, err := s.contactRepo.GetAllDepartments()
	if err != nil {
		return nil, fmt.Errorf("get departments: %w", err)
	}

	// 获取部门成员计数（注册人数）
	memberCounts, err := s.contactRepo.GetMemberCountByDepartmentIDs(deptIDsOf(depts))
	if err != nil {
		return nil, fmt.Errorf("get dept member counts: %w", err)
	}

	var stats []model.DashboardDailyStat
	for _, dept := range depts {
		dimKey := fmt.Sprintf("%d", dept.ID)
		registered := int64(memberCounts[dept.ID])

		// 该部门的注册人数
		stats = append(stats, s.stat(statDate, model.MetricUserRegistered, dimKey, registered))

		// 该部门的活跃人数：JOIN contact_departments + user_daily_stats
		deptActive, err := s.deptActiveCount(dept.ID, statDate)
		if err != nil {
			slog.Warn(fmt.Sprintf("nightly job: dept %d active count failed: %v", dept.ID, err))
			continue
		}
		stats = append(stats, s.stat(statDate, model.MetricUserActive, dimKey, deptActive))
		if registered > 0 {
			stats = append(stats, s.stat(statDate, model.MetricUserInactive, dimKey, registered-deptActive))
			stats = append(stats, s.stat(statDate, model.MetricRateActive, dimKey, deptActive*1000/registered))
		}
	}

	return stats, nil
}

// deptActiveCount 查询某部门在指定日期的活跃人数
func (s *NightlyJobService) deptActiveCount(deptID int, statDate string) (int64, error) {
	var count int64
	sql := `
		SELECT COUNT(DISTINCT uds.mobile)
		FROM user_daily_stats uds
		INNER JOIN contacts c ON uds.mobile = c.mobile
		INNER JOIN contact_departments cd ON c.user_id = cd.user_id
		WHERE cd.department = ?
		  AND uds.stat_date = ?
		  AND uds.feature_id IN (?,?,?, ?,?,?)`
	err := s.statsRepo.DB.Raw(sql,
		deptID, statDate,
		activeFeatureIDs[0], activeFeatureIDs[1], activeFeatureIDs[2],
		activeFeatureIDs[3], activeFeatureIDs[4], activeFeatureIDs[5],
	).Scan(&count).Error
	return count, err
}

// cumulativeGroupCreated 获取截至 statDate 的累计创建群数（遍历所有月表）
func (s *NightlyJobService) cumulativeGroupCreated(statDate string) (int64, error) {
	t, err := time.Parse("2006-01-02", statDate)
	if err != nil {
		return 0, err
	}
	var total int64
	for i := 0; i < 12; i++ {
		month := t.AddDate(0, -i, 0)
		month = time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		tableName := s.logRepo.GetTableName(90000038, month)
		if !s.logRepo.TableExists(tableName) {
			continue
		}
		var count int64
		sql := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE DATE(FROM_UNIXTIME(log_time)) <= ?", tableName)
		if err := s.statsRepo.DB.Raw(sql, statDate).Scan(&count).Error; err != nil {
			slog.Warn(fmt.Sprintf("cumulativeGroupCreated: query %s failed: %v", tableName, err))
			continue
		}
		total += count
	}
	return total, nil
}

// buildInactiveUserList 构建不活跃用户列表：注册(status=1)但当日无活跃记录
func (s *NightlyJobService) buildInactiveUserList(statDate string) ([]model.DashboardDailyUserList, error) {
	sql := `
		SELECT c.mobile, c.user_id, c.name, c.department
		FROM contacts c
		WHERE c.status = 1
		  AND c.mobile IS NOT NULL AND c.mobile != ''
		  AND NOT EXISTS (
			SELECT 1 FROM user_daily_stats uds
			WHERE uds.mobile = c.mobile
			  AND uds.stat_date = ?
			  AND uds.feature_id IN (?,?,?, ?,?,?)
		  )
		LIMIT 50000`

	type row struct {
		Mobile     string
		UserID     string
		Name       string
		Department string
	}
	var rows []row
	if err := s.statsRepo.DB.Raw(sql,
		statDate,
		activeFeatureIDs[0], activeFeatureIDs[1], activeFeatureIDs[2],
		activeFeatureIDs[3], activeFeatureIDs[4], activeFeatureIDs[5],
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	users := make([]model.DashboardDailyUserList, 0, len(rows))
	for _, r := range rows {
		users = append(users, model.DashboardDailyUserList{
			StatDate:   statDate,
			ListType:   model.ListTypeInactive,
			Mobile:     r.Mobile,
			UserID:     r.UserID,
			Name:       r.Name,
			Department: r.Department,
		})
	}
	return users, nil
}

// stat 构造一个 DashboardDailyStat 实例
func (s *NightlyJobService) stat(statDate, metricType, dimensionKey string, value int64) model.DashboardDailyStat {
	return model.DashboardDailyStat{
		StatDate:     statDate,
		MetricType:   metricType,
		DimensionKey: dimensionKey,
		MetricValue:  value,
	}
}

// deptIDsOf 提取部门 ID 列表
func deptIDsOf(depts []model.Department) []int {
	ids := make([]int, len(depts))
	for i, d := range depts {
		ids[i] = d.ID
	}
	return ids
}
