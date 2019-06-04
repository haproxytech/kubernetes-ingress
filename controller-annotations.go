package main

import (
	"fmt"
	"log"

	parser "github.com/haproxytech/config-parser"
	"github.com/haproxytech/config-parser/types"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleDefaultTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleDefaultTimeout("http-request") || hasChanges
	hasChanges = c.handleDefaultTimeout("connect") || hasChanges
	hasChanges = c.handleDefaultTimeout("client") || hasChanges
	hasChanges = c.handleDefaultTimeout("queue") || hasChanges
	hasChanges = c.handleDefaultTimeout("server") || hasChanges
	hasChanges = c.handleDefaultTimeout("tunnel") || hasChanges
	hasChanges = c.handleDefaultTimeout("http-keep-alive") || hasChanges
	if hasChanges {
		err := c.NativeParser.Save(HAProxyGlobalCFG)
		LogErr(err)
	}
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string) bool {
	client := c.NativeParser
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		log.Println(err)
		return false
	}
	if annTimeout.Status != "" {
		//log.Println(fmt.Sprintf("timeout [%s]", timeout), annTimeout.Value, annTimeout.OldValue, annTimeout.Status)
		data, err := client.Get(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
		if err != nil {
			log.Println(err)
			return false
		}
		timeout := data.(*types.SimpleTimeout)
		timeout.Value = annTimeout.Value
		return true
	}
	return false
}

func (c *HAProxyController) handleBackendAnnotations(balanceAlg *models.Balance, forwardedFor *StringW,
	backendName string, transaction *models.Transaction) error {
	backend := &models.Backend{
		Balance: balanceAlg,
		Name:    backendName,
		Mode:    "http",
	}
	if forwardedFor.Value == "enabled" { //disabled with anything else is ok
		forwardfor := "enabled"
		backend.Forwardfor = &models.Forwardfor{
			Enabled: &forwardfor,
		}
	}

	if err := c.NativeAPI.Configuration.EditBackend(backend.Name, backend, transaction.ID, 0); err != nil {
		return err
	}
	return nil
}
