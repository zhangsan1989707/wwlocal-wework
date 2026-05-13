package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/handler"
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

	logRepo := repository.NewLogRepository(db)

	syncStateRepo := repository.NewSyncStateRepository(db)
	if err := syncStateRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate sync state repository failed: %v", err)
	}

	contactRepo := repository.NewContactRepository(db)
	if err := contactRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate contact repository failed: %v", err)
	}

	weworkSvc := service.NewWeWorkService(&cfg.WeWork)
	decryptSvc := service.NewDecryptService(keyRepo)
	syncSvc := service.NewSyncService(weworkSvc, decryptSvc, logRepo, keyRepo, syncStateRepo, cfg)
	querySvc := service.NewQueryService(logRepo, contactRepo, weworkSvc, decryptSvc, cfg)
	keySvc := service.NewKeyService(keyRepo)

	// 启动时校验 sync_state 与实际数据是否一致
	syncSvc.VerifySyncState()

	healthHandler := handler.NewHealthHandler()
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
	contactSyncSvc := service.NewContactSyncService(contactSvc, contactRepo)
	contactHandler := handler.NewContactHandler(contactSyncSvc, contactRepo)

	operationLogRepo := repository.NewOperationLogRepository(db)
	if err := operationLogRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate operation log repository failed: %v", err)
	}
	operationLogSvc := service.NewOperationLogService(operationLogRepo)
	operationLogHandler := handler.NewOperationLogHandler(operationLogSvc)

	dashboardHandler := handler.NewDashboardHandler(db, logRepo, contactRepo, cfg)

	if cfg.Scheduler.Enabled {
		schedulerSvc.Start()
	}

	r := router.NewRouter(healthHandler, authHandler, logHandler, keyHandler, syncHandler, schedulerHandler, contactHandler, operationLogHandler, dashboardHandler, operationLogSvc, cfg.Auth.JWTSecret)

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

	// 2. 取消正在进行的同步
	syncSvc.Cancel()

	// 3. 优雅关闭 HTTP（等待在途请求完成，最多 30 秒）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	// 4. 关闭数据库连接
	sqlDB, _ = db.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}

	log.Println("server stopped")
}