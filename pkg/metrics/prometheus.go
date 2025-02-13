package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	ResultSuccess = "success"
	ResultFailure = "failure"
	ObjectMap     = "map"
	ObjectServer  = "server"
)

type PrometheusMetricsManager struct {
	unableToSyncGauge prometheus.Gauge

	// reload
	reloadsCounterVec *prometheus.CounterVec

	// runtime socket
	runtimeSocketCounterVec *prometheus.CounterVec
}

var (
	pmm     PrometheusMetricsManager // tests will fail if we try to call New() more than once
	syncPMM sync.Once
)

func New() PrometheusMetricsManager {
	syncPMM.Do(func() {
		// reload
		reloadCounter := promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "haproxy_reloads_total",
				Help: "The number of haproxy reloads partitioned by result (success/failure)",
			},
			[]string{"result"},
		)

		// runtime socket
		runtimeSocketCounter := promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "haproxy_runtime_socket_connections_total",
				Help: "The number of haproxy runtime socket connections partitioned by object (server/map) and result (success/failure)",
			},
			[]string{"object", "result"},
		)

		unableToSyncGauge := promauto.NewGauge(prometheus.GaugeOpts{
			Name: "haproxy_unable_to_sync_configuration",
			Help: "1 = there's a pending haproxy configuration that is not valid so not applicable, 0 = haproxy configuration applied",
		})

		pmm = PrometheusMetricsManager{
			reloadsCounterVec:       reloadCounter,
			runtimeSocketCounterVec: runtimeSocketCounter,
			unableToSyncGauge:       unableToSyncGauge,
		}
	})
	return pmm
}

func (pmm PrometheusMetricsManager) UpdateReloadMetrics(err error) {
	if err != nil {
		pmm.reloadsCounterVec.WithLabelValues(ResultFailure).Inc()
	} else {
		pmm.reloadsCounterVec.WithLabelValues(ResultSuccess).Inc()
	}
}

func (pmm PrometheusMetricsManager) UpdateRuntimeMetrics(object string, err error) {
	if err != nil {
		pmm.runtimeSocketCounterVec.WithLabelValues(object, ResultFailure).Inc()
	} else {
		pmm.runtimeSocketCounterVec.WithLabelValues(object, ResultSuccess).Inc()
	}
}

func (pmm PrometheusMetricsManager) SetUnableSyncGauge() {
	pmm.unableToSyncGauge.Set(float64(1))
}

func (pmm PrometheusMetricsManager) UnsetUnableSyncGauge() {
	pmm.unableToSyncGauge.Set(float64(0))
}
