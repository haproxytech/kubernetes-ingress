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

package gateway

import (
	"context"
	"time"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

// UpdateStatusGatewayclasses is responsible of updating the statuses of the accepted gateway classes.
func (statusMgr *StatusManagerImpl) UpdateStatusGatewayclasses(gatewayclasses []store.GatewayClass) {
	transitionTime := metav1.NewTime(time.Now())
	for _, gwClass := range gatewayclasses {
		if gwClass.Status == store.EMPTY || gwClass.Status == store.DELETED {
			continue
		}
		gwc := &v1beta1.GatewayClass{}
		err := statusMgr.k8sRestClient.Get(context.TODO(), types.NamespacedName{
			Name: gwClass.Name,
		}, gwc)
		if err != nil {
			logger.Error(err)
			continue
		}

		gwc.Status = v1beta1.GatewayClassStatus{
			Conditions: []metav1.Condition{{
				Type:               GatewayClassConditionStatusAccepted,
				ObservedGeneration: gwc.Generation,
				LastTransitionTime: transitionTime,
				Status:             metav1.ConditionTrue,
				Reason:             GatewayClassReasonAccepted,
			}},
		}

		logger.Error(statusMgr.k8sRestClient.Status().Update(context.TODO(), gwc))
	}
}

// UpdateStatusGateways is responsible of updating the statuses of the  gateways.
// To be able to determine if a status update is necessary because of the number of route attached has changed by attachment or detachment.
// This is mandatory because an unmodified gateway can have its status to be updated because of changes from tcp routes in term of attachment.
func (statusMgr *StatusManagerImpl) UpdateStatusGateways(gatewayStatusRecords []gatewayStatusRecord, numRoutesByListenerByGateway, previousNumRoutesByListenerByGateway map[string]map[string]int32) {
	transitionTime := metav1.NewTime(time.Now())
	for _, gatewayStatusRecord := range gatewayStatusRecords {
		numRoutesHasChanged := hasNumberOfRoutesForAnyListenerChanged(gatewayStatusRecord, numRoutesByListenerByGateway, previousNumRoutesByListenerByGateway)
		if !numRoutesHasChanged && (gatewayStatusRecord.status == store.EMPTY || gatewayStatusRecord.status == store.DELETED) {
			continue
		}

		gwStatus := v1beta1.GatewayStatus{
			Listeners: make([]v1beta1.ListenerStatus, len(gatewayStatusRecord.listenersStatusesRecords)),
			Conditions: []metav1.Condition{{
				Type:               GatewayConditionReady,
				ObservedGeneration: gatewayStatusRecord.generation,
				LastTransitionTime: transitionTime,
				Status:             metav1.ConditionTrue,
				Reason:             GatewayReasonReady,
			}},
		}
		for i, listenerStatusRecord := range gatewayStatusRecord.listenersStatusesRecords {
			listenerConditions := []metav1.Condition{}
			var numRoutes int32 = 0
			if numRoutesByListener := numRoutesByListenerByGateway[gatewayStatusRecord.namespace+"/"+gatewayStatusRecord.name]; numRoutesByListener != nil {
				numRoutes = numRoutesByListener[listenerStatusRecord.name]
			}
			gwStatus.Listeners[i] = v1beta1.ListenerStatus{
				Name:           v1beta1.SectionName(listenerStatusRecord.name),
				SupportedKinds: RouteGroupKinds(listenerStatusRecord.validRGK).asK8SGroupKind(),
				AttachedRoutes: numRoutes,
			}

			// ListenerConditionReady
			conditionReady := metav1.Condition{
				Type:               ListenerConditionReady,
				ObservedGeneration: gatewayStatusRecord.generation,
				LastTransitionTime: transitionTime,
				Reason:             ListenerReasonReady,
				Status:             metav1.ConditionTrue,
			}

			// ListenerConditionResolvedRefs
			condition := metav1.Condition{
				Type:               ListenerConditionResolvedRefs,
				ObservedGeneration: gatewayStatusRecord.generation,
				LastTransitionTime: transitionTime,
			}
			if msg, ok := listenerStatusRecord.reasons[ListenerReasonInvalidRouteKinds]; ok {
				condition.Message = msg
				condition.Reason = ListenerReasonInvalidRouteKinds
				condition.Status = metav1.ConditionFalse
				conditionReady.Status = metav1.ConditionFalse
			} else {
				condition.Message = msg
				condition.Reason = ListenerReasonResolvedRefs
				condition.Status = metav1.ConditionTrue
			}
			listenerConditions = append(listenerConditions, condition)

			// ListenerReasonUnsupportedProtocol
			condition = metav1.Condition{
				Type:               ListenerConditionDetached,
				ObservedGeneration: gatewayStatusRecord.generation,
				LastTransitionTime: transitionTime,
			}
			if msg, ok := listenerStatusRecord.reasons[ListenerReasonUnsupportedProtocol]; ok {
				condition.Message = msg
				condition.Reason = ListenerReasonUnsupportedProtocol
				condition.Status = metav1.ConditionTrue
				conditionReady.Status = metav1.ConditionFalse
			} else {
				condition.Reason = ListenerReasonAttached
				condition.Status = metav1.ConditionFalse
			}
			listenerConditions = append(listenerConditions, condition, conditionReady)
			gwStatus.Listeners[i].Conditions = listenerConditions
		}

		gw := &v1beta1.Gateway{}
		err := statusMgr.k8sRestClient.Get(context.TODO(), types.NamespacedName{
			Namespace: gatewayStatusRecord.namespace,
			Name:      gatewayStatusRecord.name,
		}, gw)
		if err != nil {
			logger.Error(err)
			continue
		}

		gw.Status = gwStatus

		err = statusMgr.k8sRestClient.Status().Update(context.TODO(), gw)
		logger.Error(err)
	}
}

// UpdateStatusTCPRoutes is responsible of updating the statuses of the  tcp routes.
func (statusMgr *StatusManagerImpl) UpdateStatusTCPRoutes(routesStatusRecords []routeStatusRecord) {
	transitionTime := metav1.NewTime(time.Now())
	for _, tcprouteStatusRecord := range routesStatusRecords {
		if tcprouteStatusRecord.status == store.EMPTY || tcprouteStatusRecord.status == store.DELETED {
			continue
		}

		tcprouteStatus := v1alpha2.TCPRouteStatus{
			RouteStatus: v1alpha2.RouteStatus{
				Parents: []v1alpha2.RouteParentStatus{},
			},
		}
		tcproute := &v1alpha2.TCPRoute{}
		err := statusMgr.k8sRestClient.Get(context.TODO(), types.NamespacedName{
			Namespace: tcprouteStatusRecord.namespace,
			Name:      tcprouteStatusRecord.name,
		}, tcproute)
		if err != nil {
			logger.Error(err)
			continue
		}

		for _, parentStatusRecord := range tcprouteStatusRecord.parentsStatusesRecords {
			parentStatusRecord := parentStatusRecord
			conditions := []metav1.Condition{}
			routeParentStatus := v1alpha2.RouteParentStatus{
				ControllerName: v1alpha2.GatewayController(statusMgr.gatewayControllerName),
				ParentRef: v1alpha2.ParentReference{
					Group:       (*v1alpha2.Group)(&parentStatusRecord.parentRef.Group),
					Kind:        (*v1alpha2.Kind)(&parentStatusRecord.parentRef.Kind),
					Namespace:   (*v1alpha2.Namespace)(parentStatusRecord.parentRef.Namespace),
					Name:        v1alpha2.ObjectName(parentStatusRecord.parentRef.Name),
					SectionName: (*v1alpha2.SectionName)(parentStatusRecord.parentRef.SectionName),
					Port:        (*v1alpha2.PortNumber)(parentStatusRecord.parentRef.Port),
				},
			}

			// RouteConditionAccepted
			condition := metav1.Condition{
				Type:               RouteConditionAccepted,
				ObservedGeneration: tcprouteStatusRecord.generation,
				LastTransitionTime: transitionTime,
			}
			if msg, ok := parentStatusRecord.reasons[RouteReasonNotAllowedByListeners]; ok {
				condition.Status = metav1.ConditionFalse
				condition.Message = msg
				condition.Reason = RouteReasonNotAllowedByListeners
			} else {
				condition.Status = metav1.ConditionTrue
				condition.Reason = RouteReasonAccepted
			}
			conditions = append(conditions, condition)

			// RouteConditionResolvedRefs
			condition = metav1.Condition{
				Type:               RouteConditionResolvedRefs,
				ObservedGeneration: tcprouteStatusRecord.generation,
				LastTransitionTime: transitionTime,
				Status:             metav1.ConditionTrue,
				Reason:             RouteReasonResolvedRefs,
			}

			if msg, ok := tcprouteStatusRecord.generalConditions[RouteReasonRefNotPermitted]; ok {
				condition.Status = metav1.ConditionFalse
				condition.Message = msg
				condition.Reason = RouteReasonRefNotPermitted
			} else if msg, ok := tcprouteStatusRecord.generalConditions[RouteReasonInvalidKind]; ok {
				condition.Status = metav1.ConditionFalse
				condition.Message = msg
				condition.Reason = RouteReasonInvalidKind
			} else if msg, ok := tcprouteStatusRecord.generalConditions[RouteReasonBackendNotFound]; ok {
				condition.Status = metav1.ConditionFalse
				condition.Message = msg
				condition.Reason = RouteReasonBackendNotFound
			}
			conditions = append(conditions, condition)

			routeParentStatus.Conditions = conditions
			tcprouteStatus.Parents = append(tcprouteStatus.Parents, routeParentStatus)
		}

		tcproute.Status = tcprouteStatus
		err = statusMgr.k8sRestClient.Status().Update(context.TODO(), tcproute)
		logger.Error(err)
	}
}

// hasNumberOfRoutesForAnyListenerChanged returns if the number of attached routes has changed for any listener of the provided gateways.
// For this, we need to be provided with two maps containing the current and previous counts for listeners for gateways.
func hasNumberOfRoutesForAnyListenerChanged(gatewayStatusRecord gatewayStatusRecord, numRoutesByListenerByGateway, previousNumRoutesByListenerByGateway map[string]map[string]int32) bool {
	var numRoutesHasChanged bool
	key := gatewayStatusRecord.namespace + "/" + gatewayStatusRecord.name
	numRoutesByListener := numRoutesByListenerByGateway[key]
	previousNumRoutesByListener := previousNumRoutesByListenerByGateway[key]
	if numRoutesByListener != nil {
		for _, listenerStatusRecord := range gatewayStatusRecord.listenersStatusesRecords {
			num := numRoutesByListener[listenerStatusRecord.name]
			if previousNumRoutesByListener != nil {
				numRoutesHasChanged = num != previousNumRoutesByListener[listenerStatusRecord.name]
				if numRoutesHasChanged {
					break
				}
			} else {
				numRoutesHasChanged = true
				break
			}
		}
	} else {
		numRoutesHasChanged = previousNumRoutesByListener != nil
	}
	return numRoutesHasChanged
}

type RouteGroupKinds []store.RouteGroupKind

func (rgk RouteGroupKinds) asK8SGroupKind() []v1beta1.RouteGroupKind {
	routeGroupKind := make([]v1beta1.RouteGroupKind, len(rgk))
	for i, rgk := range rgk {
		routeGroupKind[i] = v1beta1.RouteGroupKind{
			Group: (*v1beta1.Group)(rgk.Group),
			Kind:  v1beta1.Kind(rgk.Kind),
		}
	}
	return routeGroupKind
}
