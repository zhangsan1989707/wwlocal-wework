package router

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"wwlocal-wework/internal/handler"
	appmw "wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/service"
)

type Router struct {
	healthHandler        *handler.HealthHandler
	authHandler          *handler.AuthHandler
	logHandler           *handler.LogHandler
	keyHandler           *handler.KeyHandler
	syncHandler          *handler.SyncHandler
	schedulerHandler     *handler.SchedulerHandler
	contactHandler       *handler.ContactHandler
	operationLogHandler  *handler.OperationLogHandler
	adminOperLogHandler *handler.AdminOperLogHandler
	dashboardHandler     *handler.DashboardHandler
	syncHistoryHandler   *handler.SyncHistoryHandler
	syncFeatureHandler   *handler.SyncFeatureHandler
	systemHandler        *handler.SystemHandler
	taskHandler          *handler.TaskHandler
	operationLogSvc      *service.OperationLogService
	jwtSecret            string
}

func NewRouter(healthHandler *handler.HealthHandler, authHandler *handler.AuthHandler, logHandler *handler.LogHandler, keyHandler *handler.KeyHandler, syncHandler *handler.SyncHandler, schedulerHandler *handler.SchedulerHandler, contactHandler *handler.ContactHandler, operationLogHandler *handler.OperationLogHandler, adminOperLogHandler *handler.AdminOperLogHandler, dashboardHandler *handler.DashboardHandler, syncHistoryHandler *handler.SyncHistoryHandler, syncFeatureHandler *handler.SyncFeatureHandler, systemHandler *handler.SystemHandler, taskHandler *handler.TaskHandler, operationLogSvc *service.OperationLogService, jwtSecret string) *Router {
	return &Router{
		healthHandler:        healthHandler,
		authHandler:          authHandler,
		logHandler:           logHandler,
		keyHandler:           keyHandler,
		syncHandler:          syncHandler,
		schedulerHandler:     schedulerHandler,
		contactHandler:      contactHandler,
		operationLogHandler: operationLogHandler,
		adminOperLogHandler: adminOperLogHandler,
		dashboardHandler:    dashboardHandler,
		syncHistoryHandler:  syncHistoryHandler,
		syncFeatureHandler:  syncFeatureHandler,
		systemHandler:      systemHandler,
		taskHandler:        taskHandler,
		operationLogSvc:    operationLogSvc,
		jwtSecret:          jwtSecret,
	}
}

func (r *Router) Setup(e *echo.Echo) {
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(appmw.OperationLog(r.operationLogSvc))

	e.GET("/health", r.healthHandler.Check)
	e.POST("/api/v1/auth/login", r.authHandler.Login)

	api := e.Group("/api/v1", appmw.JWTAuth(r.jwtSecret))
	{
		operationLogs := api.Group("/operation-logs")
		{
			operationLogs.GET("", r.operationLogHandler.List)
			operationLogs.GET("/actions", r.operationLogHandler.GetActions)
		}

		adminOperLogs := api.Group("/admin-oper-logs")
		{
			adminOperLogs.GET("", r.adminOperLogHandler.List)
			adminOperLogs.POST("/sync", r.adminOperLogHandler.Sync)
			adminOperLogs.GET("/sync/status", r.adminOperLogHandler.Status)
			adminOperLogs.GET("/stats", r.adminOperLogHandler.GetStats)
			adminOperLogs.GET("/types", r.adminOperLogHandler.GetOperTypes)
			adminOperLogs.GET("/users", r.adminOperLogHandler.GetOperUsers)
			adminOperLogs.DELETE("/cleanup", r.adminOperLogHandler.Cleanup)
		}

		logs := api.Group("/logs")
		{
			logs.POST("/query", r.logHandler.Query)
			logs.POST("/query/cursor", r.logHandler.QueryByCursor)
			logs.GET("/features", r.logHandler.GetFeatures)
			logs.GET("/time-range", r.logHandler.GetTimeRange)
			logs.GET("/field-paths", r.logHandler.GetFieldPaths)
			logs.POST("/sync", r.syncHandler.Sync)
			logs.POST("/sync/cancel", r.syncHandler.Cancel)
			logs.GET("/sync/status", r.syncHandler.Status)
		}

		keys := api.Group("/keys")
		{
			keys.GET("", r.keyHandler.List)
			keys.POST("", r.keyHandler.Add)
			keys.PUT("/activate", r.keyHandler.Activate)
			keys.GET("/test", r.keyHandler.Test)
		}

		scheduler := api.Group("/scheduler")
		{
			scheduler.POST("/start", r.schedulerHandler.Start)
			scheduler.POST("/stop", r.schedulerHandler.Stop)
			scheduler.GET("/status", r.schedulerHandler.Status)
			scheduler.POST("/sync", r.schedulerHandler.IncrementalSync)
			scheduler.PUT("/interval", r.schedulerHandler.SetInterval)
		}

		syncHistory := api.Group("/sync-history")
		{
			syncHistory.GET("", r.syncHistoryHandler.List)
		}

		syncFeatures := api.Group("/sync-features")
		{
			syncFeatures.GET("", r.syncFeatureHandler.List)
			syncFeatures.PUT("", r.syncFeatureHandler.Update)
		}

		contacts := api.Group("/contacts")
		{
			contacts.GET("/tree", r.contactHandler.GetDeptTree)
			contacts.GET("/departments/:id/members", r.contactHandler.GetDeptMembers)
			contacts.GET("", r.contactHandler.List)
			contacts.GET("/departments", r.contactHandler.GetDepartments)
			contacts.POST("/sync", r.contactHandler.Sync)
			contacts.POST("/sync/incremental", r.contactHandler.SyncIncremental)
			contacts.POST("/sync/cancel", r.contactHandler.Cancel)
			contacts.GET("/sync/status", r.contactHandler.Status)
			contacts.POST("/names", r.contactHandler.GetNames)
			contacts.GET("/:userId", r.contactHandler.GetContact)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/overview", r.dashboardHandler.GetOverview)
			dashboard.GET("/inactive-users", r.dashboardHandler.GetInactiveUsers)
		}

		system := api.Group("/system")
		{
			system.GET("/status", r.systemHandler.GetStatus)
		}

		tasks := api.Group("/tasks")
		{
			tasks.POST("", r.taskHandler.SubmitTask)
			tasks.GET("", r.taskHandler.ListTasks)
			tasks.GET("/:id", r.taskHandler.GetTask)
			tasks.POST("/:id/cancel", r.taskHandler.CancelTask)
			tasks.POST("/:id/retry", r.taskHandler.RetryTask)
		}
	}
}
