package controller

import (
	"fmt"
	goruntime "runtime"
	"strconv"
	"strings"

	"github.com/haproxytech/config-parser/v2/types"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Handle Global and default Annotations

func (c *HAProxyController) handleGlobalAnnotations() (restart bool, reload bool) {
	reload = false
	reload = c.handleDefaultLogFormat() ||
		c.handleDefaultMaxconn() ||
		c.handleDefaultOptions() ||
		c.handleDefaultTimeouts() ||
		c.handleNbthread()

	restart, r := c.handleSyslog()
	reload = reload || r
	return restart, reload
}

func (c *HAProxyController) handleNbthread() bool {
	reload := false
	maxProcs := goruntime.GOMAXPROCS(0)
	numThreads := int64(maxProcs)
	annNbthread, _ := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	if annNbthread == nil || annNbthread.Status == EMPTY {
		return false
	}
	var errParser error
	if numthr, errConv := strconv.Atoi(annNbthread.Value); errConv == nil {
		if numthr < maxProcs {
			numThreads = int64(numthr)
		}
		if annNbthread.Status == DELETED {
			errParser = c.Client.SetNbthread(nil)
			reload = true
		} else if annNbthread.Status != EMPTY {
			errParser = c.Client.SetNbthread(&numThreads)
		}
		c.Logger.Error(errParser)
	}
	return reload
}

func (c *HAProxyController) handleSyslog() (restart, reload bool) {
	annSyslogSrv, _ := GetValueFromAnnotations("syslog-server", c.cfg.ConfigMap.Annotations)
	if annSyslogSrv.Status == EMPTY {
		return false, false
	}
	restart = false
	reload = false
	stdoutLog := false
	daemonMode, errParser := c.Client.EnabledConfig("daemon")
	c.Logger.Error(errParser)
	errParser = c.Client.SetLogTarget(nil, -1)
	c.Logger.Error(errParser)
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
				c.Logger.Errorf("incorrect syslog param: %s", paramLst)
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
					c.Logger.Errorf("unkown syslog param: %s ", k)
					continue
				}
			}
			errParser = c.Client.SetLogTarget(logData, index)
			if errParser == nil {
				reload = true
			}
			c.Logger.Error(errParser)
		}
	}
	if stdoutLog {
		if daemonMode {
			errParser = c.Client.SetDaemonMode(nil)
			restart = true
		}
	} else if !daemonMode {
		enabled := true
		errParser = c.Client.SetDaemonMode(&enabled)
		restart = true
	}
	c.Logger.Error(errParser)
	return restart, reload
}

func (c *HAProxyController) handleDefaultOptions() bool {
	reload := false
	reload = c.handleDefaultOption("http-server-close") || reload
	reload = c.handleDefaultOption("http-keep-alive") || reload
	reload = c.handleDefaultOption("dontlognull") || reload
	reload = c.handleDefaultOption("logasap") || reload
	return reload
}

func (c *HAProxyController) handleDefaultOption(option string) bool {
	annOption, _ := GetValueFromAnnotations(option, c.cfg.ConfigMap.Annotations)
	if annOption == nil || annOption.Status == "" {
		return false
	}
	var err error
	if annOption.Status == DELETED {
		c.Logger.Printf(fmt.Sprintf("Removing '%s' option", option))
		err = c.Client.SetDefaulOption(option, nil)
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		return true
	}
	enabled, err := utils.GetBoolValue(annOption.Value, option)
	if err != nil {
		c.Logger.Err(err)
		return false
	}
	action := "Enabling"
	if !enabled {
		action = "Disabling"
	}
	c.Logger.Printf(fmt.Sprintf("%s %s", action, option))
	err = c.Client.SetDefaulOption(option, &enabled)
	if err != nil {
		c.Logger.Err(err)
		return false
	}
	return true
}

func (c *HAProxyController) handleDefaultTimeouts() bool {
	hasChanges := false
	hasChanges = c.handleDefaultTimeout("http-request") || hasChanges
	hasChanges = c.handleDefaultTimeout("connect") || hasChanges
	hasChanges = c.handleDefaultTimeout("client") || hasChanges
	hasChanges = c.handleDefaultTimeout("client-fin") || hasChanges
	hasChanges = c.handleDefaultTimeout("queue") || hasChanges
	hasChanges = c.handleDefaultTimeout("server") || hasChanges
	hasChanges = c.handleDefaultTimeout("server-fin") || hasChanges
	hasChanges = c.handleDefaultTimeout("tunnel") || hasChanges
	hasChanges = c.handleDefaultTimeout("http-keep-alive") || hasChanges
	//no default values
	//timeout check is put in every backend, no need to put it here
	//hasChanges = c.handleDefaultTimeout("check", false) || hasChanges
	return hasChanges
}

func (c *HAProxyController) handleDefaultTimeout(timeout string) bool {
	annTimeout, _ := GetValueFromAnnotations(fmt.Sprintf("timeout-%s", timeout), c.cfg.ConfigMap.Annotations)
	if annTimeout == nil {
		return false
	}
	if annTimeout.Status != "" {
		var err error
		if annTimeout.Status == DELETED {
			c.Logger.Debugf("Removing default timeout-%s ", timeout)
			err = c.Client.SetDefaulTimeout(timeout, nil)
		} else {
			c.Logger.Debugf("Setting default timeout-%s to %s", timeout, annTimeout.Value)
			err = c.Client.SetDefaulTimeout(timeout, &annTimeout.Value)
		}
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		return true
	}
	return false
}

func (c *HAProxyController) handleDefaultMaxconn() bool {
	annMaxconn, _ := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations)
	if annMaxconn == nil {
		return false
	}
	value, err := strconv.ParseInt(annMaxconn.Value, 10, 64)
	if err != nil {
		c.Logger.Error(err)
		return false
	}

	switch annMaxconn.Status {
	case EMPTY:
		return false
	case DELETED:
		err = c.Client.SetDefaulMaxconn(nil)
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		c.Logger.Debug("Removing default maxconn")
	default:
		err = c.Client.SetDefaulMaxconn(&value)
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		c.Logger.Debugf("Setting default maxconn to %d", value)
	}
	return true
}

func (c *HAProxyController) handleDefaultLogFormat() bool {
	annLogFormat, _ := GetValueFromAnnotations("log-format", c.cfg.ConfigMap.Annotations)
	if annLogFormat.Status == EMPTY {
		return false
	}
	err := c.Client.SetDefaulLogFormat(&annLogFormat.Value)
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	return true
}
