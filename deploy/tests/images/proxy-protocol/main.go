package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/pires/go-proxyproto"
)

func main() {
	// Create a TCP listener on port 8080
	addr := ":8080"
	tcpListener, err := net.Listen("tcp", addr)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error creating TCP listener: %v\n", err)
		os.Exit(1)
	}

	// Wrap it with the PROXY protocol listener
	proxyListener := &proxyproto.Listener{Listener: tcpListener}

	// Ensure the listener is closed on shutdown
	defer proxyListener.Close()

	// HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "hello!")
	})

	// Serve using custom listener
	_, _ = fmt.Printf("Listening on %s with PROXY protocol support...\n", addr)
	if err := http.Serve(proxyListener, nil); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		os.Exit(1)
	}
}
