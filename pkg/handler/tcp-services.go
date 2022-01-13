package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
	config "github.com/haproxytech/kubernetes-ingress/pkg/configuration"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type TCPServices struct {
	IPv4     bool
	IPv6     bool
	CertDir  string
	AddrIPv4 string
	AddrIPv6 string
}

type tcpSvcParser struct {
	service    *store.Service
	port       int64
	sslOffload bool
}

func (t TCPServices) Update(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient) (reload bool, err error) {
	if k.ConfigMaps.TCPServices == nil {
		return false, nil
	}
	var r bool
	reload = t.clearFrontends(api, k)
	var p tcpSvcParser
	for port, tcpSvcAnn := range k.ConfigMaps.TCPServices.Annotations {
		frontendName := fmt.Sprintf("tcp-%s", port)
		p, err = t.parseTCPService(k, tcpSvcAnn)
		if err != nil {
			logger.Error(err)
			continue
		}
		frontend, errGet := api.FrontendGet(frontendName)
		// Create Frontend
		if errGet != nil {
			frontend, r, err = t.createTCPFrontend(api, frontendName, port, p.sslOffload)
			reload = reload || r
			if err != nil {
				logger.Error(err)
				continue
			}
		}
		// Update  Frontend
		r, err = t.updateTCPFrontend(k, cfg, api, frontend, p)
		reload = reload || r
		if err != nil {
			logger.Errorf("TCP frontend '%s': update failed: %s", frontendName, err)
		}
	}
	return reload, nil
}

func (t TCPServices) parseTCPService(store store.K8s, input string) (p tcpSvcParser, err error) {
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

func (t TCPServices) clearFrontends(api api.HAProxyClient, k store.K8s) (cleared bool) {
	frontends, err := api.FrontendsGet()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, ft := range frontends {
		_, isRequired := k.ConfigMaps.TCPServices.Annotations[strings.TrimPrefix(ft.Name, "tcp-")]
		isTCPSvc := strings.HasPrefix(ft.Name, "tcp-")
		if isTCPSvc && !isRequired {
			err = api.FrontendDelete(ft.Name)
			if err != nil {
				logger.Errorf("error deleting tcp frontend '%s': %s", ft.Name, err)
			} else {
				cleared = true
				logger.Debugf("TCP frontend '%s' deleted, reload required", ft.Name)
			}
		}
	}
	return
}

func (t TCPServices) createTCPFrontend(api api.HAProxyClient, frontendName, bindPort string, sslOffload bool) (frontend models.Frontend, reload bool, err error) {
	// Create Frontend
	frontend = models.Frontend{
		Name:   frontendName,
		Mode:   "tcp",
		Tcplog: true,
		//	DefaultBackend: route.BackendName,
	}
	var errors utils.Errors
	errors.Add(api.FrontendCreate(frontend))
	if t.IPv4 {
		errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
			Address: t.AddrIPv4 + ":" + bindPort,
			Name:    "v4",
		}))
	}
	if t.IPv6 {
		errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
			Address: t.AddrIPv6 + ":" + bindPort,
			Name:    "v6",
			V4v6:    true,
		}))
	}
	if sslOffload {
		errors.Add(api.FrontendEnableSSLOffload(frontend.Name, t.CertDir, "", false))
	}
	if errors.Result() != nil {
		err = fmt.Errorf("error configuring tcp frontend: %w", err)
		return frontend, false, err
	}
	logger.Debugf("TCP frontend '%s' created, reload required", frontendName)
	return frontend, true, nil
}

func (t TCPServices) updateTCPFrontend(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient, frontend models.Frontend, p tcpSvcParser) (reload bool, err error) {
	binds, err := api.FrontendBindsGet(frontend.Name)
	if err != nil {
		err = fmt.Errorf("failed to get bind lines: %w", err)
		return
	}
	if !binds[0].Ssl && p.sslOffload {
		err = api.FrontendEnableSSLOffload(frontend.Name, t.CertDir, "", false)
		if err != nil {
			err = fmt.Errorf("failed to enable SSL offload: %w", err)
			return
		}
		logger.Debugf("TCP frontend '%s': ssl offload enabled, reload required", frontend.Name)
		reload = true
	}
	if binds[0].Ssl && !p.sslOffload {
		err = api.FrontendDisableSSLOffload(frontend.Name)
		if err != nil {
			err = fmt.Errorf("failed to disable SSL offload: %w", err)
			return
		}
		logger.Debugf("TCP frontend '%s': ssl offload disabled, reload required", frontend.Name)
		reload = true
	}
	if p.service.Status == store.DELETED {
		frontend.DefaultBackend = ""
		err = api.FrontendEdit(frontend)
		reload = true
		return
	}

	var svc *service.Service
	var r bool
	path := &store.IngressPath{
		SvcNamespace:     p.service.Namespace,
		SvcName:          p.service.Name,
		SvcPortInt:       p.port,
		IsDefaultBackend: true,
	}
	if svc, err = service.New(k, path, nil, true); err == nil {
		r, err = svc.SetDefaultBackend(k, cfg, api, []string{frontend.Name})
	}
	return reload || r, err
}
