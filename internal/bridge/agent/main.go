package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/yamux"
)

var (
	flagControlPort int
	flagHealthPort  int
	flagPorts       string // comma-separated: "8080:http-api,9090:metrics"
)

func main() {
	flag.IntVar(&flagControlPort, "control-port", 4200, "Yamux control port")
	flag.IntVar(&flagHealthPort, "health-port", 4201, "Health check port")
	flag.StringVar(&flagPorts, "ports", "", "Service ports to mirror: port:name,port:name")
	flag.Parse()

	log.SetPrefix("[devx-bridge-agent] ")
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// Parse self-healing configuration from env
	originalSelector := os.Getenv("DEVX_ORIGINAL_SELECTOR")
	targetService := os.Getenv("DEVX_TARGET_SERVICE")
	targetNamespace := os.Getenv("DEVX_TARGET_NAMESPACE")

	// Parse port specifications
	ports := parsePorts(flagPorts)
	if len(ports) == 0 {
		log.Fatal("no service ports specified — use --ports=8080:name,9090:name")
	}

	log.Printf("starting agent: control-port=%d health-port=%d ports=%v",
		flagControlPort, flagHealthPort, ports)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start health endpoint
	go startHealthServer(flagHealthPort)

	// Start service port listeners (these accept cluster traffic)
	inbound := make(chan net.Conn, 100)
	for _, p := range ports {
		go listenServicePort(ctx, p.Port, inbound)
	}

	// Start Yamux control listener
	controlLn, err := net.Listen("tcp", fmt.Sprintf(":%d", flagControlPort))
	if err != nil {
		log.Fatalf("failed to listen on control port %d: %v", flagControlPort, err)
	}
	defer controlLn.Close()
	log.Printf("listening for devx CLI on :%d", flagControlPort)

	// Handle shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigCh
		log.Printf("received %s — triggering self-healing shutdown", sig)
		restoreSelector(originalSelector, targetService, targetNamespace)
		cancel()
		os.Exit(0)
	}()

	// Accept the CLI connection on the control port
	for {
		conn, err := controlLn.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("control accept error: %v", err)
				continue
			}
		}

		log.Printf("CLI connected from %s", conn.RemoteAddr())
		handleCLISession(ctx, conn, inbound, originalSelector, targetService, targetNamespace)
		log.Println("CLI disconnected — triggering self-healing")
		restoreSelector(originalSelector, targetService, targetNamespace)
		cancel()
		return
	}
}

// handleCLISession manages the Yamux server session with the CLI.
func handleCLISession(ctx context.Context, conn net.Conn, inbound <-chan net.Conn,
	originalSelector, targetService, targetNamespace string) {

	yamuxCfg := yamux.DefaultConfig()
	yamuxCfg.EnableKeepAlive = true
	yamuxCfg.KeepAliveInterval = 10 * time.Second
	yamuxCfg.ConnectionWriteTimeout = 30 * time.Second

	session, err := yamux.Server(conn, yamuxCfg)
	if err != nil {
		log.Printf("failed to create Yamux server session: %v", err)
		return
	}
	defer session.Close()

	log.Println("Yamux session established with CLI")

	for {
		select {
		case <-ctx.Done():
			return
		case clientConn := <-inbound:
			if session.IsClosed() {
				log.Println("Yamux session closed — dropping inbound connection")
				clientConn.Close()
				return
			}
			go proxyToStream(session, clientConn)
		}
	}
}

// proxyToStream opens a new Yamux stream and proxies bytes from a cluster client.
func proxyToStream(session *yamux.Session, clientConn net.Conn) {
	defer clientConn.Close()

	stream, err := session.OpenStream()
	if err != nil {
		log.Printf("failed to open Yamux stream: %v", err)
		return
	}
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(stream, clientConn)
	}()

	go func() {
		defer wg.Done()
		_, _ = io.Copy(clientConn, stream)
	}()

	wg.Wait()
}

// listenServicePort accepts TCP connections on a service port and feeds them to the inbound channel.
func listenServicePort(ctx context.Context, port int, inbound chan<- net.Conn) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Printf("failed to listen on service port %d: %v", port, err)
		return
	}
	defer ln.Close()
	log.Printf("listening on service port :%d", port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		inbound <- conn
	}
}

// startHealthServer runs a minimal HTTP server on the health port.
func startHealthServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	log.Printf("health server on :%d/healthz", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("health server error: %v", err)
	}
}

// restoreSelector uses kubectl to restore the original Service selector.
// This is the agent's self-healing mechanism for crash recovery.
func restoreSelector(selectorJSON, service, namespace string) {
	if selectorJSON == "" || service == "" || namespace == "" {
		log.Println("self-healing skipped: missing selector/service/namespace env vars")
		return
	}

	// Parse and re-encode to ensure valid JSON
	var selector map[string]string
	if err := json.Unmarshal([]byte(selectorJSON), &selector); err != nil {
		log.Printf("self-healing: failed to parse original selector: %v", err)
		return
	}

	selectorPatch, _ := json.Marshal(selector)
	patch := fmt.Sprintf(`{"metadata":{"annotations":{"devx-bridge-session":null}},"spec":{"selector":%s}}`, string(selectorPatch))

	log.Printf("self-healing: restoring selector for %s/%s", namespace, service)

	cmd := exec.Command("kubectl", "patch", "service", service,
		"-n", namespace, "--type=merge", "-p", patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("self-healing failed: %s: %v", string(out), err)
		return
	}
	log.Printf("self-healing successful: selector restored for %s/%s", namespace, service)
}

// portSpec holds a parsed port specification.
type portSpec struct {
	Port int
	Name string
}

// parsePorts parses "8080:http-api,9090:metrics" into port specs.
func parsePorts(raw string) []portSpec {
	if raw == "" {
		return nil
	}

	var specs []portSpec
	for _, item := range strings.Split(raw, ",") {
		parts := strings.SplitN(item, ":", 2)
		port, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		name := ""
		if len(parts) == 2 {
			name = parts[1]
		}
		specs = append(specs, portSpec{Port: port, Name: name})
	}
	return specs
}
