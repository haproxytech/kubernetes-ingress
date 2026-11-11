package common

import (
	"errors"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

var logger = utils.GetLogger()

// multiLineAnnotations are the annotations whose value is split into lines.
var multiLineAnnotations = map[string]struct{}{
	"path-rewrite":        {},
	"request-capture":     {},
	"request-set-header":  {},
	"response-set-header": {},
	"syslog-server":       {},
}

// GetValue returns value by checking in multiple annotations.
// A value holding CR or LF is discarded for annotations that take a single line:
// one line of tenant input must never become several HAProxy directives.
func GetValue(annotationName string, annotations ...map[string]string) string {
	for _, a := range annotations {
		val, ok := a[annotationName]
		if !ok {
			continue
		}
		if _, multiLine := multiLineAnnotations[annotationName]; !multiLine && strings.ContainsAny(val, "\r\n") {
			logger.Warningf("annotation %q: multiline value discarded, this annotation takes a single line", annotationName)
			return DefaultValues[annotationName]
		}
		return val
	}
	return DefaultValues[annotationName]
}

// IsSet reports whether the annotation is explicitly present, ignoring DefaultValues.
func IsSet(name string, annotations ...map[string]string) bool {
	for _, a := range annotations {
		if _, ok := a[name]; ok {
			return true
		}
	}
	return false
}

// GetMatch returns all matches of the annotation prefix name in the provided annotations.
func GetMatch(annotationPrefix string, annotations ...map[string]string) map[string]string {
	result := map[string]string{}
	for _, a := range annotations {
		for name, value := range a {
			if strings.HasPrefix(name, annotationPrefix) {
				result[name] = value
			}
		}
	}
	return result
}

func GetK8sPath(annotationName string, annotations ...map[string]string) (ns, name string, err error) {
	a := GetValue(annotationName, annotations...)
	if a == "" {
		return ns, name, err
	}
	return GetNamespaceAndName(a)
}

func GetNamespaceAndName(value string) (ns, name string, err error) {
	parts := strings.Split(value, "/")
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		ns = parts[0]
		name = parts[1]
		if ns == "" {
			err = errors.New("invalid format")
		}
	}
	if name == "" {
		err = errors.New("invalid format")
	}
	return ns, name, err
}

var DefaultValues = map[string]string{
	"auth-realm":            "Protected Content",
	"check":                 "true",
	"cors-allow-origin":     "*",
	"cors-allow-methods":    "*",
	"cors-allow-headers":    "*",
	"cors-max-age":          "5s",
	"cookie-indirect":       "true",
	"cookie-nocache":        "true",
	"cookie-type":           "insert",
	"forwarded-for":         "true",
	"load-balance":          "roundrobin",
	"request-capture-len":   "128",
	"ssl-redirect-code":     "302",
	"request-redirect-code": "302",
	"ssl-redirect-port":     "8443",
	"ssl-passthrough":       "false",
	"server-ssl":            "false",
	"client-crt-optional":   "false",
	"tls-alpn":              "h2,http/1.1",
	"quic-alt-svc-max-age":  "60",
}

// GetValuesAndIndices retrieves values of a specific annotation from multiple annotations maps.
func GetValuesAndIndices(annotationName string, customAnnotationPrefix string, annotations ...map[string]string) (map[int]string, map[string]string) {
	result := map[int]string{}
	customResult := map[string]string{}
	for i, a := range annotations {
		if val, ok := a[annotationName]; ok {
			result[i] = val
		}
	}
	for _, a := range annotations {
		for k, v := range a {
			if strings.HasPrefix(k, customAnnotationPrefix) {
				// Remove the prefix from the key
				annotationName := strings.TrimPrefix(k, customAnnotationPrefix)
				if _, ok := customResult[annotationName]; !ok {
					// add it only if we do not have it yet,
					// this is due to priority, Service > Ingress > ConfigMap
					customResult[annotationName] = v
				}
			}
		}
	}

	return result, customResult
}

// EnsureQuoted ensures that a string starts and ends with a double quote.
// It adds a quote to the beginning if one is not already present,
// and adds a quote to the end if one is not already present.
func EnsureQuoted(s string) string {
	newS := s
	if s == "\"" || s == "" {
		newS = "\"\""
		return newS
	}
	if !strings.HasPrefix(newS, "\"") {
		newS = "\"" + newS
	}
	if !strings.HasSuffix(newS, "\"") {
		newS += "\""
	}
	return newS
}
