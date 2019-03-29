package main

import (
	clientnative "github.com/haproxytech/client-native"
	"github.com/haproxytech/models"
)

const (
	RATE_LIMIT    = "rate-limit"
	HTTP_REDIRECT = "http-redirect"
)

//Configuration represents k8s state
type Configuration struct {
	Namespace           map[string]*Namespace
	ConfigMap           *ConfigMap
	NativeAPI           *clientnative.HAProxyClient
	HTTPSListeners      *MapIntW
	HTTPBindProcess     string
	SSLRedirect         string
	RateLimitingEnabled bool
	HTTPRequests        map[string][]models.HTTPRequestRule
	HTTPRequestsStatus  Status
	TCPRequests         map[string][]models.TCPRequestRule
	TCPRequestsStatus   Status
}

//Init itialize configuration
func (c *Configuration) Init(api *clientnative.HAProxyClient) {
	c.Namespace = make(map[string]*Namespace)
	c.HTTPSListeners = &MapIntW{}
	c.HTTPBindProcess = "1/1"
	c.SSLRedirect = ""
	c.NativeAPI = api

	c.HTTPRequests = map[string][]models.HTTPRequestRule{}
	c.HTTPRequests[RATE_LIMIT] = []models.HTTPRequestRule{}
	c.HTTPRequestsStatus = EMPTY

	c.TCPRequests = map[string][]models.TCPRequestRule{}
	c.TCPRequests[RATE_LIMIT] = []models.TCPRequestRule{}
	c.TCPRequestsStatus = EMPTY

	c.HTTPRequests[HTTP_REDIRECT] = []models.HTTPRequestRule{}
}

//GetNamespace returns Namespace. Creates one if not existing
func (c *Configuration) GetNamespace(name string) *Namespace {
	namespace, ok := c.Namespace[name]
	if ok {
		return namespace
	}
	newNamespace := c.NewNamespace(name)
	c.Namespace[name] = newNamespace
	return newNamespace
}

//NewNamespace returns new initialized Namespace
func (c *Configuration) NewNamespace(name string) *Namespace {
	namespace := &Namespace{
		Name:     name,
		Relevant: name == "default",
		//Annotations
		Pods:      make(map[string]*Pod),
		PodNames:  make(map[string]bool),
		Services:  make(map[string]*Service),
		Ingresses: make(map[string]*Ingress),
		Secret:    make(map[string]*Secret),
		Status:    ADDED,
	}
	return namespace
}

//Clean cleans all the statuses of various data that was changed
//deletes them completely or just resets them if needed
func (c *Configuration) Clean() {
	for _, namespace := range c.Namespace {
		for _, data := range namespace.Ingresses {
			for _, rule := range data.Rules {
				switch rule.Status {
				case DELETED:
					delete(data.Rules, rule.Host)
					continue
				default:
					rule.Status = EMPTY
					for _, path := range rule.Paths {
						switch path.Status {
						case DELETED:
							delete(rule.Paths, path.Path)
						default:
							path.Status = EMPTY
						}
					}
				}
			}
			data.Annotations.Clean()
			switch data.Status {
			case DELETED:
				delete(namespace.Ingresses, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, data := range namespace.Services {
			data.Annotations.Clean()
			switch data.Status {
			case DELETED:
				delete(namespace.Services, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, data := range namespace.Pods {
			switch data.Status {
			case DELETED:
				delete(namespace.PodNames, data.HAProxyName)
				delete(namespace.Pods, data.Name)
			default:
				data.Status = EMPTY
			}
		}
		for _, data := range namespace.Secret {
			switch data.Status {
			case DELETED:
				delete(namespace.Secret, data.Name)
			default:
				data.Status = EMPTY
			}
		}
	}
	c.ConfigMap.Annotations.Clean()
	switch c.ConfigMap.Status {
	case DELETED:
		c.ConfigMap = nil
	default:
		c.ConfigMap.Status = EMPTY
	}
	c.HTTPRequestsStatus = EMPTY
	c.TCPRequestsStatus = EMPTY
	c.HTTPSListeners.Clean()
}
