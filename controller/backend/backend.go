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

package backend

import (
	"fmt"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

type Backend models.Backend

func (b *Backend) UpdateAbortOnClose(value string) error {
	if value == "enabled" {
		b.Abortonclose = "enabled"
	} else {
		b.Abortonclose = ""
	}
	return nil
}

func (b *Backend) UpdateBalance(value string) error {
	//TODO Balance proper usage
	val := &models.Balance{
		Algorithm: &value,
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("balance algorithm: %s", err)
	}
	b.Balance = val
	return nil
}

func (b *Backend) UpdateCheckTimeout(value string) error {
	val, err := utils.ParseTime(value)
	if err != nil {
		return fmt.Errorf("timeout check: %s", err)
	}
	b.CheckTimeout = val
	return nil
}

func (b *Backend) UpdateCookie(cookie *models.Cookie) error {
	b.Cookie = cookie
	if err := cookie.Validate(nil); err != nil {
		return fmt.Errorf("cookie: %s", err)
	}
	return nil
}

func (b *Backend) UpdateForwardfor(value string) error {
	enabled, err := utils.GetBoolValue(value, "forwarded-for")
	if err != nil {
		return err
	}
	if enabled {
		b.Forwardfor = &models.Forwardfor{
			Enabled: utils.PtrString("enabled"),
		}
	} else {
		b.Forwardfor = nil
	}
	return nil
}

func (b *Backend) UpdateHttpchk(value string) error {
	var val *models.Httpchk
	httpCheckParams := strings.Fields(strings.TrimSpace(value))
	switch len(httpCheckParams) {
	case 0:
		return fmt.Errorf("httpchk option: incorrect number of params")
	case 1:
		val = &models.Httpchk{
			URI: httpCheckParams[0],
		}
	case 2:
		val = &models.Httpchk{
			Method: httpCheckParams[0],
			URI:    httpCheckParams[1],
		}
	default:
		val = &models.Httpchk{
			Method:  httpCheckParams[0],
			URI:     httpCheckParams[1],
			Version: strings.Join(httpCheckParams[2:], " "),
		}
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("httpchk option: %s", err)
	}
	b.Httpchk = val
	return nil
}
