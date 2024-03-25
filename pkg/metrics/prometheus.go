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
	// reload
	reloadsCounterVec *prometheus.CounterVec

	// restart
	restartsCounterVec *prometheus.CounterVec

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

		// restart
		restartCounter := promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "haproxy_restarts_total",
				Help: "The number of haproxy restarts partitioned by result (success/failure)",
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

		pmm = PrometheusMetricsManager{
			reloadsCounterVec:       reloadCounter,
			restartsCounterVec:      restartCounter,
			runtimeSocketCounterVec: runtimeSocketCounter,
		}
	})
	return pmm
}

func (pmm PrometheusMetricsManager) UpdateRestartMetrics(err error) {
	if err != nil {
		pmm.restartsCounterVec.WithLabelValues(ResultFailure).Inc()
	} else {
		pmm.restartsCounterVec.WithLabelValues(ResultSuccess).Inc()
	}
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
