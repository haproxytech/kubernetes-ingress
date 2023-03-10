package store

import "testing"

func TestGatewayClassEquality(t *testing.T) {
	type testGatewayClass struct {
		gwc1        *GatewayClass
		gwc2        *GatewayClass
		description string
		expected    bool
	}
	description1 := "description gatewayclass 1"
	description2 := "description gatewayclass 2"
	tests := []testGatewayClass{
		{expected: true, description: "Two nil pointers"},
		{expected: false, gwc2: &GatewayClass{}, description: "Left nil pointer"},
		{expected: false, gwc1: &GatewayClass{}, description: "Right nil pointer"},
		{expected: true, gwc1: &GatewayClass{}, gwc2: &GatewayClass{}, description: "Two empty gatewayclasses"},
		{
			expected: true, gwc1: &GatewayClass{Name: "haproxy-gw-class", ControllerName: "example.net/gateway-controller", Description: &description1},
			gwc2: &GatewayClass{Name: "haproxy-gw-class", ControllerName: "example.net/gateway-controller", Description: &description1}, description: "Two same gatewayclasses",
		},
		{
			expected:    false,
			gwc1:        &GatewayClass{Name: "gatewayclass1", ControllerName: "example.net/gateway-controller", Description: &description1},
			gwc2:        &GatewayClass{Name: "gatewayclass2", ControllerName: "example.net/gateway-controller", Description: &description1},
			description: "Two differents gatewayclasses by a name",
		},
		{
			expected:    false,
			gwc1:        &GatewayClass{Name: "gatewayclass1", ControllerName: "example.net/gateway-controller", Description: &description1},
			gwc2:        &GatewayClass{Name: "gatewayclass1", ControllerName: "example.net/other-gateway-controller", Description: &description1},
			description: "Two differents gatewayclasses by a controller name",
		},
		{
			expected:    false,
			gwc1:        &GatewayClass{Name: "gatewayclass1", ControllerName: "example.net/gateway-controller", Description: &description1},
			gwc2:        &GatewayClass{Name: "gatewayclass1", ControllerName: "example.net/gateway-controller", Description: &description2},
			description: "Two differents gatewayclasses by description",
		},
	}

	for _, test := range tests {
		equality := test.gwc1.Equal(test.gwc2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestRouteNamespacesEquality(t *testing.T) {
	type testRouteNamespaces struct {
		rn1         *RouteNamespaces
		rn2         *RouteNamespaces
		description string
		expected    bool
	}
	all := "All"
	same := "Same"
	tests := []testRouteNamespaces{
		{expected: true, description: "Two nil pointers"},
		{expected: false, rn2: &RouteNamespaces{}, description: "Left nil pointer"},
		{expected: false, rn1: &RouteNamespaces{}, description: "Right nil pointer"},
		{expected: true, rn1: &RouteNamespaces{}, rn2: &RouteNamespaces{}, description: "Two empty RouteNamespaces"},
		{expected: false, rn1: &RouteNamespaces{}, rn2: &RouteNamespaces{From: &all}, description: "Two different RouteNamespaces with nil From on left"},
		{expected: false, rn1: &RouteNamespaces{}, rn2: &RouteNamespaces{From: &all}, description: "Two different RouteNamespaces with nil From on left"},
		{expected: false, rn1: &RouteNamespaces{From: &all}, rn2: &RouteNamespaces{}, description: "Two different RouteNamespaces with nil From on right"},
		{expected: false, rn1: &RouteNamespaces{From: &all}, rn2: &RouteNamespaces{From: &same}, description: "Two different RouteNamespaces with different From"},
		{expected: true, rn1: &RouteNamespaces{From: &all}, rn2: &RouteNamespaces{From: &all}, description: "Two same RouteNamespaces with same From"},
	}

	for _, test := range tests {
		equality := test.rn1.Equal(test.rn2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestAllowedRoutesEquality(t *testing.T) {
	type testAllowedRoutes struct {
		routes1     *AllowedRoutes
		routes2     *AllowedRoutes
		description string
		expected    bool
	}
	all := "All"
	same := "Same"
	tests := []testAllowedRoutes{
		{expected: true, description: "Two nil pointers"},
		{expected: false, routes2: &AllowedRoutes{}, description: "Left nil pointer"},
		{expected: false, routes1: &AllowedRoutes{}, description: "Right nil pointer"},
		{expected: true, routes1: &AllowedRoutes{}, routes2: &AllowedRoutes{}, description: "Two empty AllowedRoutes"},
		{expected: true, routes1: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}, routes2: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}, description: "Two same AllowedRoutes"},
		{expected: false, routes1: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}, routes2: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &same}}, description: "Two different and not empty AllowedRoutes"},
	}

	for _, test := range tests {
		equality := test.routes1.Equal(test.routes2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestListenerEquality(t *testing.T) {
	type testListener struct {
		listener1   *Listener
		listener2   *Listener
		description string
		expected    bool
	}
	all := "All"
	hostname := "hostname"
	tests := []testListener{
		{expected: true, description: "Two nil pointers"},
		{expected: false, listener2: &Listener{}, description: "Left nil pointer"},
		{expected: false, listener1: &Listener{}, description: "Right nil pointer"},
		{expected: true, listener1: &Listener{}, listener2: &Listener{}, description: "Two empty Listener"},
		{
			expected:    true,
			listener1:   &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2:   &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			description: "Two same Listener",
		},
		{
			expected:    false,
			listener1:   &Listener{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2:   &Listener{Name: "listener2", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			description: "Two different Listener by Name",
		},
		{
			expected:    false,
			listener1:   &Listener{Name: "listener", Hostname: &hostname, Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2:   &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			description: "Two different Listener by Hostname",
		},
		{
			expected:    false,
			listener1:   &Listener{Name: "listener", Port: 1096, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2:   &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			description: "Two different Listener by Port",
		},
		{
			expected:    false,
			listener1:   &Listener{Name: "listener", Port: 1048, Protocol: "", AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2:   &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			description: "Two different Listener by Protocol",
		},
		{
			expected:  false,
			listener1: &Listener{Name: "listener", Port: 1048, Protocol: "", AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			listener2: &Listener{Name: "listener", Port: 1048, Protocol: TCPProtocolType}, description: "Two different Listener by AllowedRoutes",
		},
	}

	for _, test := range tests {
		equality := test.listener1.Equal(test.listener2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestGatewayEquality(t *testing.T) {
	type testListener struct {
		gw1         *Gateway
		gw2         *Gateway
		description string
		expected    bool
	}
	ns := "ns"
	ns2 := "ns2"
	all := "All"
	tests := []testListener{
		{expected: true, description: "Two nil pointers"},
		{expected: false, gw2: &Gateway{}, description: "Left nil pointer"},
		{expected: false, gw1: &Gateway{}, description: "Right nil pointer"},
		{expected: true, gw1: &Gateway{}, gw2: &Gateway{}, description: "Two empty Gateway"},
		{
			expected:    true,
			gw1:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			description: "Two same Gateway",
		},
		{
			expected:    false,
			gw1:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:         &Gateway{Name: "gw2", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			description: "Two different Gateway by Name",
		},
		{
			expected:    false,
			gw1:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:         &Gateway{Name: "gw", Namespace: ns2, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			description: "Two different Gateway by Namespace",
		},
		{
			expected:    false,
			gw1:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc2", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			description: "Two different Gateway by GatewayClassName",
		},
		{
			expected: false,
			gw1:      &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:      &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{}}, description: "Two different Gateway by Listener (one empty)",
		},
		{
			expected:    false,
			gw1:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2:         &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener2", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			description: "Two different Gateway by Listener contents",
		},
		{
			expected: false,
			gw1:      &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}}}},
			gw2: &Gateway{Name: "gw", Namespace: ns, GatewayClassName: "gwc", Listeners: []Listener{
				{Name: "listener1", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
				{Name: "listener2", Port: 1048, Protocol: TCPProtocolType, AllowedRoutes: &AllowedRoutes{Namespaces: &RouteNamespaces{From: &all}}},
			}}, description: "Two different Gateway by Listener number",
		},
	}

	for _, test := range tests {
		equality := test.gw1.Equal(test.gw2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestBackendRefEquality(t *testing.T) {
	type testResource struct {
		bref1       *BackendRef
		bref2       *BackendRef
		description string
		expected    bool
	}
	var (
		port1 int32 = 8080
		port2 int32 = 8090
		ns          = "ns"
		ns1         = "ns1"
		ns2         = "ns2"
	)
	var weight1 int32 = 1
	var weight2 int32 = 100
	tests := []testResource{
		{expected: true, description: "Two nil pointers"},
		{expected: false, bref2: &BackendRef{}, description: "Left nil pointer"},
		{expected: false, bref1: &BackendRef{}, description: "Right nil pointer"},
		{
			expected:    true,
			bref1:       &BackendRef{},
			bref2:       &BackendRef{},
			description: "Two empty BackendRef",
		},
		{
			expected:    true,
			bref1:       &BackendRef{Name: "bref", Namespace: &ns, Port: &port1, Weight: &weight1},
			bref2:       &BackendRef{Name: "bref", Namespace: &ns, Port: &port1, Weight: &weight1},
			description: "Two same BackendRef",
		},
		{
			expected:    false,
			bref1:       &BackendRef{Name: "bref", Namespace: &ns, Port: &port1, Weight: &weight1},
			bref2:       &BackendRef{Name: "bref2", Namespace: &ns, Port: &port1, Weight: &weight1},
			description: "Two different BackendRef by Name",
		},
		{
			expected:    false,
			bref1:       &BackendRef{Name: "bref", Namespace: &ns1, Port: &port1, Weight: &weight1},
			bref2:       &BackendRef{Name: "bref", Namespace: &ns2, Port: &port1, Weight: &weight1},
			description: "Two different BackendRef by Namespace",
		},
		{
			expected:    false,
			bref1:       &BackendRef{Name: "bref", Namespace: &ns1, Port: &port1, Weight: &weight1},
			bref2:       &BackendRef{Name: "bref", Namespace: &ns1, Port: &port2, Weight: &weight1},
			description: "Two different BackendRef by Port",
		},
		{
			expected:    false,
			bref1:       &BackendRef{Name: "bref", Namespace: &ns1, Port: &port1, Weight: &weight1},
			bref2:       &BackendRef{Name: "bref", Namespace: &ns1, Port: &port1, Weight: &weight2},
			description: "Two different BackendRef by Weight",
		},
	}

	for _, test := range tests {
		equality := test.bref1.Equal(test.bref2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}

func TestTCPRouteEquality(t *testing.T) {
	type testResource struct {
		tcproute1   *TCPRoute
		tcproute2   *TCPRoute
		description string
		expected    bool
	}
	var (
		port1 int32 = 8080
		port2 int32 = 8090
		ns          = "ns"
		ns2         = "ns2"
	)
	var weight1 int32 = 1
	tests := []testResource{
		{expected: true, description: "Two nil pointers"},
		{expected: false, tcproute2: &TCPRoute{}, description: "Left nil pointer"},
		{expected: false, tcproute1: &TCPRoute{}, description: "Right nil pointer"},
		{expected: true, tcproute1: &TCPRoute{}, tcproute2: &TCPRoute{}, description: "Two empty TCPRoute"},
		{
			expected:    true,
			tcproute1:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			tcproute2:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			description: "Two same TCPRoute",
		},
		{
			expected:    false,
			tcproute1:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			tcproute2:   &TCPRoute{Name: "tcp2", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			description: "Two different TCPRoute by Name",
		},
		{
			expected:    false,
			tcproute1:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			tcproute2:   &TCPRoute{Name: "tcp1", Namespace: ns2, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			description: "Two different TCPRoute by Namespace",
		},
		{
			expected:    false,
			tcproute1:   &TCPRoute{Name: "tcp1", Namespace: ns, ParentRefs: []ParentRef{{Name: "tcp1"}}, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			tcproute2:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			description: "Two different TCPRoute by ParentRefs",
		},
		{
			expected:    false,
			tcproute1:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp1", Namespace: &ns, Port: &port1, Weight: &weight1}}},
			tcproute2:   &TCPRoute{Name: "tcp1", Namespace: ns, BackendRefs: []BackendRef{{Name: "tcp2", Namespace: &ns, Port: &port2, Weight: &weight1}}},
			description: "Two different TCPRoute by BackendRefs",
		},
	}

	for _, test := range tests {
		equality := test.tcproute1.Equal(test.tcproute2)
		if equality != test.expected {
			t.Errorf("%s : Expected : %t, got %t", test.description, test.expected, equality)
		}
	}
}
