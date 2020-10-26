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

package client

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Client struct {
	T    *testing.T
	Type string
	Host string
	Port int
}

func New(t *testing.T, host string) *Client {
	return NewClient(t, host, 30080)
}

func NewClient(t *testing.T, host string, port int) *Client {
	return &Client{
		T:    t,
		Type: "http",
		Host: host,
		Port: port,
	}
}

func (c *Client) Do(url string) (*http.Response, func() error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s:%d%s", c.Type, kindURL, c.Port, url), nil)
	assert.Nil(c.T, err)

	req.Host = c.Host

	resp, err := http.DefaultClient.Do(req)
	assert.Nil(c.T, err)

	return resp, resp.Body.Close
}
