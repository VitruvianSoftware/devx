package ci

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VitruvianSoftware/devx/internal/devcontainer"
)

// RunResult holds the outcome of a single job execution.
type RunResult struct {
	Job      string        `json:"job"`
	Status   string        `json:"status"` // "passed", "failed", "skipped"
	Duration time.Duration `json:"duration"`
	Steps    []StepResult  `json:"steps"`
}

// StepResult holds the outcome of a single step.
type StepResult struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"` // "passed", "failed", "skipped"
	Duration time.Duration `json:"duration"`
	Output   string        `json:"output,omitempty"`
}

// ExecuteConfig holds configuration for a CI execution run.
type ExecuteConfig struct {
	Workflow    *Workflow
	Runtime     string            // "podman" or "docker"
	Image       string            // container image override (empty = auto-detect)
	JobFilter   []string          // run only these jobs (empty = all)
	Secrets     map[string]string // secrets from devx Vault
	DryRun      bool
	JSONOutput  bool
	ProjectDir  string
}

// Execute runs the parsed workflow according to the execution plan.
func Execute(cfg ExecuteConfig) ([]RunResult, error) {
	// Resolve the base image
	image := cfg.Image
	if image == "" {
		image = resolveImage(cfg.ProjectDir)
	}

	// Resolve job DAG
	tiers, err := ResolveJobDAG(cfg.Workflow.Jobs)
	if err != nil {
		return nil, fmt.Errorf("resolving job dependencies: %w", err)
	}

	// Expand all jobs with their matrix
	type tierEntry struct {
		expanded []ExpandedJob
	}
	var executionPlan [][]ExpandedJob

	for _, tier := range tiers {
		var tierJobs []ExpandedJob
		for _, jobKey := range tier {
			job := cfg.Workflow.Jobs[jobKey]

			// Apply job filter
			if len(cfg.JobFilter) > 0 && !stringInSlice(jobKey, cfg.JobFilter) {
				continue
			}

			expanded := ExpandMatrix(jobKey, job)
			tierJobs = append(tierJobs, expanded...)
		}
		if len(tierJobs) > 0 {
			executionPlan = append(executionPlan, tierJobs)
		}
	}

	if cfg.DryRun {
		return printDryRun(executionPlan, cfg), nil
	}

	// Execute tiers sequentially; jobs within a tier run in parallel
	var allResults []RunResult
	outputMu := &sync.Mutex{}

	for tierIdx, tierJobs := range executionPlan {
		if !cfg.JSONOutput {
			fmt.Printf("\n📋 Tier %d — %d job(s)\n", tierIdx+1, len(tierJobs))
		}

		var wg sync.WaitGroup
		results := make([]RunResult, len(tierJobs))

		for i, ej := range tierJobs {
			wg.Add(1)
			go func(idx int, expanded ExpandedJob) {
				defer wg.Done()
				results[idx] = executeJob(cfg, expanded, image, outputMu)
			}(i, ej)
		}

		wg.Wait()

		// Check for failures (unless continue-on-error)
		for _, r := range results {
			allResults = append(allResults, r)
			if r.Status == "failed" {
				// Check if the original job has continue-on-error
				// For now, we fail fast on any job failure
				if !cfg.JSONOutput {
					fmt.Printf("\n❌ Job %q failed\n", r.Job)
				}
			}
		}
	}

	// Print summary
	if cfg.JSONOutput {
		b, _ := json.MarshalIndent(allResults, "", "  ")
		fmt.Println(string(b))
	} else {
		printSummary(allResults)
	}

	// Return non-nil error if any job failed
	for _, r := range allResults {
		if r.Status == "failed" {
			return allResults, fmt.Errorf("CI run failed: job %q did not pass", r.Job)
		}
	}
	return allResults, nil
}

// executeJob runs a single expanded job inside a container.
func executeJob(cfg ExecuteConfig, ej ExpandedJob, image string, outputMu *sync.Mutex) RunResult {
	start := time.Now()
	displayName := CondensedMatrixName(ej.JobKey, ej.MatrixValues)

	result := RunResult{
		Job:    ej.DisplayName,
		Status: "passed",
	}

	// Build merged environment: workflow env → job env → matrix env
	mergedEnv := MergeEnv(cfg.Workflow.Env, ej.Job.Env)

	// Build template context
	tc := NewTemplateContext(mergedEnv, cfg.Secrets, ej.MatrixValues)

	// Evaluate job-level if: conditional
	if !tc.EvaluateCondition(ej.Job.If) {
		result.Status = "skipped"
		result.Duration = time.Since(start)
		return result
	}

	// Create the container
	containerName := fmt.Sprintf("devx-ci-%s-%d", sanitizeName(displayName), time.Now().UnixNano()%100000)

	// Build container env args
	var envArgs []string
	for k, v := range mergedEnv {
		resolved := tc.Substitute(v)
		envArgs = append(envArgs, "-e", fmt.Sprintf("%s=%s", k, resolved))
	}
	// Inject matrix values as env vars (GitHub does this)
	for k, v := range ej.MatrixValues {
		envArgs = append(envArgs, "-e", fmt.Sprintf("MATRIX_%s=%s", strings.ToUpper(k), v))
	}

	// Create container (run in background, bind-mount workspace)
	createArgs := []string{
		"run", "-d",
		"--name", containerName,
		"-v", fmt.Sprintf("%s:/workspace", cfg.ProjectDir),
		"-w", "/workspace",
	}
	createArgs = append(createArgs, envArgs...)
	createArgs = append(createArgs, image, "sleep", "infinity")

	writer := NewPrefixedWriter(displayName, os.Stdout, outputMu)

	cmd := exec.Command(cfg.Runtime, createArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		result.Status = "failed"
		result.Steps = append(result.Steps, StepResult{
			Name:   "container-create",
			Status: "failed",
			Output: fmt.Sprintf("failed to create container: %v\n%s", err, string(out)),
		})
		result.Duration = time.Since(start)
		return result
	}

	// Ensure cleanup
	defer func() {
		_ = exec.Command(cfg.Runtime, "rm", "-f", containerName).Run()
	}()

	// Execute steps
	for _, step := range ej.Job.Steps {
		stepResult := executeStep(cfg, step, containerName, tc, writer)
		result.Steps = append(result.Steps, stepResult)

		if stepResult.Status == "failed" && !step.ContinueOnError {
			result.Status = "failed"
			break
		}
	}

	// Print template warnings
	for _, w := range tc.Warnings {
		fmt.Fprintf(writer, "%s\n", w)
	}
	writer.Flush()

	result.Duration = time.Since(start)
	return result
}

// executeStep runs a single step inside the container.
func executeStep(cfg ExecuteConfig, step Step, containerName string, tc *TemplateContext, writer *PrefixedWriter) StepResult {
	start := time.Now()
	stepName := step.Name
	if stepName == "" && step.Run != "" {
		// Use first line of run block as name
		lines := strings.SplitN(step.Run, "\n", 2)
		stepName = lines[0]
		if len(stepName) > 60 {
			stepName = stepName[:57] + "..."
		}
	}
	if stepName == "" {
		stepName = step.Uses
	}

	result := StepResult{
		Name:   stepName,
		Status: "passed",
	}

	// Check step-level if: conditional
	stepEnv := MergeEnv(tc.Env, step.Env)
	stepTC := NewTemplateContext(stepEnv, tc.Secrets, tc.Matrix)

	if !stepTC.EvaluateCondition(step.If) {
		result.Status = "skipped"
		result.Duration = time.Since(start)
		return result
	}

	// LIMITATION: Skip uses: actions with visible warning
	if step.Uses != "" {
		fmt.Fprintf(writer, "⚠️  SKIPPED (uses: %s) — devx ci run only executes run: blocks\n", step.Uses)
		result.Status = "skipped"
		result.Duration = time.Since(start)
		return result
	}

	if step.Run == "" {
		result.Status = "skipped"
		result.Duration = time.Since(start)
		return result
	}

	// Template-substitute the run block
	runBlock := stepTC.Substitute(step.Run)

	// Determine shell
	shell := step.Shell
	if shell == "" {
		shell = "bash"
	}

	// Build exec command
	execArgs := []string{"exec"}

	// Working directory
	wd := "/workspace"
	if step.WorkingDirectory != "" {
		wd = filepath.Join("/workspace", stepTC.Substitute(step.WorkingDirectory))
	}
	execArgs = append(execArgs, "-w", wd)

	// Step-level env vars
	for k, v := range step.Env {
		resolved := stepTC.Substitute(v)
		execArgs = append(execArgs, "-e", fmt.Sprintf("%s=%s", k, resolved))
	}

	execArgs = append(execArgs, containerName, shell, "-e", "-c", runBlock)

	// Set up timeout
	var ctx context.Context
	var cancel context.CancelFunc
	if step.TimeoutMinutes > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(step.TimeoutMinutes)*time.Minute)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Minute) // default 30m
	}
	defer cancel()

	fmt.Fprintf(writer, "▶ %s\n", stepName)

	cmd := exec.CommandContext(ctx, cfg.Runtime, execArgs...)
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Run(); err != nil {
		result.Status = "failed"
		result.Output = err.Error()
		if step.ContinueOnError {
			fmt.Fprintf(writer, "⚠️  Step failed but continue-on-error is set — continuing\n")
			result.Status = "passed" // treat as soft failure
		} else {
			fmt.Fprintf(writer, "❌ Step failed: %v\n", err)
		}
	}

	result.Duration = time.Since(start)
	return result
}

// resolveImage determines the container image to use.
func resolveImage(projectDir string) string {
	// Try devcontainer.json first
	cfg, _, err := devcontainer.Load(projectDir)
	if err == nil && cfg.Image != "" {
		fmt.Printf("🐳 Using devcontainer image: %s\n", cfg.Image)
		return cfg.Image
	}

	fmt.Println("ℹ️  No devcontainer.json found — using ubuntu:latest")
	fmt.Println("   Tip: Tools like Go, Node, etc. may not be pre-installed.")
	fmt.Println("   Use --image to specify a richer base image.")
	return "ubuntu:latest"
}

// printDryRun displays the execution plan without running anything.
func printDryRun(plan [][]ExpandedJob, cfg ExecuteConfig) []RunResult {
	if cfg.JSONOutput {
		type dryRunJob struct {
			Tier   int               `json:"tier"`
			Job    string            `json:"job"`
			Matrix map[string]string `json:"matrix,omitempty"`
			Steps  []string          `json:"steps"`
		}
		var jobs []dryRunJob
		for tierIdx, tier := range plan {
			for _, ej := range tier {
				var steps []string
				for _, s := range ej.Job.Steps {
					if s.Uses != "" {
						steps = append(steps, fmt.Sprintf("[SKIP uses: %s]", s.Uses))
					} else if s.Run != "" {
						lines := strings.SplitN(s.Run, "\n", 2)
						steps = append(steps, lines[0])
					}
				}
				jobs = append(jobs, dryRunJob{
					Tier:   tierIdx + 1,
					Job:    ej.DisplayName,
					Matrix: ej.MatrixValues,
					Steps:  steps,
				})
			}
		}
		b, _ := json.MarshalIndent(jobs, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Println("\n🔍 Dry-run execution plan:")
	fmt.Println(strings.Repeat("─", 60))

	for tierIdx, tier := range plan {
		fmt.Printf("\n📋 Tier %d (parallel)\n", tierIdx+1)
		for _, ej := range tier {
			fmt.Printf("\n  📦 %s\n", ej.DisplayName)
			if len(ej.MatrixValues) > 0 {
				fmt.Printf("     Matrix: %v\n", ej.MatrixValues)
			}
			for i, s := range ej.Job.Steps {
				if s.Uses != "" {
					fmt.Printf("     %d. ⚠️  [SKIP] uses: %s\n", i+1, s.Uses)
				} else if s.Run != "" {
					name := s.Name
					if name == "" {
						lines := strings.SplitN(s.Run, "\n", 2)
						name = lines[0]
					}
					fmt.Printf("     %d. ▶ %s\n", i+1, name)
				}
			}
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	fmt.Println("✅ Dry-run complete. No containers were created.")
	return nil
}

// printSummary shows the final pass/fail report.
func printSummary(results []RunResult) {
	fmt.Printf("\n%s\n", strings.Repeat("─", 60))
	fmt.Println("📊 CI Run Summary")
	fmt.Printf("%s\n", strings.Repeat("─", 60))

	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		icon := "✅"
		switch r.Status {
		case "failed":
			icon = "❌"
			failed++
		case "skipped":
			icon = "⏭️ "
			skipped++
		default:
			passed++
		}
		fmt.Printf("  %s %s  (%s)\n", icon, r.Job, r.Duration.Round(time.Millisecond))
	}

	fmt.Printf("\n  Total: %d passed", passed)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	if skipped > 0 {
		fmt.Printf(", %d skipped", skipped)
	}
	fmt.Println()
}

func stringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func sanitizeName(s string) string {
	r := strings.NewReplacer(" ", "-", "(", "", ")", "", ",", "", "·", "-")
	return strings.ToLower(r.Replace(s))
}
