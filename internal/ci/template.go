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

package ci

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
)

// expressionRegex matches GitHub Actions ${{ ... }} expressions.
var expressionRegex = regexp.MustCompile(`\$\{\{\s*(.+?)\s*\}\}`)

// simpleEqualityRegex matches simple equality checks like: matrix.goos == 'linux'
var simpleEqualityRegex = regexp.MustCompile(`^(\w+(?:\.\w+)*)\s*==\s*'([^']*)'$`)

// simpleInequalityRegex matches simple inequality checks like: matrix.goos != 'linux'
var simpleInequalityRegex = regexp.MustCompile(`^(\w+(?:\.\w+)*)\s*!=\s*'([^']*)'$`)

// TemplateContext holds all the values available for ${{ }} expression substitution.
type TemplateContext struct {
	Env     map[string]string // workflow + job + step env
	Secrets map[string]string // from devx Vault providers
	Matrix  map[string]string // current matrix row values

	// Warnings collects non-fatal substitution issues for reporting.
	Warnings []string
}

// NewTemplateContext creates a TemplateContext with sensible defaults for
// the stubbed github.* and runner.* contexts.
func NewTemplateContext(env, secrets, matrix map[string]string) *TemplateContext {
	if env == nil {
		env = map[string]string{}
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	if matrix == nil {
		matrix = map[string]string{}
	}
	return &TemplateContext{
		Env:     env,
		Secrets: secrets,
		Matrix:  matrix,
	}
}

// Substitute replaces all ${{ ... }} expressions in the input string.
func (tc *TemplateContext) Substitute(input string) string {
	return expressionRegex.ReplaceAllStringFunc(input, func(match string) string {
		// Extract the inner expression
		inner := expressionRegex.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		expr := strings.TrimSpace(inner[1])
		return tc.resolveExpression(expr)
	})
}

// EvaluateCondition evaluates an `if:` conditional expression.
// Returns true if the step/job should run.
//
// Strategy:
//   - Simple equality (matrix.goos == 'linux') → exact match
//   - Simple inequality (matrix.goos != 'linux') → exact non-match
//   - Complex expressions (contains(), hashFiles()) → fail-open (return true) with warning
//   - Empty string → true (no condition = always run)
func (tc *TemplateContext) EvaluateCondition(condition string) bool {
	if condition == "" {
		return true
	}

	// Strip surrounding ${{ }} if present
	condition = strings.TrimSpace(condition)
	if strings.HasPrefix(condition, "${{") && strings.HasSuffix(condition, "}}") {
		condition = strings.TrimSpace(condition[3 : len(condition)-2])
	}

	// Simple equality: matrix.goos == 'linux'
	if m := simpleEqualityRegex.FindStringSubmatch(condition); m != nil {
		resolved := tc.resolveExpression(m[1])
		return resolved == m[2]
	}

	// Simple inequality: matrix.goos != 'linux'
	if m := simpleInequalityRegex.FindStringSubmatch(condition); m != nil {
		resolved := tc.resolveExpression(m[1])
		return resolved != m[2]
	}

	// Simple truthy check: env.SOME_VAR or matrix.goos
	if !strings.Contains(condition, "(") && !strings.Contains(condition, "==") && !strings.Contains(condition, "!=") {
		resolved := tc.resolveExpression(condition)
		return resolved != "" && resolved != "false" && resolved != "0"
	}

	// Complex expression — fail-open with warning
	tc.Warnings = append(tc.Warnings,
		fmt.Sprintf("⚠️  Complex if: condition %q cannot be fully evaluated locally — assuming true (fail-open)", condition))
	return true
}

// resolveExpression resolves a single expression like "env.FOO", "secrets.BAR",
// "matrix.goos", "github.event_name", or "runner.os".
func (tc *TemplateContext) resolveExpression(expr string) string {
	parts := strings.SplitN(expr, ".", 2)
	if len(parts) != 2 {
		tc.Warnings = append(tc.Warnings,
			fmt.Sprintf("⚠️  Unknown expression: ${{ %s }} — left as-is", expr))
		return fmt.Sprintf("${{ %s }}", expr)
	}

	namespace := parts[0]
	key := parts[1]

	switch namespace {
	case "env":
		if v, ok := tc.Env[key]; ok {
			return v
		}
		return ""

	case "secrets":
		if v, ok := tc.Secrets[key]; ok {
			return v
		}
		tc.Warnings = append(tc.Warnings,
			fmt.Sprintf("⚠️  Secret %q not found in devx Vault — replaced with empty string", key))
		return ""

	case "matrix":
		if v, ok := tc.Matrix[key]; ok {
			return v
		}
		return ""

	case "github":
		// Stub common github context values
		return tc.resolveGitHubContext(key)

	case "runner":
		return tc.resolveRunnerContext(key)

	default:
		tc.Warnings = append(tc.Warnings,
			fmt.Sprintf("⚠️  Unknown expression namespace %q in ${{ %s }} — left as-is", namespace, expr))
		return fmt.Sprintf("${{ %s }}", expr)
	}
}

// resolveGitHubContext returns stubbed values for github.* expressions.
func (tc *TemplateContext) resolveGitHubContext(key string) string {
	switch key {
	case "event_name":
		return "push"
	case "ref":
		return "refs/heads/main"
	case "sha":
		return "local"
	case "workspace":
		return "/workspace"
	case "repository":
		return "local/repo"
	case "actor":
		return "developer"
	default:
		tc.Warnings = append(tc.Warnings,
			fmt.Sprintf("⚠️  github.%s is not supported locally — stubbed as empty", key))
		return ""
	}
}

// resolveRunnerContext returns values for runner.* expressions.
func (tc *TemplateContext) resolveRunnerContext(key string) string {
	switch key {
	case "os":
		return "Linux" // containers always run Linux
	case "arch":
		arch := runtime.GOARCH
		switch arch {
		case "amd64":
			return "X64"
		case "arm64":
			return "ARM64"
		default:
			return arch
		}
	case "temp":
		return "/tmp"
	case "tool_cache":
		return "/opt/hostedtoolcache"
	default:
		tc.Warnings = append(tc.Warnings,
			fmt.Sprintf("⚠️  runner.%s is not supported locally — stubbed as empty", key))
		return ""
	}
}

// MergeEnv merges multiple env maps in order (later maps override earlier maps).
func MergeEnv(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
