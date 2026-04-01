// Package webhook implements a local HTTP catch-all server for inspecting
// outbound webhook payloads. It records every inbound request, formats the
// body as pretty-printed JSON where possible, and exposes requests over a
// channel so the TUI can live-render them.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Request captures a single inbound request to the catch server.
type Request struct {
	Time      time.Time
	Method    string
	Path      string
	UserAgent string
	Headers   map[string]string
	Body      string   // raw body
	BodyJSON  string   // pretty-printed JSON, empty if not JSON
	Duration  time.Duration
	StatusCode int     // always 200 — we always accept
}

// Server is the webhook catch server.
type Server struct {
	port     int
	requests chan Request
	srv      *http.Server
}

// New creates a new catch server on the given port.
// Received requests are sent to the returned channel.
func New(port int) *Server {
	s := &Server{
		port:     port,
		requests: make(chan Request, 256),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	s.srv = &http.Server{Handler: mux}
	return s
}

// Requests returns the channel of captured requests.
func (s *Server) Requests() <-chan Request {
	return s.requests
}

// Start begins listening on the configured port. It returns the actual bound
// address (useful when port 0 was requested for OS-assigned port).
func (s *Server) Start() (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return "", fmt.Errorf("could not bind port %d: %w", s.port, err)
	}
	addr := ln.Addr().String()
	go func() { _ = s.srv.Serve(ln) }()
	return addr, nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
	close(s.requests)
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Read body (cap at 1 MB to avoid memory issues)
	bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	_ = r.Body.Close()

	// Flatten headers (skip pseudo-headers)
	headers := make(map[string]string, len(r.Header))
	for k, vs := range r.Header {
		headers[k] = strings.Join(vs, ", ")
	}

	// Attempt JSON pretty-print
	bodyStr := string(bodyBytes)
	prettyJSON := ""
	if looksJSON(bodyStr) {
		var buf bytes.Buffer
		if err := json.Indent(&buf, bodyBytes, "", "  "); err == nil {
			prettyJSON = buf.String()
		}
	}

	req := Request{
		Time:       start,
		Method:     r.Method,
		Path:       r.URL.RequestURI(),
		UserAgent:  r.UserAgent(),
		Headers:    headers,
		Body:       bodyStr,
		BodyJSON:   prettyJSON,
		Duration:   time.Since(start),
		StatusCode: http.StatusOK,
	}

	// Always respond 200 OK — we're a sink, not a real service
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"captured"}`))

	// Non-blocking send — drop if the UI falls behind
	select {
	case s.requests <- req:
	default:
	}
}

func looksJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}
