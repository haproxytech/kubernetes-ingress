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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	kindclient "github.com/haproxytech/kubernetes-ingress/deploy/kind/tests/http"
)

type reachResponse struct {
	Service string
	ID      string
	Num     string
}

func Test_Http_Reach(t *testing.T) {
	for name, retries := range map[string]int{
		"hr.haproxy": 8,
		"fr.haproxy": 4,
	} {
		client := kindclient.New(t, name)

		counter := map[string]int{}
		for i := 0; i < retries; i++ {
			func() {
				resp, close := client.Do("/gidc")
				defer close()
				counter[newReachResponse(t, resp).Name()]++
			}()
		}
		for _, v := range counter {
			assert.Equal(t, 2, v)
		}
	}
}

func newReachResponse(t *testing.T, response *http.Response) *reachResponse {
	body, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)

	res := strings.Split(strings.Trim(string(body), "\n"), "-")
	if len(res) == 3 {
		return &reachResponse{
			Service: res[0],
			ID:      res[1],
			Num:     res[2],
		}
	}
	if len(res) == 2 {
		return &reachResponse{
			Service: res[0],
			ID:      res[1],
		}
	}
	t.Fatal("unexpected result", string(body))

	return nil
}

func (r reachResponse) Name() string {
	return fmt.Sprintf("%s-%s", r.Service, r.ID)
}
