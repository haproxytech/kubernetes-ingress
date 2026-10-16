package annotations

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestDropUnseen(t *testing.T) {
	tests := []struct {
		name        string
		entries     map[string]*annotationData
		seen        map[string]struct{}
		wantDeleted []string
		wantKept    []string
	}{
		{
			name: "reseen entries keep their lines",
			entries: map[string]*annotationData{
				"example.com/kept": {status: store.EMPTY, value: []string{"### example.com/kept ###", "timeout server 1s"}},
			},
			seen:     map[string]struct{}{"example.com/kept": {}},
			wantKept: []string{"example.com/kept"},
		},
		{
			name: "an emptied value drops its lines",
			entries: map[string]*annotationData{
				"example.com/emptied": {status: store.EMPTY, value: []string{"### example.com/emptied ###", "timeout server 1s"}},
				"example.com/kept":    {status: store.EMPTY, value: []string{"### example.com/kept ###", "timeout client 1s"}},
			},
			seen:        map[string]struct{}{"example.com/kept": {}},
			wantDeleted: []string{"example.com/emptied"},
			wantKept:    []string{"example.com/kept"},
		},
		{
			name: "no annotation left drops everything",
			entries: map[string]*annotationData{
				"example.com/gone": {status: store.EMPTY, value: []string{"### example.com/gone ###", "timeout server 1s"}},
			},
			seen:        map[string]struct{}{},
			wantDeleted: []string{"example.com/gone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dropUnseen(tt.entries, tt.seen)
			for _, origin := range tt.wantDeleted {
				entry := tt.entries[origin]
				assert.Equal(t, store.DELETED, entry.status, "%s should be marked deleted", origin)
				assert.Empty(t, entry.value, "%s should keep no lines", origin)
				assert.Empty(t, entry.updated, "%s should have nothing pending", origin)
			}
			for _, origin := range tt.wantKept {
				entry := tt.entries[origin]
				assert.NotEqual(t, store.DELETED, entry.status, "%s should be kept", origin)
				assert.NotEmpty(t, entry.value, "%s should keep its lines", origin)
			}
		})
	}
}

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
