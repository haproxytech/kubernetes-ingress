package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major int
	Minor int
}

// Implements the Unmarshaler interface of the yaml pkg.
func (v *Version) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var data string
	err := unmarshal(&data)
	if err != nil {
		return err
	}

	parts := strings.Split(data, ".")
	if len(parts) != 2 {
		return errors.New("version is not in correct format")
	}
	v.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	v.Minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	return nil
}

func (v *Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func (v *Version) LowerOrEqual(active Version) bool {
	if active.Major < v.Major {
		return false
	}
	if active.Major != v.Major {
		return true
	}
	if active.Minor < v.Minor {
		return false
	}
	return true
}

func (v Version) MarshalYAML() (interface{}, error) {
	return v.String(), nil
}
