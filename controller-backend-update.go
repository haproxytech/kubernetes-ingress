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

package main

import (
	"fmt"
	"github.com/haproxytech/models"
)

type backend models.Backend

func (b *backend) updateBalance(data *StringW) error {
	//TODO Balance proper usage
	val := &models.Balance{
		Algorithm: data.Value,
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("balance algorithm: %s", err)
	}
	b.Balance = val
	return nil
}

func (b *backend) updateCheckTimeout(data *StringW) error {
	val, err := annotationConvertTimeToMS(*data)
	if err != nil {
		return fmt.Errorf("timeout check: %s", err)
	}
	b.CheckTimeout = &val
	return nil
}

func (b *backend) updateForwardfor(data *StringW) error {
	val := &models.Forwardfor{
		Enabled: &data.Value,
	}
	if err := val.Validate(nil); err != nil {
		return fmt.Errorf("forwarded-for option: %s", err)
	}
	b.Forwardfor = val
	return nil
}
