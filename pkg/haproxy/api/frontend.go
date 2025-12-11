package api

import (
	"errors"
	"fmt"

	parser "github.com/haproxytech/client-native/v6/config-parser"
	"github.com/haproxytech/client-native/v6/config-parser/types"
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type FrontendStructured interface {
	FrontendsGetStructured() (models.Frontends, error)
	FrontendGetStructured(name string) (*models.Frontend, error)
	FrontendEditStructured(name string, data *models.Frontend) error
	FrontendCreateStructured(data *models.Frontend) error
}

// func (c *clientNative) FrontendCfgSnippetSet(frontendName string, value []string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	config, err := configuration.GetParser(c.activeTransaction)
// 	if err != nil {
// 		return err
// 	}
// 	if len(value) == 0 {
// 		err = config.Set(parser.Frontends, frontendName, "config-snippet", nil)
// 	} else {
// 		err = config.Set(parser.Frontends, frontendName, "config-snippet", types.StringSliceC{Value: value})
// 	}
// 	return err
// }

func (c *clientNative) FrontendCfgSnippetSet(frontendName string, value []string) error {
	frontend, exists := c.frontends[frontendName]
	if !exists {
		return fmt.Errorf("backend %s : %w", frontendName, ErrNotFound)
	}

	frontend.ConfigSnippets = value
	c.frontends[frontendName] = frontend
	return nil
}

func (c *clientNative) FrontendCfgSnippetApply() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	config, err := configuration.GetParser(c.activeTransaction)
	if err != nil {
		return err
	}
	frontendsConfigSnippets := map[string][]string{
		"http":  c.frontends["http"].ConfigSnippets,
		"https": c.frontends["https"].ConfigSnippets,
		"stats": c.frontends["stats"].ConfigSnippets,
	}

	for frontendName, value := range frontendsConfigSnippets {
		if len(value) == 0 {
			err = config.Set(parser.Frontends, frontendName, "config-snippet", nil)
		} else {
			err = config.Set(parser.Frontends, frontendName, "config-snippet", types.StringSliceC{Value: value})
		}
	}
	return err
}

// func (c *clientNative) FrontendCreate(frontend models.FrontendBase) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	f := &models.Frontend{FrontendBase: frontend}
// 	return configuration.CreateFrontend(f, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendCreate(frontend models.FrontendBase) error {
	oldFrontend, ok := c.frontends[frontend.Name]
	if !ok {
		c.frontends[frontend.Name] = &Frontend{
			Frontend: models.Frontend{
				FrontendBase: frontend,
			},
		}
		return nil
	}

	oldFrontend.Frontend.FrontendBase = frontend
	c.frontends[frontend.Name] = oldFrontend
	return nil
}

// func (c *clientNative) FrontendDelete(frontendName string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	return configuration.DeleteFrontend(frontendName, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendDelete(frontendName string) error {
	_, exists := c.frontends[frontendName]
	if !exists {
		return fmt.Errorf("can't delete unexisting frontend %s", frontendName)
	}
	c.frontends[frontendName] = nil
	return nil
}

func (c *clientNative) FrontendDeletePending() error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	var errs utils.Errors
	for frontendName, frontend := range c.frontends {
		if frontend == nil {
			errs.Add(configuration.DeleteFrontend(frontendName, c.activeTransaction, 0))
			delete(c.frontends, frontendName)
		}
	}
	return errs.Result()
}

// func (c *clientNative) FrontendsGet() (models.Frontends, error) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return nil, err
// 	}
// 	_, frontends, err := configuration.GetFrontends(c.activeTransaction)
// 	return frontends, err
// }

func (c *clientNative) FrontendsGet() (models.Frontends, error) {
	frontends := models.Frontends(make([]*models.Frontend, 0, len(c.frontends)))

	for _, frontend := range c.frontends {
		if frontend == nil {
			continue
		}
		frontends = append(frontends, &frontend.Frontend)
	}
	return frontends, nil
}

// func (c *clientNative) FrontendGet(frontendName string) (models.Frontend, error) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return models.Frontend{}, err
// 	}
// 	_, frontend, err := configuration.GetFrontend(frontendName, c.activeTransaction)
// 	if err != nil {
// 		return models.Frontend{}, err
// 	}
// 	return *frontend, err
// }

func (c *clientNative) FrontendGet(frontendName string) (models.Frontend, error) {
	oldFrontend, ok := c.frontends[frontendName]
	if ok {
		return oldFrontend.Frontend, nil
	}
	return models.Frontend{}, fmt.Errorf("frontend %s not found", frontendName)
}

// func (c *clientNative) FrontendEdit(frontend models.FrontendBase) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	f := &models.Frontend{FrontendBase: frontend}
// 	return configuration.EditFrontend(frontend.Name, f, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendEdit(frontend models.FrontendBase) error {
	oldFrontend, ok := c.frontends[frontend.Name]
	if !ok {
		return fmt.Errorf("can't edit unexisting frontend %s", frontend.Name)
	}
	oldFrontend.FrontendBase = frontend
	return nil
}

func (c *clientNative) FrontendEnableSSLOffload(frontendName string, certDir string, alpn string, strictSNI bool, generateCertificatesSigner string) (err error) {
	binds, err := c.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	for _, bind := range binds {
		bind.Ssl = true
		bind.SslCertificate = certDir
		if generateCertificatesSigner != "" {
			bind.GenerateCertificates = true
			bind.CaSignFile = generateCertificatesSigner
		} else {
			bind.GenerateCertificates = false
		}
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
		bind.CaSignFile = ""
		bind.Alpn = ""
		bind.StrictSni = false
		bind.GenerateCertificates = false
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

// func (c *clientNative) FrontendBindsGet(frontend string) (models.Binds, error) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return nil, err
// 	}
// 	_, binds, err := configuration.GetBinds(string(parser.Frontends), frontend, c.activeTransaction)
// 	return binds, err
// }

func (c *clientNative) FrontendBindsGet(frontendName string) (models.Binds, error) {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return nil, fmt.Errorf("frontend %s not found", frontendName)
	}
	return utils.ConvertMapIntoPointerValuesSlice(frontend.Binds), nil
}

// func (c *clientNative) FrontendBindCreate(frontend string, bind models.Bind) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	return configuration.CreateBind(string(parser.Frontends), frontend, &bind, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendBindCreate(frontendName string, bind models.Bind) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if frontend.Binds == nil {
		frontend.Binds = make(map[string]models.Bind)
	}
	if _, found := frontend.Binds[bind.Name]; found {
		return fmt.Errorf("bind %s already exists", bind.Name)
	}
	frontend.Binds[bind.Name] = bind
	return nil
}

// func (c *clientNative) FrontendBindEdit(frontend string, bind models.Bind) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	return configuration.EditBind(bind.Name, string(parser.Frontends), frontend, &bind, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendBindEdit(frontendName string, bind models.Bind) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if frontend.Binds == nil {
		return fmt.Errorf("bind %s does not exist", bind.Name)
	}
	if _, found := frontend.Binds[bind.Name]; !found {
		return fmt.Errorf("bind %s does not exist", bind.Name)
	}
	frontend.Binds[bind.Name] = bind
	return nil
}

// func (c *clientNative) FrontendBindDelete(frontend string, bind string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	return configuration.DeleteBind(bind, string(parser.Frontends), frontend, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendBindDelete(frontendName string, bindName string) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if frontend.Binds == nil {
		return fmt.Errorf("bind %s does not exist", bindName)
	}
	if _, found := frontend.Binds[bindName]; !found {
		return fmt.Errorf("bind %s does not exist", bindName)
	}
	delete(frontend.Binds, bindName)
	return nil
}

// func (c *clientNative) FrontendHTTPRequestRuleCreate(id int64, frontend string, rule models.HTTPRequestRule, ingressACL string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	if ingressACL != "" {
// 		rule.Cond = "if"
// 		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
// 	}
// 	return configuration.CreateHTTPRequestRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendHTTPRequestRuleCreate(id int64, frontendName string, rule models.HTTPRequestRule, ingressACL string) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if id != 0 && id >= int64(len(frontend.HTTPRequestRuleList)-1) {
		return errors.New("can't add rule to the last position in the list")
	}
	if frontend.HTTPRequestRuleList == nil {
		frontend.HTTPRequestRuleList = models.HTTPRequestRules{}
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}

	frontend.HTTPRequestRuleList = append(frontend.HTTPRequestRuleList, &models.HTTPRequestRule{})
	copy(frontend.HTTPRequestRuleList[id+1:], frontend.HTTPRequestRuleList[id:])
	frontend.HTTPRequestRuleList[id] = &rule
	return nil
}

// func (c *clientNative) FrontendHTTPResponseRuleCreate(id int64, frontend string, rule models.HTTPResponseRule, ingressACL string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	if ingressACL != "" {
// 		rule.Cond = "if"
// 		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
// 	}
// 	return configuration.CreateHTTPResponseRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendHTTPResponseRuleCreate(id int64, frontendName string, rule models.HTTPResponseRule, ingressACL string) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if id != 0 && id >= int64(len(frontend.HTTPResponseRuleList)-1) {
		return errors.New("can't add rule to the last position in the list")
	}
	if frontend.HTTPResponseRuleList == nil {
		frontend.HTTPResponseRuleList = models.HTTPResponseRules{}
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}

	frontend.HTTPResponseRuleList = append(frontend.HTTPResponseRuleList, &models.HTTPResponseRule{})
	copy(frontend.HTTPResponseRuleList[id+1:], frontend.HTTPResponseRuleList[id:])
	frontend.HTTPResponseRuleList[id] = &rule
	return nil
}

// func (c *clientNative) FrontendTCPRequestRuleCreate(id int64, frontend string, rule models.TCPRequestRule, ingressACL string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	if ingressACL != "" {
// 		rule.Cond = "if"
// 		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
// 	}
// 	return configuration.CreateTCPRequestRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendTCPRequestRuleCreate(id int64, frontendName string, rule models.TCPRequestRule, ingressACL string) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if id != 0 && id >= int64(len(frontend.TCPRequestRuleList)-1) {
		return errors.New("can't add rule to the last position in the list")
	}
	if frontend.TCPRequestRuleList == nil {
		frontend.TCPRequestRuleList = models.TCPRequestRules{}
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}

	frontend.TCPRequestRuleList = append(frontend.TCPRequestRuleList, &models.TCPRequestRule{})
	copy(frontend.TCPRequestRuleList[id+1:], frontend.TCPRequestRuleList[id:])
	frontend.TCPRequestRuleList[id] = &rule
	return nil
}

func (c *clientNative) FrontendRuleDeleteAll(frontendName string) {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return // not found
	}

	frontend.HTTPRequestRuleList = nil
	frontend.HTTPResponseRuleList = nil
	frontend.TCPRequestRuleList = nil
	frontend.HTTPAfterResponseRuleList = nil
}

// func (c *clientNative) FrontendRuleDeleteAll(frontend string) {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		logger := utils.GetLogger()
// 		logger.Error(err)
// 		return
// 	}

// 	for {
// 		err := configuration.DeleteHTTPRequestRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
// 		if err != nil {
// 			break
// 		}
// 	}
// 	for {
// 		err := configuration.DeleteHTTPResponseRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
// 		if err != nil {
// 			break
// 		}
// 	}
// 	for {
// 		err := configuration.DeleteTCPRequestRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
// 		if err != nil {
// 			break
// 		}
// 	}
// 	for {
// 		err := configuration.DeleteHTTPAfterResponseRule(0, string(parser.Frontends), frontend, c.activeTransaction, 0)
// 		if err != nil {
// 			break
// 		}
// 	}
// 	// No usage of TCPResponseRules yet.
// }

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

func (c *clientNative) PeerEntryDelete(peerSection, entry string) error {
	cfg, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return cfg.DeletePeerEntry(entry, peerSection, c.activeTransaction, 0)
}

// func (c *clientNative) FrontendHTTPAfterResponseRuleCreate(id int64, frontend string, rule models.HTTPAfterResponseRule, ingressACL string) error {
// 	configuration, err := c.nativeAPI.Configuration()
// 	if err != nil {
// 		return err
// 	}
// 	if ingressACL != "" {
// 		rule.Cond = "if"
// 		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
// 	}
// 	return configuration.CreateHTTPAfterResponseRule(id, string(parser.Frontends), frontend, &rule, c.activeTransaction, 0)
// }

func (c *clientNative) FrontendHTTPAfterResponseRuleCreate(id int64, frontendName string, rule models.HTTPAfterResponseRule, ingressACL string) error {
	frontend, ok := c.frontends[frontendName]
	if !ok {
		return fmt.Errorf("frontend %s not found", frontendName)
	}
	if id != 0 && id >= int64(len(frontend.HTTPAfterResponseRuleList)-1) {
		return errors.New("can't add rule to the last position in the list")
	}
	if frontend.HTTPAfterResponseRuleList == nil {
		frontend.HTTPAfterResponseRuleList = models.HTTPAfterResponseRules{}
	}
	if ingressACL != "" {
		rule.Cond = "if"
		rule.CondTest = fmt.Sprintf("%s %s", ingressACL, rule.CondTest)
	}

	frontend.HTTPAfterResponseRuleList = append(frontend.HTTPAfterResponseRuleList, &models.HTTPAfterResponseRule{})
	copy(frontend.HTTPAfterResponseRuleList[id+1:], frontend.HTTPAfterResponseRuleList[id:])
	frontend.HTTPAfterResponseRuleList[id] = &rule
	return nil
}

func (c *clientNative) FrontendsGetStructured() (models.Frontends, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, frontends, err := configuration.GetStructuredFrontends(c.activeTransaction)
	return frontends, err
}

func (c *clientNative) FrontendGetStructured(name string) (*models.Frontend, error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return nil, err
	}
	_, frontend, err := configuration.GetStructuredFrontend(name, c.activeTransaction)
	if err != nil {
		return nil, err
	}
	return frontend, err
}

func (c *clientNative) FrontendEditStructured(name string, data *models.Frontend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.EditStructuredFrontend(name, data, c.activeTransaction, 0)
}

func (c *clientNative) FrontendCreateStructured(data *models.Frontend) error {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	return configuration.CreateStructuredFrontend(data, c.activeTransaction, 0)
}

func (c *clientNative) UploadFrontends() error {
	frontends, err := c.FrontendsGetStructured()
	if err != nil {
		return err
	}
	for _, frontend := range frontends {
		c.frontends[frontend.Name] = &Frontend{Frontend: *frontend}
	}
	return nil
}
