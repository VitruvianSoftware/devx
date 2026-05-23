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

package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// startCloudRunNode deploys a runtime: cloud service to Google Cloud Run via the
// gcloud CLI (skaffold-class Cloud Run deploy). Like the kubernetes deployer, devx
// shells out — here to `gcloud run deploy` — rather than using the GCP Go SDK, and
// blocks until the service is deployed, so the DAG's generic healthcheck is skipped.
// project, region, and image are required (no implicit default project), so a
// Cloud Run deploy can never accidentally target the wrong account.
func startCloudRunNode(ctx context.Context, n *Node) error {
	c := n.CloudRun
	if c == nil || c.Image == "" || c.Region == "" || c.Project == "" {
		return fmt.Errorf("service %q has runtime: cloud but cloud_run.image, .region, and .project are all required", n.Name)
	}
	if err := validateGcloud(); err != nil {
		return fmt.Errorf("service %q: %w", n.Name, err)
	}
	service := c.Service
	if service == "" {
		service = n.Name
	}

	fmt.Printf("  ☁️  Deploying %s → Cloud Run %q (%s / %s)\n", n.Name, service, c.Project, c.Region)
	if out, err := runGcloud(ctx, cloudRunDeployArgs(c, service)...); err != nil {
		return fmt.Errorf("gcloud run deploy for %q failed: %w\n%s", n.Name, err, strings.TrimSpace(out))
	}

	// Record what we deployed so cleanup can delete it.
	n.crDeployed = c

	if url, err := cloudRunURL(ctx, c.Project, c.Region, service); err == nil && url != "" {
		fmt.Printf("  🌐 %s available at %s\n", n.Name, url)
	}
	fmt.Printf("  ✅ %s deployed\n", n.Name)
	return nil
}

// deleteCloudRunNode removes the Cloud Run service startCloudRunNode deployed
// (best-effort, on shutdown).
func deleteCloudRunNode(n *Node) {
	c := n.crDeployed
	if c == nil {
		return
	}
	service := c.Service
	if service == "" {
		service = n.Name
	}
	args := []string{"run", "services", "delete", service, "--region", c.Region, "--project", c.Project, "--quiet"}
	if out, err := runGcloud(context.Background(), args...); err != nil {
		fmt.Printf("  ⚠️  cleanup: gcloud run services delete for %s failed: %s\n", n.Name, strings.TrimSpace(out))
	}
}

// cloudRunDeployArgs builds the `gcloud run deploy` argument list. Pure function,
// separated for unit-testing the arg shape.
func cloudRunDeployArgs(c *CloudRunNodeConfig, service string) []string {
	args := []string{
		"run", "deploy", service,
		"--image", c.Image,
		"--region", c.Region,
		"--project", c.Project,
		"--platform", "managed",
		"--quiet",
	}
	if len(c.Env) > 0 {
		args = append(args, "--set-env-vars", setEnvVarsArg(c.Env))
	}
	if c.AllowUnauthenticated {
		args = append(args, "--allow-unauthenticated")
	} else {
		args = append(args, "--no-allow-unauthenticated")
	}
	return append(args, c.Flags...)
}

// setEnvVarsArg renders env as a deterministic (key-sorted) gcloud
// --set-env-vars value: "K1=V1,K2=V2". Values must not contain commas.
func setEnvVarsArg(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+env[k])
	}
	return strings.Join(parts, ",")
}

// cloudRunURL returns the deployed service's public URL.
func cloudRunURL(ctx context.Context, project, region, service string) (string, error) {
	out, err := runGcloud(ctx, "run", "services", "describe", service,
		"--region", region, "--project", project, "--format=value(status.url)")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func runGcloud(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "gcloud", args...).CombinedOutput()
	return string(out), err
}

// validateGcloud ensures the gcloud CLI is available for runtime: cloud.
func validateGcloud() error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("runtime: cloud (Cloud Run) requires the gcloud CLI on your PATH: %w", err)
	}
	return nil
}
