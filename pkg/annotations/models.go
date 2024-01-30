package annotations

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

// ModelBackend takes an annotation holding the path of a backend cr and returns corresponding Backend model
func ModelBackend(name, defaultNS string, k store.K8s, annotations ...map[string]string) (backend *models.Backend, err error) {
	b, modelErr := model(name, defaultNS, 3, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return
	}
	if b != nil {
		backend = b.(*models.Backend) //nolint:forcetypeassert
	}
	return
}

// ModelDefaults takes an annotation holding the path of a defaults cr and returns corresponding Defaults model
func ModelDefaults(name, defaultNS string, k store.K8s, annotations ...map[string]string) (defaults *models.Defaults, err error) {
	d, modelErr := model(name, defaultNS, 2, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return
	}
	if d != nil {
		defaults = d.(*models.Defaults)
	}
	return
}

// ModelGlobal takes an annotation holding the path of a global cr and returns corresponding Global model
func ModelGlobal(name, defaultNS string, k store.K8s, annotations ...map[string]string) (global *models.Global, err error) {
	g, modelErr := model(name, defaultNS, 0, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return
	}
	if g != nil {
		global = g.(*models.Global)
	}
	return
}

// ModelLog takes an annotation holding the path of a global cr and returns corresponding LogTargerts model
func ModelLog(name, defaultNS string, k store.K8s, annotations ...map[string]string) (log models.LogTargets, err error) {
	l, modelErr := model(name, defaultNS, 1, k, annotations...)
	if modelErr != nil {
		err = modelErr
		return
	}
	if l != nil {
		log = l.(models.LogTargets) //nolint:forcetypeassert
	}
	return
}

func model(name, defaultNS string, crType int, k store.K8s, annotations ...map[string]string) (model interface{}, err error) {
	var crNS, crName string
	crNS, crName, err = common.GetK8sPath(name, annotations...)
	if err != nil {
		err = fmt.Errorf("annotation '%s': %w", name, err)
		return
	}
	if crName == "" {
		return
	}
	if crNS == "" {
		crNS = defaultNS
	}
	ns, nsOk := k.Namespaces[crNS]
	if !nsOk {
		return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' doest not exist, namespace not found", name, crNS, crName)
	}
	switch crType {
	case 0:
		global, globalOk := ns.CRs.Global[crName]
		if !globalOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' doest not exist", name, crNS, crName)
		}
		return global, nil
	case 1:
		lg, lgOk := ns.CRs.LogTargets[crName]
		if !lgOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' doest not exist", name, crNS, crName)
		}
		return lg, nil
	case 2:
		defaults, defaultsOk := ns.CRs.Defaults[crName]
		if !defaultsOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' doest not exist", name, crNS, crName)
		}
		return defaults, nil
	case 3:
		backend, backendOk := ns.CRs.Backends[crName]
		if !backendOk {
			return nil, fmt.Errorf("annotation %s: custom resource '%s/%s' doest not exist", name, crNS, crName)
		}
		return backend, nil
	}
	return nil, nil //nolint:nilnil
}
