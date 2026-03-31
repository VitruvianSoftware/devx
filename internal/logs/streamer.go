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
	services map[string]context.CancelFunc
	mu       sync.Mutex
}

func NewStreamer() *Streamer {
	return &Streamer{
		Lines:    make(chan LogLine, 1000),
		Errors:   make(chan error, 10),
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
			out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").Output()
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
	cmd := exec.CommandContext(ctx, "podman", "logs", "--tail", "50", "-f", name)
	
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
			s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: scanner.Text(), Type: "container"}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: scanner.Text(), Type: "container"}
		}
	}()

	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
	}()

	cmd.Wait()

	s.mu.Lock()
	delete(s.services, name)
	s.mu.Unlock()
}

func (s *Streamer) watchHostLogs(ctx context.Context) {
	logDir := filepath.Join(os.Getenv("HOME"), ".devx", "logs")
	os.MkdirAll(logDir, 0755)

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
		s.Lines <- LogLine{Timestamp: time.Now(), Service: name, Message: scanner.Text(), Type: "host"}
	}

	s.mu.Lock()
	delete(s.services, name)
	s.mu.Unlock()
}
