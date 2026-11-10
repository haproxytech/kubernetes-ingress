package store

import (
	"reflect"
	"testing"
)

func TestSortedRuntimeEndpoints(t *testing.T) {
	endpoints := RuntimeEndpoints{
		{Address: "10.244.0.10", Port: 8080}:    {},
		{Address: "10.244.0.2", Port: 8443}:     {},
		{Address: "10.244.0.2", Port: 8080}:     {},
		{Address: "example.internal", Port: 80}: {},
	}

	got := SortedRuntimeEndpoints(endpoints)
	want := []RuntimeEndpoint{
		{Address: "10.244.0.2", Port: 8080},
		{Address: "10.244.0.2", Port: 8443},
		{Address: "10.244.0.10", Port: 8080},
		{Address: "example.internal", Port: 80},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SortedRuntimeEndpoints() = %#v, want %#v", got, want)
	}
}
