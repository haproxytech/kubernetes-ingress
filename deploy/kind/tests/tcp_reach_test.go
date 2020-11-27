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

// +build integration

package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/client"
)

func Test_TCP_Reach(t *testing.T) {
	client := kindclient.NewClient("haproxy.org", 32766)

	counter := map[string]int{}
	for i := 0; i < 4; i++ {
		func() {
			res, cls, err := client.Do("/gidc")
			if err != nil {
				return
			}
			defer cls()
			counter[newReachResponse(t, res).Name()]++
		}()
	}
	for _, v := range counter {
		assert.Equal(t, 4, v)
	}
}
