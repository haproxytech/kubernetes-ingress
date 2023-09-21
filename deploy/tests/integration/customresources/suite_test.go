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

package customresources

import (
	"testing"

	"github.com/haproxytech/client-native/v3/models"
	corev1alpha2 "github.com/haproxytech/kubernetes-ingress/crs/api/core/v1alpha2"
	"github.com/haproxytech/kubernetes-ingress/deploy/tests/integration"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CustomResourceSuite struct {
	integration.BaseSuite
	globalCREvt k8s.SyncDataEvent
}

func TestCustomResource(t *testing.T) {
	suite.Run(t, new(CustomResourceSuite))
}

func (suite *CustomResourceSuite) GlobalCRFixture() {
	suite.globalCREvt = k8s.SyncDataEvent{
		SyncType: k8s.CR_GLOBAL,
		Data: &corev1alpha2.Global{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fake",
			},
			Spec: corev1alpha2.GlobalSpec{
				Config:     &models.Global{},
				LogTargets: models.LogTargets{},
			},
		},
		Name: "globalcrjob",
	}
}
