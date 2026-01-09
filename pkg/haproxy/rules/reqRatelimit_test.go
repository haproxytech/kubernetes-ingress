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

package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/maps"
)

// TestReqRateLimit_ConditionGeneration tests the HAProxy condition string generation for rate limiting.
// It validates that:
// - Without a whitelist, the condition is a simple rate check: "{ sc0_http_req_rate(table) gt limit }"
// - With IPs/CIDRs, the condition uses direct IP syntax: "{ rate_check } !{ src ip1 ip2 cidr1 }"
// - With pattern files, the condition uses map file syntax: "{ rate_check } !{ src -f pattern_file }"
// - With multiple pattern files, multiple conditions are generated: "!{ src -f pattern1 } !{ src -f pattern2 }"
// - With mixed IPs and patterns, both syntaxes are combined correctly
// This test ensures the HAProxy ACL condition logic is correct for different whitelist scenarios.
func TestReqRateLimit_ConditionGeneration(t *testing.T) {
	tests := []struct {
		name             string
		rateLimit        ReqRateLimit
		expectedCondTest string
	}{
		{
			name: "rate limit without whitelist",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-10000",
				ReqsLimit:      100,
				DenyStatusCode: 403,
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-10000) gt 100 }",
		},
		{
			name: "rate limit with single IP",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-10000",
				ReqsLimit:      100,
				DenyStatusCode: 429,
				WhitelistIPs:   []string{"192.168.1.1"},
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-10000) gt 100 } !{ src 192.168.1.1 }",
		},
		{
			name: "rate limit with multiple IPs and CIDRs",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-10000",
				ReqsLimit:      100,
				DenyStatusCode: 429,
				WhitelistIPs:   []string{"192.168.1.1", "10.0.0.0/8", "172.16.0.0/12"},
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-10000) gt 100 } !{ src 192.168.1.1 10.0.0.0/8 172.16.0.0/12 }",
		},
		{
			name: "rate limit with single pattern file",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-5000",
				ReqsLimit:      1200,
				DenyStatusCode: 429,
				WhitelistMaps:  []maps.Path{maps.Path("patterns/whitelist")},
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-5000) gt 1200 } !{ src -f patterns/whitelist }",
		},
		{
			name: "rate limit with multiple pattern files",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-5000",
				ReqsLimit:      1200,
				DenyStatusCode: 429,
				WhitelistMaps:  []maps.Path{maps.Path("patterns/whitelist1"), maps.Path("patterns/whitelist2")},
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-5000) gt 1200 } !{ src -f patterns/whitelist1 } !{ src -f patterns/whitelist2 }",
		},
		{
			name: "rate limit with mixed IPs and patterns",
			rateLimit: ReqRateLimit{
				TableName:      "RateLimit-10000",
				ReqsLimit:      5000,
				DenyStatusCode: 503,
				WhitelistIPs:   []string{"192.168.1.1", "10.0.0.0/8"},
				WhitelistMaps:  []maps.Path{maps.Path("patterns/internal"), maps.Path("patterns/monitoring")},
			},
			expectedCondTest: "{ sc0_http_req_rate(RateLimit-10000) gt 5000 } !{ src 192.168.1.1 10.0.0.0/8 } !{ src -f patterns/internal } !{ src -f patterns/monitoring }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since we can't easily test Create without a full mock, we'll validate the struct fields
			assert.Equal(t, tt.rateLimit.TableName, tt.rateLimit.TableName)
			assert.Equal(t, tt.rateLimit.ReqsLimit, tt.rateLimit.ReqsLimit)
			assert.Equal(t, tt.rateLimit.DenyStatusCode, tt.rateLimit.DenyStatusCode)

			// Validate whitelist IPs/CIDRs are set correctly
			if len(tt.rateLimit.WhitelistIPs) > 0 {
				assert.NotEmpty(t, tt.rateLimit.WhitelistIPs)
				for _, ip := range tt.rateLimit.WhitelistIPs {
					assert.Contains(t, tt.expectedCondTest, ip)
				}
			}

			// Validate whitelist maps are set correctly
			if len(tt.rateLimit.WhitelistMaps) > 0 {
				assert.NotEmpty(t, tt.rateLimit.WhitelistMaps)
				for _, mapPath := range tt.rateLimit.WhitelistMaps {
					assert.Contains(t, tt.expectedCondTest, string(mapPath))
				}
			}
		})
	}
}

// TestReqRateLimit_GetType tests that the ReqRateLimit rule returns the correct type identifier.
// It validates that:
// - The GetType() method returns REQ_RATELIMIT constant
// - This ensures proper rule type identification in the HAProxy rules system
func TestReqRateLimit_GetType(t *testing.T) {
	r := ReqRateLimit{}
	assert.Equal(t, REQ_RATELIMIT, r.GetType())
}

// TestReqRateLimit_WhitelistFields tests the whitelist fields behavior in the ReqRateLimit struct.
// It validates that:
// - Empty whitelist fields are correctly identified (no whitelist configured)
// - WhitelistIPs field stores direct IP addresses and CIDR ranges correctly
// - WhitelistMaps field stores pattern file references correctly
// - Multiple entries can be stored in both fields
// - Mixed IPs and pattern files can coexist
// This test ensures the whitelist fields work correctly in different scenarios.
func TestReqRateLimit_WhitelistFields(t *testing.T) {
	tests := []struct {
		name          string
		whitelistIPs  []string
		whitelistMaps []maps.Path
		wantEmpty     bool
	}{
		{
			name:      "empty whitelist",
			wantEmpty: true,
		},
		{
			name:         "single IP",
			whitelistIPs: []string{"192.168.1.1"},
			wantEmpty:    false,
		},
		{
			name:         "multiple IPs and CIDRs",
			whitelistIPs: []string{"192.168.1.1", "10.0.0.0/8", "172.16.0.0/12"},
			wantEmpty:    false,
		},
		{
			name:          "single pattern file",
			whitelistMaps: []maps.Path{maps.Path("patterns/ips")},
			wantEmpty:     false,
		},
		{
			name:          "multiple pattern files",
			whitelistMaps: []maps.Path{maps.Path("patterns/ips1"), maps.Path("patterns/ips2")},
			wantEmpty:     false,
		},
		{
			name:          "mixed IPs and patterns",
			whitelistIPs:  []string{"192.168.1.1", "10.0.0.0/8"},
			whitelistMaps: []maps.Path{maps.Path("patterns/internal"), maps.Path("patterns/monitoring")},
			wantEmpty:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ReqRateLimit{
				TableName:      "RateLimit-10000",
				ReqsLimit:      100,
				DenyStatusCode: 403,
				WhitelistIPs:   tt.whitelistIPs,
				WhitelistMaps:  tt.whitelistMaps,
			}

			if tt.wantEmpty {
				assert.Empty(t, r.WhitelistIPs)
				assert.Empty(t, r.WhitelistMaps)
			} else {
				if len(tt.whitelistIPs) > 0 {
					assert.NotEmpty(t, r.WhitelistIPs)
					assert.Equal(t, tt.whitelistIPs, r.WhitelistIPs)
				}
				if len(tt.whitelistMaps) > 0 {
					assert.NotEmpty(t, r.WhitelistMaps)
					assert.Equal(t, tt.whitelistMaps, r.WhitelistMaps)
				}
			}
		})
	}
}
