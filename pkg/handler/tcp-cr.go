// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v5/models"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/certs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	rc "github.com/haproxytech/kubernetes-ingress/pkg/reference-counter"
	"github.com/haproxytech/kubernetes-ingress/pkg/secret"
	"github.com/haproxytech/kubernetes-ingress/pkg/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
)

const tcpServicePrefix = "tcpcr"

type TCPCustomResource struct{}

type tcpcontext struct {
	k         store.K8s
	namespace string
	h         haproxy.HAProxy
}

func (handler TCPCustomResource) Update(k store.K8s, h haproxy.HAProxy, a annotations.Annotations) (err error) {
	var errs utils.Errors

	for _, ns := range k.Namespaces {
		for _, tcpCR := range ns.CRs.TCPsPerCR {
			// Cleanup will done after Haproxy config transaction succeeds
			if tcpCR.Status == store.DELETED {
				continue
			}
			ctx := tcpcontext{
				k:         k,
				h:         h,
				namespace: ns.Name,
			}
			for _, tcp := range tcpCR.Items {
				if tcp.CollisionStatus == store.ERROR {
					logger.Errorf("tcp-cr: skipping tcp '%s/%s/%s' due to collision %s", ctx.namespace, tcp.ParentName, tcp.Name, tcp.Reason)
					continue
				}
				owner := tcp.Owner()
				errSvc := handler.checkService(ctx, tcp.TCPModel)
				if errSvc != nil {
					errs.Add(errSvc)
					continue
				}

				errH := handler.manageFrontend(ctx, owner, tcp.TCPModel, a)
				if errH != nil {
					errs.Add(errH)
					continue
				}
			}
		}
	}
	handler.clearFrontends(k, h)

	return errs.Result()
}

func (handler TCPCustomResource) checkService(ctx tcpcontext, tcp v1.TCPModel) (err error) {
	var ok bool
	if _, ok = ctx.k.Namespaces[ctx.namespace]; !ok {
		err = fmt.Errorf("tcp-cr: namespace of service '%s/%s' not found", ctx.namespace, tcp.Service.Name)
		return
	}
	_, ok = ctx.k.Namespaces[ctx.namespace].Services[tcp.Service.Name]
	if !ok {
		err = fmt.Errorf("tcp-cr: service '%s/%s' not found", ctx.namespace, tcp.Service.Name)
		return err
	}
	return nil
}

func (handler TCPCustomResource) manageFrontend(ctx tcpcontext, owner rc.Owner, tcp v1.TCPModel, a annotations.Annotations) error {
	cfgFrontendName := cfgFrontendName(ctx.namespace, tcp.Frontend.Frontend)
	// First get Frontend from Custom Resource
	frontend := tcp.Frontend.Frontend

	// Then apply overrides
	applyFrontendOverride(ctx.namespace, &frontend)

	if errAdd := handler.createOrEditFrontend(ctx.h, frontend); errAdd != nil {
		return errAdd
	}
	ctx.k.FrontendRC.AddOwner(rc.HaproxyCfgResourceName(cfgFrontendName), owner)

	// Add Binds
	if errBinds := handler.createOrEditOrDeleteBinds(ctx, frontend, tcp.Frontend.Binds, owner); errBinds != nil {
		return errBinds
	}

	// IngressPath
	path := &store.IngressPath{
		SvcNamespace:     ctx.namespace,
		SvcName:          tcp.Service.Name,
		SvcPortInt:       int64(tcp.Service.Port),
		IsDefaultBackend: true,
	}
	if svc, err := service.New(ctx.k, path, nil, true, nil, ctx.k.ConfigMaps.Main.Annotations); err == nil {
		errSvc := svc.SetDefaultBackend(ctx.k, ctx.h, []string{frontend.Name}, a)
		// Add reload if default backend changed
		if errSvc != nil {
			return fmt.Errorf("error configuring tcp frontend: %w", errSvc)
		}
	}

	return nil
}

func cfgFrontendName(namespace string, frontend models.Frontend) string {
	return tcpServicePrefix + "_" + namespace + "_" + frontend.Name
}

func applyFrontendOverride(namespace string, fe *models.Frontend) {
	fe.Mode = "tcp"
	// Add a "tcp-" prefix
	fe.Name = cfgFrontendName(namespace, *fe)
	// LogFormat
	format := strings.TrimSpace(fe.LogFormat)
	if format != "" {
		fe.LogFormat = "'" + strings.TrimSpace(fe.LogFormat) + "'"
	}
}

func (handler TCPCustomResource) applyBindOverride(ctx tcpcontext, bind *models.Bind, owner rc.Owner) {
	if bind.Ssl {
		// Does a secret with bind.SSlCertificate exists ?
		secretManager := secret.NewManager(ctx.k, ctx.h)
		certName := bind.SslCertificate
		// It might contain: a secret name (same namespace as the TCP CR) or a folder or a file.
		// If the a secret with the bind.SSlCertificate name is found in the same namespace as the TCP CR:
		// Place it into the folder and use it
		// if _, ok := ctx.k.Namespaces[ctx.namespace].Secret[bind.SslCertificate]; ok {
		if _, err := ctx.k.GetSecret(ctx.namespace, bind.SslCertificate); err == nil {
			sec := secret.Secret{
				Name:       types.NamespacedName{Namespace: ctx.namespace, Name: certName},
				SecretType: certs.TCP_CERT,
				OwnerType:  secret.OWNERTYPE_TCP_CR,
				OwnerName:  string(owner.Key()),
			}
			secretManager.Store(sec)
			bind.SslCertificate = ctx.h.Certs.TCPCRDir + fmt.Sprintf("/%s_%s", ctx.namespace, certName) + ".pem"
		}
		// If not a secret name
		// We take what ever is in bind.SslCertificate and use it (it it's a file, it has to be the complete path to the file)
		// Nothing special to do, bind.SSlCertificate is already containing the value needed (folder or file)
	}
}

func (handler TCPCustomResource) createOrEditFrontend(h haproxy.HAProxy, frontend models.Frontend) error {
	oldfe, err := h.FrontendGet(frontend.Name)
	// Create
	if err != nil {
		err = h.FrontendCreate(frontend)
		if err == nil {
			instance.Reload("TCP frontend '%s' created", frontend.Name)
		}
		return err
	}

	// Update
	diffs := frontend.Diff(oldfe)
	// exclude "DefaultBackend" from the diffs, DefaultBackend is set afterwards in the flow in frontend
	// A diff in DefaultBackend is normal at this stage
	delete(diffs, "DefaultBackend")
	if len(diffs) != 0 {
		err = h.FrontendEdit(frontend)
		if err == nil {
			instance.Reload("TCP frontend '%s' updated %v", frontend.Name, diffs)
		}
		return err
	}

	return nil
}

func (handler TCPCustomResource) createOrEditOrDeleteBinds(ctx tcpcontext, frontend models.Frontend, binds []*models.Bind, owner rc.Owner) (err error) {
	var errors utils.Errors
	oldBinds, err := ctx.h.FrontendBindsGet(frontend.Name)
	if err != nil {
		return err
	}

	for _, bind := range binds {
		// Copy= do not change bind in store with overrides
		abind := *bind
		handler.applyBindOverride(ctx, &abind, owner)
		oldBind := getBind(bind.Name, oldBinds)
		if err = handler.createOrEditBind(ctx, frontend.Name, &abind, oldBind); err != nil {
			errors.Add(err)
		}
	}

	handler.clearBinds(ctx.h, frontend.Name, binds, oldBinds)

	return errors.Result()
}

func (handler TCPCustomResource) createOrEditBind(ctx tcpcontext, frontendName string, bind, oldBind *models.Bind) error {
	// Create
	if oldBind == nil {
		err := ctx.h.FrontendBindCreate(frontendName, *bind)
		if err == nil {
			instance.Reload("TCP frontend '%s' bind '%s' created", frontendName, bind.Name)
		}
		return err
	}

	// Update
	diffs := bind.Diff(*oldBind)
	if len(diffs) != 0 {
		err := ctx.h.FrontendBindEdit(frontendName, *bind)
		if err == nil {
			instance.Reload("TCP frontend '%s' bind '%s' updated %v", frontendName, bind.Name, diffs)
		}
		return err
	}

	return nil
}

func getBind(name string, binds models.Binds) *models.Bind {
	for _, bind := range binds {
		if bind.Name == name {
			return bind
		}
	}
	return nil
}

func (handler TCPCustomResource) clearBinds(h haproxy.HAProxy, frontendName string, binds []*models.Bind, oldBinds models.Binds) {
	for _, bind := range oldBinds {
		found := false
		for _, newBind := range binds {
			if bind.Name == newBind.Name {
				found = true
				break
			}
		}
		// Delete bind if not found
		if !found {
			err := h.FrontendBindDelete(frontendName, bind.Name)
			if err != nil {
				logger.Errorf("tcp-ct: error deleting tcp frontend %s bind '%s': %s", frontendName, bind.Name, err)
			}
			instance.ReloadIf(err == nil, "TCP frontend %s bind '%s' deleted", frontendName, bind.Name)
		}
	}
}

func (handler TCPCustomResource) clearFrontends(k store.K8s, h haproxy.HAProxy) {
	frontends, err := h.FrontendsGet()
	if err != nil {
		logger.Error(err)
		return
	}
	for _, cfgFe := range frontends {
		isTCPServiceFe := isTCPFrontend(cfgFe)
		if !isTCPServiceFe {
			continue
		}
		isRequired := isTCPFrontendRequired(k, cfgFe.Name)

		if !isRequired {
			err = h.FrontendDelete(cfgFe.Name)
			if err != nil {
				logger.Errorf("tcp-ct: error deleting tcp frontend '%s': %s", cfgFe.Name, err)
			}
			instance.ReloadIf(err == nil, "TCP frontend '%s' deleted", cfgFe.Name)
		}
	}
}

func isTCPFrontend(frontend *models.Frontend) bool {
	return frontend.Mode == "tcp" && strings.HasPrefix(frontend.Name, tcpServicePrefix)
}

func isTCPFrontendRequired(k store.K8s, configFrontendName string) bool {
	return k.FrontendRC.HasOwners(rc.HaproxyCfgResourceName(configFrontendName))
}
