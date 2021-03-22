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
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
)

type Client struct {
	NoRedirect bool
	Path       string
	Host       string
	Port       int
	Req        *http.Request
	Transport  *http.Transport
}

type GlobalHAProxyInfo struct {
	Pid     string
	Maxconn string
	Uptime  string
}

const HTTP_PORT = 30080
const HTTPS_PORT = 30443
const STATS_PORT = 31024

func newClient(host string, port int, tls bool) (*Client, error) {
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
	req, err := http.NewRequest("GET", fmt.Sprintf("%s://%s", scheme, host), nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		Host: host,
		Port: dstPort,
		Req:  req,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
				dialer := &net.Dialer{}
				addr = fmt.Sprintf("%s:%d", kindURL, dstPort)
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}, nil
}

func NewHTTPClient(host string, port ...int) (*Client, error) {
	var dstPort int
	if len(port) > 0 {
		dstPort = port[0]
	}
	client, err := newClient(host, dstPort, false)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewHTTPSClient(host string, port ...int) (*Client, error) {
	var dstPort int
	if len(port) > 0 {
		dstPort = port[0]
	}
	client, err := newClient(host, dstPort, true)
	if err != nil {
		return nil, err
	}
	client.Transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	return client, nil
}

func (c *Client) Do() (res *http.Response, close func() error, err error) {
	client := &http.Client{}
	if c.Transport != nil {
		client.Transport = c.Transport
	}
	if c.NoRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	c.Req.Host = c.Host
	c.Req.URL.Host = c.Host
	c.Req.URL.Path = c.Path
	res, err = client.Do(c.Req)
	if err != nil {
		return
	}
	close = res.Body.Close
	return
}

func GetGlobalHAProxyInfo() (info GlobalHAProxyInfo, err error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", kindURL, STATS_PORT))
	if err != nil {
		return info, err
	}
	_, err = conn.Write([]byte("show info\n"))
	if err != nil {
		return info, err
	}
	reply := make([]byte, 1024)
	_, err = conn.Read(reply)
	if err != nil {
		return info, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(reply))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "Maxconn:"):
			info.Maxconn = strings.Split(line, ": ")[1]
		case strings.HasPrefix(line, "Uptime:"):
			info.Uptime = strings.Split(line, ": ")[1]
		case strings.HasPrefix(line, "Pid:"):
			info.Pid = strings.Split(line, ": ")[1]
		}
	}
	conn.Close()
	return info, nil
}
