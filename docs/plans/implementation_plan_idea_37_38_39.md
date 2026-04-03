# P1 Polish Pass: Polish & Onboarding (Ideas 37, 38, 39)

This plan outlines the architecture for the "P1 Polish Pass," introducing high-value operational features that solve critical developer experience and security pain points for larger teams.

## Proposed Changes

---

### Phase 1: Idea 37 — Environment Overlays & Profiles
**Goal:** Allow teams to define multiple topology footprints (e.g., lightweight local vs. full integration) within a single `devx.yaml` using Skaffold-inspired `--profile` overrides.

#### [MODIFY] `cmd/up.go`
- Add a new global or command-specific flag: `--profile string`.
- Update `DevxConfig` parsing logic to support a new `profiles:` map in `devx.yaml`.
- When `--profile` is provided, `devx` will intelligently merge the profile's specific `services`, `databases`, and `tunnels` overrides on top of the base configuration before generating the DAG.
- **Merge Behavior (Docker Compose Style):** The merging will use an **additive/merge** model. If a profile specifies a service that already exists in the base config, the fields will be merged (with the profile taking precedence). If it specifies a new service, it will be added to the topology. This additive design maintains developer familiarity.

#### [MODIFY] `devx.yaml.example`
- Provide an example of a `staging` or `backend-only` profile demonstrating how to conditionally include/exclude services based on the active profile.

---

### Phase 2: Idea 38 — Native Secrets Redaction in Logs
**Goal:** Prevent accidental credential leaks during screenshares by leveraging `devx`'s central knowledge of injected secrets and applying middleware redaction to the Bubble Tea TUI.

#### [NEW] `internal/logs/redactor.go`
- Implement a `SecretRedactor` struct initialized with all known environment variables and vault secrets.
- Add an exact-match sweeping function that takes a raw log string and replaces sensitive values with a high-contrast `[REDACTED]` lipgloss-styled string.

#### [MODIFY] `internal/logs/streamer.go` & `cmd/logs.go`
- Inject the redactor into the log multiplexing pipeline.
- All stdout/stderr lines captured from containers and native host processes will be scrubbed before being emitted to the Bubble Tea interface or JSON agent stream.

---

### Phase 3: Idea 39 — Visual Architecture Map Generator
**Goal:** Accelerate developer onboarding by auto-generating architectural topology maps directly from the `devx.yaml` configuration graph.

#### [NEW] `cmd/map.go`
- Introduce a new top-level `devx map` shell command.
- Parses the validated `devx.yaml` dependency graph (leveraging the DAG code from Idea 34).
- Emits a fully valid, interconnected Mermaid.js flowchart (markdown standard) to stdout or into a `<project_name>-topology.md` file, visually representing the network exposure, databases, and service dependencies.

#### [MODIFY] `cmd/root.go`
- Register `mapCmd` into the global Cobra command list.

---

## Open Questions

*(No open questions currently. The additive/merge model for profile overrides has been decided and approved.)*

## Verification Plan

### Automated Tests
- Create unit tests for YAML profile merging to ensure base configs properly inherit overrides.
- Create unit tests for `redactor.go` to ensure deterministic string replacement doesn't leak substrings or fail on overlapping secrets.
- Add command tests ensuring `devx map` accurately generates Mermaid syntax.

### Environment Verification
- Run a dummy `devx logs` stream containing mock `.env` patterns to visually verify TUIs correctly wipe the screen content.
