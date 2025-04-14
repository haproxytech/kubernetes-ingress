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

package utils

import (
	"strings"
)

type Experimental struct {
	UseIngressMerge bool
}

const (
	flagSeparator = ","
	// Experimental flags
	flagUseIngressMerge = "use-ingress-merge"
)

// UnmarshalFlag Unmarshal flag
func (e *Experimental) UnmarshalFlag(value string) error {
	var errors Errors
	flags := strings.Split(value, flagSeparator)

	// Then parse
	for _, flag := range flags {
		if flag == flagUseIngressMerge {
			e.UseIngressMerge = true
			continue
		}
	}

	return errors.Result()
}

// MarshalFlag Marshals flag
func (e Experimental) MarshalFlag() (string, error) {
	flags := []string{}
	if e.UseIngressMerge {
		flags = append(flags, flagUseIngressMerge)
	}
	return strings.Join(flags, flagSeparator), nil
}
