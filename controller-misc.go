package main

import (
	"fmt"
	"strconv"

	goruntime "runtime"

	parser "github.com/haproxytech/config-parser"
	"github.com/haproxytech/config-parser/types"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleGlobalAnnotations(transaction *models.Transaction) (maxProcsStat *StringW, maxThreadsStat *StringW, reloadRequested bool, err error) {
	reloadRequested = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := maxProcs
	annNumProc, _ := GetValueFromAnnotations("ssl-numproc", c.cfg.ConfigMap.Annotations)
	annNbthread, _ := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	maxProcsStat = &StringW{}
	maxThreadsStat = &StringW{}
	if numthr, err := strconv.Atoi(annNbthread.Value); err == nil {
		if numthr < maxProcs {
			numThreads = numthr
		}
	}
	if numproc, err := strconv.Atoi(annNumProc.Value); err == nil {
		if numproc < maxProcs {
			maxProcs = numproc
		}
	}

	//see global config
	p := c.NativeParser
	var nbproc *types.Int64C
	data, err := p.Get(parser.Global, parser.GlobalSectionName, "nbproc")
	if err == nil {
		nbproc = data.(*types.Int64C)
		if nbproc.Value != int64(maxProcs) {
			reloadRequested = true
			nbproc.Value = int64(maxProcs)
			maxProcsStat.Status = MODIFIED
		}
	} else {
		nbproc = &types.Int64C{
			Value: int64(maxProcs),
		}
		p.Set(parser.Global, parser.GlobalSectionName, "nbproc", nbproc)
		maxProcsStat.Status = ADDED
		reloadRequested = true
	}
	if maxProcs > 1 {
		numThreads = 1
	}

	var nbthread *types.Int64C
	data, err = p.Get(parser.Global, parser.GlobalSectionName, "nbthread")
	if err == nil {
		nbthread = data.(*types.Int64C)
		if nbthread.Value != int64(numThreads) {
			reloadRequested = true
			nbthread.Value = int64(numThreads)
			maxThreadsStat.Status = MODIFIED
		}
	} else {
		nbthread = &types.Int64C{
			Value: int64(numThreads),
		}
		p.Set(parser.Global, parser.GlobalSectionName, "nbthread", nbthread)
		maxThreadsStat.Status = ADDED
		reloadRequested = true
	}

	data, err = p.Get(parser.Global, parser.GlobalSectionName, "cpu-map")
	numCPUMap := numThreads
	namePrefix := "1/"
	if nbthread.Value < 2 {
		numCPUMap = maxProcs
		namePrefix = ""
	}
	cpuMap := make([]types.CpuMap, numCPUMap)
	for index := 0; index < numCPUMap; index++ {
		cpuMap[index] = types.CpuMap{
			Name:  fmt.Sprintf("%s%d", namePrefix, index+1),
			Value: strconv.Itoa(index),
		}
	}
	p.Set(parser.Global, parser.GlobalSectionName, "cpu-map", cpuMap)
	maxProcsStat.Value = strconv.Itoa(maxProcs)
	maxThreadsStat.Value = strconv.Itoa(numThreads)
	return maxProcsStat, maxThreadsStat, reloadRequested, err
}

func (c *HAProxyController) removeHTTPSListeners(transaction *models.Transaction) (err error) {
	listeners := *c.cfg.HTTPSListeners
	for index, data := range listeners {
		data.Status = DELETED
		listenerName := "https_" + strconv.Itoa(index+1)
		if err = c.NativeAPI.Configuration.DeleteBind(listenerName, FrontendHTTPS, transaction.ID, 0); err != nil {
			return err
		}
	}
	return nil
}

func (c *HAProxyController) handleHTTPRedirect(usingHTTPS bool, transaction *models.Transaction) (reloadRequested bool, err error) {
	//see if we need to add redirect to https redirect scheme https if !{ ssl_fc }
	// no need for error checking, we have default value,
	//if not defined as OFF, we always do redirect
	reloadRequested = false
	sslRedirect, _ := GetValueFromAnnotations("ssl-redirect", c.cfg.ConfigMap.Annotations)
	useSSLRedirect := sslRedirect.Value != "OFF"
	if !usingHTTPS {
		useSSLRedirect = false
	}
	var state Status
	if useSSLRedirect {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "ON"
			state = ADDED
		} else if c.cfg.SSLRedirect == "OFF" {
			c.cfg.SSLRedirect = "ON"
			state = ADDED
		}
	} else {
		if c.cfg.SSLRedirect == "" {
			c.cfg.SSLRedirect = "OFF"
			state = ""
		} else if c.cfg.SSLRedirect != "OFF" {
			c.cfg.SSLRedirect = "OFF"
			state = DELETED
		}
	}
	redirectCode := int64(302)
	annRedirectCode, _ := GetValueFromAnnotations("ssl-redirect-code", c.cfg.ConfigMap.Annotations)
	if value, err := strconv.ParseInt(annRedirectCode.Value, 10, 64); err == nil {
		redirectCode = value
	}
	if state == "" && annRedirectCode.Status != "" {
		state = MODIFIED
	}
	id := int64(0)
	rule := &models.HTTPRequestRule{
		ID:         &id,
		Type:       "redirect",
		RedirCode:  redirectCode,
		RedirValue: "https",
		RedirType:  "scheme",
		Cond:       "if",
		CondTest:   "!{ ssl_fc }",
	}
	switch state {
	case ADDED:
		if err = c.NativeAPI.Configuration.CreateHTTPRequestRule("frontend", "http", rule, transaction.ID, 0); err != nil {
			return reloadRequested, err
		}
		c.cfg.SSLRedirect = "ON"
		reloadRequested = true
	case MODIFIED:
		if err = c.NativeAPI.Configuration.EditHTTPRequestRule(*rule.ID, "frontend", "http", rule, transaction.ID, 0); err != nil {
			return reloadRequested, err
		}
		reloadRequested = true
	case DELETED:
		if err = c.NativeAPI.Configuration.DeleteHTTPRequestRule(*rule.ID, "frontend", "http", transaction.ID, 0); err != nil {
			return reloadRequested, err
		}
		c.cfg.SSLRedirect = "OFF"
		reloadRequested = true
	}
	return reloadRequested, nil
}
