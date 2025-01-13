package api

import (
	"fmt"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/client-native/v6/models"
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
		err = config.Set(parser.Frontends, frontendName, "config-snippet", nil)
	} else {
		err = config.Set(parser.Frontends, frontendName, "config-snippet", types.StringSliceC{Value: value})
	}
	return err
}

func (c *clientNative) FrontendCreate(frontend models.FrontendBase) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	f := &models.Frontend{FrontendBase: frontend}
	return configuration.CreateFrontend(f, c.activeTransaction, 0)
}

func (c *clientNative) FrontendDelete(frontendName string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
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

func (c *clientNative) FrontendEdit(frontend models.FrontendBase) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	f := &models.Frontend{FrontendBase: frontend}
	return configuration.EditFrontend(frontend.Name, f, c.activeTransaction, 0)
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
	_, binds, err := configuration.GetBinds(string(parser.Frontends), frontend, c.activeTransaction)
	return binds, err
}

func (c *clientNative) FrontendBindCreate(frontend string, bind models.Bind) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateBind(string(parser.Frontends), frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendBindEdit(frontend string, bind models.Bind) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.EditBind(bind.Name, string(parser.Frontends), frontend, &bind, c.activeTransaction, 0)
}

func (c *clientNative) FrontendBindDelete(frontend string, bind string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.DeleteBind(bind, string(parser.Frontends), frontend, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPRequestRuleCreate(id int64, frontend string, rule models.HTTPRequestRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateHTTPRequestRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendHTTPResponseRuleCreate(id int64, frontend string, rule models.HTTPResponseRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateHTTPResponseRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendTCPRequestRuleCreate(id int64, frontend string, rule models.TCPRequestRule, ingressACL string) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}
	return configuration.CreateTCPRequestRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
}

func (c *clientNative) FrontendRuleDeleteAll(frontend string) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		logger := utils.GetLogger()
		logger.Error(err)
		return
	}

	for {
		err := configuration.DeleteHTTPRequestRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := configuration.DeleteHTTPResponseRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	for {
		err := configuration.DeleteTCPRequestRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
		if err != nil {
			break
		}
	}
	// No usage of TCPResponseRules yet.
}

func (c *clientNative) PeerEntryEdit(peerSection string, peerEntry models.PeerEntry) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.EditPeerEntry(peerEntry.Name, peerSection, &peerEntry, c.activeTransaction, 0)
}

func (c *clientNative) PeerEntryCreateOrEdit(peerSection string, peerEntry models.PeerEntry) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	err = configuration.EditPeerEntry(peerEntry.Name, peerSection, &peerEntry, c.activeTransaction, 0)
	if err != nil {
		err = configuration.CreatePeerEntry(peerSection, &peerEntry, c.activeTransaction, 0)
	}
	return err
}
