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

package usb

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func agentSpec() JoinSpec {
	return JoinSpec{
		Role:             RoleAgent,
		Mode:             ModeEphemeral,
		K3sVersion:       "v1.30.4+k3s1",
		Pool:             "usb",
		NodeNamePrefix:   "usb",
		Token:            "K10agenttoken::node:secret",
		LANServerURL:     "https://10.0.0.5:6443",
		TailnetServerURL: "https://100.64.0.5:6443",
		TailscaleAuthKey: "tskey-auth-abc123",
		Ephemeral:        true,
		Scratch:          ScratchConfig{Strategy: ScratchNone},
	}
}

func TestRenderJoinScript_Golden(t *testing.T) {
	cases := []struct {
		name string
		spec JoinSpec
	}{
		{
			name: "agent_ephemeral_tailscale",
			spec: agentSpec(),
		},
		{
			name: "server_install_lan_only",
			spec: JoinSpec{
				Role:           RoleServer,
				Mode:           ModeInstall,
				Pool:           "lab",
				NodeNamePrefix: "node",
				Token:          "K10servertoken::node:secret",
				LANServerURL:   "https://10.0.0.5:6443",
			},
		},
		{
			name: "agent_ephemeral_scratch_existing",
			spec: func() JoinSpec {
				s := agentSpec()
				s.Scratch = ScratchConfig{Strategy: ScratchExisting, Source: "LABEL=devx-scratch"}
				return s
			}(),
		},
		{
			name: "agent_ephemeral_scratch_freespace",
			spec: func() JoinSpec {
				s := agentSpec()
				s.TailscaleAuthKey = ""
				s.TailnetServerURL = ""
				s.Scratch = ScratchConfig{Strategy: ScratchFreeSpace}
				return s
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderJoinScript(tc.spec)
			if err != nil {
				t.Fatalf("RenderJoinScript: %v", err)
			}
			golden := filepath.Join("testdata", tc.name+".sh.golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading golden (run with -update to create): %v", err)
			}
			if got != string(want) {
				t.Errorf("script mismatch with %s\n--- got ---\n%s", golden, got)
			}
		})
	}
}

// TestRenderJoinScript_Invariants asserts the correctness properties golden
// files alone cannot (a golden just freezes whatever was generated).
func TestRenderJoinScript_Invariants(t *testing.T) {
	t.Run("agent uses K3S_URL+token, not --server", func(t *testing.T) {
		got, err := RenderJoinScript(agentSpec())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `K3S_URL="$SERVER_URL"`) {
			t.Error("agent install must set K3S_URL")
		}
		if !strings.Contains(got, "sh -s - agent ") {
			t.Error("agent install must use the agent subcommand")
		}
		if strings.Contains(got, "--server=") {
			t.Error("agent must not use --server")
		}
	})

	t.Run("server uses --server", func(t *testing.T) {
		s := agentSpec()
		s.Role = RoleServer
		s.Ephemeral = false
		got, err := RenderJoinScript(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `--server="$SERVER_URL"`) {
			t.Error("server join must use --server")
		}
		if !strings.Contains(got, `INSTALL_K3S_EXEC="server"`) {
			t.Error("server join must set INSTALL_K3S_EXEC=server")
		}
	})

	t.Run("tailnet path sets flannel-iface=tailscale0", func(t *testing.T) {
		got, err := RenderJoinScript(agentSpec())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `FLANNEL_IFACE="tailscale0"`) {
			t.Error("tailnet branch must select the tailscale0 flannel iface")
		}
	})

	t.Run("ephemeral adds the ephemeral label", func(t *testing.T) {
		got, err := RenderJoinScript(agentSpec())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, EphemeralLabel+"=true") {
			t.Errorf("ephemeral node must carry the %s label", EphemeralLabel)
		}
	})

	t.Run("scratch=none omits the partition logic", func(t *testing.T) {
		got, err := RenderJoinScript(agentSpec())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "parted") || strings.Contains(got, "mkfs") {
			t.Error("scratch=none must not contain partitioning/format commands")
		}
	})

	t.Run("free-space scratch refuses to touch existing partitions", func(t *testing.T) {
		s := agentSpec()
		s.Scratch = ScratchConfig{Strategy: ScratchFreeSpace}
		got, err := RenderJoinScript(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "refusing to touch existing partitions") {
			t.Error("free-space scratch must guard against destroying existing partitions")
		}
	})

	t.Run("token is shell-quoted", func(t *testing.T) {
		s := agentSpec()
		s.Token = "tok'with'quotes"
		got, err := RenderJoinScript(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, `'\''`) {
			t.Error("embedded single quotes must be escaped")
		}
	})
}

func TestJoinSpec_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*JoinSpec)
		wantErr string
	}{
		{"valid", func(*JoinSpec) {}, ""},
		{"bad role", func(s *JoinSpec) { s.Role = "boss" }, "invalid role"},
		{"ephemeral server", func(s *JoinSpec) { s.Role = RoleServer }, "must join as agents"},
		{"no token", func(s *JoinSpec) { s.Token = "" }, "token is required"},
		{"no server url", func(s *JoinSpec) { s.LANServerURL = ""; s.TailnetServerURL = "" }, "at least one of"},
		{"tailnet without key", func(s *JoinSpec) { s.TailscaleAuthKey = "" }, "no TailscaleAuthKey"},
		{"existing scratch no source", func(s *JoinSpec) { s.Scratch = ScratchConfig{Strategy: ScratchExisting} }, "requires a source"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := agentSpec()
			tc.mutate(&s)
			err := s.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
