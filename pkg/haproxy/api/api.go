package api

import (
	"context"

	clientnative "github.com/haproxytech/client-native/v3"
	"github.com/haproxytech/client-native/v3/configuration"
	cfgoptions "github.com/haproxytech/client-native/v3/configuration/options"
	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/client-native/v3/options"
	"github.com/haproxytech/client-native/v3/runtime"
	runtimeoptions "github.com/haproxytech/client-native/v3/runtime/options"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type HAProxyClient interface { //nolint:interfacebloat
	APIStartTransaction() error
	APICommitTransaction() error
	APIDisposeTransaction()
	BackendsGet() (models.Backends, error)
	BackendGet(backendName string) (*models.Backend, error)
	BackendCreate(backend models.Backend) error
	BackendCreatePermanently(backend models.Backend) error
	BackendCreateIfNotExist(backend models.Backend) error
	BackendEdit(backend models.Backend) error
	BackendDelete(backendName string) error
	BackendCfgSnippetSet(backendName string, value []string) error
	BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error
	BackendRuleDeleteAll(backend string)
	BackendServerDeleteAll(backendName string) (deleteServers bool)
	BackendServerCreate(backendName string, data models.Server) error
	BackendServerEdit(backendName string, data models.Server) error
	BackendServerDelete(backendName string, serverName string) error
	BackendServersGet(backendName string) (models.Servers, error)
	BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error
	BackendSwitchingRuleDeleteAll(frontend string) error
	DefaultsGetConfiguration() (*models.Defaults, error)
	DefaultsPushConfiguration(models.Defaults) error
	ExecuteRaw(command string) (result []string, err error)
	FrontendCfgSnippetSet(frontendName string, value []string) error
	FrontendCreate(frontend models.Frontend) error
	FrontendDelete(frontendName string) error
	FrontendsGet() (models.Frontends, error)
	FrontendGet(frontendName string) (models.Frontend, error)
	FrontendEdit(frontend models.Frontend) error
	FrontendEnableSSLOffload(frontendName string, certDir string, alpn string, strictSNI bool) (err error)
	FrontendDisableSSLOffload(frontendName string) (err error)
	FrontendSSLOffloadEnabled(frontendName string) bool
	FrontendBindsGet(frontend string) (models.Binds, error)
	FrontendBindCreate(frontend string, bind models.Bind) error
	FrontendBindEdit(frontend string, bind models.Bind) error
	FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule, ingressACL string) error
	FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule, ingressACL string) error
	FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule, ingressACL string) error
	FrontendRuleDeleteAll(frontend string)
	GlobalGetLogTargets() (models.LogTargets, error)
	GlobalPushLogTargets(models.LogTargets) error
	GlobalGetConfiguration() (*models.Global, error)
	GlobalPushConfiguration(models.Global) error
	GlobalCfgSnippet(snippet []string) error
	GetMap(mapFile string) (*models.Map, error)
	PeerEntryEdit(peerSection string, peer models.PeerEntry) error
	RefreshBackends() (deleted []string, err error)
	SetMapContent(mapFile string, payload []string) error
	SetServerAddr(backendName string, serverName string, ip string, port int) error
	SetServerState(backendName string, serverName string, state string) error
	ServerGet(serverName, backendNa string) (models.Server, error)
	SetAuxCfgFile(auxCfgFile string)
	SyncBackendSrvs(backend *store.RuntimeBackend, portUpdated bool) error
	UserListDeleteAll() error
	UserListExistsByGroup(group string) (bool, error)
	UserListCreateByGroup(group string, userPasswordMap map[string][]byte) error
}

type clientNative struct {
	nativeAPI                   clientnative.HAProxyClient
	activeBackends              map[string]struct{}
	permanentBackends           map[string]struct{}
	activeTransaction           string
	activeTransactionHasChanges bool
}

func New(transactionDir, configFile, programPath, runtimeSocket string) (client HAProxyClient, err error) { //nolint:ireturn
	var runtimeClient runtime.Runtime
	if runtimeSocket != "" {
		runtimeClient, err = runtime.New(context.Background(), runtimeoptions.Socket(runtimeSocket))
	} else {
		runtimeClient, err = runtime.New(context.Background())
	}
	if err != nil {
		return nil, err
	}

	confClient, err := configuration.New(context.Background(),
		cfgoptions.ConfigurationFile(configFile),
		cfgoptions.HAProxyBin(programPath),
		cfgoptions.UseModelsValidation,
		cfgoptions.TransactionsDir(transactionDir),
	)
	if err != nil {
		return nil, err
	}

	opt := []options.Option{
		options.Configuration(confClient),
		options.Runtime(runtimeClient),
	}
	cnHAProxyClient, err := clientnative.New(context.Background(), opt...)
	if err != nil {
		return nil, err
	}

	cn := clientNative{
		nativeAPI:         cnHAProxyClient,
		activeBackends:    make(map[string]struct{}),
		permanentBackends: make(map[string]struct{}),
	}
	return &cn, nil
}

func (c *clientNative) APIStartTransaction() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	version, errVersion := configuration.GetVersion("")
	if errVersion != nil || version < 1 {
		// silently fallback to 1
		version = 1
	}
	transaction, err := configuration.StartTransaction(version)
	if err != nil {
		return err
	}
	c.activeTransaction = transaction.ID
	c.activeTransactionHasChanges = false
	return nil
}

func (c *clientNative) APICommitTransaction() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if !c.activeTransactionHasChanges {
		if errDel := configuration.DeleteTransaction(c.activeTransaction); errDel != nil {
			return errDel
		}
		return nil
	}
	_, err = configuration.CommitTransaction(c.activeTransaction)
	return err
}

func (c *clientNative) APIDisposeTransaction() {
	c.activeTransaction = ""
	c.activeTransactionHasChanges = false
}

func (c *clientNative) SetAuxCfgFile(auxCfgFile string) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger := utils.GetLogger()
		logger.Error(err)
	}
	if auxCfgFile == "" {
		configuration.SetValidateConfigFiles(nil, nil)
		return
	}
	configuration.SetValidateConfigFiles(nil, []string{auxCfgFile})
}
