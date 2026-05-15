package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/handler"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/internal/router"
	"wwlocal-wework/internal/service"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get sql.DB failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	keyRepo := repository.NewKeyRepository(db, cfg.Keys.StoragePath, cfg.Keys.EncryptKey)
	if err := keyRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate key repository failed: %v", err)
	}
	checkKeyPermissions(cfg.Keys.StoragePath)

	logRepo := repository.NewLogRepository(db)

	syncStateRepo := repository.NewSyncStateRepository(db)
	if err := syncStateRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate sync state repository failed: %v", err)
	}

	contactRepo := repository.NewContactRepository(db)
	if err := contactRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate contact repository failed: %v", err)
	}

	syncFeatureRepo := repository.NewSyncFeatureRepository(db)
	if err := syncFeatureRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate sync feature repository failed: %v", err)
	}
	// 从 config.yaml 初始化 feature 列表到数据库
	var initFeatures []model.SyncFeature
	for _, id := range cfg.Features.IDs {
		initFeatures = append(initFeatures, model.SyncFeature{
			FeatureID: id,
			Name:      cfg.Features.Names[id],
			Enabled:   true,
		})
	}
	if err := syncFeatureRepo.BatchUpsert(initFeatures); err != nil {
		log.Fatalf("init sync features failed: %v", err)
	}

	syncHistoryRepo := repository.NewSyncHistoryRepository(db)
	if err := syncHistoryRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate sync history repository failed: %v", err)
	}

	weworkSvc := service.NewWeWorkService(&cfg.WeWork)
	decryptSvc := service.NewDecryptService(keyRepo)
	syncSvc := service.NewSyncService(weworkSvc, decryptSvc, logRepo, keyRepo, syncStateRepo, syncHistoryRepo, syncFeatureRepo, cfg)
	querySvc := service.NewQueryService(logRepo, contactRepo, weworkSvc, decryptSvc, syncFeatureRepo, cfg)
	keySvc := service.NewKeyService(keyRepo)

	// 启动时校验 sync_state 与实际数据是否一致
	syncSvc.VerifySyncState()

	healthHandler := handler.NewHealthHandler(db, cfg)
	authHandler := handler.NewAuthHandler(&cfg.Auth)
	logHandler := handler.NewLogHandler(querySvc)
	keyHandler := handler.NewKeyHandler(keySvc)
	syncHandler := handler.NewSyncHandler(syncSvc)

	interval := time.Hour
	if cfg.Scheduler.Interval != "" {
		if d, err := time.ParseDuration(cfg.Scheduler.Interval); err == nil {
			interval = d
		}
	}
	schedulerSvc := service.NewSchedulerService(syncSvc, interval)
	schedulerHandler := handler.NewSchedulerHandler(schedulerSvc, syncSvc)

	contactSvc := service.NewContactService(&cfg.WeWork)
	contactSyncSvc := service.NewContactSyncService(contactSvc, contactRepo, syncHistoryRepo)
	contactHandler := handler.NewContactHandler(contactSyncSvc, contactRepo)

	operationLogRepo := repository.NewOperationLogRepository(db)
	if err := operationLogRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate operation log repository failed: %v", err)
	}
	operationLogSvc := service.NewOperationLogService(operationLogRepo)
	operationLogHandler := handler.NewOperationLogHandler(operationLogSvc)

	adminOperLogRepo := repository.NewAdminOperLogRepository(db)
	if err := adminOperLogRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate admin oper log repository failed: %v", err)
	}
	adminOperLogSvc := service.NewAdminOperLogService(weworkSvc, adminOperLogRepo, &cfg.WeWork)
	adminOperLogHandler := handler.NewAdminOperLogHandler(adminOperLogSvc)

	dashboardHandler := handler.NewDashboardHandler(logRepo, contactRepo, syncHistoryRepo, syncStateRepo, keyRepo, cfg)
	syncHistoryHandler := handler.NewSyncHistoryHandler(syncHistoryRepo)
	syncFeatureHandler := handler.NewSyncFeatureHandler(syncFeatureRepo)
	systemHandler := handler.NewSystemHandler(syncStateRepo, keyRepo, contactRepo, logRepo)

	// 初始化任务队列服务
	taskQueueSvc, err := service.NewTaskQueueService(cfg, syncSvc, contactSyncSvc, adminOperLogSvc)
	if err != nil {
		log.Printf("init task queue service failed: %v", err)
	}
	taskHandler := handler.NewTaskHandler(taskQueueSvc)

	if cfg.Scheduler.Enabled {
		schedulerSvc.Start(interval)
	}

	// 启动任务队列工作线程
	taskQueueSvc.Start()

	r := router.NewRouter(healthHandler, authHandler, logHandler, keyHandler, syncHandler, schedulerHandler, contactHandler, operationLogHandler, adminOperLogHandler, dashboardHandler, syncHistoryHandler, syncFeatureHandler, systemHandler, taskHandler, operationLogSvc, cfg.Auth.JWTSecret)

	e := echo.New()
	r.Setup(e)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("server starting on %s", addr)

	go func() {
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("start server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("received signal %v, shutting down...", sig)

	// 1. 停止定时调度
	schedulerSvc.Stop()

	// 2. 停止任务队列
	taskQueueSvc.Stop()

	// 3. 取消正在进行的同步
	syncSvc.Cancel()

	// 4. 优雅关闭 HTTP（等待在途请求完成，最多 30 秒）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	// 5. 关闭数据库连接
	sqlDB, _ = db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	log.Println("server stopped")
}

func checkKeyPermissions(keysDir string) {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		log.Printf("keys directory not readable: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".pem" {
			continue
		}
		path := filepath.Join(keysDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			log.Printf("WARNING: key file %s has overly permissive permissions (%o), fixing to 600", path, perm)
			os.Chmod(path, 0600)
		}
	}
	// 递归检查子目录（如 keys/v1/）
	subdirs, _ := os.ReadDir(keysDir)
	for _, sub := range subdirs {
		if sub.IsDir() {
			checkKeyPermissions(filepath.Join(keysDir, sub.Name()))
		}
	}
}