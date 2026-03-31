package inspector

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func targetPort(ts *httptest.Server) string {
	// ts.URL is like "http://127.0.0.1:PORT"
	parts := strings.Split(ts.URL, ":")
	return parts[len(parts)-1]
}

func TestProxy_CapturesGET(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "hello")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	var captured []CapturedExchange
	var mu sync.Mutex

	proxy, err := NewProxy(targetPort(target), func(ex CapturedExchange) {
		mu.Lock()
		captured = append(captured, ex)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/health", port))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("expected body {\"ok\":true}, got %q", string(body))
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(captured))
	}
	ex := captured[0]
	if ex.Method != "GET" {
		t.Errorf("method: expected GET, got %s", ex.Method)
	}
	if ex.Path != "/api/health" {
		t.Errorf("path: expected /api/health, got %s", ex.Path)
	}
	if ex.StatusCode != 200 {
		t.Errorf("status: expected 200, got %d", ex.StatusCode)
	}
	if string(ex.RespBody) != `{"ok":true}` {
		t.Errorf("resp body: expected {\"ok\":true}, got %q", string(ex.RespBody))
	}
	if ex.RespHeaders.Get("X-Test") != "hello" {
		t.Errorf("resp header X-Test: expected hello, got %q", ex.RespHeaders.Get("X-Test"))
	}
	if ex.IsReplay {
		t.Error("should not be marked as replay")
	}
}

func TestProxy_CapturesPOSTWithBody(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body) // Echo back
	}))
	defer target.Close()

	var captured []CapturedExchange
	var mu sync.Mutex

	proxy, err := NewProxy(targetPort(target), func(ex CapturedExchange) {
		mu.Lock()
		captured = append(captured, ex)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	reqBody := `{"event":"push","ref":"refs/heads/main"}`
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/webhook?secret=abc", port),
		"application/json",
		strings.NewReader(reqBody),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(captured))
	}
	ex := captured[0]
	if ex.Method != "POST" {
		t.Errorf("method: expected POST, got %s", ex.Method)
	}
	if ex.Path != "/webhook" {
		t.Errorf("path: expected /webhook, got %s", ex.Path)
	}
	if ex.Query != "secret=abc" {
		t.Errorf("query: expected secret=abc, got %s", ex.Query)
	}
	if string(ex.ReqBody) != reqBody {
		t.Errorf("req body: expected %q, got %q", reqBody, string(ex.ReqBody))
	}
	if ex.StatusCode != 201 {
		t.Errorf("status: expected 201, got %d", ex.StatusCode)
	}
}

func TestProxy_CapturesUpstreamError(t *testing.T) {
	// Target returns 500
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer target.Close()

	var captured []CapturedExchange
	var mu sync.Mutex

	proxy, err := NewProxy(targetPort(target), func(ex CapturedExchange) {
		mu.Lock()
		captured = append(captured, ex)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/fail", port))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(captured))
	}
	if captured[0].StatusCode != 500 {
		t.Errorf("status: expected 500, got %d", captured[0].StatusCode)
	}
}

func TestProxy_ConnectionRefused(t *testing.T) {
	// Point proxy at a port where nothing is listening
	proxy, err := NewProxy("19999", func(ex CapturedExchange) {})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/down", port))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}

	exchanges := proxy.Exchanges()
	if len(exchanges) != 1 {
		t.Fatalf("expected 1 exchange, got %d", len(exchanges))
	}
	if exchanges[0].StatusCode != 502 {
		t.Errorf("captured status: expected 502, got %d", exchanges[0].StatusCode)
	}
}

func TestProxy_Replay(t *testing.T) {
	var hits int
	var mu sync.Mutex

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	proxy, err := NewProxy(targetPort(target), func(ex CapturedExchange) {})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	// Initial request
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/process", port),
		"text/plain",
		strings.NewReader("payload"),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	time.Sleep(50 * time.Millisecond)

	exchanges := proxy.Exchanges()
	if len(exchanges) != 1 {
		t.Fatalf("expected 1 exchange before replay, got %d", len(exchanges))
	}

	// Replay it
	if err := proxy.Replay(exchanges[0].ID); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	exchanges = proxy.Exchanges()
	if len(exchanges) != 2 {
		t.Fatalf("expected 2 exchanges after replay, got %d", len(exchanges))
	}
	if exchanges[1].IsReplay != true {
		t.Error("replayed exchange should have IsReplay=true")
	}
	if exchanges[1].Method != "POST" {
		t.Errorf("replayed method: expected POST, got %s", exchanges[1].Method)
	}
	if exchanges[1].Path != "/api/process" {
		t.Errorf("replayed path: expected /api/process, got %s", exchanges[1].Path)
	}

	mu.Lock()
	if hits != 2 {
		t.Errorf("target should have been hit 2 times, got %d", hits)
	}
	mu.Unlock()
}

func TestProxy_Clear(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	proxy, err := NewProxy(targetPort(target), func(ex CapturedExchange) {})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer proxy.Close()

	// Send 3 requests
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test", port))
		if err != nil {
			t.Fatalf("GET %d: %v", i, err)
		}
		resp.Body.Close()
	}

	time.Sleep(50 * time.Millisecond)

	if len(proxy.Exchanges()) != 3 {
		t.Fatalf("expected 3 exchanges, got %d", len(proxy.Exchanges()))
	}

	proxy.Clear()

	if len(proxy.Exchanges()) != 0 {
		t.Fatalf("expected 0 exchanges after clear, got %d", len(proxy.Exchanges()))
	}
}

func TestProxy_ReplayNotFound(t *testing.T) {
	proxy, err := NewProxy("1234", func(ex CapturedExchange) {})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}

	if err := proxy.Replay(999); err == nil {
		t.Error("expected error replaying non-existent exchange")
	}
}
