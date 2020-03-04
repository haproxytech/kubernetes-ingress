package controller

import (
	"fmt"
	"log"
	goruntime "runtime"
	"strconv"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Handle Global and default Annotations

func (c *HAProxyController) handleGlobalAnnotations() (reloadRequested bool, err error) {
	reloadRequested = false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := int64(maxProcs)
	annNbthread, errNumThread := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	// syslog-server has default value
	annSyslogSrv, _ := GetValueFromAnnotations("syslog-server", c.cfg.ConfigMap.Annotations)
	var errParser error
	config, _ := c.ActiveConfiguration()
	if errNumThread == nil {
		if numthr, errConv := strconv.Atoi(annNbthread.Value); errConv == nil {
			if numthr < maxProcs {
				numThreads = int64(numthr)
			}
			if annNbthread.Status == DELETED {
				errParser = config.Delete(parser.Global, parser.GlobalSectionName, "nbthread")
				reloadRequested = true
			} else if annNbthread.Status != EMPTY {
				errParser = config.Insert(parser.Global, parser.GlobalSectionName, "nbthread", types.Int64C{
					Value: numThreads,
				})
				reloadRequested = true
			}
			utils.LogErr(errParser)
		}
	}

	if annSyslogSrv.Status != EMPTY {
		stdoutLog := false
		errParser = config.Set(parser.Global, parser.GlobalSectionName, "log", nil)
		utils.LogErr(errParser)
		for index, syslogSrv := range strings.Split(annSyslogSrv.Value, "\n") {
			if syslogSrv == "" {
				continue
			}
			syslogSrv = strings.Join(strings.Fields(syslogSrv), "")
			logMap := make(map[string]string)
			for _, paramStr := range strings.Split(syslogSrv, ",") {
				paramLst := strings.Split(paramStr, ":")
				if len(paramLst) == 2 {
					logMap[paramLst[0]] = paramLst[1]
				} else {
					utils.LogErr(fmt.Errorf("incorrect syslog param: %s", paramLst))
					continue
				}
			}
			if address, ok := logMap["address"]; ok {
				logData := new(types.Log)
				logData.Address = address
				for k, v := range logMap {
					switch strings.ToLower(k) {
					case "address":
						if v == "stdout" {
							stdoutLog = true
						}
					case "port":
						if logMap["address"] != "stdout" {
							logData.Address += ":" + v
						}
					case "length":
						if length, errConv := strconv.Atoi(v); errConv == nil {
							logData.Length = int64(length)
						}
					case "format":
						logData.Format = v
					case "facility":
						logData.Facility = v
					case "level":
						logData.Level = v
					case "minlevel":
						logData.Level = v
					default:
						utils.LogErr(fmt.Errorf("unkown syslog param: %s ", k))
						continue
					}
				}
				errParser = config.Insert(parser.Global, parser.GlobalSectionName, "log", logData, index)
				reloadRequested = true
			}
			utils.LogErr(errParser)
		}
		if stdoutLog {
			errParser = config.Delete(parser.Global, parser.GlobalSectionName, "daemon")
		} else {
			errParser = config.Insert(parser.Global, parser.GlobalSectionName, "daemon", types.Enabled{})
		}
		utils.LogErr(errParser)
	}

	return reloadRequested, err
}

func (c *HAProxyController) handleDefaultTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleDefaultTimeout("http-request") || hasChanges
	hasChanges = c.handleDefaultTimeout("connect") || hasChanges
	hasChanges = c.handleDefaultTimeout("client") || hasChanges
	hasChanges = c.handleDefaultTimeout("queue") || hasChanges
	hasChanges = c.handleDefaultTimeout("server") || hasChanges
	hasChanges = c.handleDefaultTimeout("tunnel") || hasChanges
	hasChanges = c.handleDefaultTimeout("http-keep-alive") || hasChanges
	//no default values
	//timeout check is put in every backend, no need to put it here
	//hasChanges = c.handleDefaultTimeout("check", false) || hasChanges
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string) bool {
	annTimeout, err := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if err != nil {
		log.Println(err)
		return false
	}
	if annTimeout.Status != "" {
		config, _ := c.ActiveConfiguration()
		//TODO use client Native instead
		err = config.Set(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout), types.SimpleTimeout{
			Value: annTimeout.Value,
		})
		if err != nil {
			log.Println(err)
			return false
		}
		log.Println(fmt.Sprintf("default timeout-%s updated from %s to %s", timeout, annTimeout.OldValue, annTimeout.Value))
		return true
	}
	return false
}
