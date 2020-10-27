package annotations

import (
	"errors"
	"strconv"
	"strings"

	"github.com/haproxytech/config-parser/v2/types"

	"github.com/haproxytech/kubernetes-ingress/controller/configsnippet"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type syslogServers struct {
	data   []*types.Log
	stdout bool
}

func (a *syslogServers) Overridden(configSnippet string) error {
	return configsnippet.NewGenericAttribute("log").Overridden(configSnippet)
}

// Input is multiple syslog lines
// Each syslog line is a list of params
// Example:
//  syslog-server: |
//    address:127.0.0.1, port:514, facility:local0
//    address:192.168.1.1, port:514, facility:local1
func (a *syslogServers) Parse(input string) error {
	a.data = nil
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
				logger.Errorf("incorrect syslog param: '%s' in '%s'", param, syslogLine)
				continue
			}
		}
		// populate annotation data
		logData := new(types.Log)
		if address, ok := logParams["address"]; !ok {
			logger.Errorf("incorrect syslog Line: no address param in '%s'", syslogLine)
			continue
		} else {
			logData.Address = address
		}
		for k, v := range logParams {
			switch strings.ToLower(k) {
			case "address":
				if v == "stdout" {
					a.stdout = true
				}
			case "port":
				if logParams["address"] != "stdout" {
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
				logger.Errorf("unkown syslog param: '%s' in '%s' ", k, syslogLine)
				continue
			}
		}
		a.data = append(a.data, logData)
	}
	if len(a.data) == 0 {
		return errors.New("could not parse syslog-server annotation")
	}
	return nil
}

func (a *syslogServers) Delete(c api.HAProxyClient) Result {
	logger.Infof("Removing log targets ")
	if err := c.LogTarget(nil, -1); err != nil {
		logger.Error(err)
		return NONE
	}
	return RELOAD
}

func (a *syslogServers) Update(c api.HAProxyClient) Result {
	if len(a.data) == 0 {
		logger.Error("unable to update syslogServer: nil value")
		return NONE
	}
	a.Delete(c)
	var r Result
	for i, syslog := range a.data {
		logger.Infof("adding syslog server: 'address: %s, facility: %s'", syslog.Address, syslog.Facility)
		if err := c.LogTarget(syslog, i); err != nil {
			logger.Error(err)
		} else {
			r = RELOAD
		}
	}
	// stdout logging won't work with daemon mode
	daemonMode, err := c.GlobalConfigEnabled("global", "daemon")
	logger.Error(err)
	if a.stdout {
		if daemonMode {
			logger.Info("Disabling Daemon mode")
			logger.Error(c.DaemonMode(nil))
			r = RESTART
		}
	} else if !daemonMode {
		logger.Info("Enabling Daemon mode")
		logger.Error(c.DaemonMode(&types.Enabled{}))
		r = RESTART
	}
	return r
}
