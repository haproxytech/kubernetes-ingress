package global

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type SyslogServers struct {
	name       string
	global     *models.Global
	logTargets models.LogTargets
	client     api.HAProxyClient
	stdout     bool
	restart    bool
}

func NewSyslogServers(n string, c api.HAProxyClient, g *models.Global) *SyslogServers {
	return &SyslogServers{name: n, client: c, global: g}
}

func (a *SyslogServers) GetName() string {
	return a.name
}

// Input is multiple syslog lines
// Each syslog line is a list of params
// Example:
//  syslog-server: |
//    address:127.0.0.1, port:514, facility:local0
//    address:192.168.1.1, port:514, facility:local1
func (a *SyslogServers) Process(input string) error {
	a.stdout = false
	for _, syslogLine := range strings.Split(input, "\n") {
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
				return fmt.Errorf("incorrect syslog param: '%s' in '%s'", param, syslogLine)
			}
		}
		// populate annotation data
		logTarget := models.LogTarget{Index: utils.PtrInt64(0)}
		address, ok := logParams["address"]
		if !ok {
			return fmt.Errorf("incorrect syslog Line: no address param in '%s'", syslogLine)
		}
		logTarget.Address = address
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
				return fmt.Errorf("unknown syslog param: '%s' in '%s' ", k, syslogLine)
			}
		}
		a.logTargets = append(a.logTargets, &logTarget)
	}

	// Update
	var err error
	a.client.GlobalDeleteLogTargets()
	if len(a.logTargets) == 0 {
		return nil
	}
	for _, logTarget := range a.logTargets {
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
			a.global.Daemon = "disabled"
			a.restart = true
		}
	} else if !daemonMode {
		a.global.Daemon = "enabled"
		a.restart = true
	}
	return nil
}
