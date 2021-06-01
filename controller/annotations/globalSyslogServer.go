package annotations

import (
	"errors"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type GlobalSyslogServers struct {
	name       string
	global     *models.Global
	logTargets models.LogTargets
	client     api.HAProxyClient
	stdout     bool
	restart    bool
}

func NewGlobalSyslogServers(n string, c api.HAProxyClient, g *models.Global) *GlobalSyslogServers {
	return &GlobalSyslogServers{name: n, client: c, global: g}
}

func (a *GlobalSyslogServers) GetName() string {
	return a.name
}

// Input is multiple syslog lines
// Each syslog line is a list of params
// Example:
//  syslog-server: |
//    address:127.0.0.1, port:514, facility:local0
//    address:192.168.1.1, port:514, facility:local1
func (a *GlobalSyslogServers) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	a.stdout = false
	for _, syslogLine := range strings.Split(input.Value, "\n") {
		if syslogLine == "" {
			continue
		}
		// strip spaces
		syslogLine = strings.Join(strings.Fields(syslogLine), "")
		// parse log params
		logParams := make(map[string]string)
		for _, param := range strings.Split(syslogLine, ",") {
			if param == "" {
				continue
			}
			parts := strings.Split(param, ":")
			// param should be key: value
			if len(parts) == 2 {
				logParams[parts[0]] = parts[1]
			} else {
				logger.Errorf("incorrect syslog param: '%s' in '%s'", param, syslogLine)
				continue
			}
		}
		// populate annotation data
		logTarget := models.LogTarget{Index: utils.PtrInt64(0)}
		if address, ok := logParams["address"]; !ok {
			logger.Errorf("incorrect syslog Line: no address param in '%s'", syslogLine)
			continue
		} else {
			logTarget.Address = address
		}
		for k, v := range logParams {
			switch strings.ToLower(k) {
			case "address":
				if v == "stdout" {
					a.stdout = true
				}
			case "port":
				if logParams["address"] != "stdout" {
					logTarget.Address += ":" + v
				}
			case "length":
				if length, errConv := strconv.Atoi(v); errConv == nil {
					logTarget.Length = int64(length)
				}
			case "format":
				logTarget.Format = v
			case "facility":
				logTarget.Facility = v
			case "level":
				logTarget.Level = v
			case "minlevel":
				logTarget.Minlevel = v
			default:
				logger.Errorf("unknown syslog param: '%s' in '%s' ", k, syslogLine)
				continue
			}
		}
		a.logTargets = append(a.logTargets, &logTarget)
	}
	if len(a.logTargets) == 0 {
		return errors.New("could not parse syslog-server annotation")
	}
	return nil
}

func (a *GlobalSyslogServers) Update() error {
	a.client.GlobalDeleteLogTargets()
	if len(a.logTargets) == 0 {
		logger.Infof("log targets removed")
		return nil
	}
	var err error
	for _, logTarget := range a.logTargets {
		logger.Infof("adding syslog server: 'address: %s, facility: %s'", logTarget.Address, logTarget.Facility)
		err = a.client.GlobalCreateLogTarget(logTarget)
		if err != nil {
			return err
		}
	}
	// stdout logging won't work with daemon mode
	var daemonMode bool
	if a.global.Daemon == "enabled" {
		daemonMode = true
	}
	if err != nil {
		return err
	}
	if a.stdout {
		if daemonMode {
			logger.Info("Disabling Daemon mode")
			a.global.Daemon = "disabled"
			a.restart = true
		}
	} else if !daemonMode {
		logger.Info("Enabling Daemon mode")
		a.global.Daemon = "enabled"
		a.restart = true
	}
	return nil
}

func (a *GlobalSyslogServers) Restart() bool {
	return a.restart
}
