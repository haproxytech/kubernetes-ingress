package annotations_test

import (
	"reflect"
	"testing"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestSourceIPPersistenceProcess(t *testing.T) {
	tests := []struct {
		name      string
		ann       []map[string]string
		initial   *models.Backend
		wantTable *models.ConfigStickTable
		wantRules models.StickRules
		wantErr   bool
		wantClear bool
	}{
		{
			name: "enabled with defaults",
			ann: []map[string]string{
				{"source-ip-persistence": "true"},
			},
			wantTable: &models.ConfigStickTable{
				Type:   "ip",
				Size:   utils.PtrInt64(1024 * 1024),
				Expire: utils.PtrInt64(30 * 60 * 1000),
				Peers:  "localinstance",
			},
			wantRules: models.StickRules{
				{
					Type:    "on",
					Pattern: "src",
				},
			},
		},
		{
			name: "enabled with custom size and expiry",
			ann: []map[string]string{
				{
					"source-ip-persistence":        "true",
					"source-ip-persistence-size":   "2m",
					"source-ip-persistence-expire": "45s",
				},
			},
			wantTable: &models.ConfigStickTable{
				Type:   "ip",
				Size:   utils.PtrInt64(2 * 1024 * 1024),
				Expire: utils.PtrInt64(45 * 1000),
				Peers:  "localinstance",
			},
			wantRules: models.StickRules{
				{
					Type:    "on",
					Pattern: "src",
				},
			},
		},
		{
			name: "service false overrides configmap true",
			ann: []map[string]string{
				{"source-ip-persistence": "false"},
				{"source-ip-persistence": "true"},
			},
			initial: &models.Backend{
				BackendBase: models.BackendBase{
					StickTable: &models.ConfigStickTable{Type: "ip"},
				},
				StickRuleList: models.StickRules{{Type: "on", Pattern: "src"}},
			},
			wantClear: true,
		},
		{
			name: "unset clears generated persistence",
			ann:  []map[string]string{{}},
			initial: &models.Backend{
				BackendBase: models.BackendBase{
					StickTable: &models.ConfigStickTable{Type: "ip"},
				},
				StickRuleList: models.StickRules{{Type: "on", Pattern: "src"}},
			},
			wantClear: true,
		},
		{
			name: "invalid boolean",
			ann: []map[string]string{
				{"source-ip-persistence": "maybe"},
			},
			wantErr: true,
		},
		{
			name: "invalid size",
			ann: []map[string]string{
				{
					"source-ip-persistence":      "true",
					"source-ip-persistence-size": "large",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid expiry",
			ann: []map[string]string{
				{
					"source-ip-persistence":        "true",
					"source-ip-persistence-expire": "soon",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &models.Backend{}
			if tt.initial != nil {
				backend = tt.initial
			}
			ann := service.NewSourceIPPersistence("source-ip-persistence", backend)

			err := ann.Process(store.K8s{}, tt.ann...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.wantClear {
				if backend.StickTable != nil {
					t.Fatalf("StickTable = %#v, want nil", backend.StickTable)
				}
				if len(backend.StickRuleList) != 0 {
					t.Fatalf("StickRuleList = %#v, want empty", backend.StickRuleList)
				}
				return
			}
			if !reflect.DeepEqual(backend.StickTable, tt.wantTable) {
				t.Fatalf("StickTable = %#v, want %#v", backend.StickTable, tt.wantTable)
			}
			if !reflect.DeepEqual(backend.StickRuleList, tt.wantRules) {
				t.Fatalf("StickRuleList = %#v, want %#v", backend.StickRuleList, tt.wantRules)
			}
		})
	}
}
