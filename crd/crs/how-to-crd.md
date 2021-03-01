# CRD

## Purpose
This document aims to provide basic understanding of the worflow of  writing a new custom resource for HAProxy Ingress Controller.

## Model
CRD definition includes an optional schema validation based on openAPIV3Schema.Consequently, we'll be in position to reuse the model specification within the dataplaneapi-specification project.
Let's start with an imaginary 'Foo' resource. Switch to your dataplaneapi-specification local repository and add into models/configuration.yaml the Foo specification:
```
foo:
  description: HAProxy foo resource
  title: foo resource
  type: object
  properties:
    spec:
      type: object
      properties:
        bar:
          type: string
        foobar:
          type: string
          pattern: '^\d+(s|m|h)$'
          x-nullable: true
```
Then in haproxy-spec.yaml:
```
  foo:
    $ref: "models/configuration.yaml#/foo"
```
In the most classical way, generate the model with:
```
cd build
go build .
./build -file ../haproxy-spec.yaml > haproxy_spec.yaml
~/go/bin/swagger generate model -f haproxy_spec.yaml -r ../copyright.txt  -t <target path>
```
### model source file tweaking
In the target path, where models were generated, copy and paste foo.go into the models directory inside kubernetes-ingress repository. The directory structure of the new model should comply with :
`<resource name>/<version>`
To determine the version you should use for your custom resource please refer to [versions](https://kubernetes.io/docs/reference/using-api/#api-versioning).
In our case we'll go with a v1alpha1 version, that means we copy foo.go into models/foo/v1alpha1.
We need to add some parts to the source file to make our new struct a valid custom resource.
* Change the package name to `v1alpha`
* Add the following import:
 `  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`
* Add the following fields into the struct:
```
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"
```
* Just under the swagger tags insert the additional tags:
```
  // swagger:model foo
  // +genclient
  // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
```
* Add a list holder struct for listers:
```
  // +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
  type FooList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`

    Items []Foo `json:"items"`
  }
```

Finally, the beginning of your source file should look like this:

```
import (
	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Foo foo resource
//
// HAProxy foo resource
//
// swagger:model foo
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Foo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// spec
	Spec *FooSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FooList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Foo `json:"items"`
}
```

### Source files addition

You also need to add a doc.go file with the following contents:

```
// Package v1alpha1 is the v1alpha1 version of the API.
// +k8s:deepcopy-gen=package,register
// +groupName=haproxy.org
package v1alpha1

```


## Custom resource artefacts generation

First, you need to install the code generators. To this end, carry out:
`make crd_install_generators`

Go back to the ingress controller project. You can use the makefile to generate all custom resources artefacts in any version by:
`make crd_generate_all` or `make` (default task)

to generate a particular couple of custom resource and version, you use:
`make crd_generate crd=<crd name> [version=<crd version>]`

The version is an optional parameter with default value of `v1`.

generated artefacts:
* models/\<crd>/\<version>/zz_generated.deepcopy.go : provides deep copy functions.
* models/\<crd>/\<version>/zz_generated.register.go : provides crd registration functions.
* crs/\<crd>/\<version>/clientset : provides crd client.
* crs/\<crd>/\<version>/informers : provides crd informers.
* crs/\<crd>/\<version>/listers : provides crd listers.

## CRD creation

Let's define now the custom resource inside foo.crd.yaml with :
```
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: foos.haproxy.org
spec:
  group: haproxy.org
  version: v1alpha1
  scope: Namespaced
  names:
    kind: Foo
    plural: foos
  validation: # optional
    openAPIV3Schema:
      description: HAProxy foo resource
      title: foo resource
      type: object
      properties:
        spec:
          type: object
          properties:
            bar:
              type: string
            foobar:
              type: string
              pattern: '^\d+(s|m|h)$'
  versions: # defaulted to the Spec.Version field
  - name: v1alpha1
    served: true
    storage: true
```
The chief points to pay attention to are :
* The name must be the plural name concatenated with group name.
* The validation can be copied from the model specification we did previously with the removal of unsupported fields like `x-nullable`. Have a look at [Specifying a structural schema](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#specifying-a-structural-schema)

Apply it with kubectl:
`kubectl apply -f foo.crd.yaml`

Now the turn of our custom resource inside foo.yaml file:
```
apiVersion: haproxy.org/v1alpha1
kind: Foo
metadata:
  name: foo-test
bar: "foobar"
```
Apply it the same way:
`kubectl apply -f foo.yaml`

## Custom resource management

We need to write an handler to react to custom resource events and take appropriate actions in response.
This handler will activate informers of its managed custom resource and react to the different lifecycle events : creation, update, deletion.
A custom resource manager already exists as a single entry point for any custom resource defined inside HAProxy Ingress Controller. Its role is to be the unique delegate for all the various existing handlers and to manage these handlers within a restricted scope.
The code of the custom resource manager is available in controller/crmanager.go.

### store counterpart

You won't use the raw custom resource struct inside your handler. Two of the many reasons we could find to support this design is the absence of status field in custom resource struct and the annoyance of the intermediate Spec struct. Thus you have to write your compliant store counterpart.
Create your "store" Foo struct into the file controller/store/types.go as follows:

```
type Foo struct {
	Name      string
	Namespace string

	// bar
	Bar string

	// foobar
	Foobar *string

	Status Status
}
```
It's simply a copy of the contents of the Spec struct of the Foo struct and the addition of Status, Name and Namespace fields. We can keep the json tags if the default names deduced from fields names are not correct.

### Writing handler

An handler must comply with the interface definition of CRHandler:

```
type CRHandler interface {
	GetAssociatedType() string
	EventCustomResource(item interface{}) bool
	Update() (reload bool, err error)
	GetInformer() cache.SharedIndexInformer
}
```

Explanation:
 * GetAssociatedType :
   permits the correct association of an handler and its custom resource by the custom resource manager.
 * GetInformer:
   delivers the informer to the custom resource manager.
 * EventCustomResource:
   takes the "store" counterpart of custom resource as input and make preamble actions : storing, checking, etc.
 * Update:
   takes the actions expected with regards to the current states and status (added, updated, deleted) of the custom resources.

A dummy handler would look like:

```
package controller

import (
	"encoding/json"
	"time"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	fooclientset "github.com/haproxytech/kubernetes-ingress/crs/foo/v1alpha1/clientset/versioned"
	fooinformers "github.com/haproxytech/kubernetes-ingress/crs/foo/v1alpha1/informers/externalversions"
	"github.com/haproxytech/kubernetes-ingress/models/foo/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type fooHandler struct {
	informer   cache.SharedIndexInformer
	controller *HAProxyController
}

// NewFooHandler creates a new custom resource foo handler
func NewFooHandler(c *HAProxyController, stop chan struct{}, k8s *K8s) *fooHandler {
	clientset := getRemoteKubernetesFooCrd(k8s.RestConfig)
	informerFactory := fooinformers.NewSharedInformerFactory(clientset, time.Minute*60)
	informer := informerFactory.Haproxytech().V1alpha1().Foos().Informer()
	handler := &fooHandler{controller: c, informer: informer}
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.addFunc,
		UpdateFunc: handler.updateFunc,
		DeleteFunc: handler.deleteFunc,
	})
	go informer.Run(stop)
	return handler

}

func (handler fooHandler) GetInformer() cache.SharedIndexInformer {
	return handler.informer
}

func (handler *fooHandler) Update() (reload bool, err error) {
	logger.Printf("******************* Update received *******************")
	return
}

func (handler *fooHandler) addFunc(object interface{}) {
	logger.Print("========================= foohandler addition =========================")
	handler.sendToChannel(object, ADDED)
}

func (handler *fooHandler) updateFunc(oldObject, newObject interface{}) {
	logger.Print("========================= foohandler update =========================")
	handler.sendToChannel(newObject, MODIFIED)
}

func (handler *fooHandler) deleteFunc(object interface{}) {
	logger.Print("========================= fooHandler delete =========================")
	handler.sendToChannel(object, DELETED)
}

func (handler *fooHandler) sendToChannel(object interface{}, status store.Status) {
	logger.Print("========================= fooHandler sendToChannel =========================")
	foo, ok := object.(*v1alpha1.Foo)
	if !ok {
		handler.controller.k8s.Logger.Errorf("%s: Invalid data from k8s api, %s", "Global Configuration", object)
		return
	}
	item := convertToFoo(foo, status)
	handler.controller.k8s.Logger.Tracef("%s %s: %s", "Global Configuration", item.Status, item.Name)
	handler.controller.eventChan <- SyncDataEvent{SyncType: CUSTOM_RESOURCE, Namespace: item.Namespace, Data: item}
}

func (handler fooHandler) EventCustomResource(item interface{}) bool {
	data := item.(*store.Foo)
	logger.Printf("***************** store foo %+v somewhere **************", data)
	return true
}

func convertToFoo(foo *v1alpha1.Foo, status store.Status) *store.Foo {

	data, err := json.Marshal(foo.Spec)
	if err != nil {
		logger.Err(err)
		return nil
	}

	var i2 store.Foo
	if err := json.Unmarshal(data, &i2); err != nil {
		logger.Err(err)
		return nil
	}
	i2.Name = foo.Name
	i2.Namespace = foo.Namespace
	i2.Status = status

	return &i2
}

func (handler fooHandler) GetAssociatedType() string {
	return "*store.Foo"
}

func getRemoteKubernetesFooCrd(restConfig *rest.Config) *fooclientset.Clientset {
	return fooclientset.NewForConfigOrDie(restConfig)
}
```

Then this handler should be added to the custom resource manager in the handlers map. This map associates the type managed by an handler with this handler. In practice, the manager calls the GetAssociatedType of the handler to get the information. This addition should take place into NewCRManager:

```
func NewCRManager(c *HAProxyController, stop chan struct{}) CRManager {
	k8s, client, store, channel := c.k8s, c.Client, c.Store, c.eventChan
	manager := CRManager{channel: channel, stop: stop, handlers: map[string]CRHandler{}}
	...
	manager.Register(NewFooHandler(c, stop, k8s))
	return manager
}
```

Now if you launch the Ingress Controller, you should see in logs the activity of your new Foo manager!