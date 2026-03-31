// Package inspector provides an HTTP reverse proxy with request/response
// capture and a terminal UI for inspecting and replaying traffic.
package inspector

import (
	"net/http"
	"time"
)

// CapturedExchange holds a recorded HTTP request/response pair.
type CapturedExchange struct {
	ID        int
	Timestamp time.Time
	IsReplay  bool

	// Request
	Method     string
	Path       string
	Query      string
	Host       string
	ReqHeaders http.Header
	ReqBody    []byte

	// Response
	StatusCode  int
	RespHeaders http.Header
	RespBody    []byte
	Duration    time.Duration
}
