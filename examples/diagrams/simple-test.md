# simple-test

```mermaid
flowchart TD
    subgraph r0["hello [9684e81f]"]
        r0_say_hello["say-hello &#10003;"]
        r0_show_date["show-date &#10003;"]
        r0_combine["combine &#10003;"]
        r0_say_hello --> r0_show_date
        r0_show_date --> r0_combine
    end

    class r0_say_hello completed
    class r0_show_date completed
    class r0_combine completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
