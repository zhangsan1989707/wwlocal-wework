package router

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"wwlocal-wework/internal/handler"
)

type Router struct {
	healthHandler *handler.HealthHandler
	logHandler    *handler.LogHandler
	keyHandler    *handler.KeyHandler
	syncHandler   *handler.SyncHandler
}

func NewRouter(healthHandler *handler.HealthHandler, logHandler *handler.LogHandler, keyHandler *handler.KeyHandler, syncHandler *handler.SyncHandler) *Router {
	return &Router{
		healthHandler: healthHandler,
		logHandler:    logHandler,
		keyHandler:    keyHandler,
		syncHandler:   syncHandler,
	}
}

func (r *Router) Setup(e *echo.Echo) {
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/health", r.healthHandler.Check)

	api := e.Group("/api/v1")
	{
		logs := api.Group("/logs")
		{
			logs.POST("/query", r.logHandler.Query)
			logs.GET("/features", r.logHandler.GetFeatures)
			logs.GET("/time-range", r.logHandler.GetTimeRange)
		}

		sync := api.Group("/logs")
		{
			sync.POST("/sync", r.syncHandler.Sync)
			sync.GET("/sync/status", r.syncHandler.Status)
		}

		keys := api.Group("/keys")
		{
			keys.GET("", r.keyHandler.List)
			keys.POST("", r.keyHandler.Add)
			keys.PUT("/activate", r.keyHandler.Activate)
		}
	}
}