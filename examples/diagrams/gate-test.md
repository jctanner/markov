# gate-test

```mermaid
flowchart TD
    subgraph r0["main [fea37eb4]"]
        r0_triage_gate["triage_gate &#10003;"]
        r0_verify_triage["verify_triage &#10003;"]
        r0_run_analysis["run_analysis &#10003;"]
        r0_notify_reviewer["notify_reviewer &#10003;"]
        r0_triage_gate --> r0_verify_triage
        r0_verify_triage --> r0_run_analysis
        r0_run_analysis --> r0_notify_reviewer
    end

    class r0_triage_gate completed
    class r0_verify_triage completed
    class r0_run_analysis completed
    class r0_notify_reviewer completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
