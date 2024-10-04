package utils

import (
	"errors"
)

type Errors []error

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
		result += err.Error() + "\n"
	}
	if result == "" {
		return nil
	}
	return errors.New(result)
}

func (e *Errors) AddErrors(errors Errors) {
	for _, err := range errors {
		e.Add((err))
	}
}
