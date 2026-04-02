package inspector

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

const maxBodyCapture = 256 * 1024 // 256 KB

// replayHeader is an internal header used to tag replayed requests.
const replayHeader = "X-Devx-Replay"

// Proxy is a reverse proxy that captures HTTP request/response exchanges.
type Proxy struct {
	target   *url.URL
	rp       *httputil.ReverseProxy
	listener net.Listener

	mu        sync.RWMutex
	exchanges []*CapturedExchange
	counter   int

	onCapture func(CapturedExchange)
}

// NewProxy creates a new inspector proxy that forwards traffic to targetPort.
// onCapture is called asynchronously for each captured exchange.
func NewProxy(targetPort string, onCapture func(CapturedExchange)) (*Proxy, error) {
	target, err := url.Parse(fmt.Sprintf("http://localhost:%s", targetPort))
	if err != nil {
		return nil, fmt.Errorf("invalid target port: %w", err)
	}

	p := &Proxy{
		target:    target,
		onCapture: onCapture,
	}

	p.rp = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, "devx proxy: %v", err)
		},
	}

	return p, nil
}

// ListenPort returns the port the proxy is listening on, or 0 if not started.
func (p *Proxy) ListenPort() int {
	if p.listener == nil {
		return 0
	}
	return p.listener.Addr().(*net.TCPAddr).Port
}

// Start begins listening on a random available port and serving in the background.
func (p *Proxy) Start() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to listen: %w", err)
	}
	p.listener = ln
	go func() { _ = http.Serve(ln, p) }()
	return p.ListenPort(), nil
}

// Close shuts down the proxy listener.
func (p *Proxy) Close() error {
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

// ServeHTTP captures the request, proxies it, then captures the response.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	isReplay := r.Header.Get(replayHeader) == "true"
	r.Header.Del(replayHeader)

	// Capture request body
	var reqBody []byte
	if r.Body != nil {
		reqBody, _ = io.ReadAll(io.LimitReader(r.Body, maxBodyCapture))
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
	}

	// Tee the response so we can capture status + body while still streaming
	tw := &teeWriter{
		ResponseWriter: w,
		statusCode:     200,
		body:           &bytes.Buffer{},
	}

	p.rp.ServeHTTP(tw, r)

	duration := time.Since(start)

	p.mu.Lock()
	p.counter++
	ex := CapturedExchange{
		ID:          p.counter,
		Timestamp:   start,
		IsReplay:    isReplay,
		Method:      r.Method,
		Path:        r.URL.Path,
		Query:       r.URL.RawQuery,
		Host:        r.Host,
		ReqHeaders:  r.Header.Clone(),
		ReqBody:     reqBody,
		StatusCode:  tw.statusCode,
		RespHeaders: tw.Header().Clone(),
		RespBody:    tw.body.Bytes(),
		Duration:    duration,
	}
	p.exchanges = append(p.exchanges, &ex)
	p.mu.Unlock()

	if p.onCapture != nil {
		p.onCapture(ex)
	}
}

// Exchanges returns a snapshot of all captured exchanges.
func (p *Proxy) Exchanges() []CapturedExchange {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]CapturedExchange, len(p.exchanges))
	for i, e := range p.exchanges {
		result[i] = *e
	}
	return result
}

// Clear removes all captured exchanges and resets the counter.
func (p *Proxy) Clear() {
	p.mu.Lock()
	p.exchanges = nil
	p.counter = 0
	p.mu.Unlock()
}

// Replay resends a previously captured request through the proxy.
// The replayed exchange is captured as a new entry with IsReplay=true.
func (p *Proxy) Replay(id int) error {
	p.mu.RLock()
	var found *CapturedExchange
	for _, e := range p.exchanges {
		if e.ID == id {
			found = e
			break
		}
	}
	p.mu.RUnlock()

	if found == nil {
		return fmt.Errorf("exchange #%d not found", id)
	}

	proxyURL := fmt.Sprintf("http://localhost:%d%s", p.ListenPort(), found.Path)
	if found.Query != "" {
		proxyURL += "?" + found.Query
	}

	req, err := http.NewRequest(found.Method, proxyURL, bytes.NewReader(found.ReqBody))
	if err != nil {
		return err
	}
	for k, vs := range found.ReqHeaders {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set(replayHeader, "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return nil
}

// teeWriter wraps http.ResponseWriter to capture status code and body
// while still writing the response to the client.
type teeWriter struct {
	http.ResponseWriter
	statusCode  int
	body        *bytes.Buffer
	wroteHeader bool
}

func (t *teeWriter) WriteHeader(code int) {
	if !t.wroteHeader {
		t.statusCode = code
		t.wroteHeader = true
		t.ResponseWriter.WriteHeader(code)
	}
}

func (t *teeWriter) Write(b []byte) (int, error) {
	if !t.wroteHeader {
		t.wroteHeader = true
	}
	if t.body.Len() < maxBodyCapture {
		t.body.Write(b)
	}
	return t.ResponseWriter.Write(b)
}

// Flush implements http.Flusher for streaming responses (SSE, etc).
func (t *teeWriter) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
