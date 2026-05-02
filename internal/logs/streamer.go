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

package logs

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/provider"
)

type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Type      string    `json:"type"` // "container" or "host"
}

type Streamer struct {
	Lines    chan LogLine
	Errors   chan error
	Redactor *SecretRedactor
	Runtime  provider.ContainerRuntime
	services map[string]context.CancelFunc
	mu       sync.Mutex
}

func NewStreamer(rt provider.ContainerRuntime) *Streamer {
	return &Streamer{
		Lines:    make(chan LogLine, 1000),
		Errors:   make(chan error, 10),
		Runtime:  rt,
		services: make(map[string]context.CancelFunc),
	}
}

func (s *Streamer) Start(ctx context.Context) {
	go s.watchContainers(ctx)
	go s.watchHostLogs(ctx)
}

func (s *Streamer) watchContainers(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			out, err := s.Runtime.Exec("ps", "--format", "{{.Names}}")
			if err != nil {
				continue
			}

			names := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}

				s.mu.Lock()
				if _, exists := s.services[name]; !exists {
					childCtx, cancel := context.WithCancel(ctx)
					s.services[name] = cancel
					go s.tailContainer(childCtx, name)
				}
				s.mu.Unlock()
			}
		}
	}
}

func (s *Streamer) tailContainer(ctx context.Context, name string) {
	cmd := s.Runtime.CommandContext(ctx, "logs", "--tail", "50", "-f", name)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}

	if err := cmd.Start(); err != nil {
		return
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			msg := scanner.Text()
			if s.Redactor != nil {
				msg = s.Redactor.Redact(msg)
			}
			s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: msg, Type: "container"}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			msg := scanner.Text()
			if s.Redactor != nil {
				msg = s.Redactor.Redact(msg)
			}
			s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: msg, Type: "container"}
		}
	}()

	go func() {
		<-ctx.Done()
		_ = cmd.Process.Kill()
	}()

	_ = cmd.Wait()

	s.mu.Lock()
	delete(s.services, name)
	s.mu.Unlock()
}

func (s *Streamer) watchHostLogs(ctx context.Context) {
	logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
	_ = os.MkdirAll(logDir, 0755)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			files, _ := os.ReadDir(logDir)
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".log") {
					name := strings.TrimSuffix(f.Name(), ".log")
					serviceName := "host:" + name

					s.mu.Lock()
					if _, exists := s.services[serviceName]; !exists {
						childCtx, cancel := context.WithCancel(ctx)
						s.services[serviceName] = cancel
						go s.tailFile(childCtx, serviceName, filepath.Join(logDir, f.Name()))
					}
					s.mu.Unlock()
				}
			}
		}
	}
}

func (s *Streamer) tailFile(ctx context.Context, name, path string) {
	cmd := exec.CommandContext(ctx, "tail", "-n", "50", "-f", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	if err := cmd.Start(); err != nil {
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		msg := scanner.Text()
		if s.Redactor != nil {
			msg = s.Redactor.Redact(msg)
		}
		s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: msg, Type: "host"}
	}

	s.mu.Lock()
	delete(s.services, name)
	s.mu.Unlock()
}
