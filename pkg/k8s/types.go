// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
