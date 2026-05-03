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

package trafficproxy

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"time"
)

// Profile maps named network profiles to actual constraints
type Profile struct {
	Latency    time.Duration
	Jitter     time.Duration
	Bandwidth  int     // bytes per second
	PacketLoss float64 // probability 0.0 to 1.0
}

var Profiles = map[string]Profile{
	"3g": {
		Latency:   200 * time.Millisecond,
		Jitter:    50 * time.Millisecond,
		Bandwidth: 250 * 1024, // ~2 Mbps
	},
	"edge": {
		Latency:   400 * time.Millisecond,
		Jitter:    100 * time.Millisecond,
		Bandwidth: 50 * 1024, // ~400 kbps
	},
	"slow": {
		Latency:    500 * time.Millisecond,
		Jitter:     200 * time.Millisecond,
		Bandwidth:  10 * 1024, // ~80 kbps
		PacketLoss: 0.05,      // 5% loss
	},
}

// Start spins up a local TCP proxy that introduces traffic shaping (latency, jitter, bandwidth limits)
// before forwarding the traffic to the destination port.
// Returns the allocated local port, a cleanup closure, and any error.
func Start(targetPort string, profileName string) (int, func(), error) {
	prof, ok := Profiles[profileName]
	if !ok {
		return 0, nil, fmt.Errorf("unknown traffic profile %q, supported: 3g, edge, slow", profileName)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return // expected shutdown
				default:
					continue
				}
			}

			go handleConnection(ctx, conn, targetPort, prof)
		}
	}()

	cleanup := func() {
		cancel()
		_ = l.Close()
	}

	port := l.Addr().(*net.TCPAddr).Port
	return port, cleanup, nil
}

func handleConnection(ctx context.Context, src net.Conn, targetPort string, prof Profile) {
	defer func() { _ = src.Close() }() 

	if prof.Latency > 0 {
		delay := prof.Latency
		if prof.Jitter > 0 {
			// +/- jitter
			j := time.Duration(rand.Int63n(int64(prof.Jitter)*2)) - prof.Jitter
			delay += j
		}
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}
	}

	dst, err := net.Dial("tcp", "127.0.0.1:"+targetPort)
	if err != nil {
		return
	}
	defer func() { _ = dst.Close() }() 

	errc := make(chan error, 2)

	go func() {
		_, err := copyThrottled(dst, src, prof.Bandwidth, prof.PacketLoss)
		errc <- err
	}()

	go func() {
		_, err := copyThrottled(src, dst, prof.Bandwidth, prof.PacketLoss)
		errc <- err
	}()

	<-errc
}

func copyThrottled(dst net.Conn, src net.Conn, bytesPerSec int, packetLoss float64) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64

	// If no shaping needed, direct copy
	if bytesPerSec <= 0 && packetLoss == 0 {
		return io.CopyBuffer(dst, src, buf)
	}

	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			// Simulate packet loss by simply dropping the payload slice and moving on
			if packetLoss > 0 && rand.Float64() < packetLoss {
				continue
			}

			// Slice to current chunk
			chunk := buf[0:nr]

			// Throttling logic
			if bytesPerSec > 0 {
				start := time.Now()
				nw, ew := dst.Write(chunk)
				if nw > 0 {
					written += int64(nw)
				}
				if ew != nil {
					err := ew
					return written, err
				}

				// Calculate how long this chunk *should* have taken at the target bandwidth
				expectedDuration := time.Duration(float64(nw)/float64(bytesPerSec)*1000) * time.Millisecond
				elapsed := time.Since(start)

				if elapsed < expectedDuration {
					time.Sleep(expectedDuration - elapsed)
				}
			} else {
				// No Bandwidth limit but had packet loss
				nw, ew := dst.Write(chunk)
				if nw > 0 {
					written += int64(nw)
				}
				if ew != nil {
					err := ew
					return written, err
				}
			}
		}
		if er != nil {
			if er != io.EOF {
				return written, er
			}
			return written, nil
		}
	}
}
