package utils

import (
	"errors"
)

type Errors []error

//nolint
func (e *Errors) Add(errors ...error) {
	for _, err := range errors {
		if err != nil {
			*e = append(*e, err)
		}
	}
}

func (e *Errors) Result() error {
	var result string
	for _, err := range *e {
		result += "\n" + err.Error()
	}
	if result == "" {
		return nil
	}
	return errors.New(result)
}
