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
