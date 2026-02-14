package metrics

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var (
	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests processed",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "bilbopass",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request latency in seconds",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"method", "path"})

	httpResponseSize = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "bilbopass",
		Subsystem: "http",
		Name:      "response_size_bytes",
		Help:      "HTTP response size in bytes",
		Buckets:   prometheus.ExponentialBuckets(100, 10, 6),
	}, []string{"method", "path"})

	// Transit-specific metrics
	VehiclePositionsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "transit",
		Name:      "vehicle_positions_ingested_total",
		Help:      "Total vehicle positions ingested from GTFS-RT feeds",
	}, []string{"agency"})

	DelaysDetected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "transit",
		Name:      "delays_detected_total",
		Help:      "Total delay events detected",
	}, []string{"agency"})

	FeedPollDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "bilbopass",
		Subsystem: "transit",
		Name:      "feed_poll_duration_seconds",
		Help:      "Duration of GTFS-RT feed polling",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	}, []string{"agency"})

	FeedPollErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "transit",
		Name:      "feed_poll_errors_total",
		Help:      "Total GTFS-RT feed poll errors",
	}, []string{"agency"})

	ActiveWebSockets = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "bilbopass",
		Subsystem: "ws",
		Name:      "active_connections",
		Help:      "Current number of active WebSocket connections",
	})

	CacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "cache",
		Name:      "hits_total",
		Help:      "Total cache hits",
	}, []string{"operation"})

	CacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "cache",
		Name:      "misses_total",
		Help:      "Total cache misses",
	}, []string{"operation"})

	// Database pool metrics
	DBPoolConnsOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_conns_open",
		Help:      "Total connections open in the database pool",
	})

	DBPoolConnsAcquired = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_conns_acquired",
		Help:      "Connections currently acquired from the database pool",
	})

	DBPoolConnsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_conns_idle",
		Help:      "Idle connections in the database pool",
	})

	DBPoolEmptyAcquires = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_empty_acquires_total",
		Help:      "Total times a connection had to be established when acquiring from pool",
	})

	DBPoolWaitCount = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_wait_count_total",
		Help:      "Total times waiting for a connection from pool",
	})

	DBPoolWaitDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "bilbopass",
		Subsystem: "db",
		Name:      "pool_wait_duration_seconds",
		Help:      "Duration waiting for a database connection",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	})
)

// normalizePath reduces path cardinality for metrics by replacing IDs with :id.
func normalizePath(path string) string {
	switch {
	case path == "/v1/health" || path == "/v1/ready" || path == "/v1/agencies" ||
		path == "/v1/stops/nearby" || path == "/v1/stops/search" || path == "/v1/routes" ||
		path == "/graphql" || path == "/metrics":
		return path
	default:
		// /v1/stops/:id, /v1/stops/:id/departures, /v1/routes/:id, etc.
		return path // fiber already resolves to route pattern
	}
}

// Middleware records request metrics.
func Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}
		method := c.Method()

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
		httpResponseSize.WithLabelValues(method, path).Observe(float64(len(c.Response().Body())))

		return err
	}
}

// Handler returns a Fiber handler serving Prometheus /metrics endpoint.
func Handler() fiber.Handler {
	handler := promhttp.Handler()
	return func(c *fiber.Ctx) error {
		fasthttpadaptor.NewFastHTTPHandler(handler)(c.Context())
		return nil
	}
}

// UpdateDBPoolMetrics updates database pool metrics from pgx pool stats.
func UpdateDBPoolMetrics(stat interface{}) {
	// pgxpool.Stat has these fields:
	// AcquiredConns()  - connections currently in use
	// IdleConns()      - connections available
	// TotalConns()     - total connections
	// EmptyAcquireCount() - times a new connection was created
	// AcquireDuration() - total time spent acquiring connections
	// AcquireCount()   - total acquisitions
	// WaitCount()      - times waiting for a connection
	// WaitDuration()   - total wait time

	// Use reflection to avoid importing pgxpool directly into metrics package
	// This allows the metrics module to stay independent
	type poolStat interface {
		AcquiredConns() int32
		IdleConns() int32
		TotalConns() int32
	}

	if s, ok := stat.(poolStat); ok {
		DBPoolConnsAcquired.Set(float64(s.AcquiredConns()))
		DBPoolConnsIdle.Set(float64(s.IdleConns()))
		DBPoolConnsOpen.Set(float64(s.TotalConns()))
	}
}
