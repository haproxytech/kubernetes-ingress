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
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	proxyproto "github.com/pires/go-proxyproto"
)

type Client struct {
	Req        *http.Request
	Transport  *http.Transport
	Path       string
	Host       string
	Port       int
	NoRedirect bool
}

type GlobalHAProxyInfo struct {
	Pid     string
	Maxconn string
	Uptime  string
}

//revive:disable:var-naming
const (
	HTTP_PORT  = 30080
	HTTPS_PORT = 30443
	STATS_PORT = 31024
)

//revive:enable:var-naming

//revive:disable-next-line:import-shadowing,flag-parameter
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
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://%s", scheme, host), nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		Host: host,
		Port: dstPort,
		Req:  req,
		Transport: &http.Transport{
			// Tests reconfigure HAProxy between requests; a reused connection would hide that.
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, network, _ string) (conn net.Conn, e error) {
				dialer := &net.Dialer{}
				return dialer.DialContext(ctx, network, net.JoinHostPort(kindURL, strconv.Itoa(dstPort)))
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
		//nolint:gosec // skipping TLS verify for testing purpose
		InsecureSkipVerify: true,
	}
	return client, nil
}

func (c *Client) DoMethod(method string) (res *http.Response, closeFunc func() error, err error) {
	client := &http.Client{}
	if c.Transport != nil {
		client.Transport = c.Transport
	}
	if c.NoRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	c.Req.Host = c.Host
	c.Req.Header["Origin"] = []string{c.Req.URL.Scheme + "://" + c.Host}
	c.Req.URL.Host = c.Host
	c.Req.URL.Path = c.Path
	c.Req.Method = method
	res, err = client.Do(c.Req)
	if err != nil {
		return res, closeFunc, err
	}
	closeFunc = res.Body.Close
	return res, closeFunc, err
}

func (c *Client) Do() (res *http.Response, closeFunc func() error, err error) {
	return c.DoMethod("GET")
}

func (c *Client) DoOptions() (res *http.Response, closeFunc func() error, err error) {
	return c.DoMethod("OPTIONS")
}

func ProxyProtoConn() (result []byte, err error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	dstPort := HTTP_PORT

	target, errAddr := net.ResolveTCPAddr("tcp", net.JoinHostPort(kindURL, strconv.Itoa(dstPort)))
	if errAddr != nil {
		return nil, errAddr
	}

	conn, errConn := net.DialTCP("tcp", nil, target)
	if err != nil {
		return nil, errConn
	}
	defer conn.Close()

	// Create a proxyprotocol header
	header := &proxyproto.Header{
		Version:           1,
		Command:           proxyproto.PROXY,
		TransportProtocol: proxyproto.TCPv4,
		SourceAddr: &net.TCPAddr{
			IP:   net.ParseIP("10.1.1.1"),
			Port: 1000,
		},
		DestinationAddr: &net.TCPAddr{
			IP:   net.ParseIP("20.2.2.2"),
			Port: 2000,
		},
	}

	_, err = header.WriteTo(conn)
	if err != nil {
		return result, err
	}

	_, err = conn.Write([]byte("HEAD / HTTP/1.0\r\n\r\n"))
	if err != nil {
		return result, err
	}

	return io.ReadAll(conn)
}

func runtimeCommand(command string) ([]byte, error) {
	kindURL := os.Getenv("KIND_URL")
	if kindURL == "" {
		kindURL = "127.0.0.1"
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(kindURL, strconv.Itoa(STATS_PORT)))
	if err != nil {
		return nil, err
	}
	_, err = conn.Write([]byte(command + "\n"))
	if err != nil {
		return nil, err
	}

	// 1024 was not enough! Need to read what needs to be read
	bufferSize := 1024
	buf := make([]byte, bufferSize)
	var data strings.Builder
	for {
		n, readErr := conn.Read(buf)
		if readErr != nil {
			break
		}
		_, _ = data.Write(buf[0:n])
	}

	result := strings.TrimSuffix(data.String(), "\n> ")
	result = strings.TrimSuffix(result, "\n")
	result = strings.TrimSpace(result)
	err = conn.Close()
	return []byte(result), err
}

func GetHAProxyMapCount(mapName string) (count int, err error) {
	var result []byte
	result, err = runtimeCommand("show map")
	if err != nil {
		return count, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(result))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, mapName+".map") {
			r := regexp.MustCompile("entry_cnt=[0-9]*")
			match := r.FindString(line)
			nbr := strings.Split(match, "=")[1]
			count, err = strconv.Atoi(nbr)
			break
		}
	}
	return count, err
}

func GetRuntimeUpServersCount(backend string) (count int, err error) {
	var result []byte
	command := fmt.Sprintf("show stat %s 4 -1 up", backend)
	result, err = runtimeCommand(command)
	if err != nil {
		return count, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(result))
	for scanner.Scan() {
		line := scanner.Text()
		// Split string by the separator
		parts := strings.Split(line, ",")

		// Get everything before the last separator
		l := strings.Join(parts[:len(parts)-1], ",")
		lineFiltered := strings.ToLower(l)
		if strings.Contains(lineFiltered, backend) {
			count++
		}
	}
	return count, err
}

func GetGlobalHAProxyInfo() (info GlobalHAProxyInfo, err error) {
	var result []byte
	result, err = runtimeCommand("show info")
	if err != nil {
		return info, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(result))
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
	return info, err
}

type CertInfo struct {
	Used    bool
	Subject string
	Issuer  string
}

func GetCertSubject(filename string) (certInfo CertInfo, err error) {
	var result []byte
	result, err = runtimeCommand("show ssl cert " + filename)
	if err != nil {
		return certInfo, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(result))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "Used:"):
			used, convErr := strconv.ParseBool(strings.Split(line, ": ")[1])
			if convErr != nil {
				return certInfo, err //nolint:nilnesserr
			}
			certInfo.Used = used
		case strings.HasPrefix(line, "Subject:"):
			certInfo.Subject = strings.Split(line, ": ")[1]
		case strings.HasPrefix(line, "Issuer:"):
			certInfo.Issuer = strings.Split(line, ": ")[1]
		}
	}
	return certInfo, err
}
