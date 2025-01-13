// Copyright 2022 HAProxy Technologies LLC
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

package converters

import (
	cnv2 "github.com/haproxytech/client-native/v5/models"
	convert "github.com/haproxytech/client-native/v6/configuration/convert/v2v3"
	cnv3 "github.com/haproxytech/client-native/v6/models"
	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func DeepConvertGlobalSpecV1toV3(i v1.GlobalSpec) v3.GlobalSpec {
	v3Global := v3.GlobalSpec{}
	logger := utils.GetLogger()

	// Global
	globalv3t, err := convert.GlobalV2ToV3(i.Config)
	if err != nil {
		logger.Error(err)
		return v3.GlobalSpec{}
	}
	v3Global.GlobalBase = *globalv3t

	// // LogTargetList
	v3lt, err := convert.ListV2ToV3[cnv2.LogTarget, cnv3.LogTarget](i.LogTargets)
	if err != nil {
		return v3.GlobalSpec{}
	}
	v3Global.LogTargetList = v3lt

	return v3Global
}
