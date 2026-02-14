package telemetry

// SLI metric names used for instrumentation.
const (
	// Latency
	MetricAPILatencyP50 = "api.latency.p50"
	MetricAPILatencyP95 = "api.latency.p95"
	MetricAPILatencyP99 = "api.latency.p99"

	// Throughput
	MetricRequestsPerSec = "api.requests_per_second"

	// Data freshness
	MetricGTFSFreshness   = "gtfs.data_age_seconds"
	MetricPositionLatency = "realtime.position_latency"

	// Availability
	MetricUptime = "service.uptime_percentage"

	// Business
	MetricDelayEvents   = "business.delays_detected"
	MetricCompensations = "business.compensations_sent"
)
