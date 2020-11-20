package controller

import (
	"fmt"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/haproxytech/config-parser/v3/types"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

// Handle Global and default Annotations

func (c *HAProxyController) handleGlobalAnnotations() (restart bool, reload bool) {
	reload = false
	reload = c.handleDefaultLogFormat() || reload
	reload = c.handleDefaultMaxconn() || reload
	reload = c.handleDefaultOptions() || reload
	reload = c.handleDefaultTimeouts() || reload
	reload = c.handleNbthread() || reload
	reload = c.handleHardStopAfter() || reload

	restart, r := c.handleSyslog()
	reload = reload || r
	return restart, reload
}

func (c *HAProxyController) handleNbthread() bool {
	annNbthread, _ := GetValueFromAnnotations("nbthread", c.cfg.ConfigMap.Annotations)
	if annNbthread == nil {
		return false
	}
	var err error
	switch annNbthread.Status {
	case EMPTY:
		return false
	case DELETED:
		err = c.Client.SetNbthread(nil)
	default:
		maxProcs := goruntime.GOMAXPROCS(0)
		numThreads := int64(maxProcs)
		numthr, errConv := strconv.Atoi(annNbthread.Value)
		if errConv != nil {
			c.Logger.Err(errConv)
			return false
		}
		if numthr < maxProcs {
			numThreads = int64(numthr)
		}
		c.Logger.Infof("Set NbThread to: '%d'", numThreads)
		err = c.Client.SetNbthread(&numThreads)
	}
	if err != nil {
		c.Logger.Err(err)
		return false
	}
	return true
}

func (c *HAProxyController) handleSyslog() (restart, reload bool) {
	annSyslogSrv, _ := GetValueFromAnnotations("syslog-server", c.cfg.ConfigMap.Annotations)
	// No need to check for non nil annotation because it has default value.
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
			if paramStr == "" {
				continue
			}
			paramLst := strings.Split(paramStr, ":")
			if len(paramLst) == 2 {
				logMap[paramLst[0]] = paramLst[1]
			} else {
				c.Logger.Errorf("incorrect syslog param: '%s' in '%s'", paramLst, syslogSrv)
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
					c.Logger.Errorf("unkown syslog param: '%s' in '%s' ", k, syslogSrv)
					continue
				}
			}
			c.Logger.Infof("Adding log target: '%s'", syslogSrv)
			errParser = c.Client.SetLogTarget(logData, index)
			if errParser == nil {
				reload = true
			}
			c.Logger.Error(errParser)
		}
	}
	if stdoutLog {
		if daemonMode {
			c.Logger.Info("Disabling Daemon mode")
			errParser = c.Client.SetDaemonMode(nil)
			restart = true
		}
	} else if !daemonMode {
		enabled := true
		c.Logger.Info("Enabling Daemon mode")
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
	if annOption == nil {
		return false
	}
	var err error
	switch annOption.Status {
	case EMPTY:
		return false
	case DELETED:
		c.Logger.Infof("Removing '%s' option", option)
		err = c.Client.SetDefaulOption(option, nil)
	default:
		enabled, parseErr := utils.GetBoolValue(annOption.Value, option)
		if parseErr != nil {
			c.Logger.Err(parseErr)
			return false
		}
		action := "Enabling"
		if !enabled {
			action = "Disabling"
		}
		c.Logger.Infof("%s %s", action, option)
		err = c.Client.SetDefaulOption(option, &enabled)
	}
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
	if c.handleDefaultTimeout("client") {
		// Update Inspect delay timeout
		if c.cfg.SSLPassthrough {
			c.cfg.FrontendRulesStatus[TCP] = MODIFIED
		}
		hasChanges = true
	}
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
	var err error
	switch annTimeout.Status {
	case EMPTY:
		return false
	case DELETED:
		c.Logger.Infof("Removing default timeout-%s ", timeout)
		err = c.Client.SetDefaulTimeout(timeout, nil)
	default:
		c.Logger.Infof("Setting default timeout-%s to %s", timeout, annTimeout.Value)
		err = c.Client.SetDefaulTimeout(timeout, &annTimeout.Value)
	}
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	return true
}

func (c *HAProxyController) handleDefaultMaxconn() bool {
	annMaxconn, _ := GetValueFromAnnotations("maxconn", c.cfg.ConfigMap.Annotations)
	if annMaxconn == nil {
		return false
	}
	var err error
	switch annMaxconn.Status {
	case EMPTY:
		return false
	case DELETED:
		c.Logger.Info("Removing default maxconn")
		err = c.Client.SetDefaulMaxconn(nil)
	default:
		value, parseErr := strconv.ParseInt(annMaxconn.Value, 10, 64)
		if parseErr != nil {
			c.Logger.Error(parseErr)
			return false
		}
		c.Logger.Infof("Setting default maxconn to %d", value)
		err = c.Client.SetDefaulMaxconn(&value)
	}
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	return true
}

func (c *HAProxyController) handleDefaultLogFormat() bool {
	annLogFormat, _ := GetValueFromAnnotations("log-format", c.cfg.ConfigMap.Annotations)
	// No need check for non nil annotation because it has default value.
	if annLogFormat.Status == EMPTY {
		return false
	}
	c.Logger.Infof("Changing default log format to '%s'", annLogFormat.Value)
	err := c.Client.SetDefaulLogFormat(&annLogFormat.Value)
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	return true
}

func (c *HAProxyController) handleHardStopAfter() bool {
	annHardStopAfter, _ := GetValueFromAnnotations("hard-stop-after", c.cfg.ConfigMap.Annotations)
	if annHardStopAfter.Status == EMPTY {
		return false
	}
	after, err := time.ParseDuration(annHardStopAfter.Value)
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	duration := after.String()
	if strings.HasSuffix(duration, "m0s") {
		duration = duration[:len(duration)-2]
	}
	if strings.HasSuffix(duration, "h0m") {
		duration = duration[:len(duration)-2]
	}
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	c.Logger.Infof("Changing hard-stop-after value to %s", duration)
	err = c.Client.SetHardStopAfter(&duration)
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	return true
}
