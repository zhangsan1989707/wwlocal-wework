package router

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"wwlocal-wework/internal/handler"
	appmw "wwlocal-wework/internal/middleware"
	"wwlocal-wework/internal/service"
)

// RouterDeps 路由依赖集合，避免 NewRouter 参数过多
type RouterDeps struct {
	Health        *handler.HealthHandler
	Auth          *handler.AuthHandler
	Log           *handler.LogHandler
	Key           *handler.KeyHandler
	Sync          *handler.SyncHandler
	Scheduler     *handler.SchedulerHandler
	Contact       *handler.ContactHandler
	OperationLog  *handler.OperationLogHandler
	AdminOperLog  *handler.AdminOperLogHandler
	Dashboard     *handler.DashboardHandler
	DashboardV2   *handler.DashboardV2Handler
	Nightly       *handler.NightlyHandler
	SyncHistory   *handler.SyncHistoryHandler
	SyncFeature   *handler.SyncFeatureHandler
	System        *handler.SystemHandler
	Task          *handler.TaskHandler
	OperationSvc  *service.OperationLogService
	JWTSecret     string
	RateLimiter   *appmw.RateLimiter
	Origins       []string
	MetricsIPs    []string
}

type Router struct {
	deps RouterDeps
}

func NewRouter(d RouterDeps) *Router {
	return &Router{deps: d}
}

func (r *Router) Setup(e *echo.Echo) {
	d := r.deps
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("10M"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: d.Origins,
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))
	e.Use(appmw.PrometheusMiddleware())
	e.Use(appmw.OperationLog(d.OperationSvc))
	if d.RateLimiter != nil {
		e.Use(d.RateLimiter.Middleware())
	}

	e.GET("/health", d.Health.Check)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()), appmw.MetricsAuth(d.MetricsIPs))
	e.POST("/api/v1/auth/login", d.Auth.Login)
	e.POST("/api/v1/auth/refresh", d.Auth.RefreshToken)

	api := e.Group("/api/v1", appmw.JWTAuth(d.JWTSecret))
	{
		api.PUT("/auth/password", d.Auth.ChangePassword)

		operationLogs := api.Group("/operation-logs")
		{
			operationLogs.GET("", d.OperationLog.List)
			operationLogs.GET("/actions", d.OperationLog.GetActions)
		}

		adminOperLogs := api.Group("/admin-oper-logs")
		{
			adminOperLogs.GET("", d.AdminOperLog.List)
			adminOperLogs.POST("/sync", d.AdminOperLog.Sync)
			adminOperLogs.GET("/sync/status", d.AdminOperLog.Status)
			adminOperLogs.GET("/stats", d.AdminOperLog.GetStats)
			adminOperLogs.GET("/types", d.AdminOperLog.GetOperTypes)
			adminOperLogs.GET("/users", d.AdminOperLog.GetOperUsers)
			adminOperLogs.DELETE("/cleanup", d.AdminOperLog.Cleanup)
		}

		logs := api.Group("/logs")
		{
			logs.POST("/query", d.Log.Query)
			logs.POST("/query/cursor", d.Log.QueryByCursor)
			logs.POST("/export", d.Log.ExportCSV)
			logs.GET("/features", d.Log.GetFeatures)
			logs.GET("/time-range", d.Log.GetTimeRange)
			logs.GET("/field-paths", d.Log.GetFieldPaths)
			logs.POST("/sync", d.Sync.Sync)
			logs.POST("/sync/cancel", d.Sync.Cancel)
			logs.GET("/sync/status", d.Sync.Status)
		}

		keys := api.Group("/keys")
		{
			keys.GET("", d.Key.List)
			keys.POST("", d.Key.Add)
			keys.PUT("/activate", d.Key.Activate)
			keys.GET("/test", d.Key.Test)
		}

		scheduler := api.Group("/scheduler")
		{
			scheduler.POST("/start", d.Scheduler.Start)
			scheduler.POST("/stop", d.Scheduler.Stop)
			scheduler.GET("/status", d.Scheduler.Status)
			scheduler.POST("/sync", d.Scheduler.IncrementalSync)
			scheduler.PUT("/interval", d.Scheduler.SetInterval)
		}

		syncHistory := api.Group("/sync-history")
		{
			syncHistory.GET("", d.SyncHistory.List)
		}

		syncFeatures := api.Group("/sync-features")
		{
			syncFeatures.GET("", d.SyncFeature.List)
			syncFeatures.PUT("", d.SyncFeature.Update)
		}

		contacts := api.Group("/contacts")
		{
			contacts.GET("/tree", d.Contact.GetDeptTree)
			contacts.GET("/departments/:id/members", d.Contact.GetDeptMembers)
			contacts.GET("", d.Contact.List)
			contacts.GET("/departments", d.Contact.GetDepartments)
			contacts.POST("/sync", d.Contact.Sync)
			contacts.POST("/sync/incremental", d.Contact.SyncIncremental)
			contacts.POST("/sync/cancel", d.Contact.Cancel)
			contacts.GET("/sync/status", d.Contact.Status)
			contacts.POST("/names", d.Contact.GetNames)
			contacts.GET("/:userId", d.Contact.GetContact)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/overview", d.Dashboard.GetOverview)
			dashboard.GET("/inactive-users", d.Dashboard.GetInactiveUsers)
			dashboard.GET("/inactive-users/export", d.Dashboard.ExportInactiveUsers)
			dashboard.GET("/trend", d.Dashboard.GetTrend)
			dashboard.GET("/trend/dept", d.Dashboard.GetTrendByDept)
			dashboard.GET("/trend/export", d.Dashboard.ExportTrend)
		}

		dashboardV2 := api.Group("/dashboard/v2")
		{
			dashboardV2.GET("/overview", d.DashboardV2.GetOverview)
			dashboardV2.GET("/trend", d.DashboardV2.GetTrend)
			dashboardV2.GET("/multi-trend", d.DashboardV2.GetMultiTrend)
			dashboardV2.GET("/departments", d.DashboardV2.GetDepartmentStats)
			dashboardV2.GET("/devices", d.DashboardV2.GetDeviceStats)
			dashboardV2.GET("/users", d.DashboardV2.GetUserList)
			dashboardV2.GET("/export/overview", d.DashboardV2.ExportOverviewCSV)
			dashboardV2.GET("/export/users", d.DashboardV2.ExportUserListCSV)
		}

		nightly := api.Group("/nightly")
		{
			nightly.POST("/run", d.Nightly.Run)
			nightly.GET("/status", d.Nightly.Status)
		}

		system := api.Group("/system")
		{
			system.GET("/status", d.System.GetStatus)
		}

		tasks := api.Group("/tasks")
		{
			tasks.POST("", d.Task.SubmitTask)
			tasks.GET("", d.Task.ListTasks)
			tasks.GET("/:id", d.Task.GetTask)
			tasks.POST("/:id/cancel", d.Task.CancelTask)
			tasks.POST("/:id/retry", d.Task.RetryTask)
		}
	}
}
