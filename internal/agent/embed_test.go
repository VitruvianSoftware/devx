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

package agent

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestEmbedSingleSourcePerSkill(t *testing.T) {
	var found []string
	err := fs.WalkDir(EmbedFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	sort.Strings(found)

	var want []string
	for _, s := range AvailableSkills {
		want = append(want, SkillTemplatePath(s.ID))
	}
	sort.Strings(want)

	if !reflect.DeepEqual(found, want) {
		t.Fatalf("embedded templates mismatch:\ngot:  %v\nwant: %v", found, want)
	}
}

func TestInstallWritesIdenticalBytesToAllAgents(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, skill := range AvailableSkills {
		canonical, err := SkillContent(skill.ID)
		if err != nil {
			t.Fatalf("SkillContent(%q): %v", skill.ID, err)
		}

		for agentName, basePath := range AgentBasePaths {
			if err := Install(agentName, skill.ID, false); err != nil {
				t.Fatalf("Install(%q, %q, false): %v", agentName, skill.ID, err)
			}

			dest := filepath.Join(basePath, skill.ID, "SKILL.md")
			got, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("read installed %s: %v", dest, err)
			}
			if !bytes.Equal(got, canonical) {
				t.Errorf("%s/%s bytes differ from canonical %s", agentName, skill.ID, SkillTemplatePath(skill.ID))
			}

			// Verify --force behavior: false preserves existing content, true overwrites.
			sentinel := []byte("custom-modified")
			if err := os.WriteFile(dest, sentinel, 0o644); err != nil {
				t.Fatalf("write sentinel %s: %v", dest, err)
			}
			if err := Install(agentName, skill.ID, false); err != nil {
				t.Fatalf("Install(%q, %q, false) skip: %v", agentName, skill.ID, err)
			}
			afterSkip, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("read after skip %s: %v", dest, err)
			}
			if !bytes.Equal(afterSkip, sentinel) {
				t.Errorf("%s was overwritten when force=false", dest)
			}

			if err := Install(agentName, skill.ID, true); err != nil {
				t.Fatalf("Install(%q, %q, true): %v", agentName, skill.ID, err)
			}
			afterForce, err := os.ReadFile(dest)
			if err != nil {
				t.Fatalf("read after force %s: %v", dest, err)
			}
			if !bytes.Equal(afterForce, canonical) {
				t.Errorf("%s was not restored when force=true", dest)
			}
		}
	}
}
