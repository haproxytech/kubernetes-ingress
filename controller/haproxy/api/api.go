package api

import (
	"fmt"

	clientnative "github.com/haproxytech/client-native/v2"
	"github.com/haproxytech/client-native/v2/configuration"
	"github.com/haproxytech/client-native/v2/runtime"
	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/models/v2"
)

type HAProxyClient interface {
	ActiveConfiguration() (*parser.Parser, error)
	ActiveConfigurationHasChanges()
	APIStartTransaction() error
	APICommitTransaction() error
	APIDisposeTransaction()
	BackendsGet() (models.Backends, error)
	BackendGet(backendName string) (models.Backend, error)
	BackendCreate(backend models.Backend) error
	BackendEdit(backend models.Backend) error
	BackendDelete(backendName string) error
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
		ConfigurationFile:      configFile,
		PersistentTransactions: false,
		Haproxy:                programPath,
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

// Return Parser of current configuration (for config-parser usage)
func (c *clientNative) ActiveConfiguration() (*parser.Parser, error) {
	if c.activeTransaction == "" {
		return nil, fmt.Errorf("no active transaction")
	}
	return c.nativeAPI.Configuration.GetParser(c.activeTransaction)
}

func (c *clientNative) ActiveConfigurationHasChanges() {
	c.activeTransactionHasChanges = true
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
