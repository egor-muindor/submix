// Command extras-ui is a local extras.yaml editor with a rule preview.
// Listens on localhost only; not part of the Docker image, never deployed to production.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/egor-muindor/submix/internal/extrasui"
)

func main() {
	configPath := flag.String("config", "deploy/extras.yaml", "path to extras.yaml")
	listen := flag.String("listen", "127.0.0.1:3040", "address to listen on")
	flag.Parse()

	// The client timeout caps a single fetch of an external subscription.
	client := &http.Client{Timeout: 30 * time.Second}
	server := extrasui.NewServer(*configPath, client)

	srv := &http.Server{
		Addr:              *listen,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extras-ui:", err)
		os.Exit(1)
	}
	fmt.Printf("extras-ui: http://%s  (config: %s)\n", ln.Addr(), *configPath)
	if err := srv.Serve(ln); err != nil {
		fmt.Fprintln(os.Stderr, "extras-ui:", err)
		os.Exit(1)
	}
}
