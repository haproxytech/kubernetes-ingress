package api

import (
	"context"
	//nolint:gosec
	"crypto/md5" // G501: Blocklisted import crypto/md5: weak cryptographic primitive
	"encoding/hex"
	"encoding/json"

	clientnative "github.com/haproxytech/client-native/v6"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/client-native/v6/configuration"
	cfgoptions "github.com/haproxytech/client-native/v6/configuration/options"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/client-native/v6/options"
	"github.com/haproxytech/client-native/v6/runtime"
	runtimeoptions "github.com/haproxytech/client-native/v6/runtime/options"

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
	ACL
	BackendsGet() models.Backends
	BackendGet(backendName string) (*models.Backend, error)
	// This function tests if a backend is existing :
	// Check if you're not rather looking for BackendUsed function.
	BackendExists(backendName string) bool
	// This function tests if a backend is existing AND IT'S USED.
	BackendUsed(backendName string) bool
	BackendCreatePermanently(backend models.Backend)
	BackendCreateIfNotExist(backend models.Backend)
	BackendCreateOrUpdate(backend models.Backend) (map[string][]interface{}, bool)
	BackendDelete(backendName string)
	BackendDeleteAllUnnecessary() ([]string, error)
	BackendCfgSnippetSet(backendName string, value []string) error
	BackendRuleDeleteAll(backend string)
	BackendServerDeleteAll(backendName string) error
	BackendServerCreate(backendName string, data models.Server) error
	BackendServerCreateOrUpdate(backendName string, data models.Server) error
	BackendServerEdit(backendName string, data models.Server) error
	BackendServerDelete(backendName string, serverName string) error
	BackendServerGet(serverName, backendNa string) (*models.Server, error)
	BackendServersGet(backendName string) (models.Servers, error)
	BackendSwitchingRule
	Capture
	DefaultsGetConfiguration() (*models.Defaults, error)
	DefaultsPushConfiguration(models.Defaults) error
	ExecuteRaw(command string) (result string, err error)
	Filter
	FrontendCfgSnippetSet(frontendName string, value []string) error
	FrontendCfgSnippetApply() error
	FrontendCreate(frontend models.FrontendBase) error
	FrontendDelete(frontendName string) error
	FrontendDeletePending() error
	FrontendsGet() (models.Frontends, error)
	FrontendGet(frontendName string) (models.Frontend, error)
	FrontendEdit(frontend models.FrontendBase) error
	FrontendEnableSSLOffload(frontendName string, certDir string, alpn string, strictSNI bool, generateCertificatesSigner string) (err error)
	FrontendDisableSSLOffload(frontendName string) (err error)
	FrontendSSLOffloadEnabled(frontendName string) bool
	UploadFrontends() error
	FrontendStructured
	Bind
	FrontendRuleDeleteAll(frontend string)
	GlobalGetLogTargets() (models.LogTargets, error)
	GlobalPushLogTargets(models.LogTargets) error
	GlobalGetConfiguration() (*models.Global, error)
	GlobalPushConfiguration(models.Global) error
	GlobalCfgSnippet(snippet []string) error
	GetMap(mapFile string) (*models.Map, error)
	HTTPRequestRule
	LogTarget
	TCPRequestRule
	PeerEntryDelete(peerSection string, name string) error
	PeerEntryEdit(peerSection string, peer models.PeerEntry) error
	PeerEntryCreateOrEdit(peerSection string, peer models.PeerEntry) error
	SetMapContent(mapFile string, payload []string) error
	SetServerAddrAndState([]RuntimeServerData) error
	SetAuxCfgFile(auxCfgFile string)
	SyncBackendSrvs(backend *store.RuntimeBackend, portUpdated bool) error
	UserListDeleteAll() error
	UserListExistsByGroup(group string) (bool, error)
	UserListCreateByGroup(group string, userPasswordMap map[string][]byte) error
	Cert
	CertAuth
	PushPreviousBackends() error
	PopPreviousBackends() error
}

type Cert interface {
	CertEntryCreate(filename string) error
	CertEntrySet(filename string, payload []byte) error
	CertEntryCommit(filename string) error
	CertEntryAbort(filename string) error
	CrtListEntryAdd(crtList string, entry runtime.CrtListEntry) error
	CrtListEntryDelete(crtList, filename string, linenumber *int64) error
	CertEntryDelete(filename string) error
}

type CertAuth interface {
	CertAuthEntryCreate(filename string) error
	CertAuthEntrySet(filename string, payload []byte) error
	CertAuthEntryCommit(filename string) error
	CertEntryAbort(filename string) error
	CertEntryDelete(filename string) error
}

type Backend struct { // use same names as in client native v6
	models.Backend
	ConfigSnippets []string
	Permanent      bool
	Used           bool
}

type ACL interface {
	ACLsGet(parentType, parentName string, aclName ...string) (models.Acls, error)
	ACLDeleteAll(parentType string, parentName string) error
	ACLCreate(id int64, parentType string, parentName string, data *models.ACL) error
	ACLsReplace(parentType, parentName string, rules models.Acls) error
}

type BackendSwitchingRule interface {
	BackendSwitchingRulesGet(frontendName string) (models.BackendSwitchingRules, error)
	BackendSwitchingRuleCreate(id int64, frontend string, rule models.BackendSwitchingRule) error
	BackendSwitchingRuleDeleteAll(frontend string) error
	BackendSwitchingRulesReplace(frontend string, rules models.BackendSwitchingRules) error
}

type Bind interface {
	FrontendBindsGet(frontend string) (models.Binds, error)
	FrontendBindCreate(frontend string, bind models.Bind) error
	FrontendBindEdit(frontend string, bind models.Bind) error
	FrontendBindDelete(frontend string, bind string) error
}

type Filter interface {
	FilterCreate(id int64, parentType, parentName string, rule models.Filter) error
	FiltersGet(parentType, parentName string) (models.Filters, error)
	FilterDeleteAll(parentType, parentName string) (err error)
	FiltersReplace(parentType, parentName string, rules models.Filters) error
}

type Capture interface {
	CaptureCreate(id int64, frontend string, rule models.Capture) error
	CaptureDeleteAll(frontend string) (err error)
	CapturesGet(frontend string) (models.Captures, error)
	CapturesReplace(frontend string, rules models.Captures) error
}

type LogTarget interface {
	LogTargetCreate(id int64, parentType, parentName string, rule models.LogTarget) error
	LogTargetsGet(parentType, parentName string) (models.LogTargets, error)
	LogTargetDeleteAll(parentType, parentName string) (err error)
	LogTargetsReplace(parentType, parentName string, rules models.LogTargets) error
}

type TCPRequestRule interface {
	TCPRequestRuleCreate(id int64, parentType, parentName string, rule models.TCPRequestRule) error
	TCPRequestRulesGet(parentType, parentName string) (models.TCPRequestRules, error)
	TCPRequestRuleDeleteAll(parentType, parentName string) (err error)
	TCPRequestRulesReplace(parentType, parentName string, rules models.TCPRequestRules) error
	FrontendTCPRequestRuleCreate(id int64, frontend string, rule models.TCPRequestRule, ingressACL string) error
}

type HTTPRequestRule interface {
	HTTPRequestRulesGet(parentType, parentName string) (models.HTTPRequestRules, error)
	HTTPRequestRuleDeleteAll(parentType string, parentName string) error
	HTTPRequestRuleCreate(id int64, parentType string, parentName string, data *models.HTTPRequestRule) error
	HTTPRequestRulesReplace(parentType, parentName string, rules models.HTTPRequestRules) error
	FrontendHTTPRequestRuleCreate(id int64, frontend string, rule models.HTTPRequestRule, ingressACL string) error
	FrontendHTTPResponseRuleCreate(id int64, frontend string, rule models.HTTPResponseRule, ingressACL string) error
	FrontendHTTPAfterResponseRuleCreate(id int64, frontend string, rule models.HTTPAfterResponseRule, ingressACL string) error
	BackendHTTPRequestRuleCreate(id int64, backend string, rule models.HTTPRequestRule) error
}

type Frontend struct {
	models.Frontend
	ConfigSnippets []string
}

type clientNative struct {
	nativeAPI                           clientnative.HAProxyClient
	activeTransaction                   string
	backends                            map[string]Backend
	frontends                           map[string]*Frontend
	previousBackends                    []byte
	configurationHashAtTransactionStart string
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
		cfgoptions.UseMd5Hash,
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
		frontends: make(map[string]*Frontend),
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

	hash, err := c.computeConfigurationHash(configuration)
	if err != nil {
		return err
	}
	c.configurationHashAtTransactionStart = hash

	return nil
}

func (c *clientNative) computeConfigurationHash(configuration configuration.Configuration) (string, error) {
	p, err := configuration.GetParser(c.activeTransaction)
	if err != nil {
		return "", err
	}
	// Note that p.String() does not include the hash!!!
	content := p.String()
	//nolint: gosec
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:]), err
}

func (c *clientNative) APICommitTransaction() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}

	hash, err := c.computeConfigurationHash(configuration)
	if err != nil {
		return err
	}

	if c.configurationHashAtTransactionStart == hash {
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
		errs.Add(c.processBackend(&backend.Backend, configuration))
		errs.AddErrors(c.processServers(backendName, configuration))
		errs.Add(c.processConfigSnippets(backendName, backend.ConfigSnippets, configuration))
		errs.AddErrors(c.processACLs(backendName, backend.ACLList, configuration))
		errs.AddErrors(c.processHTTPRequestRules(backendName, backend.HTTPRequestRuleList, configuration))
		backend.Used = false
		c.backends[backendName] = backend
	}

	hash, err := c.computeConfigurationHash(configuration)
	if err != nil {
		return err
	}

	if c.configurationHashAtTransactionStart == hash {
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
		} else {
			// Server has been created, a reload is required
			// It covers the case where there was a failure, scaleHAProxySrvs has already been called in a previous loop
			// but the sync failed (wrong config)
			// When the config is fixed, servers will be created
			instance.Reload("server '%s' created in backend '%s'", server.Name, backendName)
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
	var errs utils.Errors
	errs.Add(configuration.ReplaceAcls("backend", backendName, aclsList, c.activeTransaction, 0))
	return errs
}

func (c *clientNative) processHTTPRequestRules(backendName string, httpRequestsRules models.HTTPRequestRules, configuration configuration.Configuration) utils.Errors {
	var errs utils.Errors
	// we (re)create all http request rules
	errs.Add(configuration.ReplaceHTTPRequestRules("backend", backendName, httpRequestsRules, c.activeTransaction, 0))
	return errs
}

func (c *clientNative) PushPreviousBackends() error {
	logger.Debug("Pushing backends as previous successfully applied backends")
	jsonBackends, err := json.Marshal(c.backends)
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
	err := json.Unmarshal(c.previousBackends, &backends)
	if err != nil {
		return err
	}
	c.backends = backends
	return nil
}
