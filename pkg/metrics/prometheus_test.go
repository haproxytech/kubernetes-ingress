package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// rejectedSeries gathers the rejected resources gauge as "kind/namespace/name" -> value.
func rejectedSeries(t *testing.T, pmm PrometheusMetricsManager) map[string]float64 {
	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(pmm.rejectedCRGaugeVec))
	families, err := registry.Gather()
	require.NoError(t, err)
	series := map[string]float64{}
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			series[labels["kind"]+"/"+labels["namespace"]+"/"+labels["name"]] = metric.GetGauge().GetValue()
		}
	}
	return series
}

func TestRejectedCustomResourcesPublishesOneSeriesPerResource(t *testing.T) {
	pmm := New()

	pmm.SetRejectedCustomResources([]RejectedCustomResource{
		{Kind: "Backend", Namespace: "ns", Name: "be", Generation: 3},
		{Kind: "Global", Namespace: "ns", Name: "gl", Generation: 1},
	})

	require.Equal(t, map[string]float64{"Backend/ns/be": 3, "Global/ns/gl": 1}, rejectedSeries(t, pmm))
}

func TestRejectedCustomResourcesDropsSeriesOfResourcesNoLongerRejected(t *testing.T) {
	pmm := New()
	pmm.SetRejectedCustomResources([]RejectedCustomResource{{Kind: "Backend", Namespace: "ns", Name: "be", Generation: 3}})

	pmm.SetRejectedCustomResources(nil)

	require.Empty(t, rejectedSeries(t, pmm))
}
