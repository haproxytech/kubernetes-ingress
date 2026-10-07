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

// RejectedCustomResource is a custom resource HAProxy refused, with the rejected generation.
type RejectedCustomResource struct {
	Kind       string
	Namespace  string
	Name       string
	Generation int64
}

type PrometheusMetricsManager struct {
	unableToSyncGauge prometheus.Gauge

	// custom resources set aside after HAProxy rejected them
	rejectedCRGaugeVec *prometheus.GaugeVec

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

		rejectedCRGauge := promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "haproxy_rejected_custom_resource_generation",
				Help: "The generation of a custom resource HAProxy rejected and the controller set aside, partitioned by kind, namespace and name; " +
					"the series disappears once the resource is edited or deleted",
			},
			[]string{"kind", "namespace", "name"},
		)

		pmm = PrometheusMetricsManager{
			reloadsCounterVec:       reloadCounter,
			runtimeSocketCounterVec: runtimeSocketCounter,
			unableToSyncGauge:       unableToSyncGauge,
			rejectedCRGaugeVec:      rejectedCRGauge,
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

// SetRejectedCustomResources replaces the published set of rejected custom resources.
func (pmm PrometheusMetricsManager) SetRejectedCustomResources(rejected []RejectedCustomResource) {
	pmm.rejectedCRGaugeVec.Reset()
	for _, r := range rejected {
		pmm.rejectedCRGaugeVec.WithLabelValues(r.Kind, r.Namespace, r.Name).Set(float64(r.Generation))
	}
}
