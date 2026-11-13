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

// Command genskills regenerates the Cobra command reference inside the canonical
// devx skill template and syncs all checked-in skill copies from that source.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/VitruvianSoftware/vitruvian-devx/cmd"
	"github.com/VitruvianSoftware/vitruvian-devx/internal/agent"
)

func resolveDevxRoot(flagRoot string) (string, error) {
	if flagRoot != "" {
		return flagRoot, nil
	}
	if ws := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); ws != "" {
		return filepath.Join(ws, "apps/cli/vitruvian-devx"), nil
	}
	// When invoked via `go generate ./cmd`, cwd is apps/cli/vitruvian-devx/cmd.
	if _, err := os.Stat(filepath.Join("..", "internal/agent/embed.go")); err == nil {
		return filepath.Abs("..")
	}
	if _, err := os.Stat("internal/agent/embed.go"); err == nil {
		return os.Getwd()
	}
	return "", fmt.Errorf("could not locate apps/cli/vitruvian-devx root; pass -root")
}

func run(root string) error {
	devxSkillPath := filepath.Join(root, "internal/agent", agent.SkillTemplatePath("devx"))
	raw, err := os.ReadFile(devxSkillPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", devxSkillPath, err)
	}

	updated, err := cmd.ReplaceGeneratedCommandReference(string(raw))
	if err != nil {
		return fmt.Errorf("render command reference for %s: %w", devxSkillPath, err)
	}

	if err := os.WriteFile(devxSkillPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", devxSkillPath, err)
	}
	fmt.Printf("updated %s\n", devxSkillPath)

	for _, skill := range agent.AvailableSkills {
		srcPath := filepath.Join(root, "internal/agent", agent.SkillTemplatePath(skill.ID))
		content, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read canonical skill %s: %w", srcPath, err)
		}
		for _, dir := range agent.CheckedInSkillDirs {
			destPath := filepath.Join(root, dir, skill.ID, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(destPath), err)
			}
			if err := os.WriteFile(destPath, content, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", destPath, err)
			}
			fmt.Printf("synced  %s\n", destPath)
		}
	}

	return nil
}

func main() {
	var rootFlag string
	flag.StringVar(&rootFlag, "root", "", "Path to vitruvian-devx root directory")
	flag.Parse()

	root, err := resolveDevxRoot(rootFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genskills: %v\n", err)
		os.Exit(1)
	}
	if err := run(root); err != nil {
		fmt.Fprintf(os.Stderr, "genskills: %v\n", err)
		os.Exit(1)
	}
}
