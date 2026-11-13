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

//go:generate go run ../internal/genskills

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

const (
	// GeneratedRefBegin marks the start of the generated command reference block in SKILL.md.
	GeneratedRefBegin = "<!-- BEGIN GENERATED COMMAND REFERENCE -->"
	// GeneratedRefEnd marks the end of the generated command reference block in SKILL.md.
	GeneratedRefEnd = "<!-- END GENERATED COMMAND REFERENCE -->"
)

// SkillCommandRef records a single `devx ...` command invocation extracted from a skill file.
type SkillCommandRef struct {
	Line int
	Raw  string
}

type flagSpec struct {
	Name        string
	Shorthand   string
	Usage       string
	Type        string
	DefValue    string
	NoOptDefVal string
	Hidden      bool
}

// collectFlags reads every flag from a *pflag.FlagSet without requiring a direct
// import of github.com/spf13/pflag in go.mod / MODULE.bazel.
func collectFlags(fs any) []flagSpec {
	if fs == nil {
		return nil
	}
	v := reflect.ValueOf(fs)
	if v.IsNil() {
		return nil
	}
	visit := v.MethodByName("VisitAll")
	if !visit.IsValid() {
		return nil
	}

	var specs []flagSpec
	fnType := visit.Type().In(0)
	cb := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		f := args[0].Elem()
		var typ string
		if val := f.FieldByName("Value"); !val.IsNil() {
			if m := val.MethodByName("Type"); m.IsValid() {
				typ = m.Call(nil)[0].String()
			}
		}
		specs = append(specs, flagSpec{
			Name:        f.FieldByName("Name").String(),
			Shorthand:   f.FieldByName("Shorthand").String(),
			Usage:       f.FieldByName("Usage").String(),
			Type:        typ,
			DefValue:    f.FieldByName("DefValue").String(),
			NoOptDefVal: f.FieldByName("NoOptDefVal").String(),
			Hidden:      f.FieldByName("Hidden").Bool(),
		})
		return nil
	})
	visit.Call([]reflect.Value{cb})
	return specs
}

func formatFlagSpec(f flagSpec) string {
	var b strings.Builder
	b.WriteByte('`')
	if f.Shorthand != "" {
		b.WriteByte('-')
		b.WriteString(f.Shorthand)
		b.WriteString(", ")
	}
	b.WriteString("--")
	b.WriteString(f.Name)
	if f.Type != "" && f.Type != "bool" && f.Type != "count" {
		b.WriteString(" <")
		b.WriteString(f.Type)
		b.WriteByte('>')
	}
	b.WriteByte('`')
	return b.String()
}

func sanitizeMarkdownCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// RenderCommandReference renders a deterministic Markdown reference for all available
// commands and flags in the Cobra root command tree.
func RenderCommandReference() string {
	var b strings.Builder
	b.WriteString("_Auto-generated from the `devx` Cobra command tree (`go generate ./cmd`). Do not edit manually._\n\n")

	globalSpecs := collectFlags(rootCmd.PersistentFlags())
	globalNames := make(map[string]bool, len(globalSpecs))
	var globals []flagSpec
	for _, f := range globalSpecs {
		globalNames[f.Name] = true
		if f.Hidden || f.Name == "help" {
			continue
		}
		globals = append(globals, f)
	}
	sort.Slice(globals, func(i, j int) bool { return globals[i].Name < globals[j].Name })

	b.WriteString("### Global Flags\n\n")
	b.WriteString("| Flag | Description |\n")
	b.WriteString("|------|-------------|\n")
	for _, f := range globals {
		fmt.Fprintf(&b, "| %s | %s |\n", formatFlagSpec(f), sanitizeMarkdownCell(f.Usage))
	}

	b.WriteString("\n### Commands\n\n")
	b.WriteString("| Command | Flags | Summary |\n")
	b.WriteString("|---------|-------|---------|\n")

	var walk func(parent *cobra.Command, prefix []string)
	walk = func(parent *cobra.Command, prefix []string) {
		children := make([]*cobra.Command, 0, len(parent.Commands()))
		for _, child := range parent.Commands() {
			if !child.IsAvailableCommand() {
				continue
			}
			children = append(children, child)
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })

		for _, c := range children {
			// Pass the explicit path prefix because shared commands such as upCmd
			// are attached to both rootCmd and tunnelCmd.
			path := append(append([]string(nil), prefix...), c.Name())
			argSpec := strings.TrimSpace(strings.TrimPrefix(c.Use, c.Name()))
			cmdText := strings.Join(path, " ")
			if argSpec != "" {
				cmdText += " " + argSpec
			}
			cmdCell := "`" + cmdText + "`"
			if len(c.Aliases) > 0 {
				cmdCell += " (aliases: " + strings.Join(c.Aliases, ", ") + ")"
			}

			var flagsCell string
			if c.DisableFlagParsing {
				flagsCell = "_(passthrough)_"
			} else {
				seen := make(map[string]bool)
				var ownFlags []flagSpec
				for _, fs := range []any{c.LocalNonPersistentFlags(), c.PersistentFlags()} {
					for _, f := range collectFlags(fs) {
						if f.Hidden || f.Name == "help" || globalNames[f.Name] || seen[f.Name] {
							continue
						}
						seen[f.Name] = true
						ownFlags = append(ownFlags, f)
					}
				}
				if len(ownFlags) == 0 {
					flagsCell = "—"
				} else {
					sort.Slice(ownFlags, func(i, j int) bool { return ownFlags[i].Name < ownFlags[j].Name })
					formatted := make([]string, len(ownFlags))
					for i, f := range ownFlags {
						formatted[i] = formatFlagSpec(f)
					}
					flagsCell = strings.Join(formatted, ", ")
				}
			}

			fmt.Fprintf(&b, "| %s | %s | %s |\n", cmdCell, flagsCell, sanitizeMarkdownCell(c.Short))
			walk(c, path)
		}
	}

	walk(rootCmd, []string{rootCmd.Name()})
	return strings.TrimRight(b.String(), "\n")
}

// ReplaceGeneratedCommandReference replaces the region between GeneratedRefBegin and
// GeneratedRefEnd in markdown with the freshly rendered Cobra command reference.
func ReplaceGeneratedCommandReference(markdown string) (string, error) {
	beginIdx := strings.Index(markdown, GeneratedRefBegin)
	if beginIdx == -1 {
		return "", fmt.Errorf("missing marker %q", GeneratedRefBegin)
	}
	endIdx := strings.Index(markdown, GeneratedRefEnd)
	if endIdx == -1 {
		return "", fmt.Errorf("missing marker %q", GeneratedRefEnd)
	}
	if endIdx < beginIdx {
		return "", fmt.Errorf("marker %q appears before %q", GeneratedRefEnd, GeneratedRefBegin)
	}

	head := markdown[:beginIdx+len(GeneratedRefBegin)]
	tail := markdown[endIdx:]
	return head + "\n\n" + RenderCommandReference() + "\n\n" + tail, nil
}

// ExtractSkillDevxCommands extracts every `devx ...` command invocation from Markdown
// fenced code blocks and inline backtick spans.
func ExtractSkillDevxCommands(markdown string) []SkillCommandRef {
	var refs []SkillCommandRef
	inFence := false
	lines := strings.Split(markdown, "\n")

	for i, line := range lines {
		lineNo := i + 1
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}

		if inFence {
			cmd := stripShellComment(trimmed)
			cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "$ "))
			if isDevxInvocation(cmd) {
				refs = append(refs, SkillCommandRef{Line: lineNo, Raw: cmd})
			}
			continue
		}

		// Scan inline single-backtick spans outside fenced blocks.
		pos := 0
		for pos < len(line) {
			start := strings.IndexByte(line[pos:], '`')
			if start == -1 {
				break
			}
			absStart := pos + start
			end := strings.IndexByte(line[absStart+1:], '`')
			if end == -1 {
				break
			}
			token := strings.TrimSpace(line[absStart+1 : absStart+1+end])
			if isDevxInvocation(token) {
				refs = append(refs, SkillCommandRef{Line: lineNo, Raw: token})
			}
			pos = absStart + 1 + end + 1
		}
	}

	return refs
}

func isDevxInvocation(s string) bool {
	return s == "devx" || strings.HasPrefix(s, "devx ")
}

func stripShellComment(s string) string {
	var inSingle, inDouble, escaped bool
	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && !inSingle {
			escaped = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if c == '#' && !inSingle && !inDouble {
			if i == 0 || unicode.IsSpace(rune(s[i-1])) {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return s
}

func splitShellWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inToken := false
	var inSingle, inDouble, escaped bool

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			cur.WriteByte(c)
			escaped = false
			inToken = true
			continue
		}
		if c == '\\' && !inSingle {
			escaped = true
			inToken = true
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			inToken = true
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			inToken = true
			continue
		}
		if unicode.IsSpace(rune(c)) && !inSingle && !inDouble {
			if inToken {
				words = append(words, cur.String())
				cur.Reset()
				inToken = false
			}
			continue
		}
		cur.WriteByte(c)
		inToken = true
	}

	if escaped {
		return nil, fmt.Errorf("trailing escape in %q", s)
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in %q", s)
	}
	if inToken {
		words = append(words, cur.String())
	}
	return words, nil
}

// buildSyntheticFlagSet creates an isolated *pflag.FlagSet (via a throwaway cobra.Command)
// with fresh dummy flag targets so validation never mutates package cmd globals.
func buildSyntheticFlagSet(name string, cmds []*cobra.Command, interspersed bool) *cobra.Command {
	holder := &cobra.Command{Use: name}
	fs := holder.Flags()
	fs.SetInterspersed(interspersed)
	fs.SetOutput(io.Discard)

	seen := make(map[string]bool)
	addSpecs := func(specs []flagSpec) {
		for _, f := range specs {
			if f.Hidden || seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			switch f.Type {
			case "bool":
				fs.BoolP(f.Name, f.Shorthand, false, f.Usage)
			case "count":
				fs.CountP(f.Name, f.Shorthand, f.Usage)
			case "int", "int64", "uint":
				fs.IntP(f.Name, f.Shorthand, 0, f.Usage)
			case "duration":
				fs.DurationP(f.Name, f.Shorthand, 0, f.Usage)
			case "stringSlice", "stringArray":
				fs.StringArrayP(f.Name, f.Shorthand, nil, f.Usage)
			default:
				fs.StringP(f.Name, f.Shorthand, "", f.Usage)
			}
			if f.NoOptDefVal != "" {
				if fl := fs.Lookup(f.Name); fl != nil {
					fl.NoOptDefVal = f.NoOptDefVal
				}
			}
		}
	}

	// Register leaf/local flags first, then walk ancestors from closest to root.
	for i := len(cmds) - 1; i >= 0; i-- {
		c := cmds[i]
		if i == len(cmds)-1 {
			addSpecs(collectFlags(c.LocalNonPersistentFlags()))
		}
		addSpecs(collectFlags(c.PersistentFlags()))
	}

	return holder
}

func findAvailableSubcommand(parent *cobra.Command, token string) *cobra.Command {
	for _, child := range parent.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		if child.Name() == token || slices.Contains(child.Aliases, token) {
			return child
		}
	}
	return nil
}

// ValidateDevxCommandLine checks that a single `devx ...` command line resolves to an
// available Cobra command with valid flags and positional arguments.
func ValidateDevxCommandLine(raw string) error {
	words, err := splitShellWords(raw)
	if err != nil {
		return err
	}
	if len(words) == 0 || words[0] != "devx" {
		return fmt.Errorf("expected command starting with devx, got %q", raw)
	}

	cur := rootCmd
	ancestors := []*cobra.Command{rootCmd}
	path := []string{rootCmd.Name()}
	remaining := words[1:]
	hadParentFlags := false

	for cur.HasSubCommands() && !cur.DisableFlagParsing {
		stepCmd := buildSyntheticFlagSet(strings.Join(path, " "), ancestors, false)
		stepFS := stepCmd.Flags()
		if err := stepFS.Parse(remaining); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(path, " "), err)
		}
		stepArgs := stepFS.Args()
		if len(stepArgs) == 0 {
			break
		}
		sub := findAvailableSubcommand(cur, stepArgs[0])
		if sub == nil {
			break
		}
		if stepFS.NFlag() > 0 {
			hadParentFlags = true
		}
		cur = sub
		ancestors = append(ancestors, sub)
		path = append(path, sub.Name())
		remaining = stepArgs[1:]
	}

	if cur.DisableFlagParsing {
		return nil
	}

	leafCmd := buildSyntheticFlagSet(strings.Join(path, " "), ancestors, true)
	leafFS := leafCmd.Flags()
	if err := leafFS.Parse(remaining); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(path, " "), err)
	}

	posArgs := leafFS.Args()
	if cur.HasSubCommands() && len(strings.Fields(cur.Use)) == 1 && len(posArgs) > 0 {
		return fmt.Errorf("unknown subcommand %q for %q", posArgs[0], strings.Join(path, " "))
	}

	// Only check positional Args constraints when the invocation supplies args or flags;
	// bare prose references such as `devx db synthesize` have neither and should pass.
	if !cur.HasSubCommands() && cur.Args != nil && (len(posArgs) > 0 || leafFS.NFlag() > 0 || hadParentFlags) {
		if err := cur.Args(cur, posArgs); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(path, " "), err)
		}
	}

	return nil
}

// ValidateSkillCommands verifies that every `devx ...` invocation in markdown resolves
// to a real Cobra command and accepts the specified flags and arguments.
func ValidateSkillCommands(markdown string) error {
	refs := ExtractSkillDevxCommands(markdown)
	if len(refs) == 0 {
		return errors.New("no devx commands found in skill markdown")
	}

	var errs []error
	for _, ref := range refs {
		if err := ValidateDevxCommandLine(ref.Raw); err != nil {
			errs = append(errs, fmt.Errorf("line %d (%s): %w", ref.Line, ref.Raw, err))
		}
	}
	return errors.Join(errs...)
}
