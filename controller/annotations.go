// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
)

//ConvertToMapStringW removes prefixes in annotation
func ConvertToMapStringW(annotations map[string]string) MapStringW {
	newAnnotations := make(MapStringW, len(annotations))
	for name, value := range annotations {
		newAnnotations[convertAnnotationName(name)] = &StringW{
			Value:  value,
			Status: ADDED,
		}
	}
	return newAnnotations
}

func convertAnnotationName(annotation string) string {
	split := strings.SplitN(annotation, "/", 2)
	return split[len(split)-1]
}

//GetValueFromAnnotations returns value by checking in multiple annotatins.
// moves through list until it finds value
// if value is new or deleted, we check for next state to correctly set watch & value
func GetValueFromAnnotations(annotationName string, annotations ...MapStringW) (data *StringW, err error) {
	deleted := false
	oldValue := ""
	for _, a := range annotations {
		if item, errAnn := a.Get(annotationName); errAnn == nil {
			if item.Status == ERROR {
				continue
			}
			if item.Status == DELETED {
				if data == nil && !deleted {
					oldValue = item.Value
					deleted = true
				}
				continue
			}
			if data == nil {
				if deleted {
					watchState := MODIFIED
					if item.Value == oldValue {
						watchState = ""
					}
					item.OldValue = oldValue
					item.Status = watchState
					return item, nil
				}
				if item.Status == MODIFIED {
					return item, err
				}
				if item.Status == EMPTY {
					return item, err
				}
				watchState := item.Status // Added
				data = &StringW{
					Value:    item.Value,
					OldValue: item.OldValue,
					Status:   watchState,
				}
			} else {
				// so we have some data from previous annotations
				if item.Status == MODIFIED || item.Status == EMPTY {
					if item.Value != data.Value {
						data.OldValue = item.Value
						data.Status = MODIFIED
					} else {
						data.Status = EMPTY
					}
					return data, nil
				}
				return data, nil
			}
		}
	}
	if data != nil {
		return data, nil
	}
	data, err = defaultAnnotationValues.Get(annotationName)
	if !deleted {
		return data, err
	}
	//we only have deleted annotation, so we have to see if default exists
	if err != nil {
		data = &StringW{
			Value:    oldValue,
			OldValue: oldValue,
			Status:   DELETED,
		}
		return data, nil
	}
	// default exists, just set flags correctly
	watchState := MODIFIED
	if data.Value == oldValue {
		watchState = ""
	}
	data = &StringW{
		Value:    data.Value,
		OldValue: oldValue,
		Status:   watchState,
	}
	return data, nil
}

func SetDefaultAnnotation(annotation, value string) {
	defaultAnnotationValues[annotation] = &StringW{
		Value:  value,
		Status: ADDED,
	}
}

var defaultAnnotationValues = MapStringW{
	"check":                   &StringW{Value: "true"},
	"cookie-indirect":         &StringW{Value: "true"},
	"cookie-nocache":          &StringW{Value: "true"},
	"cookie-type":             &StringW{Value: "insert"},
	"forwarded-for":           &StringW{Value: "true"},
	"load-balance":            &StringW{Value: "roundrobin"},
	"log-format":              &StringW{Value: "%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\""},
	"rate-limit-size":         &StringW{Value: "100k"},
	"rate-limit-period":       &StringW{Value: "1s"},
	"ssl-redirect-code":       &StringW{Value: "302"},
	"ssl-passthrough":         &StringW{Value: "false"},
	"server-ssl":              &StringW{Value: "false"},
	"servers-increment":       &StringW{Value: "42"},
	"syslog-server":           &StringW{Value: "address:127.0.0.1, facility: local0, level: notice"},
	"timeout-http-request":    &StringW{Value: "5s"},
	"timeout-connect":         &StringW{Value: "5s"},
	"timeout-client":          &StringW{Value: "50s"},
	"timeout-queue":           &StringW{Value: "5s"},
	"timeout-server":          &StringW{Value: "50s"},
	"timeout-tunnel":          &StringW{Value: "1h"},
	"timeout-http-keep-alive": &StringW{Value: "1m"},
}

// Handle Global and default Annotations
func (c *HAProxyController) handleGlobalAnnotations() (restart bool, reload bool) {
	var r annotations.Result
	for name, annotation := range annotations.Global {
		cfgMapAnn, _ := GetValueFromAnnotations(name, c.cfg.ConfigMaps[Main].Annotations)
		if cfgMapAnn == nil {
			continue
		}
		switch cfgMapAnn.Status {
		case EMPTY:
			continue
		case DELETED:
			r = annotation.Delete(c.Client)
		default:
			if err := annotation.Parse(cfgMapAnn.Value); err != nil {
				c.Logger.Error(err)
			} else {
				r = annotation.Update(c.Client)
			}
		}
		if r == 1 {
			reload = true
		} else if r == 2 {
			restart = true
		}
	}
	return restart, reload
}
