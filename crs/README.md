# Custom Resources

## Purpose
This document aims to provide basic understanding of the workflow of writing a new custom resource for HAProxy Ingress Controller.  
Custom resources should be derived, whenever possible, from the [HAProxy CN (Client Native)](https://github.com/haproxytech/client-native) by reusing [HAProxy Models](https://github.com/haproxytech/client-native#haproxy-models).  
This guide describes how to create a CR (Custom Resource) to configure the global HAProxy section.  

## Prerequisites
We suppose that an HAProxy CN Model, corresponding to the kubernetes custom resource, is already available. This is the case for the HAProxy global section, that we are dealing with in this guide,  where we are going to reuse the following [global model](https://github.com/haproxytech/client-native/blob/master/models/global.go).  
If it is not the case, this needs to be done in [HAProxy Native client](https://github.com/haproxytech/client-native) by providing [swagger specification](https://github.com/haproxytech/client-native/blob/master/specification/build/haproxy_spec.yaml) of the Model and generate it via [make models](https://github.com/haproxytech/client-native/blob/master/Makefile).

## Directory Layout

```
crs
├── api
│   └── core
│       └── v1alpha1
│
├── definition
│   └── global.yaml
│
├── generated
│   ├── clientset
│   ├── informers
│   └── listers
│
├── code-generator.sh
└── README.md
```
- **crs/definition/**:  Directory for Custom Resource Definition
- **crs/api/<group>/<version>/**: GoLang Types should be created in the directory corresponding to their API Group and Version.
- **crs/code-generator.sh**: script using Kubernetes code generators to build native, versioned clients, informers and other helpers.
- **crs/generated**: code generated automatically via "code-generator.sh".

## Custom Resource Creation
### CRD
Let's start by creating the Custom Resource Definition describing the Global CR and defining the kind `Global` in the API Group `core.haproxy.org`  
To follow the above directory layout convention, the Global CRD should be in *crs/definitions/core/global.yaml*  
The following is the beginning of the CRD, full content can be found in the corresponding yaml file:  
```
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: globals.core.haproxy.org
spec:
  group: core.haproxy.org
  names:
    kind: Global
    plural: globals
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: 
                - config
              properties: 
                config: 
                  title: Global
                  description: HAProxy global configuration
                  type: object
                  properties:
                    chroot:
                      type: string
                      pattern: '^[^\s]+$'
                    group:
                      type: string
                      pattern: '^[^\s]+$'
                    hard_stop_after:
                      type: integer
                    user:
                      type: string
                      pattern: '^[^\s]+$'
                    daemon:
                      type: string
                      enum: [enabled, disabled]
```
The CRD is created via `apiextensions.k8s.io/v1` where the definition of a [structural schema](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation) is mandatory and used for OpenAPI v3.0 validation.  
The Custom Resource Definition is constructed by providing:
- API Group Name: `core.haproxy.org`, this is the group where you can find custom resources related to HAProxy configuration.
- Kind Name of the Resource: `Global` (and `globals` for plural name)
- Resource Scope: in this case it is `Namespaced`
- Group Version: `v1alpha1`
- openAPIV3Schema: For schema validation you just copy the [global schema](https://github.com/haproxytech/client-native/blob/master/specification/models/configuration.yaml#L2) from Client Native repository and insert it in the `config` field of the CRD.  
**NB:** a helper script is available under `crs/get-crd-schema.sh` in order to automatically retreive the schema from the CN repository. The script takes the name of Client Native model as parameter (ex: `get-crd-schema.sh global`) and returns to stdout the schema after applying the necessary changes to make it a [valid schema](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation) for kubernetes.

### GoLang Type
In this guide the API group is `core.haproxy.org` (this group is used for any resource dealing with haproxy configuration) and the API version is `alpha1v1`.  
So to follow the */crs/api/<group>/<version>/* convention, the GoLang Type describing the `Global` CR should be in *crs/api/core/alpha1v1/global.go* with the following content:
```
package v1alpha1

import (
	"github.com/haproxytech/client-native/v2/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Global is a specification for a Global resource
type Global struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GlobalSpec `json:"spec"`
}

// GlobalSpec defines the desired state of Global
type GlobalSpec struct {
	Config *models.Global `json:"config"`
}

// DeepCopyInto deepcopying  the GlobalSpec receiver into out. in must be non nil.
func (in *GlobalSpec) DeepCopyInto(out *GlobalSpec) {
	*out = *in
	if in.Config != nil {
		b, _ := in.Config.MarshalBinary()
		_ = out.Config.UnmarshalBinary(b)
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalList is a list of Global resources
type GlobalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Global `json:"items"`
}
```
- The GoLang types (Global for a specific resource and GlobalList for a list of resources) need to embed `TypeMeta` and `ObjectMeta` as they are mandatory for the Kubernetes type system and any resource served by Kubernetes API.
- The Spec reuses the [global configuration](https://github.com/haproxytech/client-native/blob/master/models/global.go) from HAProxy CN Models via the CR `config` field.
- Additional fields can added later to Spec if need be, but any input related to the HAProxy Global section config is provided via the `config` field.
- The deepcopy-gen tags are used to automatically generate deepCopy methods, required for any Kubernetes object, via the [code-generator](https://github.com/kubernetes/code-generator) tool. However Code Generators cannot generate the deepCopy method for types in external packages (in this case "github.com/haproxytech/client-native/v2/models") so this needs to be done manually by relying on the `MarshalBinary` and `UnmarshalBinary` of HAProxy Models.

### Code Generation
Custom Resources require more code and methods in order to be served by Kubernetes API but fortunately most of it can be generated automatically by Kubernetes code generators.  
The tool we are using is [k8s.io/code-generator](https://github.com/kubernetes/code-generator) which comes with the following code generators for CRs:
- deepcopy-gen: generates the new Type's DeepCopy methods.
- register-gen: generates methods to register the new Type in [Kubernetes scheme](https://github.com/kubernetes/apimachinery/blob/ef51ab160544f9d05b68e132a4af0b0fab459954/pkg/runtime/scheme.go#L47) and make it known to the Kubernetes type system.
- client-gen: generates the client set that will give us access to the new CR
- informer-gen: generates informer methods in order to watch and react to the CR changes.
- lister-gen: generates lister methods which provide a read-only caching layer for GET and LIST requests.

Before generating code, some global tags need to be set in the package of the versioned API group which should be in *crs/api/<group>/<version>/* directory.  
Usually this goes into the package's doc.go, so in this case it would be in *crs/api/core/alpha1v1/doc.go* with the following content:
```
// Package v1alpha1 contains the core v1alpha1 API group
//
// +k8s:deepcopy-gen=package
// +groupName=core.haproxy.org
package v1alpha1
```
In addition to including a number of global code generation tags, "doc.go" is used to describe the API group's purpose.

Now you can generate all necessary code for all CRs under the */crs/api* directory, via `make cr_generate` which calls the script `crs/code-generators.sh`


## Custom Resource Management
A custom resource manager already [exists](../controller/crmanager.go) as a single entry point for any custom resource defined inside HAProxy Ingress Controller.  
Its role is to be the unique delegate for all CR related task via the following CR interface:
```
type CR interface {
  GetKind() string
  GetInformer(chan SyncDataEvent, informers.SharedInformerFactory) cache.SharedIndexInformer
  ProcessEvent(*store.K8s, SyncDataEvent) bool
}
```
Each CR should implement the above interface in order to:
- provide the CR kind which allows the CRManager to register supported Kinds and map them to the corresponding CR implementation.
- provide the Kubernetes Informer to watch and react to CR events.
- Process the CR events which is mainly about storing the CR object in the Ingress Controller store.
