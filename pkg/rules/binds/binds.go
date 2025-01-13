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

package binds

import (
	"github.com/haproxytech/client-native/v6/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

// Binds order is not important
func ReconcileBinds(haproxy haproxy.HAProxy, frontendName string, newBinds models.Binds) error {
	var errors utils.Errors
	oldBinds, err := haproxy.FrontendBindsGet(frontendName)
	if err != nil {
		return err
	}

	for _, newBind := range newBinds {
		oldBind := findBind(newBind.Name, oldBinds)
		if err = reconcileBind(haproxy, frontendName, oldBind, newBind); err != nil {
			errors.Add(err)
		}
	}

	if errClear := clearBinds(haproxy, frontendName, oldBinds, newBinds); errClear != nil {
		errors.Add(errClear)
	}

	return errors.Result()
}

func reconcileBind(haproxy haproxy.HAProxy, frontendName string, oldBind, newBind *models.Bind) error {
	// Create
	if oldBind == nil && newBind != nil {
		err := haproxy.FrontendBindCreate(frontendName, *newBind)
		if err == nil {
			instance.Reload("Frontend '%s' bind '%s' created", frontendName, newBind.Name)
		}
		return err
	}

	// Update
	diffs := newBind.Diff(*oldBind)
	if len(diffs) != 0 && newBind != nil {
		err := haproxy.FrontendBindEdit(frontendName, *newBind)
		if err == nil {
			instance.Reload("Frontend '%s' bind '%s' updated %v", frontendName, newBind.Name, utils.JSONDiff(diffs))
		}
		return err
	}
	return nil
}

func findBind(name string, binds models.Binds) *models.Bind {
	for _, bind := range binds {
		if bind.Name == name {
			return bind
		}
	}
	return nil
}

func clearBinds(haproxy haproxy.HAProxy, frontendName string, oldBinds, newBinds models.Binds) error {
	var errors utils.Errors

	for _, oldBind := range oldBinds {
		found := findBind(oldBind.Name, newBinds)

		// Delete bind if not found
		if found == nil {
			err := haproxy.FrontendBindDelete(frontendName, oldBind.Name)
			if err != nil {
				errors.Add(err)
				utils.GetLogger().Errorf("Error deleting frontend %s bind '%s': %s", frontendName, oldBind.Name, err)
			}
			instance.ReloadIf(err == nil, "Frontend %s bind '%s' deleted", frontendName, oldBind.Name)
		}
	}
	return errors.Result()
}
