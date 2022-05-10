package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v3/models"
	"github.com/haproxytech/config-parser/v4/types"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func (c *clientNative) FrontendCfgSnippetSet(frontendName string, value []string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	config, err := configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		err = config.Set("frontend", frontendName, "config-snippet", nil)
	} else {
		err = config.Set("frontend", frontendName, "config-snippet", types.StringSliceC{Value: value})
	}
	if err != nil {
		c.activeTransactionHasChanges = true
	}
	return err
}

func (c *clientNative) FrontendCreate(frontend models.Frontend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateFrontend(&frontend, c.activeTransaction, 0)
}

func (c *clientNative) FrontendDelete(frontendName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.DeleteFrontend(frontendName, c.activeTransaction, 0)
}

func (c *clientNative) FrontendsGet() (models.Frontends, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, frontends, err := configuration.GetFrontends(c.activeTransaction)
	return frontends, err
}

func (c *clientNative) FrontendGet(frontendName string) (models.Frontend, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return models.Frontend{}, err
	}
	_, frontend, err := configuration.GetFrontend(frontendName, c.activeTransaction)
	if err != nil {
		return models.Frontend{}, err
	}
	return *frontend, err
}

func (c *clientNative) FrontendEdit(frontend models.Frontend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.EditFrontend(frontend.Name, &frontend, c.activeTransaction, 0)
}

func (c *clientNative) FrontendEnableSSLOffload(frontendName string, certDir string, alpn string, strictSNI bool) (err error) {
	binds, err := c.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = certDir
		if alpn != "" {
			bind.Alpn = alpn
			bind.StrictSni = strictSNI
		}
		err = c.FrontendBindEdit(frontendName, *bind)
	}
	if err != nil {
		return err
	}
	return err
}

func (c *clientNative) FrontendDisableSSLOffload(frontendName string) (err error) {
	binds, err := c.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = false
		bind.SslCafile = ""
		bind.Verify = ""
		bind.SslCertificate = ""
		bind.Alpn = ""
		bind.StrictSni = false
		err = c.FrontendBindEdit(frontendName, *bind)
	}
	if err != nil {
		return err
	}
	return err
}

func (c *clientNative) FrontendSSLOffloadEnabled(frontendName string) bool {
	binds, err := c.FrontendBindsGet(frontendName)
	if err != nil {
		return false
	}
	for _, bind := range binds {
		if bind.Ssl {
			return true
		}
	}
	return false
}

func (c *clientNative) FrontendBindsGet(frontend string) (models.Binds, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, binds, err := configuration.GetBinds(frontend, c.activeTransaction)
	return binds, err
}

func (c *clientNative) FrontendBindCreate(frontend string, bind models.Bind) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.CreateBind(frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendBindEdit(frontend string, bind models.Bind) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	return configuration.EditBind(bind.Name, frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateHTTPRequestRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateHTTPResponseRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateTCPRequestRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendRuleDeleteAll(frontend string) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger := utils.GetLogger()
		logger.Error(err)
		return
	}
	c.activeTransactionHasChanges = true

	for {
		err := configuration.DeleteHTTPRequestRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := configuration.DeleteHTTPResponseRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := configuration.DeleteTCPRequestRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	// No usage of TCPResponseRules yet.
}
