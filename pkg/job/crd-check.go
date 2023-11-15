// Copyright 2023 HAProxy Technologies LLC
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
package job

import (
	"context"

	"github.com/haproxytech/kubernetes-ingress/crs/definition"
	"github.com/haproxytech/kubernetes-ingress/pkg/k8s"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func CRDRefresh(log utils.Logger, osArgs utils.OSArgs) error {
	log.Info("checking CRDS")
	config, err := k8s.GetRestConfig(osArgs)
	if err != nil {
		return err
	}

	// Create a new clientset for the apiextensions API group
	clientset := apiextensionsclientset.NewForConfigOrDie(config)

	// Check if the CRD exists
	crds := definition.GetCRDs()
	crdsUpgrade := definition.GetCRDsUpgrade()
	for crdName, crdDef := range crds {
		// CustomResourceDefinition object
		var crd apiextensionsv1.CustomResourceDefinition
		err = yaml.Unmarshal(crdDef, &crd)
		if err != nil {
			return err
		}
		log.Info("")
		log.Infof("checking CRD %s", crdName)

		existingVersion, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdName, metav1.GetOptions{})
		if err != nil {
			if !apiError.IsNotFound(err) {
				return err
			}
			log.Infof("CRD %s does not exist", crdName)
			// Create the CRD
			_, err = clientset.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), &crd, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			log.Infof("CRD %s created", crdName)
			continue
		}
		log.Infof("CRD %s exists", crdName)
		versions := existingVersion.Spec.Versions
		if len(versions) == 2 {
			log.Infof("CRD %s exists as v1alpha1 and v1alpha2, nothing to do", crdName)
			continue
		}
		// check if we have alpha 2 or we need to upgrade for alpha2
		crd.ObjectMeta.ResourceVersion = existingVersion.ObjectMeta.ResourceVersion
		if versions[0].Name == "v1alpha2" {
			log.Infof("CRD %s exists as v1alpha2, nothing to do", crdName)
			continue
		}
		err = yaml.Unmarshal(crdsUpgrade[crdName], &crd)
		if err != nil {
			return err
		}
		// Upgrade the CRDl
		_, err = clientset.ApiextensionsV1().CustomResourceDefinitions().Update(context.Background(), &crd, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	log.Info("")
	log.Info("CRD update done")
	return nil
}

// IngressControllerCRDUpdater console pretty print
const IngressControllerCRDUpdater = `
  ____ ____  ____    _   _           _       _
 / ___|  _ \|  _ \  | | | |_ __   __| | __ _| |_ ___ _ __
| |   | |_) | | | | | | | | '_ \ / _` + "`" + ` |/ _` + "`" + ` | __/ _ \ '__|
| |___|  _ <| |_| | | |_| | |_) | (_| | (_| | ||  __/ |
 \____|_| \_\____/   \___/| .__/ \__,_|\__,_|\__\___|_|
                          |_|

`
