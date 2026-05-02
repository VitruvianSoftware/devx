// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
