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
	"github.com/haproxytech/models"
)

type backend models.Backend

func (b *backend) updateBalance(data *StringW) error {
	//TODO Balance proper usage
	balanceAlg := &models.Balance{
		Algorithm: data.Value,
	}
	if err := balanceAlg.Validate(nil); err != nil {
		return err
	}
	b.Balance = balanceAlg
	return nil
}

func (b *backend) updateCheckTimeout(data *StringW) error {
	val, err := annotationConvertTimeToMS(*data)
	if err != nil {
		return err
	}
	b.CheckTimeout = &val
	return nil
}

func (b *backend) updateForwardFor(data *StringW) error {
	forwardFor := &models.Forwardfor{
		Enabled: &data.Value,
	}
	if err := forwardFor.Validate(nil); err != nil {
		return err
	}
	b.Forwardfor = forwardFor
	return nil
}
