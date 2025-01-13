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

func DeepConvertTCPSpecV1toV3(i v1.TCPSpec) v3.TCPSpec {
	v3TCP := v3.TCPSpec{}
	logger := utils.GetLogger()

	for _, v2TCP := range i {
		v3TCPModel := v3.TCPModel{}

		// Name
		v3TCPModel.Name = v2TCP.Name

		// Fronted
		v3FEt, err := convert.V2Tov3[cnv2.Frontend, cnv3.FrontendBase](&v2TCP.Frontend.Frontend)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.FrontendBase = *v3FEt

		// Acls
		aclv3t, err := convert.ListV2ToV3[cnv2.ACL, cnv3.ACL](v2TCP.Frontend.Acls)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.ACLList = aclv3t
		// Binds
		bindsv3t, err := convert.NamedResourceArrayV2ToMapV3[cnv2.Bind, cnv3.Bind](v2TCP.Frontend.Binds)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.Binds = bindsv3t

		//	BackendSwitchingRules
		bsrv3t, err := convert.ListV2ToV3[cnv2.BackendSwitchingRule, cnv3.BackendSwitchingRule](v2TCP.Frontend.BackendSwitchingRules)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.BackendSwitchingRuleList = bsrv3t
		// Captures
		capturesv3t, err := convert.ListV2ToV3[cnv2.Capture, cnv3.Capture](v2TCP.Frontend.Captures)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.CaptureList = capturesv3t
		// Filters
		filtersv3t, err := convert.ListV2ToV3[cnv2.Filter, cnv3.Filter](v2TCP.Frontend.Filters)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.FilterList = filtersv3t
		// LogTargets
		logtargetv3t, err := convert.ListV2ToV3[cnv2.LogTarget, cnv3.LogTarget](v2TCP.Frontend.LogTargets)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.LogTargetList = logtargetv3t
		// TCPRequestRules
		tcprrv3t, err := convert.ListV2ToV3[cnv2.TCPRequestRule, cnv3.TCPRequestRule](v2TCP.Frontend.TCPRequestRules)
		if err != nil {
			logger.Error(err)
			return v3.TCPSpec{}
		}
		v3TCPModel.Frontend.TCPRequestRuleList = tcprrv3t

		// Service
		v3TCPModel.Service = v3.TCPService{
			Name: v2TCP.Service.Name,
			Port: v2TCP.Service.Port,
		}

		// Services
		for _, v2Service := range v2TCP.Services {
			v3TCPModel.Services = append(v3TCPModel.Services, &v3.TCPService{Name: v2Service.Name, Port: v2Service.Port})
		}

		v3TCP = append(v3TCP, v3TCPModel)
	}

	return v3TCP
}
