package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	integrationReconcileTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ksit",
			Subsystem: "integration",
			Name:      "reconcile_total",
			Help:      "Total number of integration reconciliations",
		},
		[]string{"integration", "type", "status"},
	)

	integrationReconcileDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ksit",
			Subsystem: "integration",
			Name:      "reconcile_duration_seconds",
			Help:      "Duration of integration reconciliation in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"integration", "type"},
	)

	integrationStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "ksit",
			Subsystem: "integration",
			Name:      "status",
			Help:      "Current status of integrations (1=running, 0=not running)",
		},
		[]string{"integration", "type", "cluster"},
	)

	clusterConnectionStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "ksit",
			Subsystem: "cluster",
			Name:      "connection_status",
			Help:      "Cluster connection status (1=connected, 0=disconnected)",
		},
		[]string{"cluster"},
	)

	syncOperationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ksit",
			Subsystem: "sync",
			Name:      "operations_total",
			Help:      "Total number of sync operations",
		},
		[]string{"integration", "cluster", "status"},
	)

	syncLatencySeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ksit",
			Subsystem: "sync",
			Name:      "latency_seconds",
			Help:      "Sync operation latency in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"integration", "cluster"},
	)
)

func RecordReconcile(integration, integrationType, status string) {
	integrationReconcileTotal.WithLabelValues(integration, integrationType, status).Inc()
}

func RecordReconcileDuration(integration, integrationType string, durationSeconds float64) {
	integrationReconcileDuration.WithLabelValues(integration, integrationType).Observe(durationSeconds)
}

func SetIntegrationStatus(integration, integrationType, cluster string, running bool) {
	value := 0.0
	if running {
		value = 1.0
	}
	integrationStatus.WithLabelValues(integration, integrationType, cluster).Set(value)
}

func SetClusterConnectionStatus(cluster string, connected bool) {
	value := 0.0
	if connected {
		value = 1.0
	}
	clusterConnectionStatus.WithLabelValues(cluster).Set(value)
}

func RecordSyncOperation(integration, cluster, status string) {
	syncOperationsTotal.WithLabelValues(integration, cluster, status).Inc()
}

func RecordSyncLatency(integration, cluster string, latencySeconds float64) {
	syncLatencySeconds.WithLabelValues(integration, cluster).Observe(latencySeconds)
}
