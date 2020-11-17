package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	ingressRoute "github.com/haproxytech/kubernetes-ingress/controller/ingress"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/models/v2"
)

type TCPHandler struct {
}

func (t TCPHandler) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	if k.ConfigMaps[TCPServices] == nil {
		return false, nil
	}
	for port, svc := range k.ConfigMaps[TCPServices].Annotations {
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
			err = api.FrontendDelete(frontendName)
			logger.Panic(err)
			reload = true
			continue
		case MODIFIED:
			frontend, errFt := api.FrontendGet(frontendName)
			if err != nil {
				logger.Panic(errFt)
				continue
			}
			frontend.DefaultBackend = backendName
			if sslOption == "ssl" {
				logger.Error(api.FrontendEnableSSLOffload(frontend.Name, HAProxyFtCertDir, false))
			} else {
				logger.Error(api.FrontendDisableSSLOffload(frontend.Name))
			}
			logger.Debugf("Updating TCP frontend '%s'", frontendName)
			if err = api.FrontendEdit(frontend); err != nil {
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
			err = api.FrontendCreate(frontend)
			if err != nil {
				logger.Panic(err)
			}
			err = api.FrontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "bind_1",
			})
			logger.Panic(err)
			err = api.FrontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "bind_2",
				V4v6:    true,
			})
			if err != nil {
				logger.Panic(err)
			}
			if sslOption == "ssl" {
				logger.Error(api.FrontendEnableSSLOffload(frontend.Name, HAProxyFtCertDir, false))
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
			Status:         svc.Status,
		}
		nsmmp := k.GetNamespace(namespace)
		cfg.IngressRoutes.AddRoute(&ingressRoute.Route{
			Namespace:  nsmmp,
			Ingress:    ingress,
			Path:       path,
			TCPService: true,
		})
	}
	return reload, err
}
