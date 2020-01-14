package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) handleTCPServices() (needsReload bool, err error) {
	if c.cfg.ConfigMapTCPServices == nil {
		return false, nil
	}
	for port, svc := range c.cfg.ConfigMapTCPServices.Annotations {
		parts := strings.Split(svc.Value, ":")
		portDest := parts[1]
		parts = strings.Split(parts[0], "/")
		namespace := parts[0]
		service := parts[1]

		backendName := strings.ReplaceAll(strings.ReplaceAll(svc.Value, "/", "-"), ":", "-")
		frontendName := fmt.Sprintf("tcp-frontend-%s-%s", backendName, port)
		//log.Println(port, svc, frontendName)
		var frontend models.Frontend
		frontend, err = c.frontendGet(frontendName)
		portInt64 := int64(-1)
		if prt, errParse := strconv.ParseInt(portDest, 10, 64); errParse == nil {
			portInt64 = prt
		}
		switch svc.Status {
		case ADDED:
			if err == nil {
				LogErr(err)
				continue
			}
			frontend = models.Frontend{
				Name:           frontendName,
				Mode:           "tcp",
				Tcplog:         true,
				DefaultBackend: backendName,
			}
			err = c.frontendCreate(frontend)
			LogErr(err)
			err = c.frontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			LogErr(err)
			err = c.frontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "bind_2",
				V4v6:    true,
			})
			LogErr(err)
			ingress := &Ingress{
				Namespace:   namespace,
				Annotations: MapStringW{},
				Rules:       map[string]*IngressRule{},
			}
			path := &IngressPath{
				ServiceName:    service,
				ServicePortInt: portInt64,
				PathIndex:      -1,
				IsTCPService:   true,
			}
			nsmmp := c.cfg.GetNamespace(namespace)
			_, err = c.handlePath(nsmmp, ingress, nil, path)
			LogErr(err)
			needsReload = true
		case MODIFIED:
			LogErr(err)
			//TODO delete old, create new
		case DELETED:
			LogErr(err)
			err = c.frontendDelete(frontendName)
			LogErr(err)
			needsReload = true
		}
	}
	return needsReload, err
}
