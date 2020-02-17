package controller

import "fmt"

type Mode string

const (
	ModeHTTP Mode = "http"
	ModeTCP  Mode = "tcp"
)

//UnmarshalFlag Unmarshal flag
func (n *Mode) UnmarshalFlag(value string) error {
	switch value {
	case string(ModeHTTP), string(ModeTCP):
		*n = Mode(value)
	default:
		return fmt.Errorf("mode can be only '%s' or '%s'", ModeHTTP, ModeTCP)
	}
	return nil
}

//MarshalFlag Marshals flag
func (n Mode) MarshalFlag() (string, error) {
	return string(n), nil
}
