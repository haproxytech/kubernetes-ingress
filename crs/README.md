# Custom Resources

## Purpose

This document aims to provide basic understanding of the workflow of writing a new custom resource for HAProxy Ingress Controller.
Custom resources should be derived, whenever possible, from the [HAProxy CN (Client Native)](https://github.com/haproxytech/client-native) by reusing [HAProxy Models](https://github.com/haproxytech/client-native#haproxy-models).
This guide describes how to create a CR (Custom Resource) to configure the global HAProxy section.

## Prerequisites

We suppose that an HAProxy CN Model, corresponding to the kubernetes custom resource, is already available. This is the case for the HAProxy global section, that we are dealing with in this guide.
If it is not the case, this needs to be done in [HAProxy Native client](https://github.com/haproxytech/client-native) by providing [swagger specification](https://github.com/haproxytech/client-native/blob/master/specification/build/haproxy_spec.yaml) of the Model and generate it via [make models](https://github.com/haproxytech/client-native/blob/master/Makefile).

## Directory Layout

```txt
crs
├── api
│   └── ingress
│       └── v1
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
- **crs/generated**: code generated automatically via `make cr_generate`.

### GoLang Type

In this guide the API group is `ingress.v3.haproxy.org` (this group is used for any resource dealing with haproxy configuration) and the API version is `v3`.
So to follow the */crs/api/<group>/<version>/* convention, the GoLang Type describing the `Global` CR should be in *crs/api/ingress/v1/global.go* with the following content:

```go
package v3

import (
  "github.com/haproxytech/client-native/v6/models"
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
- The deepcopy-gen tags are used to automatically generate deepCopy methods, required for any Kubernetes object, via the [code-generator](https://github.com/kubernetes/code-generator) tool. However Code Generators cannot generate the deepCopy method for types in external packages (in this case "github.com/haproxytech/client-native/v3/models") so this needs to be done manually by relying on the `MarshalBinary` and `UnmarshalBinary` of HAProxy Models.

### Code Generation

Custom Resources require more code and methods in order to be served by Kubernetes API but fortunately most of it can be generated automatically by Kubernetes code generators.
The tool we are using is [k8s.io/code-generator](https://github.com/kubernetes/code-generator) which comes with the following code generators for CRs:

- deepcopy-gen: generates the new Type's DeepCopy methods.
- register-gen: generates methods to register the new Type in [Kubernetes scheme](https://github.com/kubernetes/apimachinery/blob/ef51ab160544f9d05b68e132a4af0b0fab459954/pkg/runtime/scheme.go#L47) and make it known to the Kubernetes type system.
- client-gen: generates the client set that will give us access to the new CR
- informer-gen: generates informer methods in order to watch and react to the CR changes.
- lister-gen: generates lister methods which provide a read-only caching layer for GET and LIST requests.

Before generating code, some global tags need to be set in the package of the versioned API group which should be in *crs/api/<group>/<version>/* directory.
Usually this goes into the package's doc.go, so in this case it would be in *crs/api/ingress/v3/doc.go* with the following content:

```yml
// Package v3 contains the core v3 API group
//
// +k8s:deepcopy-gen=package
// +groupName=ingress.v3.haproxy.org
package v3
```

In addition to including a number of global code generation tags, "doc.go" is used to describe the API group's purpose.

Now you can generate all necessary code for all CRs under the */crs/api* directory, via `make cr_generate`

## Custom Resource Management

A custom resource manager already [exists](../controller/crmanager.go) as a single entry point for any custom resource defined inside HAProxy Ingress Controller.
Its role is to be the unique delegate for all CR related task via the following CR interface:

```go
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
