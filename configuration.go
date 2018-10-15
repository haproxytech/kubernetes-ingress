package main

import (
	clientnative "github.com/haproxytech/client-native"
	"k8s.io/apimachinery/pkg/watch"
)

type Configuration struct {
	Namespace map[string]*Namespace
	ConfigMap map[string]*ConfigMap
	Secret    map[string]*Secret
	NativeAPI *clientnative.HAProxyClient
}

func (c *Configuration) Init(api *clientnative.HAProxyClient) {
	c.Namespace = make(map[string]*Namespace)
	c.ConfigMap = make(map[string]*ConfigMap)
	c.Secret = make(map[string]*Secret)
	c.NativeAPI = api
}

func (c *Configuration) GetNamespace(name string) *Namespace {
	namespace, ok := c.Namespace[name]
	if ok {
		return namespace
	}
	newNamespace := c.NewNamespace(name)
	c.Namespace[name] = newNamespace
	return newNamespace
}

func (c *Configuration) NewNamespace(name string) *Namespace {
	namespace := &Namespace{
		Name:     name,
		Relevant: name == "default",
		//Annotations
		Pods:      make(map[string]*Pod),
		PodNames:  make(map[string]bool),
		Services:  make(map[string]*Service),
		Ingresses: make(map[string]*Ingress),
		Watch:     "",
	}
	return namespace
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	for _, namespace := range c.Namespace {
		for _, data := range namespace.Ingresses {
			for _, rule := range data.Rules {
				switch rule.Watch {
				case watch.Deleted:
					delete(data.Rules, rule.Host)
					continue
				default:
					rule.Watch = ""
					for _, path := range rule.Paths {
						switch path.Watch {
						case watch.Deleted:
							delete(rule.Paths, path.Path)
						default:
							path.Watch = ""
						}
					}
				}
			}
			switch data.Watch {
			case watch.Deleted:
				delete(namespace.Ingresses, data.Name)
			default:
				data.Watch = ""
			}
		}
		for _, data := range namespace.Services {
			switch data.Watch {
			case watch.Deleted:
				delete(namespace.Services, data.Name)
			default:
				data.Watch = ""
			}
		}
		for _, data := range namespace.Pods {
			switch data.Watch {
			case watch.Deleted:
				delete(namespace.PodNames, data.HAProxyName)
				delete(namespace.Pods, data.Name)
			default:
				data.Watch = ""
			}
		}
	}
}
