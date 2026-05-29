# Intelligent Failure Recovery

`devx` features an automatic, two-tier failure diagnosis engine hooked into every command. When a command fails with a non-zero exit code, `devx` intercepts the error and attempts to explain the root cause and provide an actionable fix before exiting.

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of Intelligent Failure Recovery.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph Host ["Developer Host / Environment"]
        cli["devx CLI\n(any command)"]
        subgraph DiagnosisEngine ["Failure Diagnosis Engine"]
            interceptor["Error Interceptor\n(non-zero exit code)"]
            subgraph Tier1 ["Tier 1: Pattern Matching"]
                knowledgeBase["Built-in Knowledge Base\n(port conflicts, OOM, expired certs, etc.)"]
                patternMatcher["Pattern Matcher\n(exit code + stderr analysis)"]
            end
            subgraph Tier2 ["Tier 2: AI-Enhanced Analysis"]
                contextCollector["Runtime Context Collector\n(containers, ports, redacted env)"]
                aiAnalyzer["AI Analyzer\n(15s timeout)"]
            end
            fixSuggester["Fix Suggester\n(actionable command output)"]
        end
        localAI["Local AI Provider\n(Ollama / LM Studio)"]
    end

    cli -->|"command fails"| interceptor
    interceptor --> patternMatcher
    patternMatcher -->|"scans"| knowledgeBase
    patternMatcher -->|"match found"| fixSuggester
    patternMatcher -->|"no match"| contextCollector
    contextCollector -->|"runtime state"| aiAnalyzer
    aiAnalyzer -->|"queries"| localAI
    aiAnalyzer -->|"diagnosis"| fixSuggester
    fixSuggester -->|"╭ 💡 Diagnosis ╮"| cli
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["Any devx command fails\n(non-zero exit code)"]) --> JsonFlag{"--json flag\nenabled?"}
    JsonFlag -->|"Yes"| Suppressed(["Diagnosis suppressed\n(clean JSON output only)"])
    JsonFlag -->|"No"| Tier1

    Tier1["Tier 1: Pattern Matching\n(analyze exit code + stderr)"] --> PatternMatch{"Known pattern\nfound?"}
    PatternMatch -->|"Yes"| ShowRuleDiag["Display rule-based diagnosis\nwith suggested fix command\n(rule-based)"]
    ShowRuleDiag --> ExitWithDiag(["Exit with original\nerror code + diagnosis box"])

    PatternMatch -->|"No"| AIAvailable{"Local AI provider\navailable?"}
    AIAvailable -->|"No"| NoAI(["Show standard error\n(no diagnosis)"])

    AIAvailable -->|"Yes"| CollectContext["Collect runtime context\n(container states, port bindings,\nredacted env vars)"]
    CollectContext --> QueryAI["Query local AI\n(Ollama / LM Studio)"]
    QueryAI --> Timeout{"Response within\n15 seconds?"}
    Timeout -->|"No"| TimeoutSkip(["Silently skip diagnosis\nshow standard error"])
    Timeout -->|"Yes"| ShowAIDiag["Display AI-generated diagnosis\nwith suggested fix command\n(ai)"]
    ShowAIDiag --> ExitWithDiag
```

## How it works

The diagnosis engine operates in two tiers:

### Tier 1: Pattern Matching (No AI)
`devx` maintains a built-in knowledge base of common failure modes (e.g., password mismatches, port conflicts, OOMKilled containers, expired certificates). It analyzes the exit code and `stderr` output against this knowledge base.

If a match is found, `devx` immediately prints a diagnosis and a suggested command to fix it. This tier works entirely locally, is instantaneous, and requires zero configuration.

```bash
$ devx db spawn postgres
Error: listen tcp :5432: bind: address already in use

╭───────────────────────────────────────────────────────────────────╮
│ 💡 Diagnosis                                                      │
│                                                                   │
│ Port conflict — another process is already listening on the       │
│ required port.                                                    │
│                                                                   │
│   → lsof -i :<port>   # find the conflicting process, then kill   │
│                                                                   │
│   (rule-based)                                                    │
╰───────────────────────────────────────────────────────────────────╯
```

### Tier 2: AI-Enhanced Analysis
If no pattern matches the error, and a local AI provider is available (such as Ollama or LM Studio), `devx` silently collects the runtime context (container states, port bindings, and redacted environment variables) and requests a custom diagnosis.

```bash
$ devx up
Error: connection refused

╭───────────────────────────────────────────────────────────────────╮
│ 💡 Diagnosis                                                      │
│                                                                   │
│ The 'api' container failed to start because it cannot reach the   │
│ 'postgres' database. The database is currently in an 'exited'     │
│ state, likely due to a misconfigured devx.yaml volume mount.      │
│                                                                   │
│   → devx db rm postgres && devx db spawn postgres                 │
│                                                                   │
│   (ai)                                                            │
╰───────────────────────────────────────────────────────────────────╯
```

To ensure a smooth developer experience, AI calls have a strict 15-second timeout. If the AI is too slow, or if no provider is configured, the diagnosis is silently skipped, allowing the standard error to surface without delay.

## Suppressing Output

The diagnosis engine is automatically disabled when the `--json` flag is used. This ensures that AI agents and scripts parsing structured output do not receive free-form diagnosis text injected into their JSON streams.
