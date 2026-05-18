package service

import (
	"encoding/json"
	"strconv"
	"time"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/repository"
)

type DashboardService struct {
	logRepo         *repository.LogRepository
	contactRepo     *repository.ContactRepository
	syncHistoryRepo *repository.SyncHistoryRepository
	syncStateRepo   *repository.SyncStateRepository
	keyRepo         *repository.KeyRepository
	cfg             *config.Config
}

func NewDashboardService(logRepo *repository.LogRepository, contactRepo *repository.ContactRepository, syncHistoryRepo *repository.SyncHistoryRepository, syncStateRepo *repository.SyncStateRepository, keyRepo *repository.KeyRepository, cfg *config.Config) *DashboardService {
	return &DashboardService{logRepo: logRepo, contactRepo: contactRepo, syncHistoryRepo: syncHistoryRepo, syncStateRepo: syncStateRepo, keyRepo: keyRepo, cfg: cfg}
}

func (s *DashboardService) GetOverview() (map[string]interface{}, error) {
	kpis := make(map[string]interface{})
	recentSyncs := make([]map[string]interface{}, 0)
	problems := make([]map[string]interface{}, 0)

	states, _ := s.syncStateRepo.GetAll()
	var latestSyncAt time.Time
	var latestLogTime int64
	for _, st := range states {
		if st.LastSyncAt.After(latestSyncAt) {
			latestSyncAt = st.LastSyncAt
		}
		if st.LastLogTime > latestLogTime {
			latestLogTime = st.LastLogTime
		}
	}
	kpis["latest_sync_time"] = latestSyncAt
	kpis["latest_log_time"] = latestLogTime

	since7d := time.Now().AddDate(0, 0, -7)
	_, synced7d, _, _ := s.syncHistoryRepo.GetStats("log", since7d)
	kpis["synced_7d_count"] = synced7d

	latestLog, err := s.syncHistoryRepo.GetLatest("log")
	if err == nil && latestLog.Failed > 0 {
		kpis["failed_feature_count"] = latestLog.Failed
	} else {
		kpis["failed_feature_count"] = 0
	}

	activeKey, keyErr := s.keyRepo.GetActive()
	if keyErr == nil && activeKey != nil {
		kpis["active_key_version"] = activeKey.Version
		if activeKey.ActivatedAt != nil {
			kpis["active_key_days"] = int(time.Since(*activeKey.ActivatedAt).Hours() / 24)
		} else {
			kpis["active_key_days"] = 0
		}
	} else {
		kpis["active_key_version"] = ""
		kpis["active_key_days"] = 0
	}
	allKeys, _ := s.keyRepo.GetAll()
	kpis["key_count"] = len(allKeys)

	contactCount, _ := s.contactRepo.GetTotalContacts()
	kpis["contact_count"] = contactCount

	contactLastSync, _ := s.contactRepo.GetLastSyncTime()
	if contactLastSync != nil {
		kpis["contact_last_sync"] = *contactLastSync
	} else {
		kpis["contact_last_sync"] = nil
	}

	featureIDs := s.cfg.Features.IDs
	now := time.Now()
	totalDays := int(now.Sub(now.AddDate(0, -3, 0)).Hours()/24) + 1
	startTime := now.AddDate(0, -3, 0).Unix()
	users, _ := s.logRepo.GetUsersWithDayStats(featureIDs, startTime, 0, totalDays, totalDays)
	var inactiveRate float64
	if contactCount > 0 {
		inactiveRate = float64(len(users)) / float64(contactCount) * 100
	}
	kpis["inactive_rate"] = inactiveRate
	kpis["inactive_count"] = len(users)
	kpis["total_contacts"] = contactCount

	historyItems, _, _ := s.syncHistoryRepo.List("", 1, 5)
	for _, item := range historyItems {
		recentSyncs = append(recentSyncs, map[string]interface{}{
			"start_time":  item.StartTime,
			"sync_type":   item.SyncType,
			"trigger":     item.Trigger,
			"succeeded":   item.Succeeded,
			"failed":      item.Failed,
			"duration_ms": item.DurationMs,
		})
	}

	if err == nil && latestLog.Failed > 0 {
		problems = append(problems, map[string]interface{}{
			"level":   "error",
			"message": "最近一次同步有 " + strconv.Itoa(latestLog.Failed) + " 条失败记录",
			"action":  "sync",
		})
	}

	if keyErr == nil && activeKey != nil && activeKey.ActivatedAt != nil {
		if time.Since(*activeKey.ActivatedAt) > 90*24*time.Hour {
			problems = append(problems, map[string]interface{}{
				"level":   "warning",
				"message": "当前密钥版本已使用超过 90 天，建议轮换",
				"action":  "keys",
			})
		}
	}

	latestContact, contactErr := s.syncHistoryRepo.GetLatest("contact")
	if contactErr != nil || time.Since(latestContact.StartTime) > 7*24*time.Hour {
		problems = append(problems, map[string]interface{}{
			"level":   "warning",
			"message": "通讯录超过 7 天未同步",
			"action":  "contacts",
		})
	}

	hasRecent := false
	for _, st := range states {
		if st.LastLogTime > time.Now().Add(-24*time.Hour).Unix() {
			hasRecent = true
			break
		}
	}
	if !hasRecent && len(states) > 0 {
		problems = append(problems, map[string]interface{}{
			"level":   "warning",
			"message": "最近 24 小时无新日志入库",
			"action":  "sync",
		})
	}

	return map[string]interface{}{
		"kpis":         kpis,
		"recent_syncs": recentSyncs,
		"problems":     problems,
	}, nil
}

func (s *DashboardService) GetInactiveUsers(rangeParam string, deptID int, minInactiveDays int) (map[string]interface{}, error) {
	featureIDs := s.cfg.Features.IDs

	now := time.Now()
	var startTime int64
	var totalDays int

	switch rangeParam {
	case "week":
		startTime = now.AddDate(0, 0, -7).Unix()
		totalDays = 7
	case "month":
		loc := now.Location()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		startTime = monthStart.Unix()
		totalDays = int(now.Sub(monthStart).Hours()/24) + 1
	default:
		totalDays = int(now.Sub(now.AddDate(0, -3, 0)).Hours()/24) + 1
		startTime = now.AddDate(0, -3, 0).Unix()
	}

	if minInactiveDays <= 0 {
		minInactiveDays = totalDays
	}

	users, err := s.logRepo.GetUsersWithDayStats(featureIDs, startTime, deptID, totalDays, minInactiveDays)
	if err != nil {
		return nil, err
	}

	totalContacts, _ := s.contactRepo.GetCountByDeptID(deptID)

	featureNames := make(map[int]string)
	validIDs := make(map[int]bool)
	for _, id := range s.cfg.Features.IDs {
		validIDs[id] = true
	}
	for fid, name := range s.cfg.Features.Names {
		if validIDs[fid] {
			featureNames[fid] = name
		}
	}

	depts, _ := s.contactRepo.GetAllDepartments()

	deptCounts, _ := s.contactRepo.GetDeptMemberCounts()
	deptCountMap := make(map[int]int64, len(deptCounts))
	for _, dc := range deptCounts {
		deptCountMap[dc.DeptID] = dc.Count
	}

	inactiveByDept := make(map[int]int)
	for _, u := range users {
		var userDepts []int
		json.Unmarshal([]byte(u.Department), &userDepts)
		for _, ud := range userDepts {
			inactiveByDept[ud]++
		}
	}

	deptStats := make([]map[string]interface{}, 0, len(depts))
	for _, d := range depts {
		total := deptCountMap[d.ID]
		if total == 0 {
			continue
		}
		inactive := inactiveByDept[d.ID]
		deptStats = append(deptStats, map[string]interface{}{
			"id":       d.ID,
			"name":     d.Name,
			"total":    total,
			"active":   total - int64(inactive),
			"inactive": inactive,
		})
	}

	deptList := make([]map[string]interface{}, 0, len(depts))
	for _, d := range depts {
		deptList = append(deptList, map[string]interface{}{
			"id":   d.ID,
			"name": d.Name,
		})
	}

	neverActiveCount := 0
	for _, u := range users {
		if u.ActiveDays == 0 {
			neverActiveCount++
		}
	}

	return map[string]interface{}{
		"total_contacts":    totalContacts,
		"inactive_count":    neverActiveCount,
		"filtered_count":    len(users),
		"inactive_users":    users,
		"feature_names":     featureNames,
		"departments":       deptList,
		"dept_stats":        deptStats,
		"range":             rangeParam,
		"total_days":        totalDays,
		"min_inactive_days": minInactiveDays,
	}, nil
}
