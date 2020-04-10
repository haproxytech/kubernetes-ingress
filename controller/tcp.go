package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
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
			utils.LogErr(fmt.Errorf("incorrect Service Name '%s'", parts[0]))
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
			utils.PanicErr(err)
			c.cfg.BackendSwitchingStatus["tcp-services"] = struct{}{}
			reload = true
			continue
		case MODIFIED:
			frontend, errFt := c.frontendGet(frontendName)
			if err != nil {
				utils.PanicErr(errFt)
				continue
			}
			frontend.DefaultBackend = backendName
			if sslOption == "ssl" {
				utils.LogErr(c.enableSSLOffload(frontend.Name, false))
			} else {
				utils.LogErr(c.disableSSLOffload(frontend.Name))
			}
			if err = c.frontendEdit(frontend); err != nil {
				utils.PanicErr(err)
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
				utils.PanicErr(err)
				continue
			}
			err = c.frontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			utils.PanicErr(err)
			err = c.frontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "bind_2",
				V4v6:    true,
			})
			if err != nil {
				utils.PanicErr(err)
				continue
			}
			if sslOption == "ssl" {
				utils.LogErr(c.enableSSLOffload(frontend.Name, false))
			}
			reload = true
		}

		// Handle Backend
		var servicePort int64
		if servicePort, err = strconv.ParseInt(svcPort, 10, 64); err != nil {
			utils.LogErr(err)
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
		utils.LogErr(errBck)
		reload = reload || r
	}
	return reload, err
}
