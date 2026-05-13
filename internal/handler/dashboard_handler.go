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
	db          *gorm.DB
	logRepo     *repository.LogRepository
	contactRepo *repository.ContactRepository
	cfg         *config.Config
}

func NewDashboardHandler(db *gorm.DB, logRepo *repository.LogRepository, contactRepo *repository.ContactRepository, cfg *config.Config) *DashboardHandler {
	return &DashboardHandler{db: db, logRepo: logRepo, contactRepo: contactRepo, cfg: cfg}
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
	var months []time.Time
	var startTime int64
	var totalDays int

	switch rangeParam {
	case "week":
		months = []time.Time{time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())}
		startTime = now.AddDate(0, 0, -7).Unix()
		totalDays = 7
	case "month":
		months = []time.Time{time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())}
		totalDays = now.Day()
	default: // quarter
		for i := 0; i < 3; i++ {
			m := now.AddDate(0, -i, 0)
			months = append(months, time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, m.Location()))
		}
		// 季度天数：从3个月前的今天到今天
		totalDays = int(now.Sub(now.AddDate(0, -3, 0)).Hours()/24) + 1
	}

	existingTables := h.logRepo.GetExistingLogTables(featureIDs, months)

	// 如果设置了阈值，用阈值查询；否则用天数统计查询（默认阈值为 totalDays 即完全无记录）
	if minInactiveDays <= 0 {
		minInactiveDays = totalDays // 默认：完全无记录
	}
	users, err := h.logRepo.GetUsersWithDayStats(featureIDs, months, existingTables, startTime, deptID, totalDays, minInactiveDays)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"code": 500,
			"msg":  "查询失败: " + err.Error(),
		})
	}

	// 总联系人数（包含所有状态，与政务微信管理后台一致）
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

	monthStrs := make([]string, len(months))
	for i, m := range months {
		monthStrs[i] = m.Format("2006-01")
	}

	depts, _ := h.contactRepo.GetAllDepartments()
	deptList := make([]map[string]interface{}, 0, len(depts))
	for _, d := range depts {
		deptList = append(deptList, map[string]interface{}{
			"id":   d.ID,
			"name": d.Name,
		})
	}

	// 部门统计：每个部门的总人数和未达标人数
	deptStats := make([]map[string]interface{}, 0, len(depts))
	for _, d := range depts {
		var deptTotal int64
		h.db.Table("contacts").Where("JSON_CONTAINS(department, ?)", strconv.Itoa(d.ID)).Count(&deptTotal)
		if deptTotal == 0 {
			continue
		}
		// 统计该部门未达标人数
		deptInactive := 0
		for _, u := range users {
			var userDepts []int
			json.Unmarshal([]byte(u.Department), &userDepts)
			for _, ud := range userDepts {
				if ud == d.ID {
					deptInactive++
					break
				}
			}
		}
		deptStats = append(deptStats, map[string]interface{}{
			"id":            d.ID,
			"name":          d.Name,
			"total":         deptTotal,
			"active":        deptTotal - int64(deptInactive),
			"inactive":      deptInactive,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": map[string]interface{}{
			"total_contacts":    totalContacts,
			"inactive_count":    len(users),
			"inactive_users":    users,
			"feature_names":     featureNames,
			"months":            monthStrs,
			"departments":       deptList,
			"dept_stats":        deptStats,
			"range":             rangeParam,
			"total_days":        totalDays,
			"min_inactive_days": minInactiveDays,
		},
	})
}
