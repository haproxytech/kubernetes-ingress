package api

import (
	clientnative "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/runtime"
	"github.com/haproxytech/config-parser/v3/types"
	"github.com/haproxytech/models/v2"
)

type HAProxyClient interface {
	APIStartTransaction() error
	APICommitTransaction() error
	APIDisposeTransaction()
	BackendsGet() (models.Backends, error)
	BackendGet(backendName string) (models.Backend, error)
	BackendCreate(backend models.Backend) error
	BackendEdit(backend models.Backend) error
	BackendDelete(backendName string) error
	BackendCfgSnippetSet(backendName string, value *[]string) error
	BackendHTTPRequestRuleCreate(backend string, rule models.HTTPRequestRule) error
	BackendRuleDeleteAll(backend string)
	BackendServerCreate(backendName string, data models.Server) error
	BackendServerEdit(backendName string, data models.Server) error
	BackendServerDelete(backendName string, serverName string) error
	BackendSwitchingRuleCreate(frontend string, rule models.BackendSwitchingRule) error
	BackendSwitchingRuleDeleteAll(frontend string)
	ExecuteRaw(command string) (result []string, err error)
	FrontendCreate(frontend models.Frontend) error
	FrontendDelete(frontendName string) error
	FrontendsGet() (models.Frontends, error)
	FrontendGet(frontendName string) (models.Frontend, error)
	FrontendEdit(frontend models.Frontend) error
	FrontendEnableSSLOffload(frontendName string, certDir string, alpn bool) (err error)
	FrontendDisableSSLOffload(frontendName string) (err error)
	FrontendBindsGet(frontend string) (models.Binds, error)
	FrontendBindCreate(frontend string, bind models.Bind) error
	FrontendBindEdit(frontend string, bind models.Bind) error
	FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error
	FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule) error
	FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule) error
	FrontendRuleDeleteAll(frontend string)
	GlobalConfigEnabled(section string, config string) (enabled bool, err error)
	GlobalWriteConfig(section string, config string) (result string, err error)
	DaemonMode(value *types.Enabled) error
	DefaultErrorFile(value *types.ErrorFile, index int) error
	DefaultLogFormat(value *types.StringC) error
	GlobalMaxconn(value *types.Int64C) error
	DefaultOption(option string, value *types.SimpleOption) error
	DefaultTimeout(timeout string, value *types.SimpleTimeout) error
	GlobalCfgSnippet(snippet *types.StringSliceC) error
	GlobalHardStopAfter(value *types.StringC) error
	LogTarget(value *types.Log, index int) error
	Nbthread(value *types.Int64C) error
	PIDFile(value *types.StringC) error
	RuntimeSocket(value *types.Socket) error
	ServerStateBase(value *types.StringC) error
	SetServerAddr(backendName string, serverName string, ip string, port int) error
	SetServerState(backendName string, serverName string, state string) error
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
		//silently fallback to 1
		version = 1
	}
	transaction, err := c.nativeAPI.Configuration.StartTransaction(version)
	c.activeTransaction = transaction.ID
	c.activeTransactionHasChanges = false
	return err
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
