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

package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models"
)

type Backend models.Backend

func (b *Backend) updateAbortOnClose(data *StringW) error {
	if data.Value == "enabled" {
		b.Abortonclose = "enabled"
	} else {
		b.Abortonclose = ""
	}
	return nil
}

func (b *Backend) updateBalance(data *StringW) error {
	//TODO Balance proper usage
	val := &models.Balance{
		Algorithm: &data.Value,
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("balance algorithm: %s", err)
	}
	b.Balance = val
	return nil
}

func (b *Backend) updateCheckTimeout(data *StringW) error {
	val, err := utils.ParseTime(data.Value)
	if err != nil {
		return fmt.Errorf("timeout check: %s", err)
	}
	b.CheckTimeout = val
	return nil
}

func (b *Backend) updateCookie(data *StringW, cookieAnnotations map[string]*StringW) error {
	val := &models.Cookie{
		Name: &data.Value,
	}
	for k, v := range cookieAnnotations {
		if v == nil {
			continue
		}
		switch k {
		case "cookie-domain":
			val.Domain = strings.Fields(v.Value)
		case "cookie-dynamic":
			dynamic, dynamicErr := GetBoolValue(v.Value, "cookie-dynamic")
			if dynamicErr != nil {
				return dynamicErr
			}
			val.Dynamic = dynamic
		case "cookie-httponly":
			httponly, httponlyErr := GetBoolValue(v.Value, "cookie-httponly")
			if httponlyErr != nil {
				return httponlyErr
			}
			val.Httponly = httponly
		case "cookie-indirect":
			indirect, indirectErr := GetBoolValue(v.Value, "cookie-indirect")
			if indirectErr != nil {
				return indirectErr
			}
			val.Indirect = indirect
		case "cookie-maxidle":
			maxidle, maxidleErr := strconv.ParseInt(v.Value, 10, 64)
			if maxidleErr != nil {
				return maxidleErr
			}
			val.Maxidle = maxidle
		case "cookie-maxlife":
			maxlife, maxlifeErr := strconv.ParseInt(v.Value, 10, 64)
			if maxlifeErr != nil {
				return maxlifeErr
			}
			val.Maxlife = maxlife
		case "cookie-nocache":
			nocache, nocacheErr := GetBoolValue(v.Value, "cookie-nocache")
			if nocacheErr != nil {
				return nocacheErr
			}
			val.Nocache = nocache
		case "cookie-postonly":
			postonly, postonlyErr := GetBoolValue(v.Value, "cookie-postonly")
			if postonlyErr != nil {
				return postonlyErr
			}
			val.Postonly = postonly
		case "cookie-preserve":
			preserve, preserveErr := GetBoolValue(v.Value, "cookie-preserve")
			if preserveErr != nil {
				return preserveErr
			}
			val.Preserve = preserve
		case "cookie-secure":
			secure, secureErr := GetBoolValue(v.Value, "cookie-secure")
			if secureErr != nil {
				return secureErr
			}
			val.Secure = secure
		case "cookie-type":
			val.Type = v.Value
		}
	}
	b.Cookie = val
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("cookie: %s", err)
	}
	return nil
}

func (b *Backend) updateForwardfor(data *StringW) error {
	enabled, err := GetBoolValue(data.Value, "forwarded-for")
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

func (b *Backend) updateHttpchk(data *StringW) error {
	var val *models.Httpchk
	httpCheckParams := strings.Fields(strings.TrimSpace(data.Value))
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
