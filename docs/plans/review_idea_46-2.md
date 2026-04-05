# Architectural Review: Idea 46.2 (Inbound Interception)

I have deeply reviewed the implementation plan against Kubernetes networking invariants, our system architecture, and our `planning-requirements.md` criteria. 

While the "Selector Swap" and K8s Job approaches are generally sound for a Client-Driven Architecture, there are several **critical technical gaps** that render the execution unworkable in its current form. 

Here is my gap analysis and the required remediations before we can execute this plan.

## CRITICAL GAPS

### 1. The "Reverse Port-Forward" Illusion (Hand-Waving)
**The Flaw:** The plan states we will: *"Establish reverse `kubectl port-forward` from agent pod to local machine"*. **Kubernetes `kubectl` does not natively support reverse port-forwarding.** It only forwards local ports to remote pods (Local-to-Cluster).
**The Impact:** The agent pod will receive traffic but have no mechanism to push it down to the developer's local machine.
**Resolution Required:** We must explicitly define a multiplexed tunnel layer.
1. The local `devx` CLI establishes a standard `kubectl port-forward <agent-pod> <random-local-port>:<agent-control-port>`.
2. The `devx` CLI dials that local port, creating a TCP connection *to* the agent.
3. We run a multiplexer (e.g., `hashicorp/yamux`) over this single TCP stream.
4. When the agent receives a request from a cluster client, it creates a new Yamux stream over the existing connection back to the `devx` CLI.
5. The `devx` CLI opens a connection to the local app (e.g., `localhost:8080`) and proxies the bytes.

### 2. Multi-Port Service Blackholing & Named Ports
**The Flaw:** Kubernetes Services often expose multiple ports or use named ports (`targetPort: http-api`). 
**The Impact:** 
- If we swap the Service selector, **ALL** ports for that Service will be routed to the Agent. If the Agent only listens on our one intercepted port (e.g., `8080`), requests to any other port (e.g., `9090` metrics) will be forcibly dropped / connection refused.
- If the Service uses `targetPort: http-api`, the Endpoints controller searches the target pod (our Agent Job) for a `containerPort` named `http-api`. If the generic Agent image doesn't define it, the Endpoint will fail to register, breaking the intercept entirely.
**Resolution Required:** The `devx` CLI must dynamically generate the Agent Job's Pod Spec to mirror the **exact `containerPorts` (with names)** of the original Service. The Agent binary must accept dynamic port configurations (via args) so it knows to listen on and forward the intended port, while optionally blackholing or proxying un-intercepted ports gracefully.

### 3. UDP Traffic Support
**The Flaw:** The plan only mentions TCP, but Kubernetes Services can support UDP. Yamux over a TCP `kubectl port-forward` tunnel cannot natively transport UDP packets without encapsulation.
**Resolution Required:** Explicit validation in `bridge_intercept.go` must check the Service definition. If `protocol: UDP` is detected, it must fail fast with `CodeBridgeUnsupportedProtocol` unless we build a UDP encapsulator into the Agent.

### 4. Headless Services & Service Types
**The Flaw:** The selector swap assumes standard `ClusterIP` behavior.
**The Impact:** If the target is an `ExternalName` service, or a Service without a selector (manually managed Endpoints), the swap mechanism fails completely or silently does nothing.
**Resolution Required:** `devx bridge intercept` must validate `spec.selector` is not empty, and `spec.type` is not `ExternalName` before proceeding.

### 5. Crash Recovery Fragility
**The Flaw:** If the developer's laptop dies (or `devx` process is `SIGKILL`'d mid-intercept), the Service selector remains patched. All staging traffic continues hitting the (now useless) Agent pod. The plan mentions `devx bridge disconnect --force` and identifying orphaned agents on `devx bridge status`, but manual intervention is required.
**The Impact:** High-severity outage for the staging service.
**Resolution Required:** The Agent Pod must be given a narrowly-scoped `ServiceAccount` with RBAC permission to `update` the target Service. The `devx` CLI passes the original selector to the Agent via an Environment Variable. If the Agent loses its Yamux connection to the local machine (or receives SIGTERM via `activeDeadlineSeconds`), **the Agent restores the selector itself** before exiting. This makes the system self-healing.

---

## Action Plan

Do you agree with these findings? If so, I will update the `implementation_plan.md` to explicitly incorporate the **Yamux reverse-tunneling protocol**, **dynamic port generation**, and the **self-healing RBAC Agent pattern** before we proceed to execution.
