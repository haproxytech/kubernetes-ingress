package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/models/v2"
)

func tcpOptions(raw string) (ns, name, port, sslOption, proxyOption string, err error) {
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		err = fmt.Errorf("non well-formed service '%s'", parts)
		return
	}
	// parts[0]: Service Name
	// parts[1]: Service Port
	// parts[2]: SSL option
	// parts[3]: PROXY option
	port = parts[1]
	if len(parts) > 2 {
		sslOption = parts[2]
	}
	if len(parts) > 3 {
		proxyOption = parts[3]
	}
	namespaced := strings.Split(parts[0], "/")
	if len(namespaced) != 2 {
		err = fmt.Errorf("incorrect Service Name '%s'", parts[0])
		return
	}
	ns = namespaced[0]
	name = namespaced[1]
	return
}

func (c *HAProxyController) handleTCPServices() (reload bool, err error) {
	if c.cfg.ConfigMapTCPServices == nil {
		return false, nil
	}
	for port, svc := range c.cfg.ConfigMapTCPServices.Annotations {
		svcNs, svcName, svcPort, sslOption, _, tcpErr := tcpOptions(svc.Value)
		if tcpErr != nil {
			c.Logger.Errorf(tcpErr.Error())
			continue
		}
		// Handle Frontend
		var frontendName, backendName string
		if svc.Status != EMPTY {
			backendName = fmt.Sprintf("%s-%s-%s", svcNs, svcName, svcPort)
			frontendName = fmt.Sprintf("tcp-%s", port)
		}
		switch svc.Status {
		case DELETED:
			c.Logger.Debugf("Deleting TCP frontend '%s'", frontendName)
			err = c.Client.FrontendDelete(frontendName)
			c.Logger.Panic(err)
			c.cfg.BackendSwitchingStatus["tcp-services"] = struct{}{}
			reload = true
			continue
		case MODIFIED:
			frontend, errFt := c.Client.FrontendGet(frontendName)
			if err != nil {
				c.Logger.Panic(errFt)
				continue
			}
			frontend.DefaultBackend = backendName
			if sslOption == "ssl" {
				c.Logger.Error(c.enableSSLOffload(frontend.Name, false))
			} else {
				c.Logger.Error(c.disableSSLOffload(frontend.Name))
			}
			c.Logger.Debugf("Updating TCP frontend '%s'", frontendName)
			if err = c.Client.FrontendEdit(frontend); err != nil {
				c.Logger.Panic(err)
				continue
			}
			reload = true
		case ADDED:
			frontend := models.Frontend{
				Name:           frontendName,
				Mode:           "tcp",
				Tcplog:         true,
				DefaultBackend: backendName,
			}
			c.Logger.Debugf("Creating TCP frontend '%s'", frontendName)
			err = c.Client.FrontendCreate(frontend)
			if err != nil {
				c.Logger.Panic(err)
			}
			err = c.Client.FrontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			c.Logger.Panic(err)
			err = c.Client.FrontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "bind_2",
				V4v6:    true,
			})
			if err != nil {
				c.Logger.Panic(err)
			}
			if sslOption == "ssl" {
				c.Logger.Error(c.enableSSLOffload(frontend.Name, false))
			}
			reload = true
		}

		// Handle Backend
		var servicePort int64
		if servicePort, err = strconv.ParseInt(svcPort, 10, 64); err != nil {
			c.Logger.Error(err)
			continue
		}
		ingress := &Ingress{
			Namespace:   svcNs,
			Annotations: MapStringW{},
			Rules:       map[string]*IngressRule{},
		}
		path := &IngressPath{
			ServiceName:    svcName,
			ServicePortInt: servicePort,
			IsTCPService:   true,
			Status:         svc.Status,
		}
		nsmmp := c.cfg.GetNamespace(svcNs)
		r, errBck := c.handlePath(nsmmp, ingress, nil, path)
		c.Logger.Error(errBck)
		reload = reload || r
	}
	return reload, err
}
