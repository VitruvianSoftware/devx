// Package ci implements a local GitHub Actions workflow parser and executor.
// It provides 80% parity with GitHub Actions by executing `run:` shell blocks
// inside isolated Podman/Docker containers, while intentionally skipping
// composite/JS `uses:` actions.
//
// LIMITATION: This engine does NOT support `uses:` actions. Third-party actions
// like actions/setup-go, actions/upload-artifact, or golangci/golangci-lint-action
// are skipped with a visible warning. Only `run:` shell blocks are executed.
// This is a deliberate design decision documented in docs/guide/ci.md.
package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Workflow represents a parsed GitHub Actions workflow file.
type Workflow struct {
	Name string            `yaml:"name"`
	Env  map[string]string `yaml:"env"`
	Jobs map[string]*Job   `yaml:"jobs"`
}

// Job represents a single job within a workflow.
type Job struct {
	Name            string            `yaml:"name"`
	RunsOn          string            `yaml:"runs-on"`
	Needs           StringOrSlice     `yaml:"needs"`
	If              string            `yaml:"if"`
	Env             map[string]string `yaml:"env"`
	Strategy        Strategy          `yaml:"strategy"`
	Services        map[string]Service `yaml:"services"`
	Steps           []Step            `yaml:"steps"`
	TimeoutMinutes  int               `yaml:"timeout-minutes"`
	ContinueOnError bool              `yaml:"continue-on-error"`
}

// Strategy holds the matrix strategy for a job.
type Strategy struct {
	Matrix    MatrixDef `yaml:"matrix"`
	FailFast  *bool     `yaml:"fail-fast"`
	MaxParallel int     `yaml:"max-parallel"`
}

// MatrixDef holds the matrix definition including include/exclude.
type MatrixDef struct {
	// Values holds the primary matrix axes (e.g., {"goos": ["darwin","linux"]}).
	Values  map[string][]string `yaml:"-"`
	Include []map[string]string `yaml:"include"`
	Exclude []map[string]string `yaml:"exclude"`
}

// UnmarshalYAML handles the polymorphic matrix definition where top-level keys
// are dynamic axes alongside the reserved "include" and "exclude" keys.
func (m *MatrixDef) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("matrix must be a mapping, got %d", node.Kind)
	}

	m.Values = make(map[string][]string)

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		switch key {
		case "include":
			if err := val.Decode(&m.Include); err != nil {
				return fmt.Errorf("decoding matrix.include: %w", err)
			}
		case "exclude":
			if err := val.Decode(&m.Exclude); err != nil {
				return fmt.Errorf("decoding matrix.exclude: %w", err)
			}
		default:
			// Dynamic axis: decode as []string
			var values []string
			if err := val.Decode(&values); err != nil {
				// Try decoding as []interface{} and stringifying
				var raw []interface{}
				if err2 := val.Decode(&raw); err2 != nil {
					return fmt.Errorf("decoding matrix.%s: %w", key, err)
				}
				for _, r := range raw {
					values = append(values, fmt.Sprintf("%v", r))
				}
			}
			m.Values[key] = values
		}
	}
	return nil
}

// Service represents a job-level service container (e.g., postgres for CI).
type Service struct {
	Image   string            `yaml:"image"`
	Env     map[string]string `yaml:"env"`
	Ports   []string          `yaml:"ports"`
	Options string            `yaml:"options"`
}

// Step represents a single step within a job.
type Step struct {
	Name             string            `yaml:"name"`
	ID               string            `yaml:"id"`
	Uses             string            `yaml:"uses"`
	Run              string            `yaml:"run"`
	Shell            string            `yaml:"shell"`
	Env              map[string]string `yaml:"env"`
	With             map[string]string `yaml:"with"`
	If               string            `yaml:"if"`
	WorkingDirectory string            `yaml:"working-directory"`
	ContinueOnError  bool              `yaml:"continue-on-error"`
	TimeoutMinutes   int               `yaml:"timeout-minutes"`
}

// StringOrSlice handles YAML fields that can be either a single string or a list.
type StringOrSlice []string

func (s *StringOrSlice) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*s = []string{node.Value}
	case yaml.SequenceNode:
		var items []string
		if err := node.Decode(&items); err != nil {
			return err
		}
		*s = items
	default:
		return fmt.Errorf("expected string or list, got %d", node.Kind)
	}
	return nil
}

// ExpandedJob is a concrete job instance with a specific matrix combination resolved.
type ExpandedJob struct {
	JobKey       string            // original job key in the workflow
	DisplayName  string            // human-readable name (e.g. "build (darwin, arm64)")
	Job          *Job              // reference to the original job definition
	MatrixValues map[string]string // resolved matrix values for this expansion
}

// ParseWorkflow reads and parses a GitHub Actions workflow YAML file.
func ParseWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow %s: %w", path, err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow %s: %w", path, err)
	}
	return &wf, nil
}

// DiscoverWorkflows lists all .yml/.yaml files in .github/workflows/.
func DiscoverWorkflows(projectDir string) ([]string, error) {
	dir := filepath.Join(projectDir, ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("no .github/workflows/ directory found: %w", err)
	}

	var workflows []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			workflows = append(workflows, filepath.Join(dir, name))
		}
	}

	if len(workflows) == 0 {
		return nil, fmt.Errorf("no workflow files found in %s", dir)
	}
	sort.Strings(workflows)
	return workflows, nil
}

// ExpandMatrix takes a job and produces one ExpandedJob per matrix combination.
// If no matrix is defined, returns a single ExpandedJob with empty matrix values.
func ExpandMatrix(jobKey string, job *Job) []ExpandedJob {
	axes := job.Strategy.Matrix.Values

	if len(axes) == 0 {
		// No matrix — single expansion
		return []ExpandedJob{{
			JobKey:       jobKey,
			DisplayName:  jobKey,
			Job:          job,
			MatrixValues: map[string]string{},
		}}
	}

	// Build the cartesian product of all axes
	axisNames := make([]string, 0, len(axes))
	for k := range axes {
		axisNames = append(axisNames, k)
	}
	sort.Strings(axisNames) // deterministic ordering

	combos := cartesian(axisNames, axes)

	// Apply exclude rules
	combos = applyExcludes(combos, job.Strategy.Matrix.Exclude)

	// Apply include rules (additive — each include becomes an extra combo)
	combos = append(combos, job.Strategy.Matrix.Include...)

	var expanded []ExpandedJob
	for _, combo := range combos {
		// Build a display name like "build (darwin, arm64)"
		var parts []string
		for _, k := range axisNames {
			if v, ok := combo[k]; ok {
				parts = append(parts, v)
			}
		}
		displayName := fmt.Sprintf("%s (%s)", jobKey, strings.Join(parts, ", "))

		expanded = append(expanded, ExpandedJob{
			JobKey:       jobKey,
			DisplayName:  displayName,
			Job:          job,
			MatrixValues: combo,
		})
	}

	return expanded
}

// cartesian produces the cartesian product of the given axis values.
func cartesian(axisNames []string, axes map[string][]string) []map[string]string {
	if len(axisNames) == 0 {
		return []map[string]string{{}}
	}

	first := axisNames[0]
	rest := cartesian(axisNames[1:], axes)

	var result []map[string]string
	for _, val := range axes[first] {
		for _, combo := range rest {
			newCombo := make(map[string]string, len(combo)+1)
			for k, v := range combo {
				newCombo[k] = v
			}
			newCombo[first] = val
			result = append(result, newCombo)
		}
	}
	return result
}

// applyExcludes filters out matrix combinations that match any exclude rule.
func applyExcludes(combos []map[string]string, excludes []map[string]string) []map[string]string {
	if len(excludes) == 0 {
		return combos
	}

	var result []map[string]string
	for _, combo := range combos {
		excluded := false
		for _, exc := range excludes {
			if matchesExclude(combo, exc) {
				excluded = true
				break
			}
		}
		if !excluded {
			result = append(result, combo)
		}
	}
	return result
}

func matchesExclude(combo, exclude map[string]string) bool {
	for k, v := range exclude {
		if combo[k] != v {
			return false
		}
	}
	return true
}

// ResolveJobDAG performs a topological sort on jobs based on `needs:` dependencies.
// Returns execution tiers: each tier contains jobs that can run in parallel,
// and all jobs in a tier depend only on jobs in previous tiers.
func ResolveJobDAG(jobs map[string]*Job) ([][]string, error) {
	// Build in-degree map
	inDegree := make(map[string]int, len(jobs))
	for name := range jobs {
		inDegree[name] = 0
	}
	for name, job := range jobs {
		for _, dep := range job.Needs {
			if _, exists := jobs[dep]; !exists {
				return nil, fmt.Errorf("job %q depends on unknown job %q", name, dep)
			}
			inDegree[name]++
		}
	}

	// Kahn's algorithm — identical pattern to internal/orchestrator/dag.go
	remaining := make(map[string]bool, len(jobs))
	for name := range jobs {
		remaining[name] = true
	}

	var tiers [][]string
	for len(remaining) > 0 {
		var tier []string
		for name := range remaining {
			if inDegree[name] == 0 {
				tier = append(tier, name)
			}
		}
		if len(tier) == 0 {
			var cycleNodes []string
			for name := range remaining {
				cycleNodes = append(cycleNodes, name)
			}
			sort.Strings(cycleNodes)
			return nil, fmt.Errorf("dependency cycle detected among jobs: %s", strings.Join(cycleNodes, ", "))
		}

		sort.Strings(tier) // deterministic ordering within tier

		for _, name := range tier {
			delete(remaining, name)
			for depName, depJob := range jobs {
				for _, dep := range depJob.Needs {
					if dep == name {
						inDegree[depName]--
					}
				}
			}
		}
		tiers = append(tiers, tier)
	}

	return tiers, nil
}
