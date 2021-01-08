package annotations

import (
	"errors"
	"strconv"
	"strings"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalSyslogServers struct {
	name    string
	data    []*types.Log
	client  api.HAProxyClient
	stdout  bool
	restart bool
}

func NewGlobalSyslogServers(n string, c api.HAProxyClient) *GlobalSyslogServers {
	return &GlobalSyslogServers{name: n, client: c}
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
	a.data = nil
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

func (a *GlobalSyslogServers) Update() error {
	err := a.client.LogTarget(nil, -1)
	if err != nil {
		return err
	}
	if len(a.data) == 0 {
		logger.Infof("log targets removed")
	}
	for i, syslog := range a.data {
		logger.Infof("adding syslog server: 'address: %s, facility: %s'", syslog.Address, syslog.Facility)
		if err = a.client.LogTarget(syslog, i); err != nil {
			return err
		}
	}
	// stdout logging won't work with daemon mode
	daemonMode, err := a.client.GlobalConfigEnabled("global", "daemon")
	if err != nil {
		return err
	}
	if a.stdout {
		if daemonMode {
			logger.Info("Disabling Daemon mode")
			if err = a.client.DaemonMode(nil); err != nil {
				return err
			}
			a.restart = true
		}
	} else if !daemonMode {
		logger.Info("Enabling Daemon mode")
		if err = a.client.DaemonMode(&types.Enabled{}); err != nil {
			return err
		}
		a.restart = true
	}
	return nil
}

func (a *GlobalSyslogServers) Restart() bool {
	return a.restart
}
