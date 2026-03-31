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
