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

package inspector

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestE2E_FullInspectorFlow validates the complete inspector lifecycle:
// proxy startup → request capture → replay → clear.
func TestE2E_FullInspectorFlow(t *testing.T) {
	// Spin up a fake "developer's app"
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/health":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/webhook":
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = fmt.Fprintf(w, `{"received":%d}`, len(body))
		default:
			w.WriteHeader(404)
		}
	}))
	defer func() { app.Close() }() 

	// Start inspector proxy pointing at the app
	ch := make(chan CapturedExchange, 32)
	proxy, err := NewProxy(targetPort(app), func(ex CapturedExchange) {
		ch <- ex
	})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	port, err := proxy.Start()
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = proxy.Close() }() 

	proxyBase := fmt.Sprintf("http://localhost:%d", port)

	// ── Step 1: GET /api/health ─────────────────────────────
	t.Log("Step 1: GET /api/health")
	resp, err := http.Get(proxyBase + "/api/health")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != `{"status":"ok"}` {
		t.Fatalf("unexpected body: %s", string(body))
	}

	ex := <-ch
	if ex.Method != "GET" || ex.Path != "/api/health" || ex.StatusCode != 200 {
		t.Fatalf("capture mismatch: %s %s → %d", ex.Method, ex.Path, ex.StatusCode)
	}
	t.Logf("  ✓ captured: %s %s → %d (%s)", ex.Method, ex.Path, ex.StatusCode, ex.Duration)

	// ── Step 2: POST /webhook ───────────────────────────────
	t.Log("Step 2: POST /webhook with JSON body")
	webhookBody := `{"event":"push","ref":"refs/heads/main"}`
	resp, err = http.Post(proxyBase+"/webhook", "application/json", strings.NewReader(webhookBody))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	ex = <-ch
	if ex.Method != "POST" || ex.Path != "/webhook" || ex.StatusCode != 201 {
		t.Fatalf("capture mismatch: %s %s → %d", ex.Method, ex.Path, ex.StatusCode)
	}
	if string(ex.ReqBody) != webhookBody {
		t.Fatalf("req body mismatch: %q", string(ex.ReqBody))
	}
	t.Logf("  ✓ captured: %s %s → %d, reqBody=%d bytes", ex.Method, ex.Path, ex.StatusCode, len(ex.ReqBody))

	// ── Step 3: GET /unknown → 404 ─────────────────────────
	t.Log("Step 3: GET /unknown (expect 404)")
	resp, err = http.Get(proxyBase + "/unknown")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()
	ex = <-ch
	if ex.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", ex.StatusCode)
	}
	t.Logf("  ✓ captured: %s %s → %d", ex.Method, ex.Path, ex.StatusCode)

	// ── Step 4: Replay the webhook ──────────────────────────
	t.Log("Step 4: Replay POST /webhook")
	exchanges := proxy.Exchanges()
	if len(exchanges) != 3 {
		t.Fatalf("expected 3 exchanges, got %d", len(exchanges))
	}

	// Find the POST /webhook exchange
	var webhookEx CapturedExchange
	for _, e := range exchanges {
		if e.Path == "/webhook" {
			webhookEx = e
			break
		}
	}

	err = proxy.Replay(webhookEx.ID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	ex = <-ch
	if !ex.IsReplay {
		t.Fatal("replayed exchange should have IsReplay=true")
	}
	if ex.Method != "POST" || ex.Path != "/webhook" {
		t.Fatalf("replay mismatch: %s %s", ex.Method, ex.Path)
	}
	t.Logf("  ✓ replayed: %s %s → %d (isReplay=%v)", ex.Method, ex.Path, ex.StatusCode, ex.IsReplay)

	// ── Step 5: Verify totals ───────────────────────────────
	allExchanges := proxy.Exchanges()
	if len(allExchanges) != 4 {
		t.Fatalf("expected 4 total exchanges, got %d", len(allExchanges))
	}
	t.Logf("  ✓ total exchanges: %d", len(allExchanges))

	// ── Step 6: Clear ───────────────────────────────────────
	t.Log("Step 6: Clear all exchanges")
	proxy.Clear()
	if len(proxy.Exchanges()) != 0 {
		t.Fatal("expected 0 exchanges after clear")
	}
	t.Log("  ✓ cleared")

	t.Log("\n🎉 E2E PASSED — full inspector lifecycle validated")
}
