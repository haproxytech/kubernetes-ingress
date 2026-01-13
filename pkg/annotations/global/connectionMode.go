package global

import (
	"fmt"

	"github.com/haproxytech/client-native/v5/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

type HTTPConnectionMode struct {
	defaults *models.Defaults
	name     string
}

func NewHTTPConnectionMode(n string, d *models.Defaults) *HTTPConnectionMode {
	return &HTTPConnectionMode{name: n, defaults: d}
}

func (a *HTTPConnectionMode) GetName() string {
	return a.name
}

// processAlternativeAnnotations process connection mode annotations
// Deprecated: this function can be removed when `http-server-close` and `http-keep-alive` become obsolete
func (a *HTTPConnectionMode) processAlternativeAnnotations(httpConnectionMode string, annotations ...map[string]string) bool {
	if httpConnectionMode != "" {
		return false
	}
	var mode string
	alternativeAnnotations := []string{"http-server-close", "http-keep-alive"}
	for _, annotation := range alternativeAnnotations {
		value := common.GetValue(annotation, annotations...)
		if value == "" {
			continue
		}
		enabled, err := utils.GetBoolValue(value, annotation)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		logger.Warningf("annotation [%s] is DEPRECATED, use [http-connection-mode: \"%s\"] instead", annotation, annotation)
		if enabled {
			mode = annotation
		}
	}
	if mode != "" {
		a.defaults.HTTPConnectionMode = mode
		return true
	}
	return false
}

func (a *HTTPConnectionMode) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)

	// this block can be removed when annotations `http-server-close` and `http-keep-alive` become obsolete
	if a.processAlternativeAnnotations(input, annotations...) {
		return nil
	}

	switch input {
	case
		"",
		"http-keep-alive",
		"http-server-close",
		"httpclose":
		//revive:disable-next-line:useless-break,unnecessary-stmt
		break
	default:
		return fmt.Errorf("invalid http-connection-mode value '%s'", input)
	}
	a.defaults.HTTPConnectionMode = input
	return nil
}
