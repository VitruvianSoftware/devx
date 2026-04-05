package bridge

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
)

const (
	// AgentImageDefault is the default agent container image.
	// Override via --agent-image flag or bridge.agent_image in devx.yaml.
	AgentImageDefault = "ghcr.io/VitruvianSoftware/devx-bridge-agent:v0.1.0"

	// AgentControlPort is the Yamux control port inside the agent pod.
	AgentControlPort = 4200

	// AgentHealthPort is the health check port inside the agent pod.
	AgentHealthPort = 4201

	// AgentDefaultDeadline is the default activeDeadlineSeconds for agent Jobs (4 hours).
	AgentDefaultDeadline = 14400
)

// AgentConfig defines the parameters for deploying a bridge agent.
type AgentConfig struct {
	Kubeconfig       string
	Context          string
	Namespace        string
	TargetService    string
	InterceptPort    int               // The specific port being intercepted
	ServicePorts     []ServicePortSpec // ALL ports on the target Service (for dynamic Pod spec)
	OriginalSelector map[string]string // For self-healing: passed to agent via env var
	AgentImage       string
	SessionID        string
	Deadline         int // activeDeadlineSeconds (default: AgentDefaultDeadline)
}

// AgentInfo contains runtime information about a deployed agent.
type AgentInfo struct {
	PodName     string
	PodIP       string
	ControlPort int
	HealthPort  int
	SessionID   string
}

// DeployAgent creates the agent Job, ServiceAccount, Role, and RoleBinding
// in the cluster and waits for the agent to be ready.
func DeployAgent(cfg AgentConfig) (*AgentInfo, error) {
	if cfg.AgentImage == "" {
		cfg.AgentImage = AgentImageDefault
	}
	if cfg.Deadline == 0 {
		cfg.Deadline = AgentDefaultDeadline
	}

	// Generate RBAC + Job YAML
	manifest := generateAgentManifest(cfg)

	// Apply the manifest
	args := []string{"apply", "-f", "-"}
	if cfg.Kubeconfig != "" {
		args = append(args, "--kubeconfig", cfg.Kubeconfig)
	}
	if cfg.Context != "" {
		args = append(args, "--context", cfg.Context)
	}

	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(manifest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, devxerr.New(devxerr.CodeBridgeAgentDeployFailed,
			fmt.Sprintf("failed to deploy agent: %s", string(out)), err)
	}

	// Wait for agent pod to be ready
	podName, err := waitForAgentPod(cfg)
	if err != nil {
		// Clean up on failure
		_ = RemoveAgent(cfg.Kubeconfig, cfg.Context, cfg.Namespace, cfg.SessionID)
		return nil, err
	}

	return &AgentInfo{
		PodName:     podName,
		ControlPort: AgentControlPort,
		HealthPort:  AgentHealthPort,
		SessionID:   cfg.SessionID,
	}, nil
}

// RemoveAgent deletes the agent Job, ServiceAccount, Role, and RoleBinding.
func RemoveAgent(kubeconfig, kubeCtx, namespace, sessionID string) error {
	resources := []string{
		fmt.Sprintf("job/devx-bridge-agent-%s", sessionID),
		fmt.Sprintf("serviceaccount/devx-bridge-agent-%s", sessionID),
		fmt.Sprintf("role/devx-bridge-agent-%s", sessionID),
		fmt.Sprintf("rolebinding/devx-bridge-agent-%s", sessionID),
	}

	for _, res := range resources {
		args := []string{"delete", res, "-n", namespace, "--ignore-not-found"}
		if kubeconfig != "" {
			args = append(args, "--kubeconfig", kubeconfig)
		}
		if kubeCtx != "" {
			args = append(args, "--context", kubeCtx)
		}
		_ = exec.Command("kubectl", args...).Run()
	}
	return nil
}

// waitForAgentPod polls until the agent pod is Running, up to 120s.
func waitForAgentPod(cfg AgentConfig) (string, error) {
	deadline := time.Now().Add(120 * time.Second)
	jobName := fmt.Sprintf("devx-bridge-agent-%s", cfg.SessionID)

	for time.Now().Before(deadline) {
		args := []string{"get", "pods", "-n", cfg.Namespace,
			"-l", fmt.Sprintf("job-name=%s", jobName),
			"-o", "jsonpath={.items[0].metadata.name},{.items[0].status.phase}"}
		if cfg.Kubeconfig != "" {
			args = append(args, "--kubeconfig", cfg.Kubeconfig)
		}
		if cfg.Context != "" {
			args = append(args, "--context", cfg.Context)
		}

		out, err := exec.Command("kubectl", args...).Output()
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(out)), ",", 2)
			if len(parts) == 2 && parts[1] == "Running" {
				return parts[0], nil
			}
			if len(parts) == 2 && parts[1] == "Failed" {
				return "", devxerr.New(devxerr.CodeBridgeAgentDeployFailed,
					fmt.Sprintf("agent pod %q entered Failed state", parts[0]), nil)
			}
		}
		time.Sleep(2 * time.Second)
	}

	return "", devxerr.New(devxerr.CodeBridgeAgentHealthFailed,
		"timed out waiting for agent pod to be Running (120s)", nil)
}

// generateAgentManifest creates the YAML for ServiceAccount, Role, RoleBinding, and Job.
func generateAgentManifest(cfg AgentConfig) string {
	saName := fmt.Sprintf("devx-bridge-agent-%s", cfg.SessionID)
	jobName := saName
	roleName := saName

	// Build dynamic container ports
	var containerPorts []string
	var portArgs []string
	for _, p := range cfg.ServicePorts {
		portDef := fmt.Sprintf(`        - containerPort: %d`, p.Port)
		if p.Name != "" {
			portDef += fmt.Sprintf("\n          name: %s", p.Name)
		}
		containerPorts = append(containerPorts, portDef)
		portArg := fmt.Sprintf("%d", p.Port)
		if p.Name != "" {
			portArg = fmt.Sprintf("%d:%s", p.Port, p.Name)
		}
		portArgs = append(portArgs, portArg)
	}

	// Add control + health ports
	containerPorts = append(containerPorts,
		fmt.Sprintf(`        - containerPort: %d
          name: control`, AgentControlPort),
		fmt.Sprintf(`        - containerPort: %d
          name: health`, AgentHealthPort),
	)

	// JSON-encode original selector for env var
	selectorJSON, _ := json.Marshal(cfg.OriginalSelector)

	manifest := fmt.Sprintf(`---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: %s
  namespace: %s
  labels:
    devx-bridge: agent
    devx-bridge-session: "%s"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %s
  namespace: %s
  labels:
    devx-bridge: agent
    devx-bridge-session: "%s"
rules:
- apiGroups: [""]
  resources: ["services"]
  resourceNames: ["%s"]
  verbs: ["get", "update", "patch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: %s
  labels:
    devx-bridge: agent
    devx-bridge-session: "%s"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: %s
subjects:
- kind: ServiceAccount
  name: %s
  namespace: %s
---
apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    devx-bridge: agent
    devx-bridge-target: "%s"
    devx-bridge-session: "%s"
spec:
  activeDeadlineSeconds: %d
  ttlSecondsAfterFinished: 60
  backoffLimit: 0
  template:
    metadata:
      labels:
        devx-bridge: agent
        devx-bridge-target: "%s"
        devx-bridge-session: "%s"
    spec:
      serviceAccountName: %s
      restartPolicy: Never
      containers:
      - name: agent
        image: %s
        args:
        - "--control-port=%d"
        - "--health-port=%d"
        - "--ports=%s"
        env:
        - name: DEVX_ORIGINAL_SELECTOR
          value: '%s'
        - name: DEVX_TARGET_SERVICE
          value: "%s"
        - name: DEVX_TARGET_NAMESPACE
          value: "%s"
        ports:
%s
        readinessProbe:
          httpGet:
            path: /healthz
            port: %d
          initialDelaySeconds: 2
          periodSeconds: 5
`,
		// ServiceAccount
		saName, cfg.Namespace, cfg.SessionID,
		// Role
		roleName, cfg.Namespace, cfg.SessionID, cfg.TargetService,
		// RoleBinding
		roleName, cfg.Namespace, cfg.SessionID, roleName, saName, cfg.Namespace,
		// Job metadata
		jobName, cfg.Namespace, cfg.TargetService, cfg.SessionID,
		cfg.Deadline,
		// Pod template
		cfg.TargetService, cfg.SessionID, saName,
		// Container
		cfg.AgentImage, AgentControlPort, AgentHealthPort,
		strings.Join(portArgs, ","),
		string(selectorJSON), cfg.TargetService, cfg.Namespace,
		strings.Join(containerPorts, "\n"),
		AgentHealthPort,
	)

	return manifest
}
