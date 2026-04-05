package bridge

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
)

// ServicePortSpec captures a single port from the target Service for dynamic agent generation.
type ServicePortSpec struct {
	Name       string `json:"name"`        // e.g., "http-api", "metrics" (may be empty)
	Port       int    `json:"port"`        // Service port number
	TargetPort string `json:"target_port"` // Container port number or name (e.g., "8080" or "http-api")
	Protocol   string `json:"protocol"`    // Must be "TCP" for 46.2
}

// ServiceInfo captures the full spec of a target Service for intercept planning.
type ServiceInfo struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Type           string            `json:"type"`               // ClusterIP, NodePort, etc.
	Selector       map[string]string `json:"selector"`
	Ports          []ServicePortSpec `json:"ports"`
	HasMeshSidecar bool              `json:"has_mesh_sidecar"`
}

// ServiceState captures the original state of a Service before patching (for restore).
type ServiceState struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	OriginalSelector map[string]string `json:"original_selector"`
}

// InspectService retrieves the full Service spec and validates it for intercept.
func InspectService(kubeconfig, kubeCtx, namespace, service string) (*ServiceInfo, error) {
	args := []string{"get", "service", service, "-n", namespace, "-o", "json"}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil, devxerr.New(devxerr.CodeBridgeServiceNotFound,
			fmt.Sprintf("service %q not found in namespace %q", service, namespace), err)
	}

	var raw struct {
		Metadata struct {
			Name        string            `json:"name"`
			Namespace   string            `json:"namespace"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
		Spec struct {
			Type     string            `json:"type"`
			Selector map[string]string `json:"selector"`
			Ports    []struct {
				Name       string `json:"name"`
				Port       int    `json:"port"`
				TargetPort any    `json:"targetPort"` // can be int or string
				Protocol   string `json:"protocol"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing service %q JSON: %w", service, err)
	}

	info := &ServiceInfo{
		Name:      raw.Metadata.Name,
		Namespace: raw.Metadata.Namespace,
		Type:      raw.Spec.Type,
		Selector:  raw.Spec.Selector,
	}

	// Detect mesh sidecar via annotations
	annotations := raw.Metadata.Annotations
	if annotations != nil {
		if _, ok := annotations["sidecar.istio.io/inject"]; ok {
			info.HasMeshSidecar = true
		}
		if _, ok := annotations["linkerd.io/inject"]; ok {
			info.HasMeshSidecar = true
		}
	}

	for _, p := range raw.Spec.Ports {
		tp := fmt.Sprintf("%v", p.TargetPort) // handle both int and string
		if tp == "0" || tp == "" {
			tp = fmt.Sprintf("%d", p.Port) // default targetPort = port
		}
		protocol := p.Protocol
		if protocol == "" {
			protocol = "TCP"
		}
		info.Ports = append(info.Ports, ServicePortSpec{
			Name:       p.Name,
			Port:       p.Port,
			TargetPort: tp,
			Protocol:   protocol,
		})
	}

	return info, nil
}

// ValidateInterceptable checks that a Service is safe to intercept.
func ValidateInterceptable(info *ServiceInfo) error {
	// Reject ExternalName services
	if info.Type == "ExternalName" {
		return devxerr.New(devxerr.CodeBridgeServiceNotInterceptable,
			fmt.Sprintf("service %q is type ExternalName — cannot intercept (no backing pods)", info.Name), nil)
	}

	// Reject services with no selector
	if len(info.Selector) == 0 {
		return devxerr.New(devxerr.CodeBridgeServiceNotInterceptable,
			fmt.Sprintf("service %q has no selector — cannot intercept (manually managed Endpoints or ExternalName)", info.Name), nil)
	}

	// Reject UDP ports
	for _, p := range info.Ports {
		if strings.EqualFold(p.Protocol, "UDP") {
			return devxerr.New(devxerr.CodeBridgeUnsupportedProtocol,
				fmt.Sprintf("service %q port %d uses UDP — only TCP services can be intercepted in this version", info.Name, p.Port), nil)
		}
	}

	return nil
}

// CheckInterceptConflict checks whether the Service already has an active intercept session.
func CheckInterceptConflict(kubeconfig, kubeCtx, namespace, service string) error {
	args := []string{"get", "service", service, "-n", namespace,
		"-o", "jsonpath={.metadata.annotations.devx-bridge-session}"}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil // annotation doesn't exist — no conflict
	}

	sessionID := strings.TrimSpace(string(out))
	if sessionID != "" {
		return devxerr.New(devxerr.CodeBridgeInterceptActive,
			fmt.Sprintf("service %q is already intercepted by session %s — run 'devx bridge disconnect' first", service, sessionID), nil)
	}
	return nil
}

// PatchServiceSelector replaces the target Service's selector with the agent pod's labels.
// Also sets the annotation devx-bridge-session=<sessionID> for conflict detection.
func PatchServiceSelector(kubeconfig, kubeCtx, namespace, service string, newSelector map[string]string, sessionID string) (*ServiceState, error) {
	// First, save the original selector
	origSelector, err := GetServiceSelector(kubeconfig, kubeCtx, namespace, service)
	if err != nil {
		return nil, err
	}

	state := &ServiceState{
		Name:             service,
		Namespace:        namespace,
		OriginalSelector: origSelector,
	}

	// Build the JSON patch
	selectorJSON, _ := json.Marshal(newSelector)
	patch := fmt.Sprintf(`{"metadata":{"annotations":{"devx-bridge-session":"%s"}},"spec":{"selector":%s}}`, sessionID, string(selectorJSON))

	args := []string{"patch", "service", service, "-n", namespace,
		"--type=merge", "-p", patch}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	if out, err := exec.Command("kubectl", args...).CombinedOutput(); err != nil {
		return nil, devxerr.New(devxerr.CodeBridgeSelectorPatchFailed,
			fmt.Sprintf("failed to patch service %q selector: %s", service, string(out)), err)
	}

	return state, nil
}

// RestoreServiceSelector restores the original Service selector and removes the session annotation.
func RestoreServiceSelector(kubeconfig, kubeCtx string, state *ServiceState) error {
	selectorJSON, _ := json.Marshal(state.OriginalSelector)
	patch := fmt.Sprintf(`{"metadata":{"annotations":{"devx-bridge-session":null}},"spec":{"selector":%s}}`, string(selectorJSON))

	args := []string{"patch", "service", state.Name, "-n", state.Namespace,
		"--type=merge", "-p", patch}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	if out, err := exec.Command("kubectl", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("restoring service %q selector: %s: %w", state.Name, string(out), err)
	}
	return nil
}

// GetServiceSelector returns the current selector of a Service.
func GetServiceSelector(kubeconfig, kubeCtx, namespace, service string) (map[string]string, error) {
	args := []string{"get", "service", service, "-n", namespace,
		"-o", "jsonpath={.spec.selector}"}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kubeCtx != "" {
		args = append(args, "--context", kubeCtx)
	}

	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("getting selector for service %q: %w", service, err)
	}

	var selector map[string]string
	raw := strings.TrimSpace(string(out))
	if raw == "" || raw == "{}" {
		return nil, nil
	}
	if err := json.Unmarshal([]byte(raw), &selector); err != nil {
		return nil, fmt.Errorf("parsing selector for service %q: %w", service, err)
	}
	return selector, nil
}
