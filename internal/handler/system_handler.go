package handler

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/response"
)

type SystemHandler struct {
	syncStateRepo systemSyncStateRepository
	keyRepo       systemKeyRepository
	contactRepo   systemContactRepository
	logRepo       systemLogRepository
	startTime     time.Time
}

type systemSyncStateRepository interface {
	Ping() error
	GetAll() ([]model.SyncState, error)
}

type systemKeyRepository interface {
	GetActive() (*model.RSAKeyVersion, error)
	GetAll() ([]model.RSAKeyVersion, error)
}

type systemContactRepository interface {
	GetTotalContacts() (int64, error)
	GetLastSyncTime() (*time.Time, error)
}

type systemLogRepository interface {
	GetTableSizes(limit int) ([]repository.TableSizeInfo, error)
	GetSchemaQuality() ([]repository.SchemaQualityInfo, error)
}

func NewSystemHandler(syncStateRepo *repository.SyncStateRepository, keyRepo *repository.KeyRepository, contactRepo *repository.ContactRepository, logRepo *repository.LogRepository) *SystemHandler {
	return &SystemHandler{
		syncStateRepo: syncStateRepo,
		keyRepo:       keyRepo,
		contactRepo:   contactRepo,
		logRepo:       logRepo,
		startTime:     time.Now(),
	}
}

func (h *SystemHandler) GetStatus(c echo.Context) error {
	status := make(map[string]interface{})

	health := make(map[string]interface{})
	if err := h.syncStateRepo.Ping(); err != nil {
		health["db_connected"] = false
	} else {
		health["db_connected"] = true
	}
	health["uptime_seconds"] = int(time.Since(h.startTime).Seconds())

	coverage := make(map[string]interface{})
	states, err := h.syncStateRepo.GetAll()
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("查询同步覆盖失败: %v", err))
	}
	for _, s := range states {
		info := map[string]interface{}{
			"last_log_time": s.LastLogTime,
			"last_sync_at":  s.LastSyncAt,
			"total_synced":  s.TotalSynced,
		}
		if s.LastLogTime > 0 {
			age := int(time.Since(time.Unix(s.LastLogTime, 0)).Hours())
			if age < 0 {
				age = 0
			}
			info["data_age_hours"] = age
		}
		coverage[strconv.Itoa(s.FeatureID)] = info
	}

	tables, err := h.logRepo.GetTableSizes(20)
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("查询表空间失败: %v", err))
	}
	tableSizes := make([]map[string]interface{}, 0, len(tables))
	for _, t := range tables {
		tableSizes = append(tableSizes, map[string]interface{}{
			"table":       t.TableName,
			"rows":        t.RowCount,
			"data_bytes":  t.DataSize,
			"index_bytes": t.IndexSize,
		})
	}
	schemaQuality, schemaQualityErr := h.logRepo.GetSchemaQuality()

	keyStatus := make(map[string]interface{})
	activeKey, err := h.keyRepo.GetActive()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, 500, fmt.Sprintf("查询激活密钥失败: %v", err))
		}
		activeKey = nil
	}
	if activeKey != nil {
		keyStatus["active_version"] = activeKey.Version
		if activeKey.ActivatedAt != nil {
			keyStatus["active_days"] = int(time.Since(*activeKey.ActivatedAt).Hours() / 24)
		}
	}
	allKeys, err := h.keyRepo.GetAll()
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("查询密钥列表失败: %v", err))
	}
	keyStatus["total_keys"] = len(allKeys)

	contactCount, err := h.contactRepo.GetTotalContacts()
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("查询通讯录人数失败: %v", err))
	}
	contactLastSync, err := h.contactRepo.GetLastSyncTime()
	if err != nil {
		return response.Error(c, 500, fmt.Sprintf("查询通讯录同步时间失败: %v", err))
	}
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
	status["schema_quality"] = schemaQuality
	if schemaQualityErr != nil {
		status["schema_quality_error"] = schemaQualityErr.Error()
	}
	status["key_status"] = keyStatus
	status["contacts"] = contactInfo

	return response.Success(c, status)
}
