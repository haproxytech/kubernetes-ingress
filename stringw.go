package main

import (
	"fmt"
	"strings"
)

//StringW string value that has modified flag
type StringW struct {
	Value    string
	OldValue string
	Status   Status
}

//Equal compares only Value, rest is not relevant
func (a *StringW) Equal(b *StringW) bool {
	if a.Value != b.Value {
		return false
	}
	return true
}

//MapStringW stores values and enables
type MapStringW map[string]*StringW

//Get checks if name exists and if not, returns default value if defined
func (a *MapStringW) Get(name string) (data *StringW, err error) {
	var ok bool
	if data, ok = (*a)[name]; !ok {
		return nil, fmt.Errorf("StringW %s does not exist", name)
	}
	return data, nil
}

//Get checks if name exists and if not, returns default value if defined
func (a *MapStringW) String() string {
	var s strings.Builder
	first := true
	for key := range *a {
		if first {
			first = false
			s.WriteString("[")
		} else {
			s.WriteString(", ")
		}
		s.WriteString(key)
	}
	s.WriteString("]")
	return s.String()
}

//SetStatus sets Status state for all items in map
func (a *MapStringW) SetStatus(old MapStringW) (different bool) {
	different = false
	for name, currentValue := range *a {
		if oldValue, err := old.Get(name); err != nil {
			currentValue.Status = ADDED
			different = true
		} else {
			if currentValue.Value != oldValue.Value {
				currentValue.Status = MODIFIED
				currentValue.OldValue = oldValue.Value
				different = true
			} else {
				currentValue.Status = ""
			}
		}
	}
	for name, oldValue := range old {
		if _, err := a.Get(name); err != nil {
			oldValue.Status = DELETED
			oldValue.OldValue = oldValue.Value
			(*a)[name] = oldValue
			different = true
		}
	}
	return different
}

//SetStatusState sets all watches to desired state
func (a *MapStringW) SetStatusState(state Status) {
	for _, currentValue := range *a {
		currentValue.Status = state
		currentValue.OldValue = ""
	}
}

//Clean removes all with status
func (a *MapStringW) Clean() {
	for name, currentValue := range *a {
		if currentValue.Status == DELETED {
			delete(*a, name)
		}
	}
	a.SetStatusState("")
}

//Clone removes all with status
func (a *MapStringW) Clone() MapStringW {
	result := MapStringW{}
	for name, currentValue := range *a {
		result[name] = &StringW{
			Value:    currentValue.Value,
			OldValue: currentValue.OldValue,
			Status:   currentValue.Status,
		}
	}
	return result
}

//Equal comapres if two maps are equal
func (a *MapStringW) Equal(b MapStringW) bool {
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
