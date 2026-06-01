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

package ship

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// gitT runs a git command in dir, failing the test on error. Returns trimmed stdout.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFileT(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func commitCountT(t *testing.T, dir string) int {
	t.Helper()
	n, err := strconv.Atoi(gitT(t, dir, "rev-list", "--count", "HEAD"))
	if err != nil {
		t.Fatalf("parse commit count: %v", err)
	}
	return n
}

// initRepoWithBase initializes a git repo on branch "main" with one base commit
// and repo-local identity/signing config so plain `git commit` works.
func initRepoWithBase(t *testing.T, dir string) {
	t.Helper()
	gitT(t, dir, "init")
	// `git init -b main` needs git >= 2.28; set the initial branch via symbolic-ref
	// instead so the test also runs on older git (e.g. the RBE executor's).
	gitT(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	gitT(t, dir, "config", "user.name", "test")
	gitT(t, dir, "config", "user.email", "test@example.com")
	gitT(t, dir, "config", "commit.gpgsign", "false")
	writeFileT(t, dir, "base.txt", "base")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-m", "base commit")
}

func TestHasUnpushedCommits(t *testing.T) {
	dir := t.TempDir()
	initRepoWithBase(t, dir)

	// HEAD == main: nothing ahead of the base.
	if HasUnpushedCommits(dir, "main") {
		t.Errorf("expected no unpushed commits when HEAD == base, got true")
	}

	// Feature branch with one extra commit: now ahead of main.
	gitT(t, dir, "checkout", "-b", "feature")
	writeFileT(t, dir, "feature.txt", "work")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-m", "feature work")

	if !HasUnpushedCommits(dir, "main") {
		t.Errorf("expected unpushed commits ahead of main, got false")
	}
}

func TestCommitAll_SkipsWhenClean(t *testing.T) {
	dir := t.TempDir()
	initRepoWithBase(t, dir)
	before := commitCountT(t, dir)
	if err := commitAll(dir, "should not create a commit"); err != nil {
		t.Fatalf("commitAll on clean tree: %v", err)
	}
	if after := commitCountT(t, dir); after != before {
		t.Errorf("clean tree should not commit: before=%d after=%d", before, after)
	}
}

func TestLatestCommitMessage(t *testing.T) {
	dir := t.TempDir()
	initRepoWithBase(t, dir)
	gitT(t, dir, "checkout", "-b", "feature")
	writeFileT(t, dir, "x.txt", "x")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-m", "feat: my feature\n\nbody detail line")

	msg, err := LatestCommitMessage(dir)
	if err != nil {
		t.Fatalf("LatestCommitMessage: %v", err)
	}
	if !strings.HasPrefix(msg, "feat: my feature") {
		t.Errorf("expected subject as prefix, got %q", msg)
	}
	if !strings.Contains(msg, "body detail line") {
		t.Errorf("expected body in message, got %q", msg)
	}
}

func TestCommitAll_CommitsWhenDirty(t *testing.T) {
	dir := t.TempDir()
	initRepoWithBase(t, dir)
	before := commitCountT(t, dir)
	writeFileT(t, dir, "new.txt", "content")
	if err := commitAll(dir, "feat: add new file"); err != nil {
		t.Fatalf("commitAll with changes: %v", err)
	}
	if after := commitCountT(t, dir); after != before+1 {
		t.Errorf("dirty tree should create one commit: before=%d after=%d", before, after)
	}
}
