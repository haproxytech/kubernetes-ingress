package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleTCPServices() (reload bool, err error) {
	if c.cfg.ConfigMapTCPServices == nil {
		return false, nil
	}
	for port, svc := range c.cfg.ConfigMapTCPServices.Annotations {
		// Get TCP service from ConfigMap
		// parts[0]: Service Name
		// parts[1]: Service Port
		// parts[2]: SSL option
		parts := strings.Split(svc.Value, ":")
		svcPort := parts[1]
		var sslOption string
		if len(parts) > 2 {
			sslOption = parts[2]
		}
		svcName := strings.Split(parts[0], "/")
		if len(svcName) != 2 {
			c.Logger.Errorf("incorrect Service Name '%s'", parts[0])
			continue
		}
		namespace := svcName[0]
		service := svcName[1]

		// Handle Frontend
		var frontendName, backendName string
		if svc.Status != EMPTY {
			backendName = fmt.Sprintf("%s-%s-%s", svcName[0], svcName[1], svcPort)
			frontendName = fmt.Sprintf("tcp-%s", port)
		}
		switch svc.Status {
		case DELETED:
			err = c.frontendDelete(frontendName)
			c.Logger.Panic(err)
			c.cfg.BackendSwitchingStatus["tcp-services"] = struct{}{}
			reload = true
			continue
		case MODIFIED:
			frontend, errFt := c.frontendGet(frontendName)
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
			if err = c.frontendEdit(frontend); err != nil {
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
			err = c.frontendCreate(frontend)
			if err != nil {
				c.Logger.Panic(err)
			}
			err = c.frontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			c.Logger.Panic(err)
			err = c.frontendBindCreate(frontendName, models.Bind{
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
			Namespace:   namespace,
			Annotations: MapStringW{},
			Rules:       map[string]*IngressRule{},
		}
		path := &IngressPath{
			ServiceName:    service,
			ServicePortInt: servicePort,
			IsTCPService:   true,
			Status:         svc.Status,
		}
		nsmmp := c.cfg.GetNamespace(namespace)
		r, errBck := c.handlePath(nsmmp, ingress, nil, path)
		c.Logger.Error(errBck)
		reload = reload || r
	}
	return reload, err
}
