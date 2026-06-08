package service

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type DashboardV2Service struct {
	statsRepo   *repository.DashboardStatsRepository
	contactRepo *repository.ContactRepository
	cfg         *config.Config
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

// resolveDateRange 根据 granularity 参数解析起止日期
func resolveDateRange(startDate, endDate, granularity string) (string, string) {
	now := time.Now()
	loc := now.Location()

	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	if startDate != "" {
		return startDate, endDate
	}

	switch granularity {
	case "week":
		startDate = now.AddDate(0, 0, -7).Format("2006-01-02")
	case "month":
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).Format("2006-01-02")
	case "quarter":
		startDate = now.AddDate(0, -3, 0).Format("2006-01-02")
	case "year":
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc).Format("2006-01-02")
	default:
		startDate = now.AddDate(0, 0, -7).Format("2006-01-02")
	}
	return startDate, endDate
}

// defaultGranularity 空粒度默认为 day
func defaultGranularity(g string) string {
	if g == "" {
		return "day"
	}
	return g
}

// GetOverview 获取指定日期的总览数据
func (s *DashboardV2Service) GetOverview(date string) (map[string]interface{}, error) {
	date = defaultYesterday(date)

	stats, err := s.statsRepo.GetMetricTypeStats(date)
	if err != nil {
		return nil, fmt.Errorf("查询看板指标失败: %w", err)
	}

	get := func(metricType string) int64 {
		if m, ok := stats[metricType]; ok {
			if v, ok := m["*"]; ok {
				return v
			}
		}
		return 0
	}

	// 设备数据
	deviceMetrics := []struct {
		metricType string
		name       string
	}{
		{model.MetricDeviceAndroid, model.DeviceTypeName[model.MetricDeviceAndroid]},
		{model.MetricDeviceIOS, model.DeviceTypeName[model.MetricDeviceIOS]},
		{model.MetricDeviceIPad, model.DeviceTypeName[model.MetricDeviceIPad]},
		{model.MetricDeviceWindows, model.DeviceTypeName[model.MetricDeviceWindows]},
		{model.MetricDeviceMacOS, model.DeviceTypeName[model.MetricDeviceMacOS]},
		{model.MetricDeviceLinux, model.DeviceTypeName[model.MetricDeviceLinux]},
	}
	deviceStats := make(map[string]int64)
	for _, d := range deviceMetrics {
		deviceStats[d.name] = get(d.metricType)
	}

	return map[string]interface{}{
		"date":             date,
		"registered":       get(model.MetricUserRegistered),
		"activated":        get(model.MetricUserActivated),
		"not_activated":    get(model.MetricUserNotActivated),
		"active":           get(model.MetricUserActive),
		"inactive":         get(model.MetricUserInactive),
		"rate_activation":  get(model.MetricRateActivation),
		"rate_active":      get(model.MetricRateActive),
		"msg_count":        get(model.MetricMsgCount),
		"msg_sender":       get(model.MetricMsgSender),
		"group_created":    get(model.MetricGroupCreated),
		"group_active":     get(model.MetricGroupActive),
		"rate_group_active": get(model.MetricRateGroupActive),
		"app_access_user":  get(model.MetricAppAccessUser),
		"app_access_count": get(model.MetricAppAccessCount),
		"device_total":     get(model.MetricDeviceTotal),
		"device_stats":     deviceStats,
	}, nil
}

// GetTrend 获取单指标趋势
func (s *DashboardV2Service) GetTrend(metricType, startDate, endDate, granularity, dimensionKey string) (map[string]interface{}, error) {
	if metricType == "" {
		return nil, fmt.Errorf("metric_type 不能为空")
	}
	granularity = defaultGranularity(granularity)
	startDate, endDate = resolveDateRange(startDate, endDate, granularity)

	results, err := s.statsRepo.GetStatsWithAggregation(metricType, startDate, endDate, granularity, dimensionKey)
	if err != nil {
		return nil, fmt.Errorf("查询趋势数据失败: %w", err)
	}

	dates := make([]string, 0, len(results))
	values := make([]int64, 0, len(results))
	for _, r := range results {
		dates = append(dates, r.Period)
		values = append(values, r.Value)
	}

	return map[string]interface{}{
		"metric_type": metricType,
		"granularity": granularity,
		"dates":       dates,
		"values":      values,
	}, nil
}

// GetMultiTrend 获取多指标趋势（用于趋势图）
func (s *DashboardV2Service) GetMultiTrend(metricTypes []string, startDate, endDate, granularity string) (map[string]interface{}, error) {
	if len(metricTypes) == 0 {
		return nil, fmt.Errorf("metric_types 不能为空")
	}
	granularity = defaultGranularity(granularity)
	startDate, endDate = resolveDateRange(startDate, endDate, granularity)

	// 收集所有指标的结果，合并日期轴
	allResults := make(map[string]map[string]int64, len(metricTypes))
	dateSet := make(map[string]bool)
	for _, mt := range metricTypes {
		results, err := s.statsRepo.GetStatsWithAggregation(mt, startDate, endDate, granularity, "")
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
		"dates":       dates,
		"series":      seriesMap,
	}, nil
}

// GetDepartmentStats 获取部门维度统计
func (s *DashboardV2Service) GetDepartmentStats(date string) (map[string]interface{}, error) {
	date = defaultYesterday(date)

	stats, err := s.statsRepo.GetMetricTypeStats(date)
	if err != nil {
		return nil, fmt.Errorf("查询部门指标失败: %w", err)
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

	// 从 stats 中提取部门维度数据（dimension_key != "*"）
	activeByDept := make(map[string]int64)
	inactiveByDept := make(map[string]int64)
	if m, ok := stats[model.MetricUserActive]; ok {
		for dk, v := range m {
			if dk != "*" {
				activeByDept[dk] = v
			}
		}
	}
	if m, ok := stats[model.MetricUserInactive]; ok {
		for dk, v := range m {
			if dk != "*" {
				inactiveByDept[dk] = v
			}
		}
	}

	type deptStat struct {
		DeptID      int     `json:"dept_id"`
		DeptName    string  `json:"dept_name"`
		Total       int64   `json:"total_contacts"`
		Active      int64   `json:"active"`
		Inactive    int64   `json:"inactive"`
		ActiveRate  float64 `json:"active_rate"`
	}
	result := make([]deptStat, 0, len(depts))
	for _, d := range depts {
		total := deptCountMap[d.ID]
		if total == 0 {
			continue
		}
		key := strconv.Itoa(d.ID)
		active := activeByDept[key]
		inactive := inactiveByDept[key]
		var rate float64
		if total > 0 {
			rate = float64(active) / float64(total) * 100
		}
		result = append(result, deptStat{
			DeptID:     d.ID,
			DeptName:   d.Name,
			Total:      total,
			Active:     active,
			Inactive:   inactive,
			ActiveRate: rate,
		})
	}

	return map[string]interface{}{
		"date":        date,
		"departments": result,
	}, nil
}

// GetDeviceStats 获取设备类型统计
func (s *DashboardV2Service) GetDeviceStats(date string) (map[string]interface{}, error) {
	date = defaultYesterday(date)

	stats, err := s.statsRepo.GetMetricTypeStats(date)
	if err != nil {
		return nil, fmt.Errorf("查询设备指标失败: %w", err)
	}

	get := func(metricType string) int64 {
		if m, ok := stats[metricType]; ok {
			if v, ok := m["*"]; ok {
				return v
			}
		}
		return 0
	}

	total := get(model.MetricDeviceTotal)
	deviceTypes := []struct {
		metricType string
		name       string
	}{
		{model.MetricDeviceAndroid, model.DeviceTypeName[model.MetricDeviceAndroid]},
		{model.MetricDeviceIOS, model.DeviceTypeName[model.MetricDeviceIOS]},
		{model.MetricDeviceIPad, model.DeviceTypeName[model.MetricDeviceIPad]},
		{model.MetricDeviceWindows, model.DeviceTypeName[model.MetricDeviceWindows]},
		{model.MetricDeviceMacOS, model.DeviceTypeName[model.MetricDeviceMacOS]},
		{model.MetricDeviceLinux, model.DeviceTypeName[model.MetricDeviceLinux]},
	}

	type deviceItem struct {
		Name       string  `json:"name"`
		Count      int64   `json:"count"`
		Percentage float64 `json:"percentage"`
	}
	items := make([]deviceItem, 0, len(deviceTypes))
	for _, dt := range deviceTypes {
		count := get(dt.metricType)
		var pct float64
		if total > 0 {
			pct = float64(count) / float64(total) * 100
		}
		items = append(items, deviceItem{
			Name:       dt.name,
			Count:      count,
			Percentage: pct,
		})
	}

	return map[string]interface{}{
		"date":   date,
		"total":  total,
		"devices": items,
	}, nil
}

// GetUserList 获取用户明细列表（分页）
func (s *DashboardV2Service) GetUserList(date, listType string, page, pageSize int) (map[string]interface{}, error) {
	date = defaultYesterday(date)
	if listType == "" {
		listType = model.ListTypeInactive
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	users, total, err := s.statsRepo.GetUserList(date, listType, pageSize, offset)
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
func (s *DashboardV2Service) ExportUserListCSV(date, listType string) ([][]string, error) {
	date = defaultYesterday(date)
	if listType == "" {
		listType = model.ListTypeInactive
	}

	users, err := s.statsRepo.ExportUserList(date, listType)
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
func (s *DashboardV2Service) ExportOverviewCSV(date string) ([][]string, error) {
	overview, err := s.GetOverview(date)
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
		{"设备总数", fmt.Sprintf("%v", overview["device_total"])},
	}

	if ds, ok := overview["device_stats"].(map[string]int64); ok {
		for name, count := range ds {
			rows = append(rows, []string{name, strconv.FormatInt(count, 10)})
		}
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
