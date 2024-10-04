package api

import (
	"context"
	"encoding/json"

	clientnative "github.com/haproxytech/client-native/v5"
	"github.com/haproxytech/client-native/v5/config-parser/types"
	"github.com/haproxytech/client-native/v5/configuration"
	cfgoptions "github.com/haproxytech/client-native/v5/configuration/options"
	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/client-native/v5/options"
	"github.com/haproxytech/client-native/v5/runtime"
	runtimeoptions "github.com/haproxytech/client-native/v5/runtime/options"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// BufferSize is the default value of HAproxy tune.bufsize. Not recommended to change it
// Map payload or socket data cannot be bigger than tune.bufsize
const BufferSize = 16000

var logger = utils.GetLogger()

type HAProxyClient interface { //nolint:interfacebloat
	APIStartTransaction() error
	APICommitTransaction() error
	APIFinalCommitTransaction() error
	APIDisposeTransaction()
	ACLsGet(parentType, parentName string, aclName ...string) (models.Acls, error)
	ACLGet(id int64, parentType, parentName string) (*models.ACL, error)
	ACLDelete(id int64, parentType string, parentName string) error
	ACLDeleteAll(parentType string, parentName string) error
	ACLCreate(parentType string, parentName string, data *models.ACL) error
	ACLEdit(id int64, parentType string, parentName string, data *models.ACL) error
	BackendsGet() models.Backends
	BackendGet(backendName string) (*models.Backend, error)
	BackendExists(backendName string) bool
	BackendCreatePermanently(backend models.Backend)
	BackendCreateIfNotExist(backend models.Backend)
	BackendCreateOrUpdate(backend models.Backend) (map[string][]interface{}, bool)
	BackendDelete(backendName string)
	BackendDeleteAllUnnecessary() ([]string, error)
	BackendCfgSnippetSet(backendName string, value []string) error
	BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error
	BackendRuleDeleteAll(backend string)
	BackendServerDeleteAll(backendName string) error
	BackendServerCreate(backendName string, data models.Server) error
	BackendServerEdit(backendName string, data models.Server) error
	BackendServerDelete(backendName string, serverName string) error
	BackendServerGet(serverName, backendNa string) (*models.Server, error)
	BackendServersGet(backendName string) (models.Servers, error)
	BackendServerCreateOrUpdate(backendName string, data models.Server) error
	BackendSwitchingRulesGet(frontendName string) (models.BackendSwitchingRules, error)
	BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error
	CaptureCreate(frontend string, rule models.Capture) error
	CaptureDeleteAll(frontend string) (err error)
	CapturesGet(frontend string) (models.Captures, error)
	BackendSwitchingRuleDeleteAll(frontend string) error
	DefaultsGetConfiguration() (*models.Defaults, error)
	DefaultsPushConfiguration(models.Defaults) error
	ExecuteRaw(command string) (result []string, err error)
	FilterCreate(parentType, parentName string, rule models.Filter) error
	FiltersGet(parentType, parentName string) (models.Filters, error)
	FilterDeleteAll(parentType, parentName string) (err error)
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
	FrontendBindDelete(frontend string, bind string) error
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
	HTTPRequestRulesGet(parentType, parentName string) (models.HTTPRequestRules, error)
	HTTPRequestRuleGet(id int64, parentType, parentName string) (*models.HTTPRequestRule, error)
	HTTPRequestRuleDelete(id int64, parentType string, parentName string) error
	HTTPRequestRuleDeleteAll(parentType string, parentName string) error
	HTTPRequestRuleCreate(parentType string, parentName string, data *models.HTTPRequestRule) error
	HTTPRequestRuleEdit(id int64, parentType string, parentName string, data *models.HTTPRequestRule) error
	LogTargetCreate(parentType, parentName string, rule models.LogTarget) error
	LogTargetsGet(parentType, parentName string) (models.LogTargets, error)
	LogTargetDeleteAll(parentType, parentName string) (err error)
	PushPreviousBackends() error
	PopPreviousBackends() error
	TCPRequestRuleCreate(parentType, parentName string, rule models.TCPRequestRule) error
	TCPRequestRulesGet(parentType, parentName string) (models.TCPRequestRules, error)
	TCPRequestRuleDeleteAll(parentType, parentName string) (err error)
	PeerEntryEdit(peerSection string, peer models.PeerEntry) error
	PeerEntryCreateOrEdit(peerSection string, peer models.PeerEntry) error
	SetMapContent(mapFile string, payload []string) error
	SetServerAddrAndState([]RuntimeServerData) error
	SetAuxCfgFile(auxCfgFile string)
	SyncBackendSrvs(backend *store.RuntimeBackend, portUpdated bool) error
	UserListDeleteAll() error
	UserListExistsByGroup(group string) (bool, error)
	UserListCreateByGroup(group string, userPasswordMap map[string][]byte) error
	CertEntryCreate(filename string) error
	CertEntrySet(filename string, payload []byte) error
	CertEntryCommit(filename string) error
	CertEntryAbort(filename string) error
	CrtListEntryAdd(crtList string, entry runtime.CrtListEntry) error
}

type Backend struct { // use same names as in client native v6
	BackendBase       models.Backend `json:",inline"`
	Servers           map[string]models.Server
	ACLList           models.Acls             `json:"acl_list,omitempty"`
	HTTPRequestsRules models.HTTPRequestRules `json:"http_request_rule_list,omitempty"`
	ConfigSnippets    []string
	Permanent         bool
	Used              bool
}

type clientNative struct {
	nativeAPI                   clientnative.HAProxyClient
	activeTransaction           string
	activeTransactionHasChanges bool
	backends                    map[string]Backend
	previousBackends            []byte
}

func New(transactionDir, configFile, programPath, runtimeSocket string) (client HAProxyClient, err error) { //nolint:ireturn
	var runtimeClient runtime.Runtime
	if runtimeSocket != "" {
		runtimeClient, err = runtime.New(context.Background(), runtimeoptions.Socket(runtimeSocket), runtimeoptions.DoNotCheckRuntimeOnInit)
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
		nativeAPI: cnHAProxyClient,
		backends:  make(map[string]Backend),
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
	logger.WithField(utils.LogFieldTransactionID, transaction.ID)
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

func (c *clientNative) APIFinalCommitTransaction() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	var errs utils.Errors
	// First we remove all backends ...
	deletedBackends, _ := c.BackendDeleteAllUnnecessary()
	for _, deletedBackend := range deletedBackends {
		instance.Reload("backend '%s' deleted", deletedBackend)
	}
	// ... then we parse the backends to take decisions.
	for backendName, backend := range c.backends {
		errs.Add(c.processBackend(&backend.BackendBase, configuration))
		errs.AddErrors(c.processServers(backendName, configuration))
		errs.Add(c.processConfigSnippets(backendName, backend.ConfigSnippets, configuration))
		errs.AddErrors(c.processACLs(backendName, backend.ACLList, configuration))
		errs.AddErrors(c.processHTTPRequestRules(backendName, backend.HTTPRequestsRules, configuration))
		backend.Used = false
		c.backends[backendName] = backend
	}

	if !c.activeTransactionHasChanges {
		if errDel := configuration.DeleteTransaction(c.activeTransaction); errDel != nil {
			errs.Add(errDel)
		}
		return errs.Result()
	}
	_, err = configuration.CommitTransaction(c.activeTransaction)
	logger.Error(errs.Result())
	return err
}

func (c *clientNative) APIDisposeTransaction() {
	logger.ResetFields()
	c.activeTransaction = ""
	c.activeTransactionHasChanges = false
}

func (c *clientNative) SetAuxCfgFile(auxCfgFile string) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger := logger
		logger.Error(err)
	}
	if auxCfgFile == "" {
		configuration.SetValidateConfigFiles(nil, nil)
		return
	}
	configuration.SetValidateConfigFiles(nil, []string{auxCfgFile})
}

func (c *clientNative) processBackend(backend *models.Backend, configuration configuration.Configuration) error {
	// Try to create the backend ...
	errCreateBackend := configuration.CreateBackend(backend, c.activeTransaction, 0)
	if errCreateBackend != nil {
		// ... maybe it's already existing, so just edit it.
		return configuration.EditBackend(backend.Name, backend, c.activeTransaction, 0)
	}
	return nil
}

func (c *clientNative) processServers(backendName string, configuration configuration.Configuration) utils.Errors {
	var errs utils.Errors
	// Same for servers.
	servers, _ := c.BackendServersGet(backendName)
	for _, server := range servers {
		errCreateServer := configuration.CreateServer("backend", backendName, server, c.activeTransaction, 0)
		if errCreateServer != nil {
			errs.Add(configuration.EditServer(server.Name, "backend", backendName, server, c.activeTransaction, 0))
		}
	}
	return errs
}

func (c *clientNative) processConfigSnippets(backendName string, configSnippets []string, configuration configuration.Configuration) error {
	// Same for backend configsnippets.
	config, err := configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	if len(configSnippets) > 0 {
		return config.Set("backend", backendName, "config-snippet", types.StringSliceC{Value: configSnippets})
	} else {
		return config.Set("backend", backendName, "config-snippet", nil)
	}
}

func (c *clientNative) processACLs(backendName string, aclsList models.Acls, configuration configuration.Configuration) utils.Errors {
	// we remove all acls because of permanent backend still in parsers.
	_, existingACLs, _ := configuration.GetACLs("backend", backendName, c.activeTransaction)
	for range existingACLs {
		_ = configuration.DeleteACL(0, "backend", backendName, c.activeTransaction, 0)
	}
	var errs utils.Errors
	// we (re)create all acls
	for _, acl := range aclsList {
		errs.Add(configuration.CreateACL("backend", backendName, acl, c.activeTransaction, 0))
	}
	return errs
}

func (c *clientNative) processHTTPRequestRules(backendName string, httpRequestsRules models.HTTPRequestRules, configuration configuration.Configuration) utils.Errors {
	// we remove all http request rules because of permanent backend still in parsers.
	_, existingHTTPRequestRules, _ := configuration.GetHTTPRequestRules("backend", backendName, c.activeTransaction)
	for range existingHTTPRequestRules {
		_ = configuration.DeleteHTTPRequestRule(0, "backend", backendName, c.activeTransaction, 0)
	}
	var errs utils.Errors
	// we (re)create all http request rules
	for _, httpRequestRule := range httpRequestsRules {
		errs.Add(configuration.CreateHTTPRequestRule("backend", backendName, httpRequestRule, c.activeTransaction, 0))
	}
	return errs
}

func (c *clientNative) PushPreviousBackends() error {
	logger.Debug("Pushing backends as previous successfully applied backends")
	jsonBackends, err := json.Marshal(c.backends) //nolint:musttag
	if err != nil {
		return err
	}
	c.previousBackends = jsonBackends
	return nil
}

func (c *clientNative) PopPreviousBackends() error {
	logger.Debug("Popping backends from previous successfully applied backends")
	if c.previousBackends == nil {
		clear(c.backends)
		return nil
	}
	backends := map[string]Backend{}
	err := json.Unmarshal(c.previousBackends, &backends) //nolint:musttag
	if err != nil {
		return err
	}
	c.backends = backends
	return nil
}
