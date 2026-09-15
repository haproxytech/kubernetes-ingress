package main

//revive:disable:deep-exit

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/pires/go-proxyproto"
)

// DefaultPort is the default port to use if once is not specified by the SERVER_PORT environment variable
const (
	HTTPPort  = 8888
	HTTPSPort = 8443
	ProxyPort = 8080
	UDPPort   = 8090
)

type context struct {
	params   map[string]string
	hostname string
}

func (c *context) setParams() {
	c.params = make(map[string]string)
	httpPtr := flag.Int("http", HTTPPort, "http port value")
	httpsPtr := flag.Int("https", HTTPSPort, "https port value")
	proxyPtr := flag.Int("proxy", ProxyPort, "http port value requiring PROXY protocol header")
	udpPtr := flag.Int("udp", UDPPort, "udp port value")
	defaultRsp := flag.String("default-response", "all", "what should default response include. Values can be: all, hostname")
	flag.Parse()
	c.params["http"] = strconv.Itoa(*httpPtr)
	c.params["https"] = strconv.Itoa(*httpsPtr)
	c.params["proxy"] = strconv.Itoa(*proxyPtr)
	c.params["udp"] = strconv.Itoa(*udpPtr)
	c.params["response"] = *defaultRsp
}

func listenAndServceTLS(port string) {
	cmd := exec.Command("./generate-cert.sh")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	_, err = os.Stat("server.crt")
	if os.IsNotExist(err) {
		log.Fatal("server.crt: ", err)
	}
	_, err = os.Stat("server.key")
	if os.IsNotExist(err) {
		log.Fatal("server.key: ", err)
	}
	err = http.ListenAndServeTLS(":"+port, "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func listenAndServeProxy(port string) {
	tcpListener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("Echo server (PROXY): ", err)
	}
	// REQUIRE makes a missing PROXY header a hard failure, so tests can't pass by accident
	listener := &proxyproto.Listener{
		Listener: tcpListener,
		ConnPolicy: func(proxyproto.ConnPolicyOptions) (proxyproto.Policy, error) {
			return proxyproto.REQUIRE, nil
		},
	}
	defer listener.Close()
	err = http.Serve(listener, nil)
	if err != nil {
		log.Fatal("Echo server (PROXY): ", err)
	}
}

// listenAndServeUDP answers every datagram with a JSON echo of its origin and payload
func (c context) listenAndServeUDP(port string) {
	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		log.Fatal("Echo server (UDP): ", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal("Echo server (UDP): ", err)
	}
	defer conn.Close()
	buf := make([]byte, 65535)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Echo server (UDP): read: %s", err)
			continue
		}
		log.Println("Echoing back UDP datagram to client (" + client.String() + ")")
		attr := map[string]interface{}{
			"os": map[string]string{
				"hostname": c.hostname,
			},
			"udp": map[string]string{
				"ip":   client.IP.String(),
				"port": fmt.Sprint(client.Port),
				"data": string(buf[:n]),
			},
		}
		res, _ := json.MarshalIndent(attr, "", "  ")
		if _, err := conn.WriteToUDP(append(res, '\n'), client); err != nil {
			log.Printf("Echo server (UDP): write: %s", err)
		}
	}
}

func main() {
	var err error
	ctx := context{}
	ctx.hostname, err = os.Hostname()
	if err != nil {
		log.Println(err)
	}
	ctx.setParams()
	http.HandleFunc("/hostname", ctx.echoHostname)
	http.HandleFunc("/all", ctx.echoAll)
	switch ctx.params["response"] {
	case "hostname":
		http.HandleFunc("/", ctx.echoHostname)
	default:
		http.HandleFunc("/", ctx.echoAll)
	}
	log.Printf("starting echo server, listening on ports HTTP:%s/HTTPS:%s/PROXY:%s/UDP:%s", ctx.params["http"], ctx.params["https"], ctx.params["proxy"], ctx.params["udp"])
	// HTTPS
	go func() {
		listenAndServceTLS(ctx.params["https"])
	}()
	// HTTP behind PROXY protocol
	go func() {
		listenAndServeProxy(ctx.params["proxy"])
	}()
	// UDP
	go func() {
		ctx.listenAndServeUDP(ctx.params["udp"])
	}()
	// HTTP
	err = http.ListenAndServe(":"+ctx.params["http"], nil)
	if err != nil {
		log.Fatal("Echo server (HTTP): ", err)
	}
}
