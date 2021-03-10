package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	ingressRoute "github.com/haproxytech/kubernetes-ingress/controller/ingress"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type TCPHandler struct {
}

func (t TCPHandler) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	if k.ConfigMaps[TCPServices] == nil {
		return false, nil
	}
	for port, tcpSvc := range k.ConfigMaps[TCPServices].Annotations {
		// Get TCP service from ConfigMap
		// parts[0]: Service Name
		// parts[1]: Service Port
		// parts[2]: SSL option
		parts := strings.Split(tcpSvc.Value, ":")
		if len(parts) < 2 {
			logger.Errorf("incorrect format '%s', 'ServiceName:ServicePort' is required", tcpSvc.Value)
			continue
		}
		var sslOption string
		svcName := strings.Split(parts[0], "/")
		svcPort := parts[1]
		if len(parts) > 2 {
			sslOption = parts[2]
		}
		if len(svcName) != 2 {
			logger.Errorf("incorrect Service Name '%s'. Should be in 'ServiceNS/ServiceName' format", parts[0])
			continue
		}
		namespace := svcName[0]
		service := svcName[1]
		if _, ok := k.Namespaces[namespace]; !ok {
			logger.Warningf("tcp-services: namespace of service '%s/%s' not found", namespace, service)
			continue
		}
		svc, ok := k.Namespaces[namespace].Services[service]
		if !ok {
			logger.Warningf("tcp-services: service '%s/%s' not found", namespace, service)
			continue
		}
		// Delete Frontend
		frontendName := fmt.Sprintf("tcp-%s", port)
		if tcpSvc.Status == DELETED || svc.Status == DELETED {
			logger.Debugf("Deleting TCP frontend '%s'", frontendName)
			err = api.FrontendDelete(frontendName)
			if err != nil {
				logger.Errorf("error deleting tcp frontend: %s", err)
			} else {
				reload = true
			}
			continue
		}
		// Handle Route
		var portNbr int64
		if portNbr, err = strconv.ParseInt(svcPort, 10, 64); err != nil {
			logger.Error(err)
			continue
		}
		ingress := &store.Ingress{
			Namespace:   namespace,
			Annotations: store.MapStringW{},
			Rules:       map[string]*store.IngressRule{},
		}
		path := &store.IngressPath{
			SvcName:    service,
			SvcPortInt: portNbr,
			Status:     svc.Status,
		}
		route := &ingressRoute.Route{
			Namespace:  k.GetNamespace(namespace),
			Ingress:    ingress,
			Path:       path,
			TCPService: true,
		}
		err = route.SetBackendName()
		if err != nil {
			logger.Error(err)
			continue
		}
		frontend, errGet := api.FrontendGet(frontendName)
		if errGet != nil {
			// Create Frontend
			frontend = models.Frontend{
				Name:           frontendName,
				Mode:           "tcp",
				Tcplog:         true,
				DefaultBackend: route.BackendName,
			}
			var errors utils.Errors
			logger.Debugf("Creating TCP frontend '%s'", frontendName)
			errors.Add(api.FrontendCreate(frontend))
			errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
				Address: "0.0.0.0:" + port,
				Name:    "v4",
			}))
			errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
				Address: ":::" + port,
				Name:    "v6",
				V4v6:    true,
			}))
			if sslOption == "ssl" {
				errors.Add(api.FrontendEnableSSLOffload(frontend.Name, HAProxyFtCertDir, false))
			}
			if errors.Result() != nil {
				logger.Errorf("error configuring tcp frontend: %s", err)
				continue
			}
			reload = true
		} else if svc.Status != EMPTY {
			// Update  Frontend
			var errors utils.Errors
			logger.Debugf("Updating TCP frontend '%s'", frontendName)
			frontend.DefaultBackend = route.BackendName
			if sslOption == "ssl" {
				errors.Add(api.FrontendEnableSSLOffload(frontend.Name, HAProxyFtCertDir, false))
			} else {
				errors.Add(api.FrontendDisableSSLOffload(frontend.Name))
			}
			errors.Add(api.FrontendEdit(frontend))
			if errors.Result() != nil {
				logger.Errorf("error updating tcp frontend: %s", err)
				continue
			}
			reload = true
		}
		logger.Error(cfg.IngressRoutes.AddRoute(route))
	}
	return reload, err
}
