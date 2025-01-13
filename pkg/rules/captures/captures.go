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

package captures

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func Reconcile(client api.Capture, frontend string, rules models.Captures) error {
	var errors utils.Errors
	currentRules, err := client.CapturesGet(frontend)
	if err != nil {
		return err
	}

	// Diff includes diff in order
	diffRules := rules.Diff(currentRules)
	if len(diffRules) != 0 {
		err = client.CapturesReplace(frontend, rules)
		if err != nil {
			utils.GetLogger().Err(err)
			return err
		}
		// ... we reload because we created some http requests.
		instance.Reload("frontend '%s', capture rules updated: %+v", frontend, utils.JSONDiff(diffRules))
	}

	return errors.Result()
}
