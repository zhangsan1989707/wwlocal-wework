package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	SyncOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sync_operations_total",
			Help: "Total number of sync operations",
		},
		[]string{"feature_id", "status"},
	)

	SyncOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sync_operation_duration_seconds",
			Help:    "Duration of sync operations in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"feature_id"},
	)

	SyncRecordsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sync_records_total",
			Help: "Total number of synced records",
		},
		[]string{"feature_id", "status"},
	)

	ContactSyncTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "contact_sync_total",
			Help: "Total number of contact sync operations",
		},
		[]string{"status"},
	)

	ContactMembersTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "contact_members_total",
			Help: "Total number of contact members",
		},
	)

	DBConnectionPoolSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connection_pool_size",
			Help: "Database connection pool size",
		},
	)

	DBConnectionPoolIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connection_pool_idle",
			Help: "Number of idle database connections",
		},
	)

	CacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type"},
	)

	DecryptionTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "decryption_total",
			Help: "Total number of decryption operations",
		},
		[]string{"status"},
	)

	DecryptionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "decryption_duration_seconds",
			Help:    "Duration of decryption operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	ActiveTasksGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_tasks",
			Help: "Number of currently active tasks",
		},
	)

	TaskQueueSizeGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "task_queue_size",
			Help: "Current size of the task queue",
		},
	)
)

func RecordHttpRequest(method, path string, status int, duration time.Duration) {
	HttpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
	HttpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

func RecordSyncOperation(featureID string, status string, duration time.Duration, records int) {
	SyncOperationsTotal.WithLabelValues(featureID, status).Inc()
	SyncOperationDuration.WithLabelValues(featureID).Observe(duration.Seconds())
	SyncRecordsTotal.WithLabelValues(featureID, status).Add(float64(records))
}

func RecordContactSync(status string, members int) {
	ContactSyncTotal.WithLabelValues(status).Inc()
	if status == "success" {
		ContactMembersTotal.Set(float64(members))
	}
}

func RecordCacheHit(cacheType string) {
	CacheHitsTotal.WithLabelValues(cacheType).Inc()
}

func RecordCacheMiss(cacheType string) {
	CacheMissesTotal.WithLabelValues(cacheType).Inc()
}

func RecordDecryption(success bool, duration time.Duration) {
	status := "failure"
	if success {
		status = "success"
	}
	DecryptionTotal.WithLabelValues(status).Inc()
	DecryptionDuration.Observe(duration.Seconds())
}

func SetDBPoolStats(poolSize, idle int) {
	DBConnectionPoolSize.Set(float64(poolSize))
	DBConnectionPoolIdle.Set(float64(idle))
}
