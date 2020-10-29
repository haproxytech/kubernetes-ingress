package api

import (
	"github.com/haproxytech/models/v2"
)

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

//TODO return pointer instead
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

func (c *clientNative) FrontendEnableSSLOffload(frontendName string, certDir string, alpn bool) (err error) {
	binds, err := c.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = certDir
		if alpn {
			bind.Alpn = "h2,http/1.1"
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
		bind.SslCertificate = ""
		bind.Alpn = ""
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

func (c *clientNative) FrontendHTTPRequestRuleCreate(frontend string, rule models.HTTPRequestRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateHTTPRequestRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPResponseRuleCreate(frontend string, rule models.HTTPResponseRule) error {
	c.activeTransactionHasChanges = true
	return c.nativeAPI.Configuration.CreateHTTPResponseRule("frontend", frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendTCPRequestRuleCreate(frontend string, rule models.TCPRequestRule) error {
	c.activeTransactionHasChanges = true
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
	// No usage of TCPResonpeRules yet.
}
