// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logtargets

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func Reconcile(client api.LogTarget, parentType rules.ParentType, parentName string, desired models.LogTargets) error {
	current, err := client.LogTargetsGet(string(parentType), parentName)
	if err != nil {
		utils.GetLogger().Err(err)
		return err
	}

	diff := desired.Diff(current)
	if len(diff) == 0 {
		return nil
	}
	if err = client.LogTargetsReplace(string(parentType), parentName, desired); err != nil {
		utils.GetLogger().Err(err)
		return err
	}
	instance.Reload("parent '%s/%s', log target rules updated: %+v", string(parentType), parentName, utils.JSONDiff(diff))
	return nil
}
