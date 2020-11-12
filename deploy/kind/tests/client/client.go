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
)

type Client struct {
	Type string
	Host string
	Port int
}

func New(host string) *Client {
	return NewClient(host, 30080)
}

func NewClient(host string, port int) *Client {
	return &Client{
		Type: "http",
		Host: host,
		Port: port,
	}
}

func (c *Client) Do(url string) (res *http.Response, close func() error, err error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	var req *http.Request
	req, err = http.NewRequest("GET", fmt.Sprintf("%s://%s:%d%s", c.Type, kindURL, c.Port, url), nil)
	if err != nil {
		return
	}
	req.Host = c.Host

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	close = res.Body.Close

	return
}
