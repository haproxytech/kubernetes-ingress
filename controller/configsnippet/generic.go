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

package configsnippet

import (
	"fmt"
	"strings"
)

type genericAttribute struct {
	attribute string
}

func NewGenericAttribute(attribute string) Overridden {
	return &genericAttribute{attribute: attribute}
}

func (g genericAttribute) Overridden(configSnippet string) (err error) {
	if strings.Contains(configSnippet, g.attribute) {
		err = fmt.Errorf(fmt.Sprintf("The attribute %s has been already declared in the config-snippet annotation, will be ignored", g.attribute))
	}
	return
}
