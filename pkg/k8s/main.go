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
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	k8sinformers "k8s.io/client-go/informers"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	crclientset "github.com/haproxytech/kubernetes-ingress/crs/generated/clientset/versioned"
	crinformers "github.com/haproxytech/kubernetes-ingress/crs/generated/informers/externalversions"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	crdclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	errGw "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayclientset "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	scheme "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/scheme"
	gatewaynetworking "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
)

var logger = utils.GetK8sLogger()

// TRACE_API outputs all k8s events received from k8s API
const (
	CRSGroupVersionV1alpha1 = "core.haproxy.org/v1alpha1"
	CRSGroupVersionV1alpha2 = "core.haproxy.org/v1alpha2"
	GATEWAY_API_VERSION     = "v0.5.1" //nolint:golint,stylecheck
)

var ErrIgnored = errors.New("ignored resource")

type K8s interface {
	GetRestClientset() client.Client
	GetClientset() *k8sclientset.Clientset
	MonitorChanges(eventChan chan SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs, gatewayAPIInstalled bool)
	IsGatewayAPIInstalled(gatewayControllerName string) bool
}

// A Custom Resource interface
// Any CR should be able to provide its kind, its kubernetes Informer
// and a method to process the update of a CR
type CR interface {
	GetKind() string
	GetInformer(chan SyncDataEvent, crinformers.SharedInformerFactory) cache.SharedIndexInformer
}

// k8s is structure with all data required to synchronize with k8s
type k8s struct {
	gatewayRestClient      client.Client
	crs                    map[string]CR
	builtInClient          *k8sclientset.Clientset
	crClient               *crclientset.Clientset
	publishSvc             *utils.NamespaceValue
	gatewayClient          *gatewayclientset.Clientset
	podPrefix              string
	podNamespace           string
	whiteListedNS          []string
	syncPeriod             time.Duration
	cacheResyncPeriod      time.Duration
	disableSvcExternalName bool // CVE-2021-25740
	crdClient              *crdclientset.Clientset
	gatewayAPIInstalled    bool
}

func New(osArgs utils.OSArgs, whitelist map[string]struct{}, publishSvc *utils.NamespaceValue) K8s { //nolint:ireturn
	logger.SetLevel(osArgs.LogLevel.LogLevel)
	restconfig, err := getRestConfig(osArgs)
	logger.Panic(err)
	builtInClient := k8sclientset.NewForConfigOrDie(restconfig)
	if k8sVersion, errVer := builtInClient.Discovery().ServerVersion(); errVer != nil {
		logger.Panicf("Unable to get Kubernetes version: %v\n", errVer)
	} else {
		logger.Printf("Running on Kubernetes version: %s %s", k8sVersion.String(), k8sVersion.Platform)
	}

	gatewayClient, err := gatewayclientset.NewForConfig(restconfig)
	if err != nil {
		logger.Print("Gateway API not present")
	}
	gatewayRestClient, err := client.New(restconfig, client.Options{
		Scheme: scheme.Scheme,
	})
	if err != nil {
		logger.Print("Gateway API not present")
	}

	crdClient, err := crdclientset.NewForConfig(restconfig)
	if err != nil {
		logger.Error("CRD API client not present")
	}

	prefix, _ := utils.GetPodPrefix(os.Getenv("POD_NAME"))
	k := k8s{
		builtInClient:          builtInClient,
		crClient:               crclientset.NewForConfigOrDie(restconfig),
		crs:                    map[string]CR{},
		whiteListedNS:          getWhitelistedNS(whitelist, osArgs.ConfigMap.Namespace),
		publishSvc:             publishSvc,
		podNamespace:           os.Getenv("POD_NAMESPACE"),
		podPrefix:              prefix,
		syncPeriod:             osArgs.SyncPeriod,
		cacheResyncPeriod:      osArgs.CacheResyncPeriod,
		disableSvcExternalName: osArgs.DisableServiceExternalName,
		gatewayClient:          gatewayClient,
		gatewayRestClient:      gatewayRestClient,
		crdClient:              crdClient,
	}
	// alpha1 is deprecated
	k.registerCoreCR(NewGlobalCRV1Alpha1(), CRSGroupVersionV1alpha1)
	k.registerCoreCR(NewDefaultsCRV1Alpha1(), CRSGroupVersionV1alpha1)
	k.registerCoreCR(NewBackendCRV1Alpha1(), CRSGroupVersionV1alpha1)

	k.registerCoreCR(NewGlobalCR(), CRSGroupVersionV1alpha2)
	k.registerCoreCR(NewDefaultsCR(), CRSGroupVersionV1alpha2)
	k.registerCoreCR(NewBackendCR(), CRSGroupVersionV1alpha2)
	return k
}

func (k k8s) GetRestClientset() client.Client {
	return k.gatewayRestClient
}

func (k k8s) GetClientset() *k8sclientset.Clientset {
	return k.builtInClient
}

func (k k8s) MonitorChanges(eventChan chan SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs, gatewayAPIInstalled bool) {
	informersSynced := &[]cache.InformerSynced{}
	k.runPodInformer(eventChan, stop, informersSynced)
	for _, namespace := range k.whiteListedNS {
		k.runInformers(eventChan, stop, namespace, informersSynced)
		k.runCRInformers(eventChan, stop, namespace, informersSynced)
		if gatewayAPIInstalled {
			k.runInformersGwAPI(eventChan, stop, namespace, informersSynced)
		}
	}

	if !cache.WaitForCacheSync(stop, *informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	syncPeriod := k.syncPeriod
	logger.Debugf("Executing syncPeriod every %s", syncPeriod.String())
	for {
		time.Sleep(syncPeriod)
		eventChan <- SyncDataEvent{SyncType: COMMAND}
	}
}

func (k k8s) registerCoreCR(cr CR, groupVersion string) {
	resources, err := k.crClient.DiscoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return
	}
	logger.Debugf("Custom API %s available", groupVersion)
	kindName := cr.GetKind()
	for _, resource := range resources.APIResources {
		if resource.Kind == kindName {
			k.crs[resources.GroupVersion+" - "+kindName] = cr
			logger.Infof("%s CR defined in API %s", kindName, resources.GroupVersion)
			break
		}
	}
}

func (k k8s) runCRInformers(eventChan chan SyncDataEvent, stop chan struct{}, namespace string, informersSynced *[]cache.InformerSynced) {
	informerFactory := crinformers.NewSharedInformerFactoryWithOptions(k.crClient, k.cacheResyncPeriod, crinformers.WithNamespace(namespace))
	for _, cr := range k.crs {
		informer := cr.GetInformer(eventChan, informerFactory)
		go informer.Run(stop)
		*informersSynced = append(*informersSynced, informer.HasSynced)
	}
}

func (k k8s) runInformers(eventChan chan SyncDataEvent, stop chan struct{}, namespace string, informersSynced *[]cache.InformerSynced) {
	factory := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod, k8sinformers.WithNamespace(namespace))
	// Core.V1 Resources
	nsi := k.getNamespaceInfomer(eventChan, factory)
	go nsi.Run(stop)
	svci := k.getServiceInformer(eventChan, factory)
	go svci.Run(stop)
	seci := k.getSecretInformer(eventChan, factory)
	go seci.Run(stop)
	cmi := k.getConfigMapInformer(eventChan, factory)
	go cmi.Run(stop)

	*informersSynced = append(*informersSynced, svci.HasSynced, nsi.HasSynced, seci.HasSynced, cmi.HasSynced)

	// Ingress and IngressClass Resources
	ii, ici := k.getIngressInformers(eventChan, factory)
	if ii == nil {
		logger.Panic("Ingress Resource not supported in this cluster")
	} else {
		go ii.Run(stop)
	}
	*informersSynced = append(*informersSynced, ii.HasSynced)
	if ici != nil {
		go ici.Run(stop)
		*informersSynced = append(*informersSynced, ici.HasSynced)
	}

	// Endpoints and EndpointSlices Resources discovery.k8s.io
	epsi := k.getEndpointSliceInformer(eventChan, factory)
	if epsi != nil {
		go epsi.Run(stop)
		*informersSynced = append(*informersSynced, epsi.HasSynced)
	}
	if epsi == nil || !k.endpointsMirroring() {
		epi := k.getEndpointsInformer(eventChan, factory)
		go epi.Run(stop)
		*informersSynced = append(*informersSynced, epi.HasSynced)
	}
}

func (k k8s) runInformersGwAPI(eventChan chan SyncDataEvent, stop chan struct{}, namespace string, informersSynced *[]cache.InformerSynced) {
	factory := gatewaynetworking.NewSharedInformerFactoryWithOptions(k.gatewayClient, k.cacheResyncPeriod, gatewaynetworking.WithNamespace(namespace))
	gwclassInf := k.getGatewayClassesInformer(eventChan, factory)
	if gwclassInf != nil {
		go gwclassInf.Run(stop)
		*informersSynced = append(*informersSynced, gwclassInf.HasSynced)
	}
	gwInf := k.getGatewayInformer(eventChan, factory)
	if gwInf != nil {
		go gwInf.Run(stop)
		*informersSynced = append(*informersSynced, gwInf.HasSynced)
	}
	tcprouteInf := k.getTCPRouteInformer(eventChan, factory)
	if tcprouteInf != nil {
		go tcprouteInf.Run(stop)
		*informersSynced = append(*informersSynced, tcprouteInf.HasSynced)
	}
	referenceGrantInf := k.getReferenceGrantInformer(eventChan, factory)
	if referenceGrantInf != nil {
		go referenceGrantInf.Run(stop)
		*informersSynced = append(*informersSynced, referenceGrantInf.HasSynced)
	}
}

func (k k8s) runPodInformer(eventChan chan SyncDataEvent, stop chan struct{}, informersSynced *[]cache.InformerSynced) {
	if k.podPrefix != "" {
		pi := k.getPodInformer(k.podNamespace, k.podPrefix, k.cacheResyncPeriod, eventChan)
		go pi.Run(stop)
		*informersSynced = append(*informersSynced, pi.HasSynced)
	}
}

// if EndpointSliceMirroring is supported we can just watch endpointSlices
// Ref: https://github.com/kubernetes/enhancements/tree/master/keps/sig-network/0752-endpointslices#endpointslicemirroring-controller
func (k k8s) endpointsMirroring() bool {
	var major, minor int
	var err error
	version, _ := k.builtInClient.ServerVersion()
	if version == nil {
		return false
	}
	major, err = strconv.Atoi(version.Major)
	if err != nil {
		return false
	}
	minor, err = strconv.Atoi(version.Minor)
	if err != nil {
		return false
	}
	if major == 1 && minor < 19 {
		return false
	}
	return true
}

func getRestConfig(osArgs utils.OSArgs) (restConfig *rest.Config, err error) {
	if osArgs.External {
		kubeconfig := filepath.Join(utils.HomeDir(), ".kube", "config")
		if osArgs.KubeConfig != "" {
			kubeconfig = osArgs.KubeConfig
		}
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		restConfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return
	}
	restConfig.WarningHandler = logger
	return restConfig, err
}

func getWhitelistedNS(whitelist map[string]struct{}, cfgMapNS string) []string {
	if len(whitelist) == 0 {
		return []string{""}
	}
	// Add one because of potential whitelisting of configmap namespace
	namespaces := []string{}
	for ns := range whitelist {
		namespaces = append(namespaces, ns)
	}
	if _, ok := whitelist[cfgMapNS]; !ok {
		namespaces = append(namespaces, cfgMapNS)
		logger.Warningf("configmap Namespace '%s' not whitelisted. Whitelisting it anyway", cfgMapNS)
	}
	logger.Infof("Whitelisted Namespaces: %s", namespaces)
	return namespaces
}

func (k k8s) IsGatewayAPIInstalled(gatewayControllerName string) (installed bool) {
	installed = true
	defer func() {
		k.gatewayAPIInstalled = installed
	}()
	gatewayCrd, err := k.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "gateways.gateway.networking.k8s.io", metav1.GetOptions{})
	if err != nil {
		var errStatus *errGw.StatusError
		if !errors.As(err, &errStatus) || errStatus.ErrStatus.Code != 404 {
			logger.Error(err)
			return false
		}
	}

	if gatewayCrd.Name == "" {
		if gatewayControllerName != "" {
			logger.Errorf("No gateway api is installed, please install experimental yaml version %s", GATEWAY_API_VERSION)
		}
		return false
	}

	log := logger.Warningf
	if gatewayControllerName != "" {
		log = logger.Errorf
	}

	version := gatewayCrd.Annotations["gateway.networking.k8s.io/bundle-version"]
	if version != GATEWAY_API_VERSION {
		log("Unsupported version '%s' of gateway api is installed, please install experimental yaml version %s", version, GATEWAY_API_VERSION)
		installed = false
	}

	// gatewayCrd is not nil so gateway API is present
	tcprouteCrd, err := k.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "tcproutes.gateway.networking.k8s.io", metav1.GetOptions{})
	if tcprouteCrd == nil || err != nil {
		log("No tcproute crd is installed, please install experimental yaml version %s", GATEWAY_API_VERSION)
		installed = false
	}
	return
}
