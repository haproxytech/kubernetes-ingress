package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/models/v2"
)

func (c *HAProxyController) handleTCPServices() (reload bool, err error) {
	if c.Store.ConfigMaps[TCPServices] == nil {
		return false, nil
	}
	for port, svc := range c.Store.ConfigMaps[TCPServices].Annotations {
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
			logger.Errorf("incorrect Service Name '%s'", parts[0])
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
			logger.Debugf("Deleting TCP frontend '%s'", frontendName)
			err = c.Client.FrontendDelete(frontendName)
			logger.Panic(err)
			c.cfg.BackendSwitchingModified["tcp-services"] = struct{}{}
			reload = true
			continue
		case MODIFIED:
			frontend, errFt := c.Client.FrontendGet(frontendName)
			if err != nil {
				logger.Panic(errFt)
				continue
			}
			frontend.DefaultBackend = backendName
			if sslOption == "ssl" {
				logger.Error(c.enableSSLOffload(frontend.Name, false))
			} else {
				logger.Error(c.disableSSLOffload(frontend.Name))
			}
			logger.Debugf("Updating TCP frontend '%s'", frontendName)
			if err = c.Client.FrontendEdit(frontend); err != nil {
				logger.Panic(err)
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
			logger.Debugf("Creating TCP frontend '%s'", frontendName)
			err = c.Client.FrontendCreate(frontend)
			if err != nil {
				logger.Panic(err)
			}
			err = c.Client.FrontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			logger.Panic(err)
			err = c.Client.FrontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "bind_2",
				V4v6:    true,
			})
			if err != nil {
				logger.Panic(err)
			}
			if sslOption == "ssl" {
				logger.Error(c.enableSSLOffload(frontend.Name, false))
			}
			reload = true
		}

		// Handle Backend
		var servicePort int64
		if servicePort, err = strconv.ParseInt(svcPort, 10, 64); err != nil {
			logger.Error(err)
			continue
		}
		ingress := &store.Ingress{
			Namespace:   namespace,
			Annotations: store.MapStringW{},
			Rules:       map[string]*store.IngressRule{},
		}
		path := &store.IngressPath{
			ServiceName:    service,
			ServicePortInt: servicePort,
			IsTCPService:   true,
			Status:         svc.Status,
		}
		nsmmp := c.Store.GetNamespace(namespace)
		reload = c.handlePath(nsmmp, ingress, nil, path) || reload
	}
	return reload, err
}
