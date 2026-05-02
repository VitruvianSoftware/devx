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

package authproxy

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Start spins up a local HTTP reverse proxy that adds Basic Authentication
// before forwarding traffic to the target port.
// Returns the port the proxy is listening on, a cleanup function, and any error.
func Start(targetPort string, authCreds string) (int, func(), error) {
	if !strings.Contains(authCreds, ":") {
		return 0, nil, fmt.Errorf("invalid basic-auth format, expected user:pass")
	}

	parts := strings.SplitN(authCreds, ":", 2)
	expectedUser := parts[0]
	expectedPass := parts[1]

	targetURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", targetPort))
	if err != nil {
		return 0, nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte(expectedUser)) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(expectedPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		proxy.ServeHTTP(w, r)
	})

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(l)
	}()

	cleanup := func() {
		_ = server.Close()
	}

	return port, cleanup, nil
}
