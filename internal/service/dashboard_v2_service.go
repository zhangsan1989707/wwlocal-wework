package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type DashboardV2Service struct {
	statsRepo   dashboardV2StatsRepository
	contactRepo dashboardV2ContactRepository
	cfg         *config.Config
}

var ErrDashboardInvalidParam = errors.New("invalid dashboard parameter")

type dashboardV2StatsRepository interface {
	CountDistinctUsersFromDailyStats(featureIDs []int, startDate, endDate string, deptIDs []int, unrestricted bool) (int64, error)
	CountDistinctUsersFromDailyStatsThroughDate(featureIDs []int, endDate string, deptIDs []int, unrestricted bool) (int64, error)
	CountLogRowsScoped(featureIDs []int, startDate, endDate, userField string, deptIDs []int, unrestricted bool) (int64, error)
	GetPeopleTrend(featureIDs []int, startDate, endDate, granularity string, deptIDs []int, unrestricted bool) ([]repository.AggregatedStat, error)
	GetEventTrendScoped(featureIDs []int, startDate, endDate, granularity, userField string, deptIDs []int, unrestricted bool) ([]repository.AggregatedStat, error)
	GetDeviceStatsScoped(statDate string, deptIDs []int, unrestricted bool) (map[int]int64, error)
	GetScopedUserList(statDate, listType string, activeFeatureIDs []int, deptIDs []int, unrestricted bool, limit, offset int) ([]model.DashboardDailyUserList, int64, error)
	GetLatestDate() (string, error)
}

type dashboardV2ContactRepository interface {
	GetScopedContactCount(deptIDs []int, unrestricted bool) (int64, error)
	GetAllDepartments() ([]model.Department, error)
	GetDeptMemberCounts() ([]repository.DeptMemberCount, error)
}

func NewDashboardV2Service(statsRepo *repository.DashboardStatsRepository, contactRepo *repository.ContactRepository, cfg *config.Config) *DashboardV2Service {
	return &DashboardV2Service{statsRepo: statsRepo, contactRepo: contactRepo, cfg: cfg}
}

// defaultYesterday 空日期默认为昨天
func defaultYesterday(date string) string {
	if date == "" {
		return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}
	return date
}

func validateDashboardDate(date string) (string, error) {
	date = defaultYesterday(date)
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", fmt.Errorf("%w: date must be YYYY-MM-DD", ErrDashboardInvalidParam)
	}
	return date, nil
}

// resolveDateRange 根据 granularity 参数解析起止日期
func resolveDateRange(startDate, endDate, granularity string) (string, string, error) {
	if err := validateGranularity(granularity); err != nil {
		return "", "", err
	}

	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", fmt.Errorf("%w: end_date must be YYYY-MM-DD", ErrDashboardInvalidParam)
	}
	if startDate != "" {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return "", "", fmt.Errorf("%w: start_date must be YYYY-MM-DD", ErrDashboardInvalidParam)
		}
		if start.After(end) {
			return "", "", fmt.Errorf("%w: start_date cannot be after end_date", ErrDashboardInvalidParam)
		}
		return startDate, endDate, nil
	}

	loc := end.Location()
	switch granularity {
	case "week":
		startDate = end.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		startDate = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, loc).Format("2006-01-02")
	case "quarter":
		startDate = end.AddDate(0, -3, 0).Format("2006-01-02")
	default:
		startDate = end.AddDate(0, 0, -7).Format("2006-01-02")
	}
	return startDate, endDate, nil
}

// defaultGranularity 空粒度默认为 day
func defaultGranularity(g string) string {
	if g == "" {
		return "day"
	}
	return g
}

func validateGranularity(granularity string) error {
	switch defaultGranularity(granularity) {
	case "day", "week", "month", "quarter":
		return nil
	default:
		return fmt.Errorf("%w: granularity must be one of day, week, month, quarter", ErrDashboardInvalidParam)
	}
}

func normalizeListType(listType string) (string, error) {
	switch listType {
	case "":
		return model.ListTypeInactive, nil
	case "not_login":
		return model.ListTypeNoLogin, nil
	case model.ListTypeInactive, model.ListTypeActive, model.ListTypeNoLogin:
		return listType, nil
	default:
		return "", fmt.Errorf("%w: list_type must be one of active, inactive, no_login", ErrDashboardInvalidParam)
	}
}

func validateMetricType(metricType string) error {
	switch metricType {
	case "login_users", "usage_users", "active", model.MetricUserActive,
		model.MetricAppAccessUser, model.MetricMsgSender, model.MetricMsgCount,
		model.MetricAppAccessCount, model.MetricGroupCreated:
		return nil
	default:
		return fmt.Errorf("%w: unsupported metric_type %q", ErrDashboardInvalidParam, metricType)
	}
}

func dayRangeFromDate(date string) (string, string) {
	date = defaultYesterday(date)
	return date, date
}

// GetOverview 获取指定日期的总览数据
func (s *DashboardV2Service) GetOverview(date string, scope *DataScope) (map[string]interface{}, error) {
	date, err := validateDashboardDate(date)
	if err != nil {
		return nil, err
	}
	startDate, endDate := dayRangeFromDate(date)
	date = startDate
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}

	registered, err := s.contactRepo.GetScopedContactCount(scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询注册人数失败: %w", err)
	}
	activated, err := s.statsRepo.CountDistinctUsersFromDailyStatsThroughDate([]int{90000048}, endDate, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询激活人数失败: %w", err)
	}
	loginUsers, err := s.statsRepo.CountDistinctUsersFromDailyStats([]int{90000031}, startDate, endDate, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询登录人数失败: %w", err)
	}
	usageUsers, err := s.statsRepo.CountDistinctUsersFromDailyStats(activeFeatureIDs, startDate, endDate, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询使用人数失败: %w", err)
	}
	msgCount, err := s.statsRepo.CountLogRowsScoped(msgFeatureIDs, startDate, endDate, "sender_openid", scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询消息数失败: %w", err)
	}
	msgSender, err := s.statsRepo.CountDistinctUsersFromDailyStats(msgFeatureIDs, startDate, endDate, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询消息发送人数失败: %w", err)
	}
	groupCreated, err := s.statsRepo.CountLogRowsScoped([]int{90000038}, startDate, endDate, "root_openid", scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询创建群数失败: %w", err)
	}
	appAccessUser, err := s.statsRepo.CountDistinctUsersFromDailyStats([]int{90000033}, startDate, endDate, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询应用访问人数失败: %w", err)
	}
	appAccessCount, err := s.statsRepo.CountLogRowsScoped([]int{90000033}, startDate, endDate, "login_openid", scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询应用访问次数失败: %w", err)
	}
	devices, err := s.GetDeviceStats(date, scope)
	if err != nil {
		return nil, fmt.Errorf("查询设备统计失败: %w", err)
	}

	var rateActivation int64
	var rateActive int64
	if registered > 0 {
		rateActivation = activated * 1000 / registered
		rateActive = usageUsers * 1000 / registered
	}
	inactive := registered - usageUsers
	if inactive < 0 {
		inactive = 0
	}

	return map[string]interface{}{
		"date":              date,
		"registered":        registered,
		"activated":         activated,
		"not_activated":     registered - activated,
		"login_users":       loginUsers,
		"usage_users":       usageUsers,
		"active":            usageUsers,
		"inactive":          inactive,
		"rate_activation":   rateActivation,
		"rate_active":       rateActive,
		"msg_count":         msgCount,
		"msg_sender":        msgSender,
		"group_created":     groupCreated,
		"group_active":      int64(0),
		"rate_group_active": int64(0),
		"app_access_user":   appAccessUser,
		"app_access_count":  appAccessCount,
		"devices":           devices,
		"scope": map[string]interface{}{
			"role":     scope.Role,
			"dept_ids": scope.DeptIDs,
		},
	}, nil
}

// GetTrend 获取单指标趋势
func (s *DashboardV2Service) GetTrend(metricType, startDate, endDate, granularity, dimensionKey string, scope *DataScope) (map[string]interface{}, error) {
	if metricType == "" {
		return nil, fmt.Errorf("metric_type 不能为空")
	}
	granularity = defaultGranularity(granularity)
	if err := validateMetricType(metricType); err != nil {
		return nil, err
	}
	var err error
	startDate, endDate, err = resolveDateRange(startDate, endDate, granularity)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}

	results, err := s.metricTrend(metricType, startDate, endDate, granularity, scope)
	if err != nil {
		return nil, fmt.Errorf("查询趋势数据失败: %w", err)
	}

	periods := make([]string, 0, len(results))
	values := make([]int64, 0, len(results))
	for _, r := range results {
		periods = append(periods, r.Period)
		values = append(values, r.Value)
	}

	return map[string]interface{}{
		"granularity": granularity,
		"periods":     periods,
		"series": map[string][]int64{
			metricType: values,
		},
	}, nil
}

// GetMultiTrend 获取多指标趋势（用于趋势图）
func (s *DashboardV2Service) GetMultiTrend(metricTypes []string, startDate, endDate, granularity string, scope *DataScope) (map[string]interface{}, error) {
	if len(metricTypes) == 0 {
		return nil, fmt.Errorf("metric_types 不能为空")
	}
	granularity = defaultGranularity(granularity)
	for _, mt := range metricTypes {
		if err := validateMetricType(mt); err != nil {
			return nil, err
		}
	}
	var err error
	startDate, endDate, err = resolveDateRange(startDate, endDate, granularity)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}

	// 收集所有指标的结果，合并日期轴
	allResults := make(map[string]map[string]int64, len(metricTypes))
	dateSet := make(map[string]bool)
	for _, mt := range metricTypes {
		results, err := s.metricTrend(mt, startDate, endDate, granularity, scope)
		if err != nil {
			return nil, fmt.Errorf("查询指标 %s 趋势失败: %w", mt, err)
		}
		valueMap := make(map[string]int64, len(results))
		for _, r := range results {
			valueMap[r.Period] = r.Value
			dateSet[r.Period] = true
		}
		allResults[mt] = valueMap
	}

	// 排序日期轴
	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	// 按统一日期轴填充各指标数据
	seriesMap := make(map[string][]int64, len(metricTypes))
	for _, mt := range metricTypes {
		vals := make([]int64, len(dates))
		for i, d := range dates {
			vals[i] = allResults[mt][d]
		}
		seriesMap[mt] = vals
	}

	return map[string]interface{}{
		"granularity": granularity,
		"periods":     dates,
		"series":      seriesMap,
	}, nil
}

func (s *DashboardV2Service) metricTrend(metricType, startDate, endDate, granularity string, scope *DataScope) ([]repository.AggregatedStat, error) {
	switch metricType {
	case "login_users":
		return s.statsRepo.GetPeopleTrend([]int{90000031}, startDate, endDate, granularity, scope.DeptIDs, scope.Unrestricted)
	case "usage_users", "active", model.MetricUserActive:
		return s.statsRepo.GetPeopleTrend(activeFeatureIDs, startDate, endDate, granularity, scope.DeptIDs, scope.Unrestricted)
	case "app_access_user":
		return s.statsRepo.GetPeopleTrend([]int{90000033}, startDate, endDate, granularity, scope.DeptIDs, scope.Unrestricted)
	case model.MetricMsgSender:
		return s.statsRepo.GetPeopleTrend(msgFeatureIDs, startDate, endDate, granularity, scope.DeptIDs, scope.Unrestricted)
	case model.MetricMsgCount:
		return s.statsRepo.GetEventTrendScoped(msgFeatureIDs, startDate, endDate, granularity, "sender_openid", scope.DeptIDs, scope.Unrestricted)
	case model.MetricAppAccessCount:
		return s.statsRepo.GetEventTrendScoped([]int{90000033}, startDate, endDate, granularity, "login_openid", scope.DeptIDs, scope.Unrestricted)
	case model.MetricGroupCreated:
		return s.statsRepo.GetEventTrendScoped([]int{90000038}, startDate, endDate, granularity, "root_openid", scope.DeptIDs, scope.Unrestricted)
	default:
		return nil, fmt.Errorf("%w: unsupported metric_type %q", ErrDashboardInvalidParam, metricType)
	}
}

// GetDepartmentStats 获取部门维度统计
func (s *DashboardV2Service) GetDepartmentStats(date string, scope *DataScope) ([]map[string]interface{}, error) {
	date, err := validateDashboardDate(date)
	if err != nil {
		return nil, err
	}
	startDate, endDate := dayRangeFromDate(date)
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}
	depts, err := s.contactRepo.GetAllDepartments()
	if err != nil {
		return nil, fmt.Errorf("查询部门列表失败: %w", err)
	}
	deptCounts, err := s.contactRepo.GetDeptMemberCounts()
	if err != nil {
		return nil, fmt.Errorf("查询部门人数失败: %w", err)
	}
	deptCountMap := make(map[int]int64, len(deptCounts))
	for _, dc := range deptCounts {
		deptCountMap[dc.DeptID] = dc.Count
	}
	allowed := make(map[int]bool)
	if !scope.Unrestricted {
		for _, id := range scope.DeptIDs {
			allowed[id] = true
		}
	}
	result := make([]map[string]interface{}, 0, len(depts))
	for _, d := range depts {
		if !scope.Unrestricted && !allowed[d.ID] {
			continue
		}
		total := deptCountMap[d.ID]
		if total == 0 {
			continue
		}
		active, err := s.statsRepo.CountDistinctUsersFromDailyStats(activeFeatureIDs, startDate, endDate, []int{d.ID}, false)
		if err != nil {
			return nil, err
		}
		inactive := total - active
		if inactive < 0 {
			inactive = 0
		}
		var rate float64
		if total > 0 {
			rate = float64(active) / float64(total) * 100
		}
		result = append(result, map[string]interface{}{
			"dept_id":        d.ID,
			"dept_name":      d.Name,
			"total_contacts": total,
			"active":         active,
			"inactive":       inactive,
			"active_rate":    rate,
		})
	}
	return result, nil
}

// GetDeviceStats 获取设备类型统计
func (s *DashboardV2Service) GetDeviceStats(date string, scope *DataScope) (map[string]interface{}, error) {
	var err error
	date, err = validateDashboardDate(date)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}

	deviceStats, err := s.statsRepo.GetDeviceStatsScoped(date, scope.DeptIDs, scope.Unrestricted)
	if err != nil {
		return nil, fmt.Errorf("查询设备指标失败: %w", err)
	}
	return buildDevicePayload(date, deviceStats), nil
}

func buildDevicePayload(date string, deviceStats map[int]int64) map[string]interface{} {
	deviceTypes := []struct {
		devtype    int
		metricType string
		name       string
	}{
		{131073, model.MetricDeviceAndroid, model.DeviceTypeName[model.MetricDeviceAndroid]},
		{131074, model.MetricDeviceIOS, model.DeviceTypeName[model.MetricDeviceIOS]},
		{131075, model.MetricDeviceIPad, model.DeviceTypeName[model.MetricDeviceIPad]},
		{65537, model.MetricDeviceWindows, model.DeviceTypeName[model.MetricDeviceWindows]},
		{65538, model.MetricDeviceMacOS, model.DeviceTypeName[model.MetricDeviceMacOS]},
		{65540, model.MetricDeviceLinux, model.DeviceTypeName[model.MetricDeviceLinux]},
	}

	var total int64
	for _, count := range deviceStats {
		total += count
	}
	items := make([]map[string]interface{}, 0, len(deviceTypes))
	for _, dt := range deviceTypes {
		count := deviceStats[dt.devtype]
		var pct float64
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		items = append(items, map[string]interface{}{
			"type":       dt.metricType,
			"name":       dt.name,
			"count":      count,
			"percentage": pct,
		})
	}

	return map[string]interface{}{
		"date":  date,
		"total": total,
		"types": items,
	}
}

// GetUserList 获取用户明细列表（分页）
func (s *DashboardV2Service) GetUserList(date, listType string, page, pageSize int, scope *DataScope) (map[string]interface{}, error) {
	var err error
	date, err = validateDashboardDate(date)
	if err != nil {
		return nil, err
	}
	listType, err = normalizeListType(listType)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}
	listFeatureIDs := activeFeatureIDs
	if listType == model.ListTypeNoLogin {
		listFeatureIDs = []int{90000031}
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	users, total, err := s.statsRepo.GetScopedUserList(date, listType, listFeatureIDs, scope.DeptIDs, scope.Unrestricted, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}

	type userItem struct {
		Mobile     string `json:"mobile"`
		UserID     string `json:"user_id"`
		Name       string `json:"name"`
		Department string `json:"department"`
	}
	items := make([]userItem, 0, len(users))
	for _, u := range users {
		items = append(items, userItem{
			Mobile:     u.Mobile,
			UserID:     u.UserID,
			Name:       u.Name,
			Department: u.Department,
		})
	}

	return map[string]interface{}{
		"users":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, nil
}

// ExportUserListCSV 导出用户明细 CSV
func (s *DashboardV2Service) ExportUserListCSV(date, listType string, scope *DataScope) ([][]string, error) {
	var err error
	date, err = validateDashboardDate(date)
	if err != nil {
		return nil, err
	}
	listType, err = normalizeListType(listType)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		scope = &DataScope{Unrestricted: true}
	}
	listFeatureIDs := activeFeatureIDs
	if listType == model.ListTypeNoLogin {
		listFeatureIDs = []int{90000031}
	}

	users, _, err := s.statsRepo.GetScopedUserList(date, listType, listFeatureIDs, scope.DeptIDs, scope.Unrestricted, 100000, 0)
	if err != nil {
		return nil, fmt.Errorf("导出用户列表失败: %w", err)
	}

	rows := make([][]string, 0, len(users)+1)
	rows = append(rows, []string{"姓名", "手机号", "UserID", "部门"})
	for _, u := range users {
		rows = append(rows, []string{u.Name, u.Mobile, u.UserID, u.Department})
	}
	return rows, nil
}

// ExportOverviewCSV 导出总览指标 CSV
func (s *DashboardV2Service) ExportOverviewCSV(date string, scope *DataScope) ([][]string, error) {
	overview, err := s.GetOverview(date, scope)
	if err != nil {
		return nil, err
	}

	rows := [][]string{
		{"指标", "值"},
		{"日期", fmt.Sprintf("%v", overview["date"])},
		{"注册人数", fmt.Sprintf("%v", overview["registered"])},
		{"已激活人数", fmt.Sprintf("%v", overview["activated"])},
		{"未激活人数", fmt.Sprintf("%v", overview["not_activated"])},
		{"活跃人数", fmt.Sprintf("%v", overview["active"])},
		{"不活跃人数", fmt.Sprintf("%v", overview["inactive"])},
		{"激活率", fmt.Sprintf("%v", overview["rate_activation"])},
		{"活跃率", fmt.Sprintf("%v", overview["rate_active"])},
		{"消息数", fmt.Sprintf("%v", overview["msg_count"])},
		{"消息发送人数", fmt.Sprintf("%v", overview["msg_sender"])},
		{"创建群数", fmt.Sprintf("%v", overview["group_created"])},
		{"活跃群数", fmt.Sprintf("%v", overview["group_active"])},
		{"群活跃率", fmt.Sprintf("%v", overview["rate_group_active"])},
		{"应用访问人数", fmt.Sprintf("%v", overview["app_access_user"])},
		{"应用访问次数", fmt.Sprintf("%v", overview["app_access_count"])},
		{"设备总数", fmt.Sprintf("%v", overview["devices"].(map[string]interface{})["total"])},
	}

	return rows, nil
}

// GetLatestAvailableDate 获取最新可用数据日期
func (s *DashboardV2Service) GetLatestAvailableDate() (string, error) {
	date, err := s.statsRepo.GetLatestDate()
	if err != nil {
		return "", fmt.Errorf("查询最新日期失败: %w", err)
	}
	return date, nil
}
