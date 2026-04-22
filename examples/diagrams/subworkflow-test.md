# subworkflow-test

```mermaid
flowchart TD
    subgraph r0["main [4f2a06f6]"]
        r0_fan_out["fan-out &#10003;"]
        r0_done["done &#10003;"]
        r0_fan_out --> r0_done
    end
    subgraph r1["per-item [4f2a06f6-fan-out-1]"]
        r1_step_one["step-one &#10003;"]
        r1_step_two["step-two &#10003;"]
        r1_step_one --> r1_step_two
    end
    r0_fan_out --> r1_step_one

    class r0_fan_out completed
    class r0_done completed
    class r1_step_one completed
    class r1_step_two completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
