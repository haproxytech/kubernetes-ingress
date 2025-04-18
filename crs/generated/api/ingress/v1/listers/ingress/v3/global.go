//
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

// Code generated by lister-gen. DO NOT EDIT.

package v3

import (
	v3 "github.com/haproxytech/kubernetes-ingress/crs/api/ingress/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GlobalLister helps list Globals.
// All objects returned here must be treated as read-only.
type GlobalLister interface {
	// List lists all Globals in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v3.Global, err error)
	// Globals returns an object that can list and get Globals.
	Globals(namespace string) GlobalNamespaceLister
	GlobalListerExpansion
}

// globalLister implements the GlobalLister interface.
type globalLister struct {
	indexer cache.Indexer
}

// NewGlobalLister returns a new GlobalLister.
func NewGlobalLister(indexer cache.Indexer) GlobalLister {
	return &globalLister{indexer: indexer}
}

// List lists all Globals in the indexer.
func (s *globalLister) List(selector labels.Selector) (ret []*v3.Global, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v3.Global))
	})
	return ret, err
}

// Globals returns an object that can list and get Globals.
func (s *globalLister) Globals(namespace string) GlobalNamespaceLister {
	return globalNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// GlobalNamespaceLister helps list and get Globals.
// All objects returned here must be treated as read-only.
type GlobalNamespaceLister interface {
	// List lists all Globals in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v3.Global, err error)
	// Get retrieves the Global from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v3.Global, error)
	GlobalNamespaceListerExpansion
}

// globalNamespaceLister implements the GlobalNamespaceLister
// interface.
type globalNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Globals in the indexer for a given namespace.
func (s globalNamespaceLister) List(selector labels.Selector) (ret []*v3.Global, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v3.Global))
	})
	return ret, err
}

// Get retrieves the Global from the indexer for a given namespace and name.
func (s globalNamespaceLister) Get(name string) (*v3.Global, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v3.Resource("global"), name)
	}
	return obj.(*v3.Global), nil
}
