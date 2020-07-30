package api

import (
	clientnative "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/runtime"
	"github.com/haproxytech/config-parser/v2/types"
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
	BackendHTTPRequestRuleDeleteAll(backend string)
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
	FrontendBindsGet(frontend string) (models.Binds, error)
	FrontendBindCreate(frontend string, bind models.Bind) error
	FrontendBindEdit(frontend string, bind models.Bind) error
	FrontendBindDeleteAll(frontend string) error
	FrontendHTTPRequestRuleDeleteAll(frontend string)
	FrontendHTTPResponseRuleDeleteAll(frontend string)
	FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error
	FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule) error
	FrontendTCPRequestRuleDeleteAll(frontend string)
	FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule) error
	GetConfig(configType string) (enabled bool, err error)
	SetConfigSnippet(snippet *types.StringSliceC) error
	SetDaemonMode(value *types.Enabled) error
	SetDefaulLogFormat(value *types.StringC) error
	SetDefaulMaxconn(value *types.Int64C) error
	SetDefaulOption(option string, value *types.SimpleOption) error
	SetDefaulTimeout(timeout string, value *types.SimpleTimeout) error
	SetLogTarget(value *types.Log, index int) error
	SetNbthread(value *(types.Int64C)) error
	SetServerAddr(backendName string, serverName string, ip string, port int) error
	SetServerState(backendName string, serverName string, state string) error
}

type clientNative struct {
	nativeAPI                   clientnative.HAProxyClient
	activeTransaction           string
	activeTransactionHasChanges bool
}

func Init(configFile, programPath, runtimeSocket string) (client HAProxyClient, err error) {
	runtimeClient := runtime.Client{}
	err = runtimeClient.InitWithSockets(map[int]string{
		0: runtimeSocket,
	})
	if err != nil {
		return nil, err
	}

	confClient := configuration.Client{}
	err = confClient.Init(configuration.ClientParams{
		ConfigurationFile:         configFile,
		PersistentTransactions:    false,
		Haproxy:                   programPath,
		ValidateConfigurationFile: true,
	})
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
