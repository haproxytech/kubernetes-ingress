package controller

import "fmt"

type Mode string

const (
	HTTP Mode = "http"
	TCP  Mode = "tcp"
)

// UnmarshalFlag Unmarshal flag
func (n *Mode) UnmarshalFlag(value string) error {
	switch value {
	case string(HTTP), string(TCP):
		*n = Mode(value)
	default:
		return fmt.Errorf("mode can be only '%s' or '%s'", HTTP, TCP)
	}
	return nil
}

// MarshalFlag Marshals flag
func (n Mode) MarshalFlag() (string, error) {
	return string(n), nil
}
