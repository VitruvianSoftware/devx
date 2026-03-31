package trafficproxy

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func TestTrafficProxy(t *testing.T) {
	// Start a dummy echo server
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	// Apply shaping
	proxyPort, cleanup, err := Start(fmt.Sprintf("%d", port), "3g")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	start := time.Now()
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", proxyPort))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	payload := []byte("hello shaping proxy")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)

	if !bytes.Equal(buf, payload) {
		t.Errorf("expected %q, got %q", payload, buf)
	}

	// 3G profile has 200ms latency, +/- 50ms jitter. Our roundtrip should take at least 150ms.
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected latency to delay response, but got response in %s", elapsed)
	}
}
