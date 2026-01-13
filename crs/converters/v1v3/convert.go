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
package convert

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	v1 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v1"
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"github.com/haproxytech/kubernetes-ingress/crs/converters"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func ConvertV1V3(osArgs utils.OSArgs) error {
	inputIsFile := isFile(osArgs.CRDInputFile)
	if !inputIsFile {
		return fmt.Errorf("%s is not a directory", osArgs.CRDInputFile)
	}
	// Perform actions on the file
	utils.GetLogger().Infof("Processing %s", osArgs.CRDInputFile)
	// Example: Read and print the file's content
	content, err := os.ReadFile(osArgs.CRDInputFile)
	if err != nil {
		return err
	}

	unstructured, err := yamlToCRD(content)
	if err != nil {
		return err
	}

	if unstructured.Object["apiVersion"] != "ingress.v1.haproxy.org/v1" {
		return errors.New("apiVersion is not ingress.v1.haproxy.org/v1 - Only conversion from ingress.v1.haproxy.org/v1 is supported")
	}
	utils.GetLogger().Infof("Converting from version %s", unstructured.Object["apiVersion"])

	switch unstructured.Object["kind"] {
	case "Backend":
		err = convertBackend(osArgs, unstructured)
	case "Defaults":
		err = convertDefaults(osArgs, unstructured)
	case "Global":
		err = convertGlobal(osArgs, unstructured)
	case "TCP":
		err = convertTCP(osArgs, unstructured)
	}

	if err != nil {
		return err
	}

	return nil
}

func convertBackend(args utils.OSArgs, unstructured *unstructured.Unstructured) error {
	v1Backend := &v1.Backend{}
	// read CRD
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, v1Backend)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to Backend: %w", err)
	}
	// convert
	v3BackendSpec := converters.DeepConvertBackendSpecV1toV3(v1Backend.Spec)
	v3Backend := v3.Backend{
		ObjectMeta: v1Backend.ObjectMeta,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Backend",
			APIVersion: "ingress.v3.haproxy.org/v3",
		},
		Spec: v3BackendSpec,
	}
	// Marshal the Backend object to YAML
	yamlData, err := yaml.Marshal(v3Backend)
	if err != nil {
		return err
	}

	// Write the YAML to a file
	err = os.WriteFile(args.CRDOutputFile, yamlData, 0o600)
	if err != nil {
		return err
	}

	utils.GetLogger().Infof("v3 Backend has been written to %s\n", args.CRDOutputFile)

	return nil
}

func convertGlobal(args utils.OSArgs, unstructured *unstructured.Unstructured) error {
	v1Global := &v1.Global{}
	// read CRD
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, v1Global)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to Global: %w", err)
	}
	// convert
	v3GlobalSpec := converters.DeepConvertGlobalSpecV1toV3(v1Global.Spec)
	v3Global := v3.Global{
		ObjectMeta: v1Global.ObjectMeta,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Global",
			APIVersion: "ingress.v3.haproxy.org/v3",
		},
		Spec: v3GlobalSpec,
	}
	// Marshal the Backend object to YAML
	yamlData, err := yaml.Marshal(v3Global)
	if err != nil {
		return err
	}

	// Write the YAML to a file
	err = os.WriteFile(args.CRDOutputFile, yamlData, 0o600)
	if err != nil {
		return err
	}

	utils.GetLogger().Infof("v3 Global has been written to %s\n", args.CRDOutputFile)

	return nil
}

func convertDefaults(args utils.OSArgs, unstructured *unstructured.Unstructured) error {
	v1Defaults := &v1.Defaults{}
	// read CRD
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, v1Defaults)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to Global: %w", err)
	}
	// convert
	v3DefaultsSpec := converters.DeepConvertDefaultsSpecV1toV3(v1Defaults.Spec)
	v3Defaults := v3.Defaults{
		ObjectMeta: v1Defaults.ObjectMeta,
		TypeMeta: metav1.TypeMeta{
			Kind:       "Defaults",
			APIVersion: "ingress.v3.haproxy.org/v3",
		},
		Spec: v3DefaultsSpec,
	}
	// Marshal the Backend object to YAML
	yamlData, err := yaml.Marshal(v3Defaults)
	if err != nil {
		return err
	}

	// Write the YAML to a file
	err = os.WriteFile(args.CRDOutputFile, yamlData, 0o600)
	if err != nil {
		return err
	}

	utils.GetLogger().Infof("v3 Global has been written to %s\n", args.CRDOutputFile)

	return nil
}

func convertTCP(args utils.OSArgs, unstructured *unstructured.Unstructured) error {
	v1TCP := &v1.TCP{}
	// read CRD
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, v1TCP)
	if err != nil {
		return fmt.Errorf("failed to convert unstructured to Backend: %w", err)
	}
	// convert
	v3TCPSpec := converters.DeepConvertTCPSpecV1toV3(v1TCP.Spec)
	v3TCP := v3.TCP{
		ObjectMeta: v1TCP.ObjectMeta,
		TypeMeta: metav1.TypeMeta{
			Kind:       "TCP",
			APIVersion: "ingress.v3.haproxy.org/v3",
		},
		Spec: v3TCPSpec,
	}
	// Marshal the Backend object to YAML
	yamlData, err := yaml.Marshal(v3TCP)
	if err != nil {
		return err
	}

	// Write the YAML to a file
	err = os.WriteFile(args.CRDOutputFile, yamlData, 0o600)
	if err != nil {
		return err
	}

	utils.GetLogger().Infof("v3 TCP has been written to %s\n", args.CRDOutputFile)

	return nil
}

// yamlToCRD converts YAML data into an unstructured Kubernetes object.
func yamlToCRD(yamlData []byte) (*unstructured.Unstructured, error) {
	// Parse the YAML into a map
	var obj map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Convert the map into an unstructured Kubernetes object
	unstructuredObj := &unstructured.Unstructured{Object: obj}

	// Validate that the object has the necessary CRD fields
	if _, found, _ := unstructured.NestedString(unstructuredObj.Object, "apiVersion"); !found {
		return nil, errors.New("missing apiVersion in YAML")
	}
	if _, found, _ := unstructured.NestedString(unstructuredObj.Object, "kind"); !found {
		return nil, errors.New("missing kind in YAML")
	}

	return unstructuredObj, nil
}

func isFile(fileName string) bool {
	isFile := true
	file, err := os.Open(fileName)
	if err == nil {
		var fileInfo fs.FileInfo
		fileInfo, err = file.Stat()
		if err == nil {
			isFile = !fileInfo.IsDir()
		}
	}
	defer file.Close()
	return isFile
}
