package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/internal/repository"
)

type SystemHandler struct {
	db          *gorm.DB
	syncStateRepo *repository.SyncStateRepository
	keyRepo       *repository.KeyRepository
	contactRepo   *repository.ContactRepository
	startTime     time.Time
}

func NewSystemHandler(db *gorm.DB, syncStateRepo *repository.SyncStateRepository, keyRepo *repository.KeyRepository, contactRepo *repository.ContactRepository) *SystemHandler {
	return &SystemHandler{
		db:            db,
		syncStateRepo: syncStateRepo,
		keyRepo:       keyRepo,
		contactRepo:   contactRepo,
		startTime:     time.Now(),
	}
}

func (h *SystemHandler) GetStatus(c echo.Context) error {
	status := make(map[string]interface{})

	// 1. 系统健康
	health := make(map[string]interface{})
	sqlDB, _ := h.db.DB()
	dbErr := sqlDB.Ping()
	health["db_connected"] = dbErr == nil
	health["uptime_seconds"] = int(time.Since(h.startTime).Seconds())

	// 2. 同步覆盖
	coverage := make(map[string]interface{})
	states, _ := h.syncStateRepo.GetAll()
	for _, s := range states {
		info := map[string]interface{}{
			"last_log_time": s.LastLogTime,
			"last_sync_at":  s.LastSyncAt,
			"total_synced":  s.TotalSynced,
		}
		if s.LastLogTime > 0 {
			info["data_age_hours"] = int(time.Since(time.Unix(s.LastLogTime, 0)).Hours())
		}
		coverage[string(rune('0'+s.FeatureID))] = info
	}

	// 3. 数据库表大小
	tableSizes := make([]map[string]interface{}, 0)
	var tables []struct {
		TableName string `gorm:"column:TABLE_NAME"`
		RowCount  int64  `gorm:"column:TABLE_ROWS"`
		DataSize  string `gorm:"column:DATA_LENGTH"`
		IndexSize string `gorm:"column:INDEX_LENGTH"`
	}
	h.db.Raw(`
		SELECT TABLE_NAME, TABLE_ROWS, DATA_LENGTH, INDEX_LENGTH
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME LIKE 'log_%'
		ORDER BY TABLE_ROWS DESC
		LIMIT 20
	`).Scan(&tables)
	for _, t := range tables {
		tableSizes = append(tableSizes, map[string]interface{}{
			"table":      t.TableName,
			"rows":       t.RowCount,
			"data_bytes": t.DataSize,
			"index_bytes": t.IndexSize,
		})
	}

	// 4. 密钥状态
	keyStatus := make(map[string]interface{})
	activeKey, _ := h.keyRepo.GetActive()
	if activeKey != nil {
		keyStatus["active_version"] = activeKey.Version
		if activeKey.ActivatedAt != nil {
			keyStatus["active_days"] = int(time.Since(*activeKey.ActivatedAt).Hours() / 24)
		}
	}
	allKeys, _ := h.keyRepo.GetAll()
	keyStatus["total_keys"] = len(allKeys)

	// 5. 通讯录
	var contactCount int64
	h.db.Table("contacts").Count(&contactCount)
	var contactLastSync *time.Time
	h.db.Table("contacts").Select("MAX(synced_at)").Scan(&contactLastSync)
	contactInfo := map[string]interface{}{
		"total": contactCount,
	}
	if contactLastSync != nil {
		contactInfo["last_sync"] = *contactLastSync
		contactInfo["sync_age_hours"] = int(time.Since(*contactLastSync).Hours())
	}

	status["health"] = health
	status["sync_coverage"] = coverage
	status["table_sizes"] = tableSizes
	status["key_status"] = keyStatus
	status["contacts"] = contactInfo

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": status,
	})
}
