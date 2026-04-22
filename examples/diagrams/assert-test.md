# assert-test

```mermaid
flowchart TD
    subgraph r0["main [464936ad]"]
        r0_check_preconditions["check_preconditions &#10003;"]
        r0_check_high_score["check_high_score &#10003;"]
        r0_done["done &#10003;"]
        r0_check_preconditions --> r0_check_high_score
        r0_check_high_score --> r0_done
    end

    class r0_check_preconditions completed
    class r0_check_high_score completed
    class r0_done completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
