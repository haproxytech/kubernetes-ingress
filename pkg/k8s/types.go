package k8s

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncType represents type of k8s received message
type SyncType string

// k8s.SyncDataEvent represents converted k8s received message
type SyncDataEvent struct {
	_ [0]int
	SyncType
	Namespace      string
	Name           string
	Data           interface{}
	EventProcessed chan struct{}
}

//nolint:golint,stylecheck
const (
	// SyncType values
	COMMAND         SyncType = "COMMAND"
	CONFIGMAP       SyncType = "CONFIGMAP"
	ENDPOINTS       SyncType = "ENDPOINTS"
	INGRESS         SyncType = "INGRESS"
	INGRESS_CLASS   SyncType = "INGRESS_CLASS"
	NAMESPACE       SyncType = "NAMESPACE"
	POD             SyncType = "POD"
	SERVICE         SyncType = "SERVICE"
	SECRET          SyncType = "SECRET"
	CR_GLOBAL       SyncType = "Global"
	CR_DEFAULTS     SyncType = "Defaults"
	CR_BACKEND      SyncType = "Backend"
	PUBLISH_SERVICE SyncType = "PUBLISH_SERVICE"
	GATEWAYCLASS    SyncType = "GATEWAYCLASS"
	GATEWAY         SyncType = "GATEWAY"
	TCPROUTE        SyncType = "TCPROUTE"
	REFERENCEGRANT  SyncType = "REFERENCEGRANT"
)

func ConvertToK8SLabelSelector(labelSelector *store.LabelSelector) *metav1.LabelSelector {
	if labelSelector == nil {
		return nil
	}
	matchLabels := make(map[string]string, len(labelSelector.MatchLabels))
	for k, v := range labelSelector.MatchLabels {
		matchLabels[k] = v
	}
	matchExpressions := make([]metav1.LabelSelectorRequirement, len(labelSelector.MatchExpressions))
	for i, matchExpression := range labelSelector.MatchExpressions {
		values := make([]string, len(matchExpression.Values))
		copy(values, matchExpression.Values)
		matchExpressions[i] = metav1.LabelSelectorRequirement{
			Key:      matchExpression.Key,
			Operator: metav1.LabelSelectorOperator(matchExpression.Operator),
			Values:   values,
		}
	}
	selector := &metav1.LabelSelector{
		MatchLabels:      matchLabels,
		MatchExpressions: matchExpressions,
	}

	return selector
}

func ConvertFromK8SLabelSelector(labelSelector *metav1.LabelSelector) *store.LabelSelector {
	if labelSelector == nil {
		return nil
	}
	matchLabels := make(map[string]string, len(labelSelector.MatchLabels))
	for k, v := range labelSelector.MatchLabels {
		matchLabels[k] = v
	}
	matchExpressions := make([]store.LabelSelectorRequirement, len(labelSelector.MatchExpressions))
	for i, matchExpression := range labelSelector.MatchExpressions {
		values := make([]string, len(matchExpression.Values))
		copy(values, matchExpression.Values)
		matchExpressions[i] = store.LabelSelectorRequirement{
			Key:      matchExpression.Key,
			Operator: string(matchExpression.Operator),
			Values:   values,
		}
	}
	selector := &store.LabelSelector{
		MatchLabels:      matchLabels,
		MatchExpressions: matchExpressions,
	}

	return selector
}
