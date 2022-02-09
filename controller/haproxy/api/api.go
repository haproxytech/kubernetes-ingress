package api

import (
	clientnative "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/models"
	"github.com/haproxytech/client-native/v2/runtime"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type HAProxyClient interface {
	APIStartTransaction() error
	APICommitTransaction() error
	APIDisposeTransaction()
	BackendsGet() (models.Backends, error)
	BackendGet(backendName string) (*models.Backend, error)
	BackendCreate(backend models.Backend) error
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
	BackendSwitchingRuleDeleteAll(frontend string)
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
	SetMapContent(mapFile string, payload string) error
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
	activeTransaction           string
	activeTransactionHasChanges bool
}

func Init(transactionDir, configFile, programPath, runtimeSocket string) (client HAProxyClient, err error) {
	runtimeClient := runtime.Client{}
	err = runtimeClient.InitWithSockets(map[int]string{
		0: runtimeSocket,
	})
	if err != nil {
		return nil, err
	}

	confClient := configuration.Client{}
	confParams := configuration.ClientParams{
		ConfigurationFile:         configFile,
		PersistentTransactions:    false,
		Haproxy:                   programPath,
		ValidateConfigurationFile: true,
		UseValidation:             true,
	}

	if transactionDir != "" {
		confParams.TransactionDir = transactionDir
	}
	err = confClient.Init(confParams)
	if err != nil {
		return nil, err
	}

	cn := clientNative{
		nativeAPI: clientnative.HAProxyClient{
			Configuration: &confClient,
			Runtime:       &runtimeClient,
		},
	}
	return &cn, nil
}

func (c *clientNative) APIStartTransaction() error {
	version, errVersion := c.nativeAPI.Configuration.GetVersion("")
	if errVersion != nil || version < 1 {
		// silently fallback to 1
		version = 1
	}
	transaction, err := c.nativeAPI.Configuration.StartTransaction(version)
	if err != nil {
		return err
	}
	c.activeTransaction = transaction.ID
	c.activeTransactionHasChanges = false
	return nil
}

func (c *clientNative) APICommitTransaction() error {
	if !c.activeTransactionHasChanges {
		if err := c.nativeAPI.Configuration.DeleteTransaction(c.activeTransaction); err != nil {
			return err
		}
		return nil
	}
	_, err := c.nativeAPI.Configuration.CommitTransaction(c.activeTransaction)
	return err
}

func (c *clientNative) APIDisposeTransaction() {
	c.activeTransaction = ""
	c.activeTransactionHasChanges = false
}

func (c *clientNative) SetAuxCfgFile(auxCfgFile string) {
	if auxCfgFile == "" {
		c.nativeAPI.Configuration.Transaction.ValidateConfigFilesAfter = nil
		return
	}
	c.nativeAPI.Configuration.Transaction.ValidateConfigFilesAfter = []string{auxCfgFile}
}
