package handler

import (
	"reflect"
	"testing"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

var (
	tcpServivesTest = TCPServices{}
	k8sStoreTest    = store.K8s{
		Namespaces: map[string]*store.Namespace{},
	}
)

func TestParseTCPService(t *testing.T) {
	type ioTest struct {
		input  string
		output tcpSvcParser
	}
	// initialize
	k8sStoreTest.Namespaces["ns"] = &store.Namespace{
		Name:            "ns",
		Relevant:        false,
		Ingresses:       map[string]*store.Ingress{},
		Endpoints:       map[string]map[string]*store.Endpoints{},
		Services:        map[string]*store.Service{},
		Secret:          map[string]*store.Secret{},
		HAProxyRuntime:  map[string]map[string]*store.RuntimeBackend{},
		CRs:             &store.CustomResources{},
		Gateways:        map[string]*store.Gateway{},
		TCPRoutes:       map[string]*store.TCPRoute{},
		ReferenceGrants: map[string]*store.ReferenceGrant{},
		Labels:          map[string]string{},
		Status:          "",
	}
	k8sStoreTest.Namespaces["ns"].Services["svc"] = &store.Service{
		Namespace:   "ns",
		Name:        "svc",
		Ports:       []store.ServicePort{},
		Addresses:   []string{},
		DNS:         "",
		Annotations: map[string]string{},
		Status:      "",
	}
	ioParserTest := [...]ioTest{
		{
			"ns/svc:8888",
			tcpSvcParser{
				service: &store.Service{
					Namespace: "ns",
					Name:      "svc",
				},
				port:       8888,
				sslOffload: false,
				annList:    map[string]string{},
			},
		},
		{
			"ns/svc:8888:ssl",
			tcpSvcParser{
				service: &store.Service{
					Namespace: "ns",
					Name:      "svc",
				},
				port:       8888,
				sslOffload: true,
				annList:    map[string]string{},
			},
		},
		{
			"ns/svc:8888:ssl:load-balance=leastconn,default-server=check",
			tcpSvcParser{
				service: &store.Service{
					Namespace: "ns",
					Name:      "svc",
				},
				port:       8888,
				sslOffload: true,
				annList: map[string]string{
					"load-balance":   "leastconn",
					"default-server": "check",
				},
			},
		},
		{
			"ns/svc:8888:ssl:load-balance=leastconn,default-server=check:todo",
			tcpSvcParser{
				service: &store.Service{
					Namespace: "ns",
					Name:      "svc",
				},
				port:       8888,
				sslOffload: true,
				annList: map[string]string{
					"load-balance":   "leastconn",
					"default-server": "check",
				},
			},
		},
	}

	for _, v := range ioParserTest {
		p, err := tcpServivesTest.parseTCPService(k8sStoreTest, v.input)
		if err != nil {
			t.Errorf("got error %v", err)
		}
		if !reflect.DeepEqual(p.annList, v.output.annList) && (len(p.annList) != 0 && len(v.output.annList) != 0) {
			t.Errorf("got %v, wanted %v", p.annList, v.output.annList)
		}
		if p.port != v.output.port {
			t.Errorf("got %v, wanted %v", p.port, v.output.port)
		}
		if p.service.Name != v.output.service.Name || p.service.Namespace != v.output.service.Namespace {
			t.Errorf("got %v, wanted %v", p.port, v.output.service.Ports[0])
		}
		if p.sslOffload != v.output.sslOffload {
			t.Errorf("got %v, wanted %v", p.sslOffload, v.output.sslOffload)
		}
	}
}
