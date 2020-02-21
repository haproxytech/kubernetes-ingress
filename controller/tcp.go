package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleTCPServices() (needsReload bool, err error) {
	if c.cfg.ConfigMapTCPServices == nil {
		return false, nil
	}
	for port, svc := range c.cfg.ConfigMapTCPServices.Annotations {
		// Get TCP service from ConfigMap
		parts := strings.Split(svc.Value, ":")
		portDest := parts[1]
		parts = strings.Split(parts[0], "/")
		namespace := parts[0]
		service := parts[1]

		// Handle Frontend
		var frontendName, backendName string
		if svc.Status != EMPTY {
			backendName = strings.ReplaceAll(strings.ReplaceAll(svc.Value, "/", "-"), ":", "-")
			frontendName = fmt.Sprintf("tcp-%s", port)
		}
		switch svc.Status {
		case DELETED:
			err = c.frontendDelete(frontendName)
			utils.PanicErr(err)
			needsReload = true
			c.cfg.BackendSwitchingStatus["tcp-services"] = struct{}{}
			continue
		case MODIFIED:
			frontend, errFt := c.frontendGet(frontendName)
			if err != nil {
				utils.PanicErr(errFt)
				continue
			}
			frontend.DefaultBackend = backendName
			if err = c.frontendEdit(frontend); err != nil {
				utils.PanicErr(err)
				continue
			}
		case ADDED:
			frontend := models.Frontend{
				Name:           frontendName,
				Mode:           "tcp",
				Tcplog:         true,
				DefaultBackend: backendName,
			}
			err = c.frontendCreate(frontend)
			utils.PanicErr(err)
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
			utils.PanicErr(err)
			needsReload = true
		}

		// Handle Backend
		var servicePort int64
		if servicePort, err = strconv.ParseInt(portDest, 10, 64); err != nil {
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
		reload, errBck := c.handlePath(nsmmp, ingress, nil, path)
		utils.PanicErr(errBck)
		needsReload = needsReload || reload
	}
	return needsReload, err
}
