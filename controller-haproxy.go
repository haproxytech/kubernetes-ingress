package main

import (
	"log"
	"strconv"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) updateHAProxy(reloadRequested bool) error {
	nativeAPI := c.NativeAPI

	c.handleDefaultTimeouts()
	version, err := nativeAPI.Configuration.GetVersion("")
	if err != nil || version < 1 {
		//silently fallback to 1
		version = 1
	}
	//log.Println("Config version:", version)
	transaction, err := nativeAPI.Configuration.StartTransaction(version)
	c.ActiveTransaction = transaction.ID
	defer func() {
		c.ActiveTransaction = ""
	}()
	if err != nil {
		log.Println(err)
		return err
	}

	if maxconnAnn, err := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations); err == nil {
		if maxconn, err := strconv.ParseInt(maxconnAnn.Value, 10, 64); err == nil {
			if maxconnAnn.Status == DELETED {
				maxconnAnn, _ = GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations) // has default
				maxconn, _ = strconv.ParseInt(maxconnAnn.Value, 10, 64)
			}
			if maxconnAnn.Status != "" {
				if frontend, err := nativeAPI.Configuration.GetFrontend("http", transaction.ID); err == nil {
					frontend.Data.Maxconn = &maxconn
					err := nativeAPI.Configuration.EditFrontend("http", frontend.Data, transaction.ID, 0)
					LogErr(err)
				} else {
					return err
				}
			}
		}
	}

	maxProcs, maxThreads, reload, err := c.handleGlobalAnnotations(transaction)
	LogErr(err)
	reloadRequested = reloadRequested || reload

	for _, namespace := range c.cfg.Namespace {
		if !namespace.Relevant {
			continue
		}
		var usingHTTPS bool
		reload, usingHTTPS, err = c.handleHTTPS(namespace, maxProcs, maxThreads, transaction)
		if err != nil {
			return err
		}
		err = c.handleRateLimiting(transaction, usingHTTPS)
		if err != nil {
			return err
		}
		numProcs, _ := strconv.Atoi(maxProcs.Value)
		numThreads, _ := strconv.Atoi(maxThreads.Value)
		port := int64(80)
		listener := &models.Bind{
			Name:    "http_1",
			Address: "0.0.0.0",
			Port:    &port,
			Process: "1/1",
		}
		if !usingHTTPS {
			if numProcs > 1 {
				listener.Process = "all"
			}
			if numThreads > 1 {
				listener.Process = "all"
			}
		}
		if listener.Process != c.cfg.HTTPBindProcess {
			if err = nativeAPI.Configuration.EditBind(listener.Name, FrontendHTTP, listener, transaction.ID, 0); err != nil {
				return err
			}
			c.cfg.HTTPBindProcess = listener.Process
		}
		reloadRequested = reloadRequested || reload
		reload, err = c.handleHTTPRedirect(usingHTTPS, transaction)
		if err != nil {
			return err
		}
		reloadRequested = reloadRequested || reload
		//TODO, do not just go through them, sort them to handle /web,/ maybe?
		for _, ingress := range namespace.Ingresses {
			//no need for switch/case for now
			backendsUsed := map[string]int{}
			for _, rule := range ingress.Rules {
				//nothing to switch/case for now
				for _, path := range rule.Paths {
					err := c.handlePath(namespace, ingress, rule, path, transaction, backendsUsed)
					LogErr(err)
				}
			}
			for backendName, numberOfTimesBackendUsed := range backendsUsed {
				if numberOfTimesBackendUsed < 1 {
					err := nativeAPI.Configuration.DeleteBackend(backendName, transaction.ID, 0)
					LogErr(err)
				}
			}
		}
	}
	err = c.requestsTCPRefresh(transaction)
	LogErr(err)
	err = c.RequestsHTTPRefresh(transaction)
	LogErr(err)
	_, err = nativeAPI.Configuration.CommitTransaction(transaction.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	c.cfg.Clean()
	if reloadRequested {
		if err := c.HAProxyReload(); err != nil {
			log.Println(err)
		} else {
			log.Println("HAProxy reloaded")
		}
	} else {
		log.Println("HAProxy updated without reload")
	}
	return nil
}
