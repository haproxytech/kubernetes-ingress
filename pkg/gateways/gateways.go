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
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

var logger = utils.GetLogger()

// Gateway management
const (
	K8S_CORE_GROUP       = ""
	K8S_NETWORKING_GROUP = networkingv1.GroupName
	K8S_GATEWAY_GROUP    = v1beta1.GroupName
	K8S_TCPROUTE_KIND    = "TCPRoute"
	K8S_SERVICE_KIND     = "Service"
)

type GatewayManager interface {
	ManageGateway() bool
}

func New(k8sStore store.K8s,
	haproxyClient api.HAProxyClient,
	osArgs utils.OSArgs) GatewayManager {
	return GatewayManagerImpl{
		k8sStore:      k8sStore,
		haproxyClient: haproxyClient,
		osArgs:        osArgs,
		frontends:     map[string]struct{}{},
	}
}

type GatewayManagerImpl struct {
	k8sStore      store.K8s
	haproxyClient api.HAProxyClient
	osArgs        utils.OSArgs
	frontends     map[string]struct{}
}

func (gm GatewayManagerImpl) ManageGateway() bool {
	gm.clean()
	listenersReload := gm.manageListeners()
	if listenersReload {
		logger.Debug("gwapi: gateway event, haproxy reload required.")
	}
	tcproutesReload := gm.manageTCPRoutes()
	if tcproutesReload {
		logger.Debug("gwapi: tcproute event, haproxy reload required.")
	}
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
func (gm GatewayManagerImpl) manageListeners() (reload bool) {
	for _, ns := range gm.k8sStore.Namespaces {
		if !ns.Relevant {
			logger.Debugf("gwapi: skipping namespace '%s", ns.Name)
			continue
		}
		for _, gw := range ns.Gateways {
			reload = reload || gw.Status != store.EMPTY
			if gw.Status == store.DELETED {
				delete(ns.Gateways, gw.Name)
				logger.Warningf("gwapi: deleted gateway'%s/%s'", gw.Namespace, gw.Name)
				continue
			}

			gw.Status = store.EMPTY
			// We don't care about gatewayclass with deleted status because they're already removed by store handler.
			gwc, gwcfound := gm.k8sStore.GatewayClasses[gw.GatewayClassName]
			if !gwcfound || gwc.ControllerName != gm.k8sStore.GatewayControllerName {
				continue
			}
			logger.Error(gm.createAllListeners(*gw))
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
			reload = reload || tcproute.Status != store.EMPTY
			// Nothing to do to delete the corresponding backend as an automatic mechanism will remove it.
			if gm.isToBeDeleted(*tcproute) {
				logger.Debugf("gwapi: backend of tcproute '%s/%s' is/will be deleted", tcproute.Namespace, tcproute.Name)
				continue
			}
			if tcproute.Status == store.DELETED {
				delete(ns.TCPRoutes, tcproute.Name)
				continue
			}

			tcproute.Status = store.EMPTY
			// If not called on the route, the afferent backend will be automatically deleted.
			errBackendCreate := gm.haproxyClient.BackendCreateIfNotExist(
				models.Backend{
					Name:          gm.getBackendName(*tcproute),
					Mode:          "tcp",
					DefaultServer: &models.DefaultServer{Check: "enabled"},
				})
			if errBackendCreate != nil {
				logger.Error(errBackendCreate)
				continue
			}
			// Adds the servers to the backends
			logger.Error(gm.addServersToRoute(*tcproute))
			// Get the list of listeners (frontends) this tcproute (backend) wants to be attached to.
			listeners, errListeners := gm.getListenersFromTcpRoute(*tcproute)
			logger.Error(errListeners)
			for _, listener := range listeners {
				if listener.Protocol != store.TCPProtocolType {
					continue
				}

				frontendName := gm.getFrontendName(listener)
				if rbl, ok := routesByListeners[frontendName]; ok {
					rbl.P2 = append(rbl.P2, *tcproute)
				} else {
					pair := utils.NewPair(listener, store.TCPRoutes{*tcproute})
					routesByListeners[frontendName] = &pair
				}
			}
		}

	}

	// Sorts the list of routes by listener and then attaches the first one to the listener.
	for fontendName, rbl := range routesByListeners {
		if len(rbl.P2) == 0 {
			continue
		}
		sort.SliceStable(rbl.P2, rbl.P2.Less)
		logger.Error(gm.addRouteToListener(fontendName, rbl.P2[0]))
	}
	return
}

// createAllListeners creates all TCP frontends from gateway and their bindings.
func (gm GatewayManagerImpl) createAllListeners(gateway store.Gateway) error {
	var errs utils.Errors
	for _, listener := range gateway.Listeners {
		if listener.Protocol != store.TCPProtocolType {
			continue
		}
		frontendName := gm.getFrontendName(listener)
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
		return false
	}
	if backendRef.Kind != nil && *backendRef.Kind != K8S_SERVICE_KIND {
		return false
	}
	if backendRef.Port == nil {
		return false
	}
	return true
}

// isNamespaceGranted checks that backendref can refer to a resource.
// This check depends on cross namespace reference and authorization to do so by referenceGrant if necessary.
func (gm GatewayManagerImpl) isNamespaceGranted(namespace string, backendRef store.BackendRef) bool {
	// If namespace of backendRef is specified ...
	if backendRef.Namespace != nil {
		ns, found := gm.k8sStore.Namespaces[*backendRef.Namespace]
		if !found {
			return false
		}
		if !ns.Relevant {
			return false
		}
		granted := false
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
		return granted
	}
	return true
}

// addServersToRoute adds all the servers from the backendrefs from tcproute according validation rules.
func (gm GatewayManagerImpl) addServersToRoute(route store.TCPRoute) error {

	gm.haproxyClient.BackendServerDeleteAll(gm.getBackendName(route))
	i := 0
	for id, backendRef := range route.BackendRefs {

		if !gm.isBackendRefValid(backendRef) {
			continue
		}

		if !gm.isNamespaceGranted(route.Namespace, backendRef) {
			continue
		}

		nsBackendRef := backendRef.Namespace
		if nsBackendRef == nil {
			nsBackendRef = &route.Namespace
		}
		ns, found := gm.k8sStore.Namespaces[*nsBackendRef]
		if !found {
			logger.Errorf("gwapi: unexisting namespace '%s' for backendRef number '%d' from tcp route '%s/%s'", *nsBackendRef, id, route.Namespace, route.Name)
			continue
		}
		slice, found := ns.Endpoints[backendRef.Name]
		if !found {
			logger.Errorf("gwapi: unexisting endpoints '%s' for backendRef number '%d' from tcp route '%s/%s'", backendRef.Name, id, route.Namespace, route.Name)
			continue
		}
		for _, endpoints := range slice {
			if endpoints.Status == store.DELETED {
				continue
			}
			for _, port := range endpoints.Ports {
				if port.Port != int64(*backendRef.Port) {
					continue
				}
				for address := range port.Addresses {
					errSrv := gm.haproxyClient.BackendServerCreate(route.Namespace+"_"+route.Name, models.Server{
						Address:     address,
						Port:        &port.Port,
						Name:        fmt.Sprintf("SRV_%d", i+1),
						Maintenance: "disabled",
					})
					if errSrv != nil {
						return errSrv
					}
					i++
				}
			}
		}

	}
	return nil
}

// getListenersFromTcpRoute computes the list of listeners the tcproute can be attached to according matching and authorizations rules.
func (gm GatewayManagerImpl) getListenersFromTcpRoute(tcproute store.TCPRoute) ([]store.Listener, error) {
	var errors utils.Errors
	listeners := []store.Listener{}
	// Iterates over parentRefs  which must be a gateway
	for i, parentRef := range tcproute.ParentRefs {
		gatewayNs := tcproute.Namespace
		if parentRef.Namespace != nil {
			gatewayNs = *(*string)(parentRef.Namespace)
		}
		ns, found := gm.k8sStore.Namespaces[gatewayNs]
		if !found {
			errors.Add(fmt.Errorf("gwapi: unexisting namespace '%s' in parentRef number '%d' from tcp route '%s/%s'", gatewayNs, i, tcproute.Namespace, tcproute.Name))
			continue
		}
		gw, found := ns.Gateways[parentRef.Name]
		if !found || gw == nil {
			errors.Add(fmt.Errorf("gwapi: unexisting gateway in parentRef number '%d' from tcp route '%s/%s'", i, tcproute.Namespace, tcproute.Name))
			continue
		}
		if !gm.isGatewayManaged(*gw) || gw.Status == store.DELETED {
			continue
		}
		// We found the gateway, let's see if there's a match.
		hasSectionName := parentRef.SectionName != nil
		for _, listener := range gw.Listeners {
			// Does listener allow the route to be attached ?
			if !gm.isTCPRouteAllowedByListener(listener, tcproute.Namespace, gatewayNs) {
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
func (gm GatewayManagerImpl) addRouteToListener(frontendName string, route store.TCPRoute) error {

	frontend, err := gm.haproxyClient.FrontendGet(frontendName)
	if err != nil {
		return err
	}
	frontend.DefaultBackend = gm.getBackendName(route)
	return gm.haproxyClient.FrontendEdit(frontend)
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
func (gm GatewayManagerImpl) isTCPRouteAllowedByListener(listener store.Listener, routeNamespace, gatewayNamespace string) bool {
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
		if *from == "Same" && routeNamespace != gatewayNamespace {
			return false
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

// getBackendName provides backend name from tcproute attributes.
func (gm GatewayManagerImpl) getBackendName(tcproute store.TCPRoute) string {
	return tcproute.Namespace + "_" + tcproute.Name
}

// getFrontendName provides frontend name from the listener attributes.
func (gm GatewayManagerImpl) getFrontendName(listener store.Listener) string {
	return listener.GwNamespace + "-" + listener.GwName + "-" + listener.Name
}
