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
	"strings"
	"time"

	k8sinformers "k8s.io/client-go/informers"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	crclientsetv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/clientset/versioned"
	crinformersv1 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v1/informers/externalversions"
	crclientsetv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/clientset/versioned"
	crinformersv3 "github.com/haproxytech/kubernetes-ingress/crs/generated/api/ingress/v3/informers/externalversions"

	k8ssync "github.com/haproxytech/kubernetes-ingress/pkg/k8s/sync"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	crdclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/fields"

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
	CRSGroupVersionV1   = "ingress.v1.haproxy.org/v1"
	CRSGroupVersionV3   = "ingress.v3.haproxy.org/v3"
	GATEWAY_API_VERSION = "v0.5.1" //nolint:golint,stylecheck
)

var ErrIgnored = errors.New("ignored resource")

type K8s interface {
	GetRestClientset() client.Client
	GetClientset() *k8sclientset.Clientset
	MonitorChanges(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs, gatewayAPIInstalled bool)
	IsGatewayAPIInstalled(gatewayControllerName string) bool
}

// A Custom Resource interface
// Any CR should be able to provide its kind, its kubernetes Informer
// and a method to process the update of a CR
type CRKind interface {
	GetKind() string
}
type CRV1 interface {
	CRKind
	GetInformerV1(chan k8ssync.SyncDataEvent, crinformersv1.SharedInformerFactory) cache.SharedIndexInformer //nolint:inamedparam
}

type CRV3 interface {
	CRKind
	GetInformerV3(chan k8ssync.SyncDataEvent, crinformersv3.SharedInformerFactory, utils.OSArgs) cache.SharedIndexInformer //nolint:inamedparam
}

// k8s is structure with all data required to synchronize with k8s
type k8s struct {
	gatewayRestClient      client.Client
	crsV1                  map[string]CRV1
	crsV3                  map[string]CRV3
	builtInClient          *k8sclientset.Clientset
	crClientV1             *crclientsetv1.Clientset
	crClientV3             *crclientsetv3.Clientset
	apiExtensionsClient    *crdclientset.Clientset
	publishSvc             *utils.NamespaceValue
	gatewayClient          *gatewayclientset.Clientset
	crdClient              *crdclientset.Clientset
	podPrefix              string
	podNamespace           string
	whiteListedNS          []string
	syncPeriod             time.Duration
	initialSyncPeriod      time.Duration
	cacheResyncPeriod      time.Duration
	disableSvcExternalName bool // CVE-2021-25740
	gatewayAPIInstalled    bool
}

func New(osArgs utils.OSArgs, whitelist map[string]struct{}, publishSvc *utils.NamespaceValue) K8s { //nolint:ireturn
	logger.SetLevel(osArgs.LogLevel.LogLevel)
	restconfig, err := GetRestConfig(osArgs)
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
		crClientV1:             crclientsetv1.NewForConfigOrDie(restconfig),
		crClientV3:             crclientsetv3.NewForConfigOrDie(restconfig),
		apiExtensionsClient:    crdclientset.NewForConfigOrDie(restconfig),
		crsV1:                  map[string]CRV1{},
		crsV3:                  map[string]CRV3{},
		whiteListedNS:          getWhitelistedNS(whitelist, osArgs.ConfigMap.Namespace),
		publishSvc:             publishSvc,
		podNamespace:           os.Getenv("POD_NAMESPACE"),
		podPrefix:              prefix,
		syncPeriod:             osArgs.SyncPeriod,
		initialSyncPeriod:      osArgs.InitialSyncPeriod,
		cacheResyncPeriod:      osArgs.CacheResyncPeriod,
		disableSvcExternalName: osArgs.DisableServiceExternalName,
		gatewayClient:          gatewayClient,
		gatewayRestClient:      gatewayRestClient,
		crdClient:              crdClient,
	}

	// ingress/v1 is deprecated
	k.registerCoreCRV1(NewGlobalCRV1())
	k.registerCoreCRV1(NewDefaultsCRV1())
	k.registerCoreCRV1(NewBackendCRV1())
	k.registerCoreCRV1(NewTCPCRV1())

	k.registerCoreCRV3(NewGlobalCRV3())
	k.registerCoreCRV3(NewDefaultsCRV3())
	k.registerCoreCRV3(NewBackendCRV3())
	k.registerCoreCRV3(NewTCPCRV3())
	return k
}

func (k k8s) GetRestClientset() client.Client {
	return k.gatewayRestClient
}

func (k k8s) GetClientset() *k8sclientset.Clientset {
	return k.builtInClient
}

func (k k8s) MonitorChanges(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, osArgs utils.OSArgs, gatewayAPIInstalled bool) {
	informersSynced := &[]cache.InformerSynced{}
	k.runPodInformer(eventChan, stop, informersSynced)
	for _, namespace := range k.whiteListedNS {
		k.runInformers(eventChan, stop, namespace, informersSynced, osArgs)
		k.runCRInformers(eventChan, stop, namespace, informersSynced, k.crsV1, k.crsV3, osArgs)
		if gatewayAPIInstalled {
			k.runInformersGwAPI(eventChan, stop, namespace, informersSynced)
		}
	}
	// check if we need to also watch CRS creation (in case not all alpha2 definitions are already installed)
	k.RunCRSCreationMonitoring(eventChan, stop, osArgs)

	if !cache.WaitForCacheSync(stop, *informersSynced...) {
		logger.Panic("Caches are not populated due to an underlying error, cannot run the Ingress Controller")
	}

	syncPeriod := k.syncPeriod
	initialSyncPeriod := k.initialSyncPeriod
	logger.Debugf("Executing first transaction after %s", initialSyncPeriod.String())
	logger.Debugf("Executing new transaction every %s", syncPeriod.String())
	time.Sleep(k.initialSyncPeriod)
	eventChan <- k8ssync.SyncDataEvent{SyncType: k8ssync.COMMAND}
	for {
		time.Sleep(syncPeriod)
		ep := make(chan struct{})
		eventChan <- k8ssync.SyncDataEvent{
			SyncType:       k8ssync.COMMAND,
			EventProcessed: ep,
		}
		<-ep
	}
}

func (k k8s) registerCoreCRV1(cr CRV1) {
	groupVersion := CRSGroupVersionV1
	resources, err := k.crClientV1.DiscoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return
	}
	logger.Debugf("Custom API %s available", groupVersion)
	kindName := cr.GetKind()
	groupVersion = strings.Split(resources.GroupVersion, "/")[0]
	for _, resource := range resources.APIResources {
		if resource.Kind == kindName {
			k.crsV1[groupVersion+" - "+kindName] = cr
			logger.Infof("%s CR defined in API %s", kindName, resources.GroupVersion)
			break
		}
	}
}

func (k k8s) registerCoreCRV3(cr CRV3) {
	groupVersion := CRSGroupVersionV3
	resources, err := k.crClientV3.DiscoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return
	}
	logger.Debugf("Custom API %s available", groupVersion)
	kindName := cr.GetKind()
	groupVersion = strings.Split(resources.GroupVersion, "/")[0]
	for _, resource := range resources.APIResources {
		if resource.Kind == kindName {
			k.crsV3[groupVersion+" - "+kindName] = cr
			logger.Infof("%s CR defined in API %s", kindName, resources.GroupVersion)
			break
		}
	}
}

func (k k8s) runCRInformers(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, namespace string,
	informersSynced *[]cache.InformerSynced, crsV1 map[string]CRV1, crsV3 map[string]CRV3,
	osArgs utils.OSArgs,
) {
	informerFactoryV3 := crinformersv3.NewSharedInformerFactoryWithOptions(k.crClientV3, k.cacheResyncPeriod, crinformersv3.WithNamespace(namespace))
	informerFactoryV1 := crinformersv1.NewSharedInformerFactoryWithOptions(k.crClientV1, k.cacheResyncPeriod, crinformersv1.WithNamespace(namespace))

	for _, cr := range crsV1 {
		informer := cr.GetInformerV1(eventChan, informerFactoryV1)
		go informer.Run(stop)
		*informersSynced = append(*informersSynced, informer.HasSynced)
	}
	for _, cr := range crsV3 {
		informer := cr.GetInformerV3(eventChan, informerFactoryV3, osArgs)
		go informer.Run(stop)
		*informersSynced = append(*informersSynced, informer.HasSynced)
	}
}

func (k k8s) runConfigMapInformers(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, informersSynced *[]cache.InformerSynced, configMap utils.NamespaceValue) {
	if configMap.Name != "" {
		fieldSelector := fields.OneTermEqualSelector("metadata.name", configMap.Name).String()
		factory := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod, k8sinformers.WithNamespace(configMap.Namespace),
			k8sinformers.WithTweakListOptions(func(opts *metav1.ListOptions) {
				opts.FieldSelector = fieldSelector
			}))

		cmi := k.getConfigMapInformer(eventChan, factory)
		go cmi.Run(stop)
		*informersSynced = append(*informersSynced, cmi.HasSynced)
	}
}

func (k k8s) runInformers(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, namespace string, informersSynced *[]cache.InformerSynced, osArgs utils.OSArgs) {
	factory := k8sinformers.NewSharedInformerFactoryWithOptions(k.builtInClient, k.cacheResyncPeriod, k8sinformers.WithNamespace(namespace))
	// Core.V1 Resources
	nsi := k.getNamespaceInfomer(eventChan, factory)
	go nsi.Run(stop)
	svci := k.getServiceInformer(eventChan, factory)
	go svci.Run(stop)
	seci := k.getSecretInformer(eventChan, factory)
	go seci.Run(stop)
	*informersSynced = append(*informersSynced, svci.HasSynced, nsi.HasSynced, seci.HasSynced)

	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMap)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapTCPServices)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapErrorFiles)
	k.runConfigMapInformers(eventChan, stop, informersSynced, osArgs.ConfigMapPatternFiles)

	// Ingress and IngressClass Resources
	ii, ici := k.getIngressInformers(eventChan, factory, osArgs)
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

func (k k8s) runInformersGwAPI(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, namespace string, informersSynced *[]cache.InformerSynced) {
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

func (k k8s) runPodInformer(eventChan chan k8ssync.SyncDataEvent, stop chan struct{}, informersSynced *[]cache.InformerSynced) {
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

func GetRestConfig(osArgs utils.OSArgs) (restConfig *rest.Config, err error) {
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
