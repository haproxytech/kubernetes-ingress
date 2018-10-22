package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/watch"
)

//StringW string value that has modified flag
type StringW struct {
	Value    string
	OldValue string
	Status   watch.EventType
}

//MapStringW stores values and enables
type MapStringW map[string]*StringW

//Get checks if name exists and if not, returns default value if defined
func (a MapStringW) Get(name string) (data *StringW, err error) {
	var ok bool
	if data, ok = a[name]; !ok {
		return nil, fmt.Errorf("StringW %s does not exist", name)
	}
	return data, nil
}

//SetStatus sets Status state for all items in map
func (a *MapStringW) SetStatus(old MapStringW) (different bool) {
	different = false
	for name, currentValue := range *a {
		if oldValue, err := old.Get(name); err != nil {
			currentValue.Status = watch.Added
			if name == "load-balance" {
			}
		} else {
			if name == "load-balance" {
			}
			if currentValue.Value != oldValue.Value {
				currentValue.Status = watch.Modified
				currentValue.OldValue = oldValue.OldValue
				different = true
			} else {
				currentValue.Status = ""
			}
		}
	}
	for name, oldValue := range old {
		if _, err := a.Get(name); err != nil {
			oldValue.Status = watch.Deleted
			oldValue.OldValue = oldValue.Value
			(*a)[name] = oldValue
			different = true
		}
	}
	return different
}

//SetStatusState sets all watches to desired state
func (a *MapStringW) SetStatusState(state watch.EventType) {
	for _, currentValue := range *a {
		currentValue.Status = state
		currentValue.OldValue = ""
	}
}
