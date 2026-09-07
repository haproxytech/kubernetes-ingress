package annotations

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// Lists merge modes of the cr-frontend-ssl annotation.
const (
	// ListsMergeAppend appends the custom resource entries after the live ones. Default.
	ListsMergeAppend = "append"
	// ListsMergePrepend puts the custom resource entries before the live ones.
	ListsMergePrepend = "prepend"
	// ListsMergeOverride replaces a live list with a non-empty custom resource one.
	ListsMergeOverride = "override"
)

// ModelBackend takes an annotation holding the path of a backend cr and returns corresponding Backend model
func ModelBackend(name, defaultNS string, k store.K8s, annotations ...map[string]string) (backend *v3.BackendSpec, err error) {
	b, modelErr := model(name, defaultNS, 3, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return backend, err
	}
	if b != nil {
		//revive:disable-next-line:unchecked-type-assertion
		backend = b.(*v3.BackendSpec)
	}
	return backend, err
}

// ModelDefaults takes an annotation holding the path of a defaults cr and returns corresponding Defaults model
func ModelDefaults(name, defaultNS string, k store.K8s, annotations ...map[string]string) (defaults *models.Defaults, err error) {
	d, modelErr := model(name, defaultNS, 2, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return defaults, err
	}
	if d != nil {
		//revive:disable-next-line:unchecked-type-assertion
		defaults = d.(*models.Defaults)
	}
	return defaults, err
}

// ModelGlobal takes an annotation holding the path of a global cr and returns corresponding Global model
func ModelGlobal(name, defaultNS string, k store.K8s, annotations ...map[string]string) (global *models.Global, err error) {
	g, modelErr := model(name, defaultNS, 0, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return global, err
	}
	if g != nil {
		//revive:disable-next-line:unchecked-type-assertion
		global = g.(*models.Global)
	}
	return global, err
}

// ModelLog takes an annotation holding the path of a global cr and returns corresponding LogTargerts model
func ModelLog(name, defaultNS string, k store.K8s, annotations ...map[string]string) (log models.LogTargets, err error) {
	l, modelErr := model(name, defaultNS, 1, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return log, err
	}
	if l != nil {
		//revive:disable-next-line:unchecked-type-assertion
		log = l.(models.LogTargets)
	}
	return log, err
}

// ModelFrontend takes an annotation holding the path of a frontend cr and returns corresponding Frontend model
func ModelFrontend(name, defaultNS string, k store.K8s, annotations ...map[string]string) (frontend *models.Frontend, err error) {
	modelFound, err := model(name, defaultNS, 4, k, annotations...)
	if err != nil {
		return nil, err
	}
	if modelFound != nil {
		//revive:disable-next-line:unchecked-type-assertion
		frontend = &modelFound.(*v3.FrontendSpec).Frontend
	}
	return frontend, err
}

// ModelFrontendSSL parses a "namespace/name[:mode]" annotation value into a
// Frontend model and its lists merge mode ("append" default, "prepend", "override").
func ModelFrontendSSL(name, defaultNS string, k store.K8s, annotations ...map[string]string) (frontend *models.Frontend, mode string, err error) {
	value := common.GetValue(name, annotations...)
	if value == "" {
		return nil, "", nil
	}
	path, modeSuffix, hasMode := strings.Cut(value, ":")
	mode = ListsMergeAppend
	if hasMode {
		switch modeSuffix {
		case ListsMergeAppend, ListsMergePrepend, ListsMergeOverride:
			mode = modeSuffix
		default:
			return nil, "", fmt.Errorf("annotation '%s': invalid lists merge mode '%s', must be one of '%s', '%s' or '%s'",
				name, modeSuffix, ListsMergeAppend, ListsMergePrepend, ListsMergeOverride)
		}
	}
	crNS, crName, pathErr := common.GetNamespaceAndName(path)
	if pathErr != nil {
		return nil, "", fmt.Errorf("annotation '%s': %w", name, pathErr)
	}
	if crNS == "" {
		crNS = defaultNS
	}
	ns, nsOk := k.Namespaces[crNS]
	if !nsOk {
		return nil, "", fmt.Errorf("annotation '%s': custom resource '%s/%s' does not exist, namespace not found", name, crNS, crName)
	}
	cr, crOk := ns.CRs.Frontends[crName]
	if !crOk {
		return nil, "", fmt.Errorf("annotation '%s': custom resource '%s/%s' does not exist", name, crNS, crName)
	}
	return &cr.Frontend, mode, nil
}

func model(name, defaultNS string, crType int, k store.K8s, annotations ...map[string]string) (model interface{}, err error) {
	var crNS, crName string
	crNS, crName, err = common.GetK8sPath(name, annotations...)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return model, err
	}
	if crName == "" {
		return model, err
	}
	if crNS == "" {
		crNS = defaultNS
	}
	ns, nsOk := k.Namespaces[crNS]
	if !nsOk {
		return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist, namespace not found", name, crNS, crName)
	}
	switch crType {
	case 0:
		global, globalOk := ns.CRs.Global[crName]
		if !globalOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist", name, crNS, crName)
		}
		return global, nil
	case 1:
		global, globalOk := ns.CRs.Global[crName]
		if !globalOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist", name, crNS, crName)
		}
		return global.LogTargetList, nil
	case 2:
		defaults, defaultsOk := ns.CRs.Defaults[crName]
		if !defaultsOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist", name, crNS, crName)
		}
		return defaults, nil
	case 3:
		backend, backendOk := ns.CRs.Backends[crName]
		if !backendOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist", name, crNS, crName)
		}
		return backend, nil
	case 4:
		frontend, frontendFound := ns.CRs.Frontends[crName]
		if !frontendFound {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' does not exist", name, crNS, crName)
		}
		return frontend, nil
	}

	return nil, nil //nolint:nilnil
}
