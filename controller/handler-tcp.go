package controller

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type TCPHandler struct {
	setDefaultService func(ingress *store.Ingress, frontends []string) (reload bool, err error)
}

type tcpSvcParser struct {
	service    *store.Service
	port       int64
	sslOffload bool
}

func (t TCPHandler) Update(k store.K8s, cfg *Configuration, api api.HAProxyClient) (reload bool, err error) {
	//if k.ConfigMaps.TCPServices == nil {
	//	return false, nil
	//}
	var p tcpSvcParser
	//for port, tcpSvcAnn := range k.ConfigMaps.TCPServices.Annotations {
	for _, namespace := range k.Namespaces {
		if !namespace.Relevant {
			continue
		}
		for _, svc := range namespace.Services {
			svcName := svc.Namespace + "/" + svc.Name
			var exposeNew, exposeOld bool
			if exposeAnn, err := svc.Annotations.Get("expose"); err != nil{
				continue
			} else if exposeNew, err = strconv.ParseBool(exposeAnn.Value); err != nil {
				logger.Errorf("Service Name '%s' has annotation expose: %s which cannot parse to true/false", svcName, exposeAnn)
				continue
			} else if exposeOld, err = strconv.ParseBool(exposeAnn.OldValue); err != nil {
				exposeOld = false
			}
			var exposeStatus store.Status
			if !exposeNew && !exposeOld {
				continue
			} else if exposeNew && !exposeOld {
				exposeStatus = ADDED
			} else if exposeNew && exposeOld {
				exposeStatus = MODIFIED
			} else {
				exposeStatus = DELETED
			}
			var sslOffloading bool = false
			if sslOffloadingAnn, err := svc.Annotations.Get("ssl-offloading"); err == nil {
				if sslOffloading, err = strconv.ParseBool(sslOffloadingAnn.Value); err != nil {
					logger.Errorf("Service Name '%s' has annotation load-balancer-use-ssl: %s which cannot parse to true/false", svcName, sslOffloadingAnn)
					continue
				}
			}
			for _, svcPortSpec := range svc.Ports {
				var port = strconv.FormatInt(svcPortSpec.NodePort, 10)
				var tcpSvcAnn = store.StringW{Value: svcName + ":" + strconv.FormatInt(svcPortSpec.Port, 10), Status: exposeStatus}
				if sslOffloading {
					tcpSvcAnn.Value += ":ssl"
				}

				p, err = t.parseTCPService(k, tcpSvcAnn.Value)
				if err != nil {
					logger.Error(err)
					continue
				}
				// Delete Frontend
				frontendName := fmt.Sprintf("tcp-%s", port)
				if tcpSvcAnn.Status == DELETED || p.service.Status == DELETED {
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
				handleWhitelisting(k, cfg, frontendName, svc.Annotations)
				// Create Frontend
				if errGet != nil {
					frontend, reload, err = t.createTCPFrontend(api, frontendName, port, p.sslOffload)
					if err != nil {
						logger.Error(err)
						continue
					}
				}
				// Update  Frontend
				reload, err = t.updateTCPFrontend(api, frontend, p)
				if err != nil {
					logger.Errorf("TCP frontend '%s': update failed: %s", frontendName, err)
				}
			}
		}
	}
	return reload, nil
}

func (t TCPHandler) parseTCPService(store store.K8s, input string) (p tcpSvcParser, err error) {
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

func (t TCPHandler) createTCPFrontend(api api.HAProxyClient, frontendName, bindPort string, sslOffload bool) (frontend models.Frontend, reload bool, err error) {
	// Create Frontend
	frontend = models.Frontend{
		Name:   frontendName,
		Mode:   "tcp",
		Tcplog: true,
		//	DefaultBackend: route.BackendName,
	}
	var errors utils.Errors
	errors.Add(api.FrontendCreate(frontend))
	errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
		Address: "0.0.0.0:" + bindPort,
		Name:    "v4",
	}))
	errors.Add(api.FrontendBindCreate(frontendName, models.Bind{
		Address: ":::" + bindPort,
		Name:    "v6",
		V4v6:    true,
	}))
	if sslOffload {
		errors.Add(api.FrontendEnableSSLOffload(frontend.Name, FrontendCertDir, false))
	}
	if errors.Result() != nil {
		err = fmt.Errorf("error configuring tcp frontend: %w", err)
		return frontend, false, err
	}
	logger.Debugf("TCP frontend '%s' created, reload required", frontendName)
	return frontend, true, nil
}

// Copied and modified from frontend-annotations.go
// HAProxyController.handleWhitelisting should be refactored to have the same interface as this
// since the ingress parameter that it takes is only used for logging
// while the actual frontends that it modifies are hardcoded
func handleWhitelisting(k store.K8s, cfg *Configuration, frontend string, annotations store.MapStringW) {
	//  Get annotation status
	annWhitelist, _ := k.GetValueFromAnnotations("whitelist", annotations)
	if annWhitelist == nil {
		return
	}
	if annWhitelist.Status == DELETED {
		logger.Tracef("Frontend %s: Deleting whitelist configuration", frontend)
		// TODO: Shouldn't we delete the mapfile? I guess that complicates things
		return
	}
	// Validate annotation
	mapName := "whitelist-" + utils.Hash([]byte(annWhitelist.Value))
	if !cfg.MapFiles.Exists(mapName) {
		for _, address := range strings.Split(annWhitelist.Value, ",") {
			address = strings.TrimSpace(address)
			if ip := net.ParseIP(address); ip == nil {
				if _, _, err := net.ParseCIDR(address); err != nil {
					logger.Errorf("incorrect address '%s' in whitelist annotation in frontend '%s'", address, frontend)
					continue
				}
			}
			cfg.MapFiles.AppendRow(mapName, address)
		}
	}
	// Configure annotation
	logger.Tracef("Frontend %s: Configuring whitelist annotation", frontend)
	reqWhitelist := rules.ReqDeny{
		SrcIPsMap: mapName,
		Whitelist: true,
	}
	logger.Error(cfg.HAProxyRules.AddRule(reqWhitelist, "", frontend))
}

func (t TCPHandler) updateTCPFrontend(api api.HAProxyClient, frontend models.Frontend, p tcpSvcParser) (reload bool, err error) {
	binds, err := api.FrontendBindsGet(frontend.Name)
	if err != nil {
		err = fmt.Errorf("failed to get bind lines: %w", err)
		return
	}
	if !binds[0].Ssl && p.sslOffload {
		err = api.FrontendEnableSSLOffload(frontend.Name, FrontendCertDir, false)
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
	r, err := t.setDefaultService(ingress, []string{frontend.Name})
	if err != nil {
		return
	}

	return reload || r, err
}
