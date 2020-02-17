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

		if svc.Status == EMPTY {
			continue
		}

		// Set frontend and backend names
		backendName := strings.ReplaceAll(strings.ReplaceAll(svc.Value, "/", "-"), ":", "-")
		frontendName := fmt.Sprintf("tcp-%s", port)

		// Handle Frontend
		var frontend models.Frontend
		portInt64 := int64(-1)
		if prt, errParse := strconv.ParseInt(portDest, 10, 64); errParse == nil {
			portInt64 = prt
		}
		switch svc.Status {
		case DELETED:
			err = c.frontendDelete(frontendName)
			utils.PanicErr(err)
			needsReload = true
			c.cfg.BackendSwitchingStatus["tcp-services"] = struct{}{}
			continue
		case MODIFIED:
			if frontend, err = c.frontendGet(frontendName); err != nil {
				utils.PanicErr(err)
				continue
			}
			frontend.DefaultBackend = backendName
			if err = c.frontendEdit(frontend); err != nil {
				utils.PanicErr(err)
				continue
			}
		case ADDED:
			frontend = models.Frontend{
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
		ingress := &Ingress{
			Namespace:   namespace,
			Annotations: MapStringW{},
			Rules:       map[string]*IngressRule{},
		}
		path := &IngressPath{
			ServiceName:    service,
			ServicePortInt: portInt64,
			IsTCPService:   true,
			Status:         svc.Status,
		}
		nsmmp := c.cfg.GetNamespace(namespace)
		_, err = c.handlePath(nsmmp, ingress, nil, path)
		utils.PanicErr(err)
		needsReload = true
	}
	return needsReload, err
}
