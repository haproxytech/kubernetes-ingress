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
	"io/ioutil"
	"net/http"
	"strings"
)

type Client struct {
	Type string
	Host string
	Port int
}

type Response struct {
	Service string
	ID      string
	Num     string
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

func (r Response) Name() string {
	return fmt.Sprintf("%s-%s", r.Service, r.ID)
}

func (c *Client) Do() Response {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://127.0.0.1:%d/gidc", c.Type, c.Port), nil)
	checkErr(err)
	req.Host = c.Host

	resp, err := http.DefaultClient.Do(req)
	checkErr(err)
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	checkErr(err)

	response := strings.Split(strings.Trim(string(body), "\n"), "-")
	if len(response) == 3 {
		return Response{
			Service: response[0],
			ID:      response[1],
			Num:     response[2],
		}
	}
	if len(response) == 2 {
		return Response{
			Service: response[0],
			ID:      response[1],
		}
	}
	failAndExit("unexpected result [%s]", string(body))
	return Response{}
}
