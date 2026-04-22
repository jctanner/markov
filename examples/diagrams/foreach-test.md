# foreach-test

```mermaid
flowchart TD
    subgraph r0["main [8f5a52f3]"]
        r0_process_items["process-items &#10003;"]
        r0_summary["summary &#10003;"]
        r0_process_items --> r0_summary
    end

    class r0_process_items completed
    class r0_summary completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
