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

package e2e

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
)

type Client struct {
	Host      string
	Port      int
	Req       *http.Request
	Transport *http.Transport
}

const HTTP_PORT = 30080
const HTTPS_PORT = 30443

func newClient(port int, tls bool) (*Client, error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	dstPort := HTTP_PORT
	scheme := "http"
	if tls {
		scheme = "https"
		dstPort = HTTPS_PORT
	}
	if port != 0 {
		dstPort = port
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s:%d%s", scheme, kindURL, dstPort, ""), nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		Port: dstPort,
		Req:  req,
	}, nil
}

func NewHTTPClient(host string, port ...int) (*Client, error) {
	var dstPort int
	if len(port) > 0 {
		dstPort = port[0]
	}
	client, err := newClient(dstPort, false)
	if err != nil {
		return nil, err
	}
	client.Host = host
	return client, nil
}

func NewHTTPSClient(host string, port ...int) (*Client, error) {
	var dstPort int
	if len(port) > 0 {
		dstPort = port[0]
	}
	client, err := newClient(dstPort, true)
	if err != nil {
		return nil, err
	}
	client.Host = host
	client.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	return client, nil
}

func (c *Client) Do() (res *http.Response, close func() error, err error) {
	c.Req.Host = c.Host
	if c.Transport != nil {
		res, err = (&http.Client{Transport: c.Transport}).Do(c.Req)
	} else {
		res, err = http.DefaultClient.Do(c.Req)
	}
	if err != nil {
		return
	}
	close = res.Body.Close
	return
}
