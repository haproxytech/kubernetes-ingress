package annotations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestSortedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]*annotationData
		expected []string
	}{
		{
			name:     "no annotations",
			data:     map[string]*annotationData{},
			expected: []string{},
		},
		{
			name: "higher priority first",
			data: map[string]*annotationData{
				"example.com/low":    {orderPriority: 1},
				"example.com/high":   {orderPriority: 10},
				"example.com/middle": {orderPriority: 5},
			},
			expected: []string{"example.com/high", "example.com/middle", "example.com/low"},
		},
		{
			name: "same priority sorted alphabetically",
			data: map[string]*annotationData{
				"example.com/zeta":  {orderPriority: 5},
				"example.com/alpha": {orderPriority: 5},
				"example.com/mid":   {orderPriority: 5},
			},
			expected: []string{"example.com/alpha", "example.com/mid", "example.com/zeta"},
		},
		{
			name: "deleted entries keep their position",
			data: map[string]*annotationData{
				"example.com/kept":    {orderPriority: 10},
				"example.com/deleted": {orderPriority: 5, status: store.DELETED},
			},
			expected: []string{"example.com/kept", "example.com/deleted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sortedOrigins(tt.data))
		})
	}
}
