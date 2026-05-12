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
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ── DetectStack Tests ────────────────────────────────────────────────────────

func TestDetectStack_Go(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Go stack, got nil")
	}
	if stack.Name != "Go" {
		t.Errorf("expected name 'Go', got %q", stack.Name)
	}
	if len(stack.TestCmd) == 0 || stack.TestCmd[0] != "go" {
		t.Errorf("expected go test command, got %v", stack.TestCmd)
	}
	// Verify universal default (not org-specific)
	if len(stack.LintCmd) < 2 || stack.LintCmd[1] != "vet" {
		t.Errorf("expected 'go vet' as default lint, got %v", stack.LintCmd)
	}
}

func TestDetectStack_NodeJS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Node stack, got nil")
	}
	if stack.Name != "Node/JS/TS" {
		t.Errorf("expected name 'Node/JS/TS', got %q", stack.Name)
	}
}

func TestDetectStack_Rust(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Rust stack, got nil")
	}
	if stack.Name != "Rust" {
		t.Errorf("expected name 'Rust', got %q", stack.Name)
	}
}

func TestDetectStack_Python(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.ruff]"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Python stack, got nil")
	}
	if stack.Name != "Python" {
		t.Errorf("expected name 'Python', got %q", stack.Name)
	}
	if stack.BuildCmd != nil {
		t.Errorf("expected nil BuildCmd for Python, got %v", stack.BuildCmd)
	}
}

func TestDetectStack_JavaMaven(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project></project>"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Java/Maven stack, got nil")
	}
	if stack.Name != "Java/Maven" {
		t.Errorf("expected name 'Java/Maven', got %q", stack.Name)
	}
	if len(stack.TestCmd) == 0 || stack.TestCmd[0] != "mvn" {
		t.Errorf("expected mvn test, got %v", stack.TestCmd)
	}
}

func TestDetectStack_JavaGradle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("apply plugin: 'java'"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Java/Gradle stack, got nil")
	}
	if stack.Name != "Java/Gradle" {
		t.Errorf("expected name 'Java/Gradle', got %q", stack.Name)
	}
	if len(stack.TestCmd) == 0 || stack.TestCmd[0] != "./gradlew" {
		t.Errorf("expected gradlew test, got %v", stack.TestCmd)
	}
}

func TestDetectStack_KotlinGradle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte("plugins { kotlin(\"jvm\") }"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected Kotlin/Gradle stack, got nil")
	}
	if stack.Name != "Kotlin/Gradle" {
		t.Errorf("expected name 'Kotlin/Gradle', got %q", stack.Name)
	}
}

func TestDetectStack_DotNet_Csproj(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte("<Project></Project>"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected .NET stack, got nil")
	}
	if stack.Name != ".NET" {
		t.Errorf("expected name '.NET', got %q", stack.Name)
	}
	if len(stack.TestCmd) == 0 || stack.TestCmd[0] != "dotnet" {
		t.Errorf("expected dotnet test, got %v", stack.TestCmd)
	}
}

func TestDetectStack_DotNet_Sln(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MyApp.sln"), []byte("Microsoft Visual Studio Solution File"), 0644); err != nil {
		t.Fatal(err)
	}
	stack := DetectStack(dir)
	if stack == nil {
		t.Fatal("expected .NET stack, got nil")
	}
	if stack.Name != ".NET" {
		t.Errorf("expected name '.NET', got %q", stack.Name)
	}
}

func TestDetectStack_UnknownReturnsNil(t *testing.T) {
	dir := t.TempDir()
	// Empty directory — no marker files
	stack := DetectStack(dir)
	if stack != nil {
		t.Errorf("expected nil for unknown stack, got %+v", stack)
	}
}

// ── DetectAllStacks Tests ────────────────────────────────────────────────────

func TestDetectAllStacks_MultipleMarkers(t *testing.T) {
	dir := t.TempDir()
	// Go + Node monorepo
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	stacks := DetectAllStacks(dir)
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(stacks))
	}

	names := make(map[string]bool)
	for _, s := range stacks {
		names[s.Name] = true
	}
	if !names["Go"] {
		t.Error("expected Go stack in results")
	}
	if !names["Node/JS/TS"] {
		t.Error("expected Node/JS/TS stack in results")
	}
}

func TestDetectAllStacks_DeduplicatesSameName(t *testing.T) {
	dir := t.TempDir()
	// Both .csproj and .sln should yield only one .NET entry
	if err := os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte("<Project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyApp.sln"), []byte("Solution"), 0644); err != nil {
		t.Fatal(err)
	}

	stacks := DetectAllStacks(dir)
	count := 0
	for _, s := range stacks {
		if s.Name == ".NET" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 .NET entry, got %d", count)
	}
}

func TestDetectAllStacks_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	stacks := DetectAllStacks(dir)
	if len(stacks) != 0 {
		t.Errorf("expected 0 stacks for empty dir, got %d", len(stacks))
	}
}

// ── Precedence Tests ─────────────────────────────────────────────────────────

func TestDetectStack_GoPrecedenceOverNode(t *testing.T) {
	dir := t.TempDir()
	// When both go.mod and package.json exist, Go should win (listed first)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	stack := DetectStack(dir)
	if stack == nil || stack.Name != "Go" {
		t.Errorf("expected Go to take precedence, got %v", stack)
	}
}

func TestDetectStack_MavenPrecedenceOverGradle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte("<project/>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	stack := DetectStack(dir)
	if stack == nil || stack.Name != "Java/Maven" {
		t.Errorf("expected Maven precedence over Gradle, got %v", stack)
	}
}

// ── ErrNoStack Tests ─────────────────────────────────────────────────────────

func TestErrNoStack_ReturnedForUnknownStack(t *testing.T) {
	dir := t.TempDir()
	_, err := runAutoDetectedPipeline(dir, false)
	if err == nil {
		t.Fatal("expected ErrNoStack, got nil")
	}
	if !errors.Is(err, ErrNoStack) {
		t.Errorf("expected ErrNoStack, got %v", err)
	}
}

func TestErrNoStack_NotReturnedForKnownStack(t *testing.T) {
	dir := t.TempDir()
	// Create a go.mod so Go is detected — the actual go test will fail
	// but the error should NOT be ErrNoStack
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := runAutoDetectedPipeline(dir, false)
	// err may be non-nil (go test fails on empty module) but should not be ErrNoStack
	if errors.Is(err, ErrNoStack) {
		t.Error("should not return ErrNoStack when a stack is detected")
	}
}

// ── Explicit Pipeline Tests ──────────────────────────────────────────────────

func TestRunPreFlight_ExplicitPipelineOverridesAutoDetect(t *testing.T) {
	dir := t.TempDir()
	// Create a go.mod so auto-detect would find Go
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Provide an explicit pipeline with a trivially passing command
	pipeline := &PipelineConfig{
		Test: &PipelineStage{
			Cmds: [][]string{{"echo", "custom test"}},
		},
		Lint: &PipelineStage{
			Cmds: [][]string{{"echo", "custom lint"}},
		},
		Build: &PipelineStage{
			Cmds: [][]string{{"echo", "custom build"}},
		},
	}

	result, err := RunPreFlight(dir, false, pipeline)
	if err != nil {
		t.Fatalf("explicit pipeline should pass: %v", err)
	}
	// When explicit pipeline is used, Stack should be "pipeline"
	if result.Stack != "pipeline" {
		t.Errorf("expected stack 'pipeline', got %q", result.Stack)
	}
	if !result.TestPass || !result.LintPass || !result.BuildPass {
		t.Errorf("expected all stages to pass: test=%v lint=%v build=%v",
			result.TestPass, result.LintPass, result.BuildPass)
	}
}

func TestRunPreFlight_NilPipelineFallsBackToAutoDetect(t *testing.T) {
	dir := t.TempDir()
	// Empty dir, no pipeline → should return ErrNoStack
	result, err := RunPreFlight(dir, false, nil)
	if !errors.Is(err, ErrNoStack) {
		t.Errorf("expected ErrNoStack, got %v", err)
	}
	if result.Stack != "unknown" {
		t.Errorf("expected stack 'unknown', got %q", result.Stack)
	}
}

func TestRunPreFlight_ExplicitPipelineSkipsEmptyStages(t *testing.T) {
	dir := t.TempDir()

	// Only define lint, leave test and build nil
	pipeline := &PipelineConfig{
		Lint: &PipelineStage{
			Cmds: [][]string{{"echo", "lint only"}},
		},
	}

	result, err := RunPreFlight(dir, false, pipeline)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if !result.TestSkipped {
		t.Error("expected test to be skipped")
	}
	if !result.BuildSkipped {
		t.Error("expected build to be skipped")
	}
	if !result.LintPass {
		t.Error("expected lint to pass")
	}
}

func TestRunPreFlight_ExplicitPipelineWithHooks(t *testing.T) {
	dir := t.TempDir()

	pipeline := &PipelineConfig{
		Lint: &PipelineStage{
			Before: [][]string{{"echo", "before lint"}},
			Cmds:   [][]string{{"echo", "lint"}},
			After:  [][]string{{"echo", "after lint"}},
		},
	}

	result, err := RunPreFlight(dir, false, pipeline)
	if err != nil {
		t.Fatalf("expected hooks to succeed: %v", err)
	}
	if !result.LintPass {
		t.Error("expected lint to pass with hooks")
	}
}

// ── stackDefinitions Tests ───────────────────────────────────────────────────

func TestStackDefinitions_ContainsAllSupportedStacks(t *testing.T) {
	defs := stackDefinitions()

	expected := map[string]bool{
		"Go":             false,
		"Node/JS/TS":     false,
		"Rust":           false,
		"Python":         false,
		"Java/Maven":     false,
		"Java/Gradle":    false,
		"Kotlin/Gradle":  false,
		".NET":           false,
	}

	for _, d := range defs {
		if _, ok := expected[d.stack.Name]; ok {
			expected[d.stack.Name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("stack %q not found in stackDefinitions()", name)
		}
	}
}

func TestStackDefinitions_GlobMarkersAreFlagged(t *testing.T) {
	for _, d := range stackDefinitions() {
		if d.stack.Name == ".NET" && !d.glob {
			t.Errorf(".NET marker %q should use glob matching", d.marker)
		}
	}
}

// ── Hook Tests ───────────────────────────────────────────────────────────────

func TestInstallPrePushHook_NewRepo(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := InstallPrePushHook(dir); err != nil {
		t.Fatalf("failed to install hook: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(gitDir, "pre-push"))
	if err != nil {
		t.Fatalf("hook not written: %v", err)
	}
	if !isDevxHook(string(data)) {
		t.Error("installed hook should be recognized as devx hook")
	}
}

func TestInstallPrePushHook_RejectsNonDevxHook(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho custom hook"), 0755); err != nil {
		t.Fatal(err)
	}

	err := InstallPrePushHook(dir)
	if err == nil {
		t.Fatal("expected error when non-devx hook exists")
	}
}

func TestIsPrePushHookInstalled_DetectsDevxHook(t *testing.T) {
	dir := t.TempDir()
	hookPath := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hookPath, []byte(PrePushHookContent), 0755); err != nil {
		t.Fatal(err)
	}

	if !IsPrePushHookInstalled(dir) {
		t.Error("expected devx hook to be detected")
	}
}

func TestIsPrePushHookInstalled_ReturnsFalseWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	if IsPrePushHookInstalled(dir) {
		t.Error("expected false when no hook exists")
	}
}

// ── HasStagedChanges / CurrentBranch Tests ───────────────────────────────────

func TestHasStagedChanges_FalseInNonGitDir(t *testing.T) {
	dir := t.TempDir()
	if HasStagedChanges(dir) {
		t.Error("expected false for non-git directory")
	}
}

func TestCurrentBranch_UnknownInNonGitDir(t *testing.T) {
	dir := t.TempDir()
	branch := CurrentBranch(dir)
	if branch != "unknown" {
		t.Errorf("expected 'unknown', got %q", branch)
	}
}
