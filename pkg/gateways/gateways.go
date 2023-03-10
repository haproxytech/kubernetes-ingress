// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gateway

import (
	"fmt"
	"sort"

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

var logger = utils.GetLogger()

// Gateway management
//
//nolint:golint,stylecheck
const (
	K8S_CORE_GROUP       = ""
	K8S_NETWORKING_GROUP = networkingv1.GroupName
	K8S_GATEWAY_GROUP    = v1beta1.GroupName
	K8S_TCPROUTE_KIND    = "TCPRoute"
	K8S_SERVICE_KIND     = "Service"
)

//nolint:golint
type GatewayManager interface {
	ManageGateway() bool
}

func New(k8sStore store.K8s,
	haproxyClient api.HAProxyClient,
	osArgs utils.OSArgs,
	k8sRestClient client.Client,
) GatewayManager {
	return GatewayManagerImpl{
		k8sStore:         k8sStore,
		haproxyClient:    haproxyClient,
		osArgs:           osArgs,
		frontends:        map[string]struct{}{},
		gateways:         map[string]struct{}{},
		statusManager:    NewStatusManager(k8sRestClient, k8sStore.GatewayControllerName),
		listenersByRoute: make(map[string][]store.Listener),
		backends:         map[string]struct{}{},
		serversByBackend: map[string][]string{},
	}
}

//nolint:golint
type GatewayManagerImpl struct {
	haproxyClient    api.HAProxyClient
	statusManager    StatusManager
	frontends        map[string]struct{}
	gateways         map[string]struct{}
	listenersByRoute map[string][]store.Listener
	backends         map[string]struct{}
	serversByBackend map[string][]string
	k8sStore         store.K8s
	osArgs           utils.OSArgs
}

func (gm GatewayManagerImpl) ManageGateway() bool {
	gm.clean()
	gm.manageGatewayClass()

	listenersReload := gm.manageListeners()
	if listenersReload {
		logger.Debug("gwapi: gateway event, haproxy reload required.")
	}
	tcproutesReload := gm.manageTCPRoutes()
	if tcproutesReload {
		logger.Debug("gwapi: tcproute event, haproxy reload required.")
	}
	gm.statusManager.ProcessStatuses()
	gm.resetStatuses()
	return listenersReload || tcproutesReload
}

// clean deletes the frontends created by the gateway controller
// We recreate all frontends on each round but we reload only if a frontend has been added, modified or deleted.
func (gm GatewayManagerImpl) clean() {
	// We manage Gateway frontends in a separate place because we don't want to interfere with Ingress frontends.
	for frontend := range gm.frontends {
		err := gm.haproxyClient.FrontendDelete(frontend)
		if err != nil {
			logger.Error(err)
		}
		delete(gm.frontends, frontend)
	}
}

// manageListeners loops over every gateway present in store and if managed gatewayclass matches, it creates the frontend if not marked as deleted .
// We order a reload only if the status of the gateway suggests an addition, modification or removal.
func (gm *GatewayManagerImpl) manageListeners() (reload bool) {
	for _, ns := range gm.k8sStore.Namespaces {
		if !ns.Relevant {
			logger.Debugf("gwapi: skipping namespace '%s'", ns.Name)
			continue
		}
		for _, gw := range ns.Gateways {
			gwName := getGatewayName(*gw)
			gwDeleted := gw.Status == store.DELETED
			gwc, gwcfound := gm.k8sStore.GatewayClasses[gw.GatewayClassName]
			gwManaged := gwcfound && gwc.ControllerName == gm.k8sStore.GatewayControllerName
			if gwManaged && gw.Status != store.DELETED {
				gm.statusManager.PrepareGatewayStatus(*gw)
				logger.Error(gm.createAllListeners(*gw))
			}
			_, gwConfigured := gm.gateways[gwName]
			reload = reload || !((!gwConfigured && !gwManaged) || (gwConfigured && gwManaged && gw.Status == store.EMPTY))
			if !gwDeleted && gwManaged {
				gm.gateways[gwName] = struct{}{}
			}
			if gwDeleted {
				delete(gm.gateways, gwName)
				delete(ns.Gateways, gw.Name)
				logger.Warningf("gwapi: deleted gateway'%s/%s'", gw.Namespace, gw.Name)
			}
		}
	}
	return
}

// manageTCPRoutes creates backends from tcproutes and attaches them to corresponding frontends according attachment rules.
func (gm GatewayManagerImpl) manageTCPRoutes() (reload bool) {
	// Multiple routes can refer to the same listener (on the same gateway).
	// If so we must keep a list of them so that we can elect a single route among them.
	routesByListeners := map[string]*utils.Pair[store.Listener, store.TCPRoutes]{}
	for _, ns := range gm.k8sStore.Namespaces {
		if !ns.Relevant {
			logger.Debugf("gwapi: skipping namespace '%s'", ns.Name)
			continue
		}
		logger.Debugf("gwapi: namespace '%s' has %d tcproutes", ns.Name, len(ns.TCPRoutes))
		for tcproutename, tcproute := range ns.TCPRoutes {
			if tcproute == nil {
				logger.Warningf("gwapi: nil tcproute under name '%s'", tcproutename)
				continue
			}
			tcpRouteBackendName := getBackendName(*tcproute)
			if tcproute.Status == store.DELETED {
				delete(ns.TCPRoutes, tcproute.Name)
				delete(gm.listenersByRoute, tcpRouteBackendName)
				delete(gm.backends, tcpRouteBackendName)
				reload = true
				continue
			}
			gm.statusManager.PrepareTCPRouteStatusRecord(*tcproute)

			// Get the list of listeners (frontends) this tcproute (set of backends) wants to be attached to.
			listeners, errListeners := gm.getOurListenersFromTCPRoute(*tcproute)
			logger.Error(errListeners)
			for _, listener := range listeners {
				frontendName := getFrontendName(listener)
				if rbl, ok := routesByListeners[frontendName]; ok {
					rbl.P2 = append(rbl.P2, *tcproute)
				} else {
					pair := utils.NewPair(listener, store.TCPRoutes{*tcproute})
					routesByListeners[frontendName] = &pair
				}
			}
			previousAssociatedListeners := gm.listenersByRoute[tcpRouteBackendName]
			gm.listenersByRoute[tcpRouteBackendName] = listeners

			needReload := ((len(listeners) != 0 || len(listeners) == 0 && len(previousAssociatedListeners) != 0) &&
				!utils.EqualSliceByIDFunc(listeners, previousAssociatedListeners, extractNameFromListener))
			reload = reload || needReload

			if needReload {
				logger.Debugf("modification in listeners for tcproute '%s/%s', reload required", tcproute.Namespace, tcproute.Name)
			}

			if len(listeners) == 0 {
				continue
			}

			// Nothing to do to delete the corresponding backend as an automatic mechanism will remove it.
			if gm.isToBeDeleted(*tcproute) {
				_, backendExists := gm.backends[tcpRouteBackendName]
				if backendExists {
					logger.Debugf("modification in backend for tcproute '%s/%s', reload required", tcproute.Namespace, tcproute.Name)
				}
				reload = reload || backendExists
				if backendExists {
					delete(gm.backends, tcpRouteBackendName)
				}
				logger.Debugf("gwapi: backend of tcproute '%s/%s' is/will be deleted", tcproute.Namespace, tcproute.Name)
				continue
			}

			// If not called on the route, the afferent backend will be automatically deleted.
			errBackendCreate := gm.haproxyClient.BackendCreateIfNotExist(
				models.Backend{
					Name:          getBackendName(*tcproute),
					Mode:          "tcp",
					DefaultServer: &models.DefaultServer{Check: "enabled"},
				})
			if errBackendCreate != nil {
				logger.Error(errBackendCreate)
				continue
			}
			_, backendExists := gm.backends[tcpRouteBackendName]
			if !backendExists {
				logger.Debugf("modification in backend for tcproute '%s/%s', reload required", tcproute.Namespace, tcproute.Name)
			}
			reload = reload || !backendExists
			gm.backends[tcpRouteBackendName] = struct{}{}
			// Adds the servers to the backends
			reloadServers, errServers := gm.addServersToRoute(*tcproute)
			if reloadServers {
				logger.Debugf("modification in servers of backend '%s' from tcproute '%s/%s'", tcpRouteBackendName, tcproute.Namespace, tcproute.Name)
			}
			logger.Error(errServers)
			reload = reload || reloadServers
		}
	}

	// Sorts the list of routes by listener and then attaches the first one to the listener.
	for fontendName, rbl := range routesByListeners {
		if len(rbl.P2) == 0 {
			continue
		}
		sort.SliceStable(rbl.P2, rbl.P2.Less)
		logger.Error(gm.addRouteToListener(fontendName, rbl.P2[0], rbl.P1))
	}
	return
}

// createAllListeners creates all TCP frontends from gateway and their bindings.
func (gm GatewayManagerImpl) createAllListeners(gateway store.Gateway) error {
	var errs utils.Errors
MAIN_LOOP:
	for _, listener := range gateway.Listeners {
		gm.statusManager.PrepareListenerStatus(listener)
		if listener.Protocol != store.TCPProtocolType {
			gm.statusManager.SetListenerReasonUnsupportedProtocol(fmt.Sprintf("Listener protocol '%s' is not supported", listener.Protocol))
			continue
		}
		if listener.AllowedRoutes != nil {
			validRGK := []store.RouteGroupKind{}
			for _, kind := range listener.AllowedRoutes.Kinds {
				if (kind.Group == nil || *kind.Group == v1alpha2.GroupName) && kind.Kind == K8S_TCPROUTE_KIND {
					validRGK = append(validRGK, kind)
				}
			}
			if len(validRGK) != len(listener.AllowedRoutes.Kinds) {
				gm.statusManager.SetListenerReasonInvalidRouteKinds("Invalid Group/Kind in allowedRoutes: only gateway.networking.k8s.io or empty group and TCPRoute kind are supported", validRGK)
			}
			if len(validRGK) == 0 && len(listener.AllowedRoutes.Kinds) != 0 {
				continue MAIN_LOOP
			}
		}

		frontendName := getFrontendName(listener)
		errFrontendCreate := gm.haproxyClient.FrontendCreate(models.Frontend{
			Name:   frontendName,
			Mode:   "tcp",
			Tcplog: true,
		})
		if errFrontendCreate != nil {
			errs.Add(errFrontendCreate)
			continue
		}
		gm.frontends[frontendName] = struct{}{}
		port := int64(listener.Port)
		if !gm.osArgs.DisableIPV4 {
			errBinCreate := gm.haproxyClient.FrontendBindCreate(
				frontendName, models.Bind{
					Port: &port,
					Address: func() string {
						if gm.osArgs.IPV4BindAddr != "" {
							return gm.osArgs.IPV4BindAddr
						}
						return "0.0.0.0"
					}(),
					BindParams: models.BindParams{Name: "v4"},
				})
			if errBinCreate != nil {
				errs.Add(errBinCreate)
				continue
			}
		}
		if !gm.osArgs.DisableIPV6 {
			errBinCreate := gm.haproxyClient.FrontendBindCreate(
				frontendName, models.Bind{
					Port: &port,
					Address: func() string {
						if gm.osArgs.IPV6BindAddr != "" {
							return gm.osArgs.IPV6BindAddr
						}
						return ":::"
					}(),
					BindParams: models.BindParams{Name: "v6"},
				})
			if errBinCreate != nil {
				errs.Add(errBinCreate)
				continue
			}
		}
	}
	return errs.Result()
}

// isBackendRefValid valids the backendRef according internal state validation rules.
func (gm GatewayManagerImpl) isBackendRefValid(backendRef store.BackendRef) bool {
	if backendRef.Group != nil &&
		*backendRef.Group != K8S_NETWORKING_GROUP &&
		*backendRef.Group != K8S_CORE_GROUP {
		gm.statusManager.SetRouteReasonInvalidKind(fmt.Sprintf("backendref group %s not managed", utils.PointerDefaultValueIfNil(backendRef.Group)))
		return false
	}
	if backendRef.Kind != nil && *backendRef.Kind != K8S_SERVICE_KIND {
		gm.statusManager.SetRouteReasonInvalidKind(fmt.Sprintf("backendref kind %s not managed", utils.PointerDefaultValueIfNil(backendRef.Kind)))
		return false
	}
	if backendRef.Port == nil {
		return false
	}
	return true
}

// isNamespaceGranted checks that backendref can refer to a resource.
// This check depends on cross namespace reference and authorization to do so by referenceGrant if necessary.
func (gm GatewayManagerImpl) isNamespaceGranted(namespace string, backendRef store.BackendRef) (granted bool) {
	// If namespace of backendRef is specified ...
	if backendRef.Namespace != nil {
		ns, found := gm.k8sStore.Namespaces[*backendRef.Namespace]
		if !found || !ns.Relevant {
			gm.statusManager.SetRouteReasonBackendNotFound(fmt.Sprintf("backend '%s/%s' not found", utils.PointerDefaultValueIfNil(backendRef.Namespace), backendRef.Name))
			return
		}
		// .. we iterate over referenceGrants in this namespace.
		for _, referenceGrant := range ns.ReferenceGrants {
			fromGranted := false
			// If referenceGrant allows tcproutes from their namespace to be origin to attach ...
			for _, from := range referenceGrant.From {
				if from.Group == K8S_GATEWAY_GROUP && from.Kind == K8S_TCPROUTE_KIND && from.Namespace == namespace {
					fromGranted = true
					break
				}
			}
			// ... then check if it allows the target backendRef which must be a service potentially named.
			if fromGranted {
				for _, to := range referenceGrant.To {
					if to.Group == K8S_CORE_GROUP && to.Kind == K8S_SERVICE_KIND &&
						(to.Name == nil || *to.Name == backendRef.Name) {
						granted = true
						break
					}
				}
			}
		}
		if !granted {
			gm.statusManager.SetRouteReasonRefNotPermitted(fmt.Sprintf("backendref '%s/%s' not allowed by any referencegrant",
				*backendRef.Namespace, backendRef.Name))
		}
		return
	}
	return true
}

// addServersToRoute adds all the servers from the backendrefs from tcproute according validation rules.
func (gm GatewayManagerImpl) addServersToRoute(route store.TCPRoute) (reload bool, err error) {
	backendName := getBackendName(route)
	gm.haproxyClient.BackendServerDeleteAll(backendName)
	i := 0
	var servers []string
	defer func() {
		previousServers := gm.serversByBackend[backendName]
		reload = reload || !utils.EqualSliceStringsWithoutOrder(servers, previousServers)
		gm.serversByBackend[backendName] = servers
	}()
	for id, backendRef := range route.BackendRefs {
		if !gm.isBackendRefValid(backendRef) {
			continue
		}

		if !gm.isNamespaceGranted(route.Namespace, backendRef) {
			gm.statusManager.SetRouteReasonRefNotPermitted(fmt.Sprintf("backend '%s/%s' reference not allowed", utils.PointerDefaultValueIfNil(backendRef.Namespace), backendRef.Name))
			continue
		}

		nsBackendRef := backendRef.Namespace
		if nsBackendRef == nil {
			nsBackendRef = &route.Namespace
		}
		ns, found := gm.k8sStore.Namespaces[*nsBackendRef]
		if !found {
			gm.statusManager.SetRouteReasonBackendNotFound(fmt.Sprintf("backend '%s/%s' not found", *nsBackendRef, backendRef.Name))
			logger.Errorf("gwapi: unexisting namespace '%s' for backendRef number '%d' from tcp route '%s/%s'", *nsBackendRef, id, route.Namespace, route.Name)
			continue
		}
		service, found := ns.Services[backendRef.Name]
		if !found {
			gm.statusManager.SetRouteReasonBackendNotFound(fmt.Sprintf("backend '%s/%s' not found", *nsBackendRef, backendRef.Name))
			logger.Errorf("gwapi: unexisting endpoints '%s' for backendRef number '%d' from tcp route '%s/%s'", backendRef.Name, id, route.Namespace, route.Name)
			continue
		}
		var portName *string
		backendRefPort := int64(*backendRef.Port)
		for _, svcPort := range service.Ports {
			if svcPort.Port == backendRefPort {
				svcPortName := svcPort.Name
				portName = &svcPortName
				break
			}
		}
		if portName == nil {
			gm.statusManager.SetRouteReasonBackendNotFound(fmt.Sprintf("backend port '%s/%s' not found", *nsBackendRef, backendRef.Name))
			logger.Errorf("gwapi: unexisting port '%d' for backendRef '%spkg/gateways/gateways.go' number '%d' from tcp route '%s/%s'", backendRefPort, backendRef.Name, id, route.Namespace, route.Name)
			continue
		}
		slice, found := ns.Endpoints[backendRef.Name]
		if !found {
			gm.statusManager.SetRouteReasonBackendNotFound(fmt.Sprintf("backend '%s/%s' not found", *nsBackendRef, backendRef.Name))
			logger.Errorf("gwapi: unexisting endpoints '%s' for backendRef number '%d' from tcp route '%s/%s'", backendRef.Name, id, route.Namespace, route.Name)
			continue
		}

		for _, endpoints := range slice {
			if endpoints.Status == store.DELETED {
				continue
			}
			if port, found := endpoints.Ports[*portName]; found {
				for address := range port.Addresses {
					servers = append(servers, fmt.Sprintf("%s:%d", address, port.Port))
					err = gm.haproxyClient.BackendServerCreate(backendName, models.Server{
						Address:     address,
						Port:        &port.Port,
						Name:        fmt.Sprintf("SRV_%d", i+1),
						Maintenance: "disabled",
					})
					if err != nil {
						return
					}
					i++
				}
			}
		}
	}
	return
}

// getOurListenersFromTCPRoute computes the list of listeners the tcproute can be attached to according matching and authorizations rules.
func (gm GatewayManagerImpl) getOurListenersFromTCPRoute(tcproute store.TCPRoute) ([]store.Listener, error) {
	var errors utils.Errors
	listeners := []store.Listener{}
	// Iterates over parentRefs  which must be a gateway
	for i, parentRef := range tcproute.ParentRefs {
		gatewayNs := tcproute.Namespace
		if parentRef.Namespace != nil {
			gatewayNs = *parentRef.Namespace
		}
		ns, found := gm.k8sStore.Namespaces[gatewayNs]
		if !found {
			errors.Add(fmt.Errorf("gwapi: unexisting namespace '%s' in parentRef number '%d' from tcp route '%s/%s'", gatewayNs, i, tcproute.Namespace, tcproute.Name))
			continue
		}
		gw, found := ns.Gateways[parentRef.Name]
		if !found || gw == nil {
			errors.Add(fmt.Errorf("gwapi: unexisting gateway in parentRef '%s' from tcp route '%s/%s'", parentRef.Name, tcproute.Namespace, tcproute.Name))
			continue
		}
		if !gm.isGatewayManaged(*gw) || gw.Status == store.DELETED {
			continue
		}
		gm.statusManager.AddManagedParentRef(parentRef)
		// We found the gateway, let's see if there's a match.
		hasSectionName := parentRef.SectionName != nil
		for _, listener := range gw.Listeners {
			// if listener.Protocol != store.TCPProtocolType || (hasSectionName && listener.Name != *parentRef.SectionName) {
			if hasSectionName && listener.Name != *parentRef.SectionName {
				continue
			}
			// Does listener allow the route to be attached ?
			if !gm.isTCPRouteAllowedByListener(listener, tcproute.Namespace, gatewayNs, parentRef) {
				continue
			}
			// Does the listener have the expected name if provided ?
			if hasSectionName {
				if listener.Name == *parentRef.SectionName {
					listeners = append(listeners, listener)
					break
				}
			} else {
				listeners = append(listeners, listener)
			}
		}
	}
	return listeners, errors.Result()
}

// addRouteToListener attaches the route to the frontend.
func (gm GatewayManagerImpl) addRouteToListener(frontendName string, route store.TCPRoute, listener store.Listener) error {
	frontend, err := gm.haproxyClient.FrontendGet(frontendName)
	if err != nil {
		return err
	}
	frontend.DefaultBackend = getBackendName(route)
	errEdit := gm.haproxyClient.FrontendEdit(frontend)
	if errEdit == nil {
		// the counter of attached routes for listener status is incremented.
		gm.statusManager.IncrementRouteForListener(listener)
	}
	return errEdit
}

// isToBeDeleted check if the backend corresponding to the tcproute should be removed if existing.
func (gm GatewayManagerImpl) isToBeDeleted(tcproute store.TCPRoute) bool {
	noBackendRefs := len(tcproute.BackendRefs) == 0
	if noBackendRefs {
		logger.Warningf("gwapi: no backendrefs in tcproute '%s/%s'", tcproute.Namespace, tcproute.Name)
	}
	noParentRefs := len(tcproute.ParentRefs) == 0
	if noParentRefs {
		logger.Warningf("gwapi: no parentrefs in tcproute '%s/%s'", tcproute.Namespace, tcproute.Name)
	}

	return noBackendRefs || noParentRefs
}

// isGatewayManaged checks that the gateway refers to a managed gatewayclass.
func (gm GatewayManagerImpl) isGatewayManaged(gateway store.Gateway) bool {
	gwc := gm.k8sStore.GatewayClasses[gateway.GatewayClassName]
	if gwc == nil {
		logger.Errorf("gwapi: gateway class '%s' not found from gateway '%s/%s'", gateway.GatewayClassName, gateway.Namespace, gateway.Name)
		return false
	}
	return gwc.ControllerName == gm.k8sStore.GatewayControllerName
}

// isTCPRouteAllowedByListener checks if the tcproute can refer to the listener according listener's authorization rules.
func (gm GatewayManagerImpl) isTCPRouteAllowedByListener(listener store.Listener, routeNamespace, gatewayNamespace string, parentRef store.ParentRef) (allowed bool) {
	defer func() {
		if !allowed {
			gm.statusManager.SetRouteReasonNotAllowedByListeners(fmt.Sprintf("not allowed by listener '%s/%s/%s'", listener.GwNamespace, listener.GwName, listener.Name), parentRef)
		}
	}()

	if listener.AllowedRoutes == nil {
		// If the listener has no restrictions rules simply checks that the route and the listener (gateway) are in the same namespace.
		return routeNamespace != gatewayNamespace
	}

	gkAllowed := listener.AllowedRoutes.Kinds == nil || len(listener.AllowedRoutes.Kinds) == 0
	for _, kind := range listener.AllowedRoutes.Kinds {
		if (kind.Group != nil && *kind.Group != v1alpha2.GroupName) || kind.Kind != K8S_TCPROUTE_KIND {
			continue
		}
		gkAllowed = true
	}
	if !gkAllowed {
		return false
	}
	allowedRoutesNamespaces := listener.AllowedRoutes.Namespaces

	if allowedRoutesNamespaces != nil {
		from := allowedRoutesNamespaces.From
		if from == nil {
			v := (string)(v1alpha2.NamespacesFromSame)
			from = &v
		}
		if *from == "Same" {
			return routeNamespace == gatewayNamespace
		}
		if *from == (string)(v1alpha2.NamespacesFromSelector) {
			if allowedRoutesNamespaces.Selector == nil {
				return false
			}
			selector, err := v1.LabelSelectorAsSelector(k8s.ConvertToK8SLabelSelector(allowedRoutesNamespaces.Selector))
			if err != nil {
				return false
			}
			ns := gm.k8sStore.Namespaces[routeNamespace]
			if ns == nil {
				return false
			}
			if !selector.Matches(labels.Set(ns.Labels)) {
				return false
			}
		}
	}
	return true
}

// manageGatewayClass has the sole purpose of updating status for matching gatewayclasses.
func (gm GatewayManagerImpl) manageGatewayClass() {
	for _, gatewayclass := range gm.k8sStore.GatewayClasses {
		if gatewayclass.ControllerName == gm.k8sStore.GatewayControllerName &&
			(gatewayclass.Status == store.ADDED || gatewayclass.Status == store.MODIFIED) {
			gm.statusManager.SetGatewayClassConditionStatusAccepted(*gatewayclass)
		}
	}
}

// getBackendName provides backend name from tcproute attributes.
func getBackendName(tcproute store.TCPRoute) string {
	return tcproute.Namespace + "_" + tcproute.Name
}

// getFrontendName provides frontend name from the listener attributes.
func getFrontendName(listener store.Listener) string {
	return listener.GwNamespace + "-" + listener.GwName + "-" + listener.Name
}

// getGatewayName provides frontend name from the listener attributes.
func getGatewayName(gateway store.Gateway) string {
	return gateway.Namespace + "-" + gateway.Name
}

func extractNameFromListener(l store.Listener) string {
	return l.Name
}

// resetStatuses sets the status of every processed resource to EMPTY.
func (gm *GatewayManagerImpl) resetStatuses() {
	// gatewayclass
	for _, gatewayclass := range gm.k8sStore.GatewayClasses {
		if gatewayclass.ControllerName == gm.k8sStore.GatewayControllerName &&
			(gatewayclass.Status == store.ADDED || gatewayclass.Status == store.MODIFIED) {
			gatewayclass.Status = store.EMPTY
		}
	}

	// gateway
	for _, ns := range gm.k8sStore.Namespaces {
		if !ns.Relevant {
			logger.Debugf("gwapi: skipping namespace '%s'", ns.Name)
			continue
		}
		for _, gw := range ns.Gateways {
			if gw.Status == store.ADDED || gw.Status == store.MODIFIED {
				gw.Status = store.EMPTY
			}
		}
	}

	// tcproutes
	for _, ns := range gm.k8sStore.Namespaces {
		if !ns.Relevant {
			logger.Debugf("gwapi: skipping namespace '%s'", ns.Name)
			continue
		}
		for _, tcproute := range ns.TCPRoutes {
			if tcproute.Status == store.ADDED || tcproute.Status == store.MODIFIED {
				tcproute.Status = store.EMPTY
			}
		}
	}
}
