package main

import (
	"strings"

	"k8s.io/apimachinery/pkg/watch"
)

//ConvertToMapStringW removes prefixes in annotation
func ConvertToMapStringW(anotations map[string]string) MapStringW {
	newAnnotations := make(MapStringW, len(anotations))
	for name, value := range anotations {
		newAnnotations[convertAnnotationName(name)] = &StringW{
			Value:  value,
			Status: watch.Added,
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
		if item, err := a.Get(annotationName); err == nil {
			if item.Status == watch.Error {
				continue
			}
			if item.Status == watch.Deleted {
				if data == nil && !deleted {
					oldValue = item.Value
					deleted = true
				}
				continue
			}
			if data == nil {
				if deleted {
					watchState := watch.Modified
					if item.Value == oldValue {
						watchState = ""
					}
					item.OldValue = oldValue
					item.Status = watchState
					return item, nil
				}
				if item.Status == watch.Modified {
					return item, err
				}
				if item.Status == "" {
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
				if item.Status == watch.Modified || item.Status == "" {
					if item.Value != data.Value {
						data.OldValue = item.Value
						data.Status = watch.Modified
					} else {
						data.Status = ""
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
			Status:   watch.Deleted,
		}
		return data, nil
	}
	// default exists, just set flags correctly
	watchState := watch.Modified
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

var defaultAnnotationValues = MapStringW{
	"ingress.class": &StringW{Value: ""},

	"check":         &StringW{Value: "enabled"},
	"forwarded-for": &StringW{Value: "enabled"},
	"load-balance":  &StringW{Value: "roundrobin"},
	"pod-maxconn":   &StringW{Value: "2000"},
	"maxconn":       &StringW{Value: "2000"},
	"ssl-redirect":  &StringW{Value: ""},
}
