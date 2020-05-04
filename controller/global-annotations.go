package controller

import (
	"fmt"
	goruntime "runtime"
	"strconv"
	"strings"

	parser "github.com/haproxytech/config-parser/v2"
	"github.com/haproxytech/config-parser/v2/types"
)

// Handle Global and default Annotations

func (c *HAProxyController) handleGlobalAnnotations() (restart bool, reload bool) {
	reload = false
	reload = c.handleDefaultLogFormat() ||
		c.handleDefaultMaxconn() ||
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
	config, _ := c.ActiveConfiguration()
	if numthr, errConv := strconv.Atoi(annNbthread.Value); errConv == nil {
		if numthr < maxProcs {
			numThreads = int64(numthr)
		}
		if annNbthread.Status == DELETED {
			errParser = config.Delete(parser.Global, parser.GlobalSectionName, "nbthread")
			c.ActiveTransactionHasChanges = true
			reload = true
		} else if annNbthread.Status != EMPTY {
			errParser = config.Insert(parser.Global, parser.GlobalSectionName, "nbthread", types.Int64C{
				Value: numThreads,
			})
			c.ActiveTransactionHasChanges = true
			reload = true
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
	config, _ := c.ActiveConfiguration()
	restart = false
	reload = false
	stdoutLog := false
	daemonMode := false
	if val, _ := config.Get(parser.Global, parser.GlobalSectionName, "daemon"); val != nil {
		daemonMode = true
	}
	errParser := config.Set(parser.Global, parser.GlobalSectionName, "log", nil)
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
			errParser = config.Insert(parser.Global, parser.GlobalSectionName, "log", logData, index)
			if errParser == nil {
				c.ActiveTransactionHasChanges = true
				reload = true
			}
			c.Logger.Error(errParser)
		}
	}
	if stdoutLog {
		if daemonMode {
			errParser = config.Delete(parser.Global, parser.GlobalSectionName, "daemon")
			restart = true
		}
	} else if !daemonMode {
		errParser = config.Insert(parser.Global, parser.GlobalSectionName, "daemon", types.Enabled{})
		restart = true
	}
	c.Logger.Error(errParser)
	return restart, reload
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
		config, _ := c.ActiveConfiguration()
		if annTimeout.Status == DELETED {
			c.Logger.Debugf("Removing default timeout-%s ", timeout)
			err = config.Delete(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout))
		} else {
			c.Logger.Debugf("Setting default timeout-%s to %s", timeout, annTimeout.Value)
			err = config.Set(parser.Defaults, parser.DefaultSectionName, fmt.Sprintf("timeout %s", timeout), types.SimpleTimeout{
				Value: annTimeout.Value,
			})
		}
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		c.ActiveTransactionHasChanges = true
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

	config, _ := c.ActiveConfiguration()
	switch annMaxconn.Status {
	case EMPTY:
		return false
	case DELETED:
		err = config.Set(parser.Defaults, parser.DefaultSectionName, "maxconn", nil)
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		c.Logger.Debug("Removing default maxconn")
	default:
		err = config.Set(parser.Defaults, parser.DefaultSectionName, "maxconn", types.Int64C{
			Value: value,
		})
		if err != nil {
			c.Logger.Error(err)
			return false
		}
		c.Logger.Debugf("Setting default maxconn to %d", value)
	}
	c.ActiveTransactionHasChanges = true
	return true
}

func (c *HAProxyController) handleDefaultLogFormat() bool {
	annLogFormat, _ := GetValueFromAnnotations("log-format", c.cfg.ConfigMap.Annotations)
	if annLogFormat.Status == EMPTY {
		return false
	}
	config, _ := c.ActiveConfiguration()
	err := config.Set(parser.Defaults, parser.DefaultSectionName, "log-format", types.StringC{
		Value: "'" + annLogFormat.Value + "'",
	})
	if err != nil {
		c.Logger.Error(err)
		return false
	}
	c.ActiveTransactionHasChanges = true
	return true
}
