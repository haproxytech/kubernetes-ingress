package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/client-native/v2/models"
	config "github.com/haproxytech/kubernetes-ingress/controller/configuration"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type TCPServices struct {
	SetDefaultService func(ingress *store.Ingress, frontends []string) (reload bool, err error)
	IPv4              bool
	IPv6              bool
	CertDir           string
	AddrIPv4          string
	AddrIPv6          string
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
	var p tcpSvcParser
	for port, tcpSvcAnn := range k.ConfigMaps.TCPServices.Annotations {
		p, err = t.parseTCPService(k, tcpSvcAnn.Value)
		if err != nil {
			logger.Error(err)
			continue
		}
		// Delete Frontend
		frontendName := fmt.Sprintf("tcp-%s", port)
		if tcpSvcAnn.Status == store.DELETED || p.service.Status == store.DELETED {
			err = api.FrontendDelete(frontendName)
			if err != nil {
				logger.Errorf("error deleting tcp frontend '%s': %s", frontendName, err)
			} else {
				reload = true
				logger.Debugf("TCP frontend '%s' deleted, reload required", frontendName)
			}
			continue
		}
		frontend, errGet := api.FrontendGet(frontendName)
		// Create Frontend
		if errGet != nil {
			frontend, reload, err = t.createTCPFrontend(k, cfg, api, frontendName, port, p.sslOffload)
			if err != nil {
				logger.Error(err)
				continue
			}
		}
		// Update  Frontend
		reload, err = t.updateTCPFrontend(k, cfg, api, frontend, p)
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

func (t TCPServices) createTCPFrontend(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient,
	frontendName, bindPort string, sslOffload bool) (frontend models.Frontend, reload bool, err error) {
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
		errors.Add(api.FrontendEnableSSLOffload(frontend.Name, t.CertDir, false))
		errors.Add(t.handleClientTLSAuth(k, cfg, api, frontendName))
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
		err = api.FrontendEnableSSLOffload(frontend.Name, t.CertDir, false)
		if err != nil {
			err = fmt.Errorf("failed to enable SSL offload: %w", err)
			return
		}
		err = t.handleClientTLSAuth(k, cfg, api, frontend.Name)
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
	ingress := &store.Ingress{
		Namespace:   p.service.Namespace,
		Annotations: store.MapStringW{},
		DefaultBackend: &store.IngressPath{
			SvcName:    p.service.Name,
			SvcPortInt: p.port,
		},
	}
	r, err := t.SetDefaultService(ingress, []string{frontend.Name})
	if err != nil {
		return
	}

	return reload || r, err
}

func (t TCPServices) handleClientTLSAuth(k store.K8s, cfg *config.ControllerCfg, api api.HAProxyClient, frontendName string) error {
	annTLSAuth, _ := k.GetValueFromAnnotations("client-ca", k.ConfigMaps.Main.Annotations)
	annTLSVerify, _ := k.GetValueFromAnnotations("client-crt-optional", k.ConfigMaps.Main.Annotations)
	if annTLSAuth == nil {
		return nil
	}
	binds, err := api.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}
	caFile, secretUpdated, secretErr := cfg.Certificates.HandleTLSSecret(k, haproxy.SecretCtx{
		DefaultNS:  "",
		SecretPath: annTLSAuth.Value,
		SecretType: haproxy.CA_CERT,
	})
	// Annotation or secret DELETED
	if annTLSAuth.Status == store.DELETED || (secretUpdated && caFile == "") {
		logger.Infof("removing client TLS authentication")
		for i := range binds {
			binds[i].SslCafile = ""
			binds[i].Verify = ""
			if err = api.FrontendBindEdit(frontendName, *binds[i]); err != nil {
				return err
			}
		}
		return nil
	}
	// Handle secret errors
	if secretErr != nil {
		if errors.Is(secretErr, haproxy.ErrCertNotFound) {
			logger.Warning("unable to configure TLS authentication secret '%s' not found", annTLSAuth.Value)
			return nil
		}
		return secretErr
	}
	// No changes
	if annTLSAuth.Status == store.EMPTY && !secretUpdated {
		return nil
	}
	verify := "required"
	enabled, annErr := utils.GetBoolValue("client-crt-optional", annTLSVerify.Value)
	logger.Error(annErr)
	if enabled {
		verify = "optional"
	}
	// Configure TLS Authentication
	logger.Infof("enabling client TLS authentication")
	for i := range binds {
		binds[i].SslCafile = caFile
		binds[i].Verify = verify
		if err = api.FrontendBindEdit(frontendName, *binds[i]); err != nil {
			return err
		}
	}
	return nil
}
