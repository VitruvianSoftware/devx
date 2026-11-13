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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/vitruvian-devx/internal/agent"
)

func TestShippedSkillGeneratedReferenceMatchesCobra(t *testing.T) {
	content, err := agent.SkillContent("devx")
	if err != nil {
		t.Fatalf("SkillContent(devx): %v", err)
	}

	rendered, err := ReplaceGeneratedCommandReference(string(content))
	if err != nil {
		t.Fatalf("ReplaceGeneratedCommandReference: %v", err)
	}

	if string(content) != rendered {
		t.Errorf("shipped devx/SKILL.md generated command reference is out of date; run `go generate ./cmd` or `bazel run //apps/cli/vitruvian-devx/internal/genskills`")
	}
}

func TestShippedSkillsCommandsResolveAgainstCobra(t *testing.T) {
	for _, skill := range agent.AvailableSkills {
		t.Run(skill.ID, func(t *testing.T) {
			content, err := agent.SkillContent(skill.ID)
			if err != nil {
				t.Fatalf("SkillContent(%q): %v", skill.ID, err)
			}
			if err := ValidateSkillCommands(string(content)); err != nil {
				t.Errorf("invalid devx commands in %s:\n%v", agent.SkillTemplatePath(skill.ID), err)
			}
		})
	}
}

func TestCheckedInSkillsMatchSingleSource(t *testing.T) {
	for _, skill := range agent.AvailableSkills {
		canonical, err := agent.SkillContent(skill.ID)
		if err != nil {
			t.Fatalf("SkillContent(%q): %v", skill.ID, err)
		}

		for _, dir := range agent.CheckedInSkillDirs {
			relPath := filepath.Join("..", dir, skill.ID, "SKILL.md")
			got, err := os.ReadFile(relPath)
			if err != nil {
				t.Fatalf("read checked-in skill %s: %v", relPath, err)
			}
			if !bytes.Equal(got, canonical) {
				t.Errorf("%s drifted from canonical %s; regenerate with `bazel run //apps/cli/vitruvian-devx/internal/genskills`",
					filepath.Join(dir, skill.ID, "SKILL.md"), agent.SkillTemplatePath(skill.ID))
			}
		}
	}
}

func TestValidateSkillCommands_RejectsInvalidCommands(t *testing.T) {
	cases := []struct {
		name string
		md   string
	}{
		{
			name: "stale root init subcommand",
			md:   "- `devx init -y`\n",
		},
		{
			name: "nonexistent flag",
			md:   "- `devx vm init --no-such-flag`\n",
		},
		{
			name: "missing required positional engine arg",
			md:   "- `devx db ask --sizes`\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateSkillCommands(tc.md); err == nil {
				t.Errorf("expected ValidateSkillCommands(%q) to fail, got nil", tc.md)
			}
		})
	}
}

func TestAgentInitHelpTextDescribesSkillsOnly(t *testing.T) {
	for _, forbidden := range []string{".cursorrules", "CLAUDE.md"} {
		if strings.Contains(agentInitCmd.Short, forbidden) || strings.Contains(agentInitCmd.Long, forbidden) {
			t.Errorf("agentInitCmd help text should not mention %q", forbidden)
		}
	}
	if !strings.Contains(strings.ToLower(agentInitCmd.Short), "skill") || !strings.Contains(strings.ToLower(agentInitCmd.Long), "skill") {
		t.Errorf("agentInitCmd Short and Long should describe installing skills")
	}
}
