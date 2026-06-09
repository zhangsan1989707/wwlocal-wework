package main

import (
	"context"
	"fmt"
	"log/slog"
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
	appmw "wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/internal/router"
	"wwlocal-wework/internal/service"
)

func main() {
	// 配置结构化日志
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error(fmt.Sprintf("load config failed: %v", err))
		os.Exit(1)
	}

	db, err := gorm.Open(mysql.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		slog.Error(fmt.Sprintf("connect database failed: %v", err))
		os.Exit(1)
	}

	sqlDB, err := db.DB()
	if err != nil {
		slog.Error(fmt.Sprintf("get sql.DB failed: %v", err))
		os.Exit(1)
	}
	maxOpen := cfg.Database.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxIdle := cfg.Database.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 25
	}
	connLifetime := cfg.Database.ConnMaxLifetime
	if connLifetime <= 0 {
		connLifetime = 5 * time.Minute
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(connLifetime)

	settingRepo := repository.NewSettingRepository(db)
	if err := settingRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate setting repository failed: %v", err))
		os.Exit(1)
	}

	keyRepo := repository.NewKeyRepository(db, cfg.Keys.StoragePath, cfg.Keys.EncryptKey)
	if err := keyRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate key repository failed: %v", err))
		os.Exit(1)
	}
	checkKeyPermissions(cfg.Keys.StoragePath)

	logRepo := repository.NewLogRepository(db)

	syncStateRepo := repository.NewSyncStateRepository(db)
	if err := syncStateRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate sync state repository failed: %v", err))
		os.Exit(1)
	}

	contactRepo := repository.NewContactRepository(db)
	if err := contactRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate contact repository failed: %v", err))
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	if err := userRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate user repository failed: %v", err))
		os.Exit(1)
	}
	userSvc := service.NewUserService(userRepo, contactRepo, &cfg.Auth)
	if err := userSvc.EnsureInitialAdmin(); err != nil {
		slog.Error(fmt.Sprintf("init admin user failed: %v", err))
		os.Exit(1)
	}

	syncFeatureRepo := repository.NewSyncFeatureRepository(db)
	if err := syncFeatureRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate sync feature repository failed: %v", err))
		os.Exit(1)
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
		slog.Error(fmt.Sprintf("init sync features failed: %v", err))
		os.Exit(1)
	}

	syncHistoryRepo := repository.NewSyncHistoryRepository(db)
	if err := syncHistoryRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate sync history repository failed: %v", err))
		os.Exit(1)
	}

	weworkSvc := service.NewWeWorkService(&cfg.WeWork)
	decryptSvc := service.NewDecryptService(keyRepo)
	syncSvc := service.NewSyncService(weworkSvc, decryptSvc, logRepo, keyRepo, syncStateRepo, syncHistoryRepo, syncFeatureRepo, cfg)
	querySvc := service.NewQueryService(logRepo, contactRepo, weworkSvc, decryptSvc, syncFeatureRepo, cfg)
	behaviorQuerySvc := service.NewBehaviorQueryService(logRepo, syncFeatureRepo, contactRepo, cfg)
	keySvc := service.NewKeyService(keyRepo)

	// 启动时校验 sync_state 与实际数据是否一致
	syncSvc.VerifySyncState()

	healthHandler := handler.NewHealthHandler(db, cfg)
	authHandler := handler.NewAuthHandler(&cfg.Auth, userSvc)
	userHandler := handler.NewUserHandler(userSvc)
	logHandler := handler.NewLogHandler(querySvc, userSvc)
	behaviorQueryHandler := handler.NewBehaviorQueryHandler(behaviorQuerySvc, userSvc)
	keyHandler := handler.NewKeyHandler(keySvc)
	syncHandler := handler.NewSyncHandler(syncSvc)

	contactSvc := service.NewContactService(&cfg.WeWork)
	contactSyncSvc := service.NewContactSyncService(contactSvc, contactRepo, syncHistoryRepo)
	contactHandler := handler.NewContactHandler(contactSyncSvc, contactRepo)

	operationLogRepo := repository.NewOperationLogRepository(db)
	if err := operationLogRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate operation log repository failed: %v", err))
		os.Exit(1)
	}
	operationLogSvc := service.NewOperationLogService(operationLogRepo)
	operationLogHandler := handler.NewOperationLogHandler(operationLogSvc)

	adminOperLogRepo := repository.NewAdminOperLogRepository(db)
	if err := adminOperLogRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate admin oper log repository failed: %v", err))
		os.Exit(1)
	}
	adminOperLogSvc := service.NewAdminOperLogService(weworkSvc, adminOperLogRepo, &cfg.WeWork)
	adminOperLogHandler := handler.NewAdminOperLogHandler(adminOperLogSvc)

	interval := time.Hour
	if cfg.Scheduler.Interval != "" {
		if d, err := time.ParseDuration(cfg.Scheduler.Interval); err == nil {
			interval = d
		}
	}
	schedulerSvc := service.NewSchedulerService(syncSvc, adminOperLogSvc, interval)
	schedulerHandler := handler.NewSchedulerHandler(schedulerSvc, syncSvc)

	dashboardSvc := service.NewDashboardService(logRepo, contactRepo, syncHistoryRepo, syncStateRepo, keyRepo, cfg)
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc)

	dashboardStatsRepo := repository.NewDashboardStatsRepository(db)
	if err := dashboardStatsRepo.AutoMigrate(); err != nil {
		slog.Error(fmt.Sprintf("migrate dashboard stats repository failed: %v", err))
		os.Exit(1)
	}
	dashboardV2Svc := service.NewDashboardV2Service(dashboardStatsRepo, contactRepo, cfg)
	dashboardV2Handler := handler.NewDashboardV2Handler(dashboardV2Svc, userSvc)
	nightlySvc := service.NewNightlyJobService(syncSvc, contactSyncSvc, dashboardStatsRepo, contactRepo, logRepo, cfg)
	nightlyHandler := handler.NewNightlyHandler(nightlySvc)
	syncHistoryHandler := handler.NewSyncHistoryHandler(syncHistoryRepo)
	syncFeatureHandler := handler.NewSyncFeatureHandler(syncFeatureRepo)
	systemHandler := handler.NewSystemHandler(syncStateRepo, keyRepo, contactRepo, logRepo)

	// 初始化任务队列服务
	taskQueueSvc, err := service.NewTaskQueueService(cfg, syncSvc, contactSyncSvc, adminOperLogSvc)
	if err != nil {
		slog.Error(fmt.Sprintf("init task queue service failed: %v", err))
	}
	taskHandler := handler.NewTaskHandler(taskQueueSvc)

	rateLimiter := appmw.NewRateLimiter(cfg.RateLimit.RequestsPerMin, cfg.RateLimit.Burst)

	if cfg.Scheduler.Enabled {
		schedulerSvc.Start(interval)
	}

	if cfg.Nightly.Enabled {
		nightlySvc.Start()
	}

	// 启动任务队列工作线程
	taskQueueSvc.Start()

	allowedOrigins := cfg.Server.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"http://localhost:18073", "http://127.0.0.1:18073"}
	}
	metricsIPs := []string{"127.0.0.1", "::1"}

	r := router.NewRouter(router.RouterDeps{
		Health:       healthHandler,
		Auth:         authHandler,
		Log:          logHandler,
		Behavior:     behaviorQueryHandler,
		Key:          keyHandler,
		Sync:         syncHandler,
		Scheduler:    schedulerHandler,
		Contact:      contactHandler,
		OperationLog: operationLogHandler,
		AdminOperLog: adminOperLogHandler,
		Dashboard:    dashboardHandler,
		DashboardV2:  dashboardV2Handler,
		Nightly:      nightlyHandler,
		SyncHistory:  syncHistoryHandler,
		SyncFeature:  syncFeatureHandler,
		System:       systemHandler,
		Task:         taskHandler,
		User:         userHandler,
		OperationSvc: operationLogSvc,
		JWTSecret:    cfg.Auth.JWTSecret,
		RateLimiter:  rateLimiter,
		Origins:      allowedOrigins,
		MetricsIPs:   metricsIPs,
	})

	e := echo.New()
	r.Setup(e)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	slog.Info(fmt.Sprintf("server starting on %s", addr))

	go func() {
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error(fmt.Sprintf("start server failed: %v", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info(fmt.Sprintf("received signal %v, shutting down...", sig))

	// 二次信号处理：收到第二个信号时强制退出
	go func() {
		sig2 := <-quit
		slog.Info(fmt.Sprintf("received second signal %v, forcing shutdown", sig2))
		os.Exit(1)
	}()

	// 1. 停止定时调度
	schedulerSvc.Stop()
	nightlySvc.Stop()

	// 2. 停止任务队列
	taskQueueSvc.Stop()

	// 3. 取消正在进行的同步
	syncSvc.Cancel()

	// 4. 优雅关闭 HTTP（等待在途请求完成，最多 30 秒）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		slog.Error(fmt.Sprintf("shutdown error: %v", err))
	}

	// 5. 停止后台 goroutine
	authHandler.Stop()

	// 6. 关闭数据库连接
	if sqlDB != nil {
		sqlDB.Close()
	}

	slog.Info("server stopped")
}

func checkKeyPermissions(keysDir string) {
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		slog.Warn(fmt.Sprintf("keys directory not readable: %v", err))
		return
	}
	for _, entry := range entries {
		path := filepath.Join(keysDir, entry.Name())
		if entry.IsDir() {
			checkKeyPermissions(path)
			continue
		}
		if filepath.Ext(entry.Name()) != ".pem" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			slog.Warn(fmt.Sprintf("key file %s has overly permissive permissions (%o), fixing to 600", path, perm))
			os.Chmod(path, 0600)
		}
	}
}
