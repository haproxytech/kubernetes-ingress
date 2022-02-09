package api

import (
	"fmt"

	"github.com/haproxytech/client-native/v2/models"
	"github.com/haproxytech/config-parser/v4/types"
)

func (c *clientNative) FrontendCfgSnippetSet(frontendName string, value []string) error {
	config, err := c.nativeAPI.Configuration.GetParser(c.activeTransaction)
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
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateFrontend(&frontend, c.activeTransaction, 0)
}

func (c *clientNative) FrontendDelete(frontendName string) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.DeleteFrontend(frontendName, c.activeTransaction, 0)
}

func (c *clientNative) FrontendsGet() (models.Frontends, error) {
	_, frontends, err := c.nativeAPI.Configuration.GetFrontends(c.activeTransaction)
	return frontends, err
}

func (c *clientNative) FrontendGet(frontendName string) (models.Frontend, error) {
	_, frontend, err := c.nativeAPI.Configuration.GetFrontend(frontendName, c.activeTransaction)
	if err != nil {
		return models.Frontend{}, err
	}
	return *frontend, err
}

func (c *clientNative) FrontendEdit(frontend models.Frontend) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.EditFrontend(frontend.Name, &frontend, c.activeTransaction, 0)
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

func (c *clientNative) FrontendBindsGet(frontend string) (models.Binds, error) {
	_, binds, err := c.nativeAPI.Configuration.GetBinds(frontend, c.activeTransaction)
	return binds, err
}

func (c *clientNative) FrontendBindCreate(frontend string, bind models.Bind) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateBind(frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendBindEdit(frontend string, bind models.Bind) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.EditBind(bind.Name, frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule, ingressACL string) error {
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return c.nativeAPI.Configuration.CreateHTTPRequestRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule, ingressACL string) error {
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return c.nativeAPI.Configuration.CreateHTTPResponseRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule, ingressACL string) error {
	c.activeTransactionHasChanges = true
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return c.nativeAPI.Configuration.CreateTCPRequestRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendRuleDeleteAll(frontend string) {
	c.activeTransactionHasChanges = true

	for {
		err := c.nativeAPI.Configuration.DeleteHTTPRequestRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := c.nativeAPI.Configuration.DeleteHTTPResponseRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := c.nativeAPI.Configuration.DeleteTCPRequestRule(0, "frontend", frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	// No usage of TCPResponseRules yet.
}
