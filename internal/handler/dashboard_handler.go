package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/repository"
)

type DashboardHandler struct {
	db               *gorm.DB
	logRepo          *repository.LogRepository
	contactRepo      *repository.ContactRepository
	syncHistoryRepo  *repository.SyncHistoryRepository
	syncStateRepo    *repository.SyncStateRepository
	keyRepo          *repository.KeyRepository
	cfg              *config.Config
}

func NewDashboardHandler(db *gorm.DB, logRepo *repository.LogRepository, contactRepo *repository.ContactRepository, syncHistoryRepo *repository.SyncHistoryRepository, syncStateRepo *repository.SyncStateRepository, keyRepo *repository.KeyRepository, cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{db: db, logRepo: logRepo, contactRepo: contactRepo, syncHistoryRepo: syncHistoryRepo, syncStateRepo: syncStateRepo, keyRepo: keyRepo, cfg: cfg}
}

func (h *DashboardHandler) GetOverview(c echo.Context) error {
	kpis := make(map[string]interface{})
	recentSyncs := make([]map[string]interface{}, 0)
	problems := make([]map[string]interface{}, 0)

	// 1. 最新同步时间 & 最新日志时间
	states, _ := h.syncStateRepo.GetAll()
	var latestSyncAt time.Time
	var latestLogTime int64
	for _, s := range states {
		if s.LastSyncAt.After(latestSyncAt) {
			latestSyncAt = s.LastSyncAt
		}
		if s.LastLogTime > latestLogTime {
			latestLogTime = s.LastLogTime
		}
	}
	kpis["latest_sync_time"] = latestSyncAt
	kpis["latest_log_time"] = latestLogTime

	// 2. 近 7 日同步记录数
	since7d := time.Now().AddDate(0, 0, -7)
	_, synced7d, _, _ := h.syncHistoryRepo.GetStats("log", since7d)
	kpis["synced_7d_count"] = synced7d

	// 3. 同步失败数据类型数
	latestLog, err := h.syncHistoryRepo.GetLatest("log")
	if err == nil && latestLog.Failed > 0 {
		kpis["failed_feature_count"] = latestLog.Failed
	} else {
		kpis["failed_feature_count"] = 0
	}

	// 4. 当前激活密钥版本 & 使用天数
	activeKey, keyErr := h.keyRepo.GetActive()
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
	allKeys, _ := h.keyRepo.GetAll()
	kpis["key_count"] = len(allKeys)

	// 5. 通讯录人数 & 上次同步时间
	var contactCount int64
	h.db.Table("contacts").Count(&contactCount)
	kpis["contact_count"] = contactCount

	var contactLastSync *time.Time
	h.db.Table("contacts").Select("MAX(synced_at)").Scan(&contactLastSync)
	if contactLastSync != nil {
		kpis["contact_last_sync"] = *contactLastSync
	} else {
		kpis["contact_last_sync"] = nil
	}

	// 6. 未使用率（简化版，取季度默认值）
	featureIDs := []int{90000031, 90000032}
	now := time.Now()
	totalDays := int(now.Sub(now.AddDate(0, -3, 0)).Hours()/24) + 1
	startTime := now.AddDate(0, -3, 0).Unix()
	users, _ := h.logRepo.GetUsersWithDayStats(featureIDs, startTime, 0, totalDays, totalDays)
	var inactiveRate float64
	if contactCount > 0 {
		inactiveRate = float64(len(users)) / float64(contactCount) * 100
	}
	kpis["inactive_rate"] = inactiveRate
	kpis["inactive_count"] = len(users)
	kpis["total_contacts"] = contactCount

	// 最近 5 条同步任务
	historyItems, _, _ := h.syncHistoryRepo.List("", 1, 5)
	for _, h := range historyItems {
		recentSyncs = append(recentSyncs, map[string]interface{}{
			"start_time": h.StartTime,
			"sync_type":  h.SyncType,
			"trigger":    h.Trigger,
			"succeeded":  h.Succeeded,
			"failed":     h.Failed,
			"duration_ms": h.DurationMs,
		})
	}

	// 问题提醒
	// 1. 最近一次日志同步有失败
	if err == nil && latestLog.Failed > 0 {
		problems = append(problems, map[string]interface{}{
			"level":   "error",
			"message": "最近一次同步有 " + strconv.Itoa(latestLog.Failed) + " 条失败记录",
			"action":  "sync",
		})
	}

	// 2. 密钥使用超过 90 天
	if keyErr == nil && activeKey != nil && activeKey.ActivatedAt != nil {
		if time.Since(*activeKey.ActivatedAt) > 90*24*time.Hour {
			problems = append(problems, map[string]interface{}{
				"level":   "warning",
				"message": "当前密钥版本已使用超过 90 天，建议轮换",
				"action":  "keys",
			})
		}
	}

	// 3. 通讯录超过 7 天未同步
	latestContact, contactErr := h.syncHistoryRepo.GetLatest("contact")
	if contactErr != nil || time.Since(latestContact.StartTime) > 7*24*time.Hour {
		problems = append(problems, map[string]interface{}{
			"level":   "warning",
			"message": "通讯录超过 7 天未同步",
			"action":  "contacts",
		})
	}

	// 4. 最近 24 小时无日志入库
	hasRecent := false
	for _, s := range states {
		if s.LastLogTime > time.Now().Add(-24*time.Hour).Unix() {
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": map[string]interface{}{
			"kpis":         kpis,
			"recent_syncs": recentSyncs,
			"problems":     problems,
		},
	})
}

func (h *DashboardHandler) GetInactiveUsers(c echo.Context) error {
	rangeParam := c.QueryParam("range")
	if rangeParam == "" {
		rangeParam = "quarter"
	}
	deptID, _ := strconv.Atoi(c.QueryParam("dept_id"))
	minInactiveDays, _ := strconv.Atoi(c.QueryParam("min_inactive_days"))

	featureIDs := []int{90000031, 90000032}

	now := time.Now()
	var startTime int64
	var totalDays int

	switch rangeParam {
	case "week":
		startTime = now.AddDate(0, 0, -7).Unix()
		totalDays = 7
	case "month":
		totalDays = now.Day()
	default: // quarter
		totalDays = int(now.Sub(now.AddDate(0, -3, 0)).Hours()/24) + 1
	}

	if minInactiveDays <= 0 {
		minInactiveDays = totalDays
	}

	users, err := h.logRepo.GetUsersWithDayStats(featureIDs, startTime, deptID, totalDays, minInactiveDays)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code": 500,
			"msg":  "查询失败: " + err.Error(),
		})
	}

	var totalContacts int64
	countQuery := h.db.Table("contacts")
	if deptID > 0 {
		countQuery = countQuery.Where("JSON_CONTAINS(department, ?)", strconv.Itoa(deptID))
	}
	countQuery.Count(&totalContacts)

	featureNames := make(map[int]string)
	for fid, name := range h.cfg.Features.Names {
		if fid == 90000031 || fid == 90000032 {
			featureNames[fid] = name
		}
	}

	depts, _ := h.contactRepo.GetAllDepartments()

	// 部门总人数：一次 GROUP BY 查询
	var deptCounts []struct {
		DeptID int
		Count  int64
	}
	h.db.Raw(`
		SELECT d.id AS dept_id, COUNT(c.user_id) AS count
		FROM departments d
		LEFT JOIN contacts c ON JSON_CONTAINS(c.department, CAST(d.id AS CHAR))
		GROUP BY d.id
	`).Scan(&deptCounts)
	deptCountMap := make(map[int]int64, len(deptCounts))
	for _, dc := range deptCounts {
		deptCountMap[dc.DeptID] = dc.Count
	}

	// 部门未达标人数：从已查出的 users 在内存中统计
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

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": map[string]interface{}{
			"total_contacts":    totalContacts,
			"inactive_count":    len(users),
			"inactive_users":    users,
			"feature_names":     featureNames,
			"departments":       deptList,
			"dept_stats":        deptStats,
			"range":             rangeParam,
			"total_days":        totalDays,
			"min_inactive_days": minInactiveDays,
		},
	})
}
