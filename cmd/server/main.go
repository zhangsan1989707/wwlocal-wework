package main

import (
	"fmt"
	"log"

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

	if err := db.AutoMigrate(&model.RSAKeyVersion{}); err != nil {
		log.Fatalf("migrate database failed: %v", err)
	}

	keyRepo := repository.NewKeyRepository(db, cfg.Keys.StoragePath)
	if err := keyRepo.AutoMigrate(); err != nil {
		log.Fatalf("migrate key repository failed: %v", err)
	}

	logRepo := repository.NewLogRepository(db)

	weworkSvc := service.NewWeWorkService(&cfg.WeWork)
	decryptSvc := service.NewDecryptService(keyRepo)
	syncSvc := service.NewSyncService(weworkSvc, decryptSvc, logRepo, keyRepo, cfg)
	querySvc := service.NewQueryService(logRepo, weworkSvc, decryptSvc, cfg)

	healthHandler := handler.NewHealthHandler()
	logHandler := handler.NewLogHandler(querySvc)
	keyHandler := handler.NewKeyHandler(keyRepo)
	syncHandler := handler.NewSyncHandler(syncSvc)

	r := router.NewRouter(healthHandler, logHandler, keyHandler, syncHandler)

	e := echo.New()
	r.Setup(e)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("server starting on %s", addr)
	if err := e.Start(addr); err != nil {
		log.Fatalf("start server failed: %v", err)
	}
}