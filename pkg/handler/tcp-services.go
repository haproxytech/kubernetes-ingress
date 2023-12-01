package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type TCPServices struct {
	CertDir  string
	AddrIPv4 string
	AddrIPv6 string
	IPv4     bool
	IPv6     bool
}

type tcpSvcParser struct {
	service    *store.Service
	port       int64
	sslOffload bool
}

func (handler TCPServices) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	if k.ConfigMaps.TCPServices == nil {
		return nil
	}
	handler.clearFrontends(k, h)
	var p tcpSvcParser
	logFormat := k.ConfigMaps.TCPServices.Annotations["log-format-tcp"]

	for port, tcpSvcAnn := range k.ConfigMaps.TCPServices.Annotations {
		if port == "log-format-tcp" {
			continue
		}
		frontendName := fmt.Sprintf("tcp-%s", port)
		p, err = handler.parseTCPService(k, tcpSvcAnn)
		if err != nil {
			logger.Error(err)
			continue
		}
		frontend, errGet := h.FrontendGet(frontendName)
		if errGet != nil {
			frontend = models.Frontend{
				Name: frontendName,
				Mode: "tcp",
			}
		}

		if logFormat != "" {
			frontend.LogFormat = "'" + strings.TrimSpace(logFormat) + "'"
			frontend.Tcplog = false
		} else {
			frontend.LogFormat = ""
			frontend.Tcplog = true
		}

		// Create Frontend
		if errGet != nil {
			err = handler.createTCPFrontend(h, frontend, port, p.sslOffload)
			if err != nil {
				logger.Error(err)
				continue
			}
		}

		// Update  Frontend
		err = handler.updateTCPFrontend(k, h, frontend, p, a)
		if err != nil {
			logger.Errorf("TCP frontend '%s': update failed: %s", frontendName, err)
		}
	}
	return nil
}

func (handler TCPServices) parseTCPService(store store.K8s, input string) (p tcpSvcParser, err error) {
	// parts[0]: Service Name
	// parts[1]: Service Port
	// parts[2]: SSL option
	parts := strings.Split(input, ":")
	if len(parts) < 2 {
		err = fmt.Errorf("incorrect format '%s', 'ServiceName:ServicePort' is required", input)
		return
	}
	svcName := strings.Split(parts[0], "/")
	svcPort := parts[1]
	if len(parts) > 2 {
		if parts[2] == "ssl" {
			p.sslOffload = true
		}
	}
	if len(svcName) != 2 {
		err = fmt.Errorf("incorrect Service Name '%s'. Should be in 'ServiceNS/ServiceName' format", parts[0])
		return
	}
	namespace := svcName[0]
	service := svcName[1]
	var ok bool
	if _, ok = store.Namespaces[namespace]; !ok {
		err = fmt.Errorf("tcp-services: namespace of service '%s/%s' not found", namespace, service)
		return
	}
	p.service, ok = store.Namespaces[namespace].Services[service]
	if !ok {
		err = fmt.Errorf("tcp-services: service '%s/%s' not found", namespace, service)
		return
	}
	if p.port, err = strconv.ParseInt(svcPort, 10, 64); err != nil {
		return
	}
	return p, err
}

func (handler TCPServices) clearFrontends(k store.K8s, h haproxy.HAProxy) {
	frontends, err := h.FrontendsGet()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, ft := range frontends {
		_, isRequired := k.ConfigMaps.TCPServices.Annotations[strings.TrimPrefix(ft.Name, "tcp-")]
		isTCPSvc := strings.HasPrefix(ft.Name, "tcp-")
		if isTCPSvc && !isRequired {
			err = h.FrontendDelete(ft.Name)
			if err != nil {
				logger.Errorf("error deleting tcp frontend '%s': %s", ft.Name, err)
			}
			instance.ReloadIf(err == nil, "TCP frontend '%s' deleted", ft.Name)
		}
	}
}

func (handler TCPServices) createTCPFrontend(h haproxy.HAProxy, frontend models.Frontend, bindPort string, sslOffload bool) (err error) {
	var errors utils.Errors
	errors.Add(h.FrontendCreate(frontend))
	if handler.IPv4 {
		errors.Add(h.FrontendBindCreate(frontend.Name, models.Bind{
			Address: handler.AddrIPv4 + ":" + bindPort,
			BindParams: models.BindParams{
				Name: "v4",
			},
		}))
	}
	if handler.IPv6 {
		errors.Add(h.FrontendBindCreate(frontend.Name, models.Bind{
			Address: handler.AddrIPv6 + ":" + bindPort,
			BindParams: models.BindParams{
				Name: "v6",
				V4v6: true,
			},
		}))
	}
	if sslOffload {
		errors.Add(h.FrontendEnableSSLOffload(frontend.Name, handler.CertDir, "", false))
	}
	if errors.Result() != nil {
		err = fmt.Errorf("error configuring tcp frontend: %w", err)
		return err
	}
	instance.Reload("TCP frontend '%s' created", frontend.Name)
	return
}

func (handler TCPServices) updateTCPFrontend(k store.K8s, h haproxy.HAProxy, frontend models.Frontend, p tcpSvcParser, a annotations.Annotations) (err error) {
	prevFrontend, err := h.FrontendGet(frontend.Name)
	if err != nil {
		err = fmt.Errorf("failed to get frontend '%s' : %w", frontend.Name, err)
		return
	}

	if prevFrontend.LogFormat != frontend.LogFormat {
		err = h.FrontendEdit(frontend)
		if err != nil {
			return
		}

		instance.Reload("log format TCP changed from configmap '%s/%s'", k.ConfigMaps.TCPServices.Namespace, k.ConfigMaps.TCPServices.Name)
	}
	binds, err := h.FrontendBindsGet(frontend.Name)
	if err != nil {
		err = fmt.Errorf("failed to get bind lines: %w", err)
		return
	}
	if !binds[0].Ssl && p.sslOffload {
		err = h.FrontendEnableSSLOffload(frontend.Name, handler.CertDir, "", false)
		if err != nil {
			err = fmt.Errorf("failed to enable SSL offload: %w", err)
			return
		}
		instance.Reload("TCP frontend '%s': ssl offload enabled", frontend.Name)
	}
	if binds[0].Ssl && !p.sslOffload {
		err = h.FrontendDisableSSLOffload(frontend.Name)
		if err != nil {
			err = fmt.Errorf("failed to disable SSL offload: %w", err)
			return
		}
		instance.Reload("TCP frontend '%s': ssl offload disabled", frontend.Name)
	}
	if p.service.Status == store.DELETED {
		frontend.DefaultBackend = ""
		err = h.FrontendEdit(frontend)
		instance.Reload("TCP frontend '%s': service '%s/%s' deleted", frontend.Name, p.service.Namespace, p.service.Name)
		return
	}

	var svc *service.Service
	path := &store.IngressPath{
		SvcNamespace:     p.service.Namespace,
		SvcName:          p.service.Name,
		SvcPortInt:       p.port,
		IsDefaultBackend: true,
	}
	if svc, err = service.New(k, path, nil, true, nil); err == nil {
		err = svc.SetDefaultBackend(k, h, []string{frontend.Name}, a)
	}
	return err
}
