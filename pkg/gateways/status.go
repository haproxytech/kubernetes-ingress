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
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GatewayConditionReady           = "Ready"
	GatewayReasonReady              = "Ready"
	GatewayReasonListenersNotValid  = "ListenersNotValid"
	GatewayReasonListenersNotReady  = "ListenersNotReady"
	GatewayReasonAddressNotAssigned = "AddressNotAssigned"

	ListenerConditionConflicted    = "Conflicted"
	ListenerReasonHostnameConflict = "HostnameConflict"
	ListenerReasonProtocolConflict = "ProtocolConflict"
	ListenerReasonNoConflicts      = "NoConflicts"

	ListenerConditionReady = "Ready"
	ListenerReasonReady    = "Ready"
	ListenerReasonInvalid  = "Invalid"
	ListenerReasonPending  = "Pending"

	ListenerConditionDetached         = "Detached"
	ListenerReasonPortUnavailable     = "PortUnavailable"
	ListenerReasonUnsupportedProtocol = "UnsupportedProtocol"
	ListenerReasonUnsupportedAddress  = "UnsupportedAddress"
	ListenerReasonAttached            = "Attached"

	ListenerConditionResolvedRefs       = "ResolvedRefs"
	ListenerReasonResolvedRefs          = "ResolvedRefs"
	ListenerReasonInvalidCertificateRef = "InvalidCertificateRef"
	ListenerReasonInvalidRouteKinds     = "InvalidRouteKinds"
	ListenerReasonRefNotPermitted       = "RefNotPermitted"

	GatewayClassConditionStatusAccepted = "Accepted"
	GatewayClassReasonAccepted          = "Accepted"
	GatewayClassReasonInvalidParameters = "InvalidParameters"
	GatewayClassReasonWaiting           = "Waiting"

	RouteConditionAccepted                = "Accepted"
	RouteReasonAccepted                   = "Accepted"
	RouteReasonNotAllowedByListeners      = "NotAllowedByListeners"
	RouteReasonNoMatchingListenerHostname = "NoMatchingListenerHostname"
	RouteReasonUnsupportedValue           = "UnsupportedValue"

	RouteConditionResolvedRefs = "ResolvedRefs"
	RouteReasonResolvedRefs    = "ResolvedRefs"
	RouteReasonRefNotPermitted = "RefNotPermitted"
	RouteReasonInvalidKind     = "InvalidKind"
	RouteReasonBackendNotFound = "BackendNotFound"
)

// NewStatusManager creates the default implementation for status management with gateway controller.
func NewStatusManager(k8sRestClient client.Client, gatewayControllerName string) StatusManager {
	return &StatusManagerImpl{
		k8sRestClient:                        k8sRestClient,
		gatewayControllerName:                gatewayControllerName,
		numRoutesByListenerByGateway:         map[string]map[string]int32{},
		previousNumRoutesByListenerByGateway: map[string]map[string]int32{},
	}
}

type RouteStatusManager interface {
	SetRouteReasonInvalidKind(string)
	SetRouteReasonBackendNotFound(string)
	SetRouteReasonRefNotPermitted(string)
	SetRouteReasonNotAllowedByListeners(string, store.ParentRef)
}

type StatusManager interface {
	ProcessStatuses()
	PrepareGatewayStatus(store.Gateway)
	PrepareTCPRouteStatusRecord(store.TCPRoute)
	PrepareListenerStatus(store.Listener)
	SetListenerReasonUnsupportedProtocol(string)
	SetListenerReasonInvalidRouteKinds(string, []store.RouteGroupKind)
	RouteStatusManager
	SetGatewayClassConditionStatusAccepted(store.GatewayClass)
	AddManagedParentRef(parentRef store.ParentRef)
	IncrementRouteForListener(store.Listener)
}

type StatusManagerImpl struct {
	gateway                              *gatewayStatusRecord
	gatewayclasses                       []store.GatewayClass
	listener                             *listenerStatusRecord
	tcproute                             *routeStatusRecord
	gateways                             []gatewayStatusRecord
	tcproutes                            []routeStatusRecord
	k8sRestClient                        client.Client
	gatewayControllerName                string
	numRoutesByListenerByGateway         map[string]map[string]int32
	previousNumRoutesByListenerByGateway map[string]map[string]int32
}

// status records are created for two purposes:
// - we need to record all the informations for each status type (gateway, listener, routes)
// - we need to provide a copy of all recorded status informations to feed safely goroutines for asynchronous updates.
type routeStatusRecord struct {
	name, namespace        string
	parentsStatusesRecords map[string]parentrefStatusRecord
	generalConditions      map[string]string
	generation             int64
	status                 store.Status
}

type parentrefStatusRecord struct {
	parentRef store.ParentRef
	reasons   map[string]string
}

type gatewayStatusRecord struct {
	name, namespace          string
	generation               int64
	status                   store.Status
	listenerWithError        bool
	listenersStatusesRecords []listenerStatusRecord
}

type listenerStatusRecord struct {
	name     string
	reasons  map[string]string
	validRGK []store.RouteGroupKind
}

// copy returns a copy of the gateway status record.
func (gwStatusRecord *gatewayStatusRecord) copy() gatewayStatusRecord {
	listenersStatusesRecords := make([]listenerStatusRecord, len(gwStatusRecord.listenersStatusesRecords))

	for i, gwListenerStatusesRecords := range gwStatusRecord.listenersStatusesRecords {
		listenersStatusesRecords[i] = listenerStatusRecord{
			name:     gwListenerStatusesRecords.name,
			reasons:  utils.CopyMap(gwListenerStatusesRecords.reasons),
			validRGK: gwListenerStatusesRecords.validRGK,
		}
	}
	return gatewayStatusRecord{
		name:                     gwStatusRecord.name,
		namespace:                gwStatusRecord.namespace,
		generation:               gwStatusRecord.generation,
		listenerWithError:        gwStatusRecord.listenerWithError,
		listenersStatusesRecords: listenersStatusesRecords,
		status:                   gwStatusRecord.status,
	}
}

// copy returns a copy of the route status record.
func (rteStatusRecord *routeStatusRecord) copy() routeStatusRecord {
	parentsStatusesRecords := make(map[string]parentrefStatusRecord, len(rteStatusRecord.parentsStatusesRecords))
	for key, value := range rteStatusRecord.parentsStatusesRecords {
		parentsStatusesRecords[key] = parentrefStatusRecord{
			reasons:   utils.CopyMap(value.reasons),
			parentRef: value.parentRef,
		}
	}

	return routeStatusRecord{
		name:                   rteStatusRecord.name,
		namespace:              rteStatusRecord.namespace,
		generalConditions:      utils.CopyMap(rteStatusRecord.generalConditions),
		generation:             rteStatusRecord.generation,
		parentsStatusesRecords: parentsStatusesRecords,
		status:                 rteStatusRecord.status,
	}
}

// pushTCPRoute pushes the current tcproute whose status is set by gatewaycontroller to the list of previous ones
func (statusMgr *StatusManagerImpl) pushTCPRoute() {
	if statusMgr.tcproute != nil {
		statusMgr.tcproutes = append(statusMgr.tcproutes, *statusMgr.tcproute)
		statusMgr.tcproute = nil
	}
}

// pushGateway pushes the current gateway whose status is set by gatewaycontroller to the list of previous ones
func (statusMgr *StatusManagerImpl) pushGateway() {
	if statusMgr.gateway != nil {
		statusMgr.gateways = append(statusMgr.gateways, *statusMgr.gateway)
		statusMgr.gateway = nil
	}
}

// pushListener pushes the current listener whose status is set by gatewaycontroller to the list of previous ones
func (statusMgr *StatusManagerImpl) pushListener() {
	if statusMgr.listener != nil {
		statusMgr.gateway.listenersStatusesRecords = append(statusMgr.gateway.listenersStatusesRecords, *statusMgr.listener)
		statusMgr.listener = nil
	}
}

// copyTCPRoutesStatusRecords returns a copy of all the routes statuses.
func (statusMgr *StatusManagerImpl) copyTCPRoutesStatusRecords() []routeStatusRecord {
	copies := make([]routeStatusRecord, len(statusMgr.tcproutes))
	for i, data := range statusMgr.tcproutes {
		copies[i] = data.copy()
	}
	return copies
}

// copyGatewayclasses returns a copy of all the gatewayclasses. Note we don't need a specific gatewayclass status record because of simplicity.
func (statusMgr *StatusManagerImpl) copyGatewayclasses() []store.GatewayClass {
	copies := make([]store.GatewayClass, len(statusMgr.gatewayclasses))
	copy(copies, statusMgr.gatewayclasses)
	return copies
}

// copyGatewaysStatusRecords returns a copy of all the gateways statuses.
func (statusMgr *StatusManagerImpl) copyGatewaysStatusRecords() []gatewayStatusRecord {
	copies := make([]gatewayStatusRecord, len(statusMgr.gateways))
	for i, data := range statusMgr.gateways {
		copies[i] = data.copy()
	}
	return copies
}

// PrepareGatewayStatus sets the gateway status record for a gateway.
// Every upcoming status information about a gateway provided by the gateway controller will be set into this record.
func (statusMgr *StatusManagerImpl) PrepareGatewayStatus(gateway store.Gateway) {
	statusMgr.pushListener()
	statusMgr.pushGateway()

	statusMgr.gateway = &gatewayStatusRecord{
		name:       gateway.Name,
		namespace:  gateway.Namespace,
		generation: gateway.Generation,
		status:     gateway.Status,
	}
}

// PrepareListenerStatus sets the listener status record for a listener.
// Every upcoming status information about a listener provided by the gateway controller will be set into this record.
func (statusMgr *StatusManagerImpl) PrepareListenerStatus(listener store.Listener) {
	if statusMgr.gateway == nil {
		logger.Errorf("no gateway status record present for listener '%s' of gateway '%s/%s'", listener.Name, listener.GwNamespace, listener.GwName)
		return
	}

	statusMgr.pushListener()

	statusMgr.listener = &listenerStatusRecord{
		name:    listener.Name,
		reasons: map[string]string{},
	}
}

// PrepareTCPRouteStatusRecord sets the tcproute status record for a tcproute.
// Every upcoming status information about a tcproute provided by the gateway controller will be set into this record.
func (statusMgr *StatusManagerImpl) PrepareTCPRouteStatusRecord(tcproute store.TCPRoute) {
	statusMgr.pushTCPRoute()

	statusMgr.tcproute = &routeStatusRecord{
		name:                   tcproute.Name,
		namespace:              tcproute.Namespace,
		generation:             tcproute.Generation,
		parentsStatusesRecords: map[string]parentrefStatusRecord{},
		generalConditions:      map[string]string{},
		status:                 tcproute.Status,
	}
}

// ProcessStatuses goes over all status records to update their counterparts in k8s with the corresponding resource.
func (statusMgr *StatusManagerImpl) ProcessStatuses() {
	statusMgr.pushListener()
	statusMgr.pushGateway()
	statusMgr.pushTCPRoute()
	copyGatewaysStatusRecords := statusMgr.copyGatewaysStatusRecords()
	copyTCPRouteStatusRecords := statusMgr.copyTCPRoutesStatusRecords()
	copyGatewayclasses := statusMgr.copyGatewayclasses()
	statusMgr.gatewayclasses = nil
	statusMgr.gateways = nil
	statusMgr.tcproutes = nil
	// we update asynchonously all statuses.
	go statusMgr.UpdateStatusGatewayclasses(copyGatewayclasses)
	go statusMgr.UpdateStatusGateways(copyGatewaysStatusRecords, utils.CopyMapOfMap(statusMgr.numRoutesByListenerByGateway), utils.CopyMapOfMap(statusMgr.previousNumRoutesByListenerByGateway))
	go statusMgr.UpdateStatusTCPRoutes(copyTCPRouteStatusRecords)

	statusMgr.previousNumRoutesByListenerByGateway = statusMgr.numRoutesByListenerByGateway
	statusMgr.numRoutesByListenerByGateway = map[string]map[string]int32{}
}

// SetListenerReasonUnsupportedProtocol sets the msg and the reason ListenerReasonUnsupportedProtocol for the current listener pushed by PrepareListenerStatus.
func (statusMgr *StatusManagerImpl) SetListenerReasonUnsupportedProtocol(msg string) {
	statusMgr.listener.reasons[ListenerReasonUnsupportedProtocol] = msg
	statusMgr.gateway.listenerWithError = true
}

// SetListenerReasonInvalidRouteKinds sets the msg and the reason ListenerReasonInvalidRouteKinds for the current listener pushed by PrepareListenerStatus.
func (statusMgr *StatusManagerImpl) SetListenerReasonInvalidRouteKinds(msg string, validRGK []store.RouteGroupKind) {
	statusMgr.listener.reasons[ListenerReasonInvalidRouteKinds] = msg
	statusMgr.listener.validRGK = validRGK
	statusMgr.gateway.listenerWithError = true
}

// SetRouteReasonBackendNotFound sets the msg and the reason RouteReasonBackendNotFound for the current tcp route pushed by PrepareTCPRouteStatus.
func (statusMgr *StatusManagerImpl) SetRouteReasonBackendNotFound(msg string) {
	statusMgr.tcproute.generalConditions[RouteReasonBackendNotFound] = msg
}

// SetRouteReasonBackendNotFound sets the msg and the reason RouteReasonRefNotPermitted for the current tcp route pushed by PrepareTCPRouteStatus.
func (statusMgr *StatusManagerImpl) SetRouteReasonRefNotPermitted(msg string) {
	statusMgr.tcproute.generalConditions[RouteReasonRefNotPermitted] = msg
}

// SetRouteReasonBackendNotFound sets the msg and the reason RouteReasonNotAllowedByListeners for the current tcp route pushed by PrepareTCPRouteStatus.
func (statusMgr *StatusManagerImpl) SetRouteReasonNotAllowedByListeners(msg string, parentRef store.ParentRef) {
	parentStatusRecord := statusMgr.tcproute.parentsStatusesRecords[*parentRef.Namespace+"/"+parentRef.Name]
	if parentStatusRecord.reasons == nil {
		parentStatusRecord.reasons = map[string]string{}
	}
	parentStatusRecord.reasons[RouteReasonNotAllowedByListeners] += msg + "\n"
	statusMgr.tcproute.parentsStatusesRecords[*parentRef.Namespace+"/"+parentRef.Name] = parentStatusRecord
}

// SetRouteReasonBackendNotFound sets the msg and the reason RouteReasonInvalidKind for the current tcp route pushed by PrepareTCPRouteStatus.
func (statusMgr *StatusManagerImpl) SetRouteReasonInvalidKind(msg string) {
	statusMgr.tcproute.generalConditions[RouteReasonInvalidKind] = msg
}

// SetGatewayClassConditionStatusAccepted adds the provided gatewayclass to the list of accepted gatewayclasses.
func (statusMgr *StatusManagerImpl) SetGatewayClassConditionStatusAccepted(gwClass store.GatewayClass) {
	statusMgr.gatewayclasses = append(statusMgr.gatewayclasses, gwClass)
}

// AddManagedParentRef adds the parentref inside a new parentrefStatusRecord for the current tcp route.
func (statusMgr *StatusManagerImpl) AddManagedParentRef(parentRef store.ParentRef) {
	statusMgr.tcproute.parentsStatusesRecords[*parentRef.Namespace+"/"+parentRef.Name] = parentrefStatusRecord{
		parentRef: parentRef,
	}
}

// IncrementRouteForListener incremenents the counter of attached routes to the provided listener.
func (statusMgr *StatusManagerImpl) IncrementRouteForListener(listener store.Listener) {
	numRoutesByListenerByGateway := statusMgr.numRoutesByListenerByGateway[listener.GwNamespace+"/"+listener.GwName]
	if numRoutesByListenerByGateway == nil {
		numRoutesByListenerByGateway = map[string]int32{}
		statusMgr.numRoutesByListenerByGateway[listener.GwNamespace+"/"+listener.GwName] = numRoutesByListenerByGateway
	}
	numRoutesByListenerByGateway[listener.Name]++
}
