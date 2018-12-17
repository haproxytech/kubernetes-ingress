package main

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/watch"
)

//IntW string value that has modified flag
type IntW struct {
	Value    string
	OldValue string
	Status   watch.EventType
}

//Equal compares only Value, rest is not relevant
func (a *IntW) Equal(b *IntW) bool {
	if a.Value != b.Value {
		return false
	}
	return true
}

//MapIntW stores values and enables
type MapIntW map[int]*IntW

//Get checks if name exists and if not, returns default value if defined
func (a *MapIntW) Get(name int) (data *IntW, err error) {
	var ok bool
	if data, ok = (*a)[name]; !ok {
		return nil, fmt.Errorf("IntW %d does not exist", name)
	}
	return data, nil
}

//Get checks if name exists and if not, returns default value if defined
func (a *MapIntW) String() string {
	var s strings.Builder
	first := true
	for key := range *a {
		if first {
			first = false
			s.WriteString("[")
		} else {
			s.WriteString(", ")
		}
		s.WriteString(strconv.Itoa(key))
	}
	s.WriteString("]")
	return s.String()
}

//SetStatus sets Status state for all items in map
func (a *MapIntW) SetStatus(old MapIntW) (different bool) {
	different = false
	for name, currentValue := range *a {
		if oldValue, err := old.Get(name); err != nil {
			currentValue.Status = watch.Added
			different = true
		} else {
			if currentValue.Value != oldValue.Value {
				currentValue.Status = watch.Modified
				currentValue.OldValue = oldValue.Value
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
func (a *MapIntW) SetStatusState(state watch.EventType) {
	for _, currentValue := range *a {
		currentValue.Status = state
		currentValue.OldValue = ""
	}
}

//Clean removes all with status
func (a *MapIntW) Clean() {
	for name, currentValue := range *a {
		if currentValue.Status == watch.Deleted {
			delete(*a, name)
		}
	}
	a.SetStatusState("")
}

//Clone removes all with status
func (a *MapIntW) Clone() MapIntW {
	result := MapIntW{}
	for name, currentValue := range *a {
		result[name] = &IntW{
			Value:    currentValue.Value,
			OldValue: currentValue.OldValue,
			Status:   currentValue.Status,
		}
	}
	return result
}

//Equal comapres if two maps are equal
func (a *MapIntW) Equal(b MapIntW) bool {
	if len(*a) != len(b) {
		return false
	}
	for k, v := range *a {
		value, ok := (b)[k]
		if !ok || !v.Equal(value) {
			return false
		}
	}
	return true
}
