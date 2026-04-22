# loop-test

```mermaid
flowchart TD
    subgraph r0["main [4a697cbf]"]
        r0_start["start &#10003;"]
        r0_run_loop["run_loop &#10003;"]
        r0_finish["finish &#10003;"]
        r0_start --> r0_run_loop
        r0_run_loop --> r0_finish
    end
    subgraph r1["iterate [4a697cbf-run_loop]"]
        r1_loop_gate["loop_gate &#10003;"]
        r1_run_work["run_work &#10003;"]
        r1_increment["increment &#10003;"]
        r1_next_iteration["next_iteration &#10003;"]
        r1_loop_gate --> r1_run_work
        r1_run_work --> r1_increment
        r1_increment --> r1_next_iteration
    end
    subgraph r2["do_work [4a697cbf-run_loop-run_work]"]
        r2_show_iteration["show_iteration &#10003;"]
    end
    r1_run_work --> r2_show_iteration
    subgraph r3["iterate [4a697cbf-run_loop-next_iteration]"]
        r3_loop_gate["loop_gate &#10003;"]
        r3_run_work["run_work &#10003;"]
        r3_increment["increment &#10003;"]
        r3_next_iteration["next_iteration &#10003;"]
        r3_loop_gate --> r3_run_work
        r3_run_work --> r3_increment
        r3_increment --> r3_next_iteration
    end
    subgraph r4["do_work [4a697cbf-run_loop-next_iteration-run_work]"]
        r4_show_iteration["show_iteration &#10003;"]
    end
    r3_run_work --> r4_show_iteration
    subgraph r5["iterate [4a697cbf-run_loop-next_iteration-next_iteration]"]
        r5_loop_gate["loop_gate &#10003;"]
        r5_run_work["run_work &#10003;"]
        r5_increment["increment &#10003;"]
        r5_next_iteration["next_iteration &#10003;"]
        r5_loop_gate --> r5_run_work
        r5_run_work --> r5_increment
        r5_increment --> r5_next_iteration
    end
    subgraph r6["do_work [4a697cbf-run_loop-next_iteration-next_iteration-run_work]"]
        r6_show_iteration["show_iteration &#10003;"]
    end
    r5_run_work --> r6_show_iteration
    subgraph r7["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration]"]
        r7_loop_gate["loop_gate &#10003;"]
        r7_run_work["run_work &#10003;"]
        r7_increment["increment &#10003;"]
        r7_next_iteration["next_iteration &#10003;"]
        r7_loop_gate --> r7_run_work
        r7_run_work --> r7_increment
        r7_increment --> r7_next_iteration
    end
    subgraph r8["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-run_work]"]
        r8_show_iteration["show_iteration &#10003;"]
    end
    r7_run_work --> r8_show_iteration
    subgraph r9["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r9_loop_gate["loop_gate &#10003;"]
        r9_run_work["run_work &#10003;"]
        r9_increment["increment &#10003;"]
        r9_next_iteration["next_iteration &#10003;"]
        r9_loop_gate --> r9_run_work
        r9_run_work --> r9_increment
        r9_increment --> r9_next_iteration
    end
    subgraph r10["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r10_show_iteration["show_iteration &#10003;"]
    end
    r9_run_work --> r10_show_iteration
    subgraph r11["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r11_loop_gate["loop_gate &#10003;"]
        r11_run_work["run_work &#10003;"]
        r11_increment["increment &#10003;"]
        r11_next_iteration["next_iteration &#10003;"]
        r11_loop_gate --> r11_run_work
        r11_run_work --> r11_increment
        r11_increment --> r11_next_iteration
    end
    subgraph r12["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r12_show_iteration["show_iteration &#10003;"]
    end
    r11_run_work --> r12_show_iteration
    subgraph r13["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r13_loop_gate["loop_gate &#10003;"]
        r13_run_work["run_work &#10003;"]
        r13_increment["increment &#10003;"]
        r13_next_iteration["next_iteration &#10003;"]
        r13_loop_gate --> r13_run_work
        r13_run_work --> r13_increment
        r13_increment --> r13_next_iteration
    end
    subgraph r14["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r14_loop_gate["loop_gate &#10003;"]
        r14_run_work["run_work &#10003;"]
        r14_increment["increment &#10003;"]
        r14_next_iteration["next_iteration &#10003;"]
        r14_loop_gate --> r14_run_work
        r14_run_work --> r14_increment
        r14_increment --> r14_next_iteration
    end
    subgraph r15["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r15_show_iteration["show_iteration &#10003;"]
    end
    r14_run_work --> r15_show_iteration
    subgraph r16["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r16_loop_gate["loop_gate &#10003;"]
        r16_run_work["run_work &#10003;"]
        r16_increment["increment &#10003;"]
        r16_next_iteration["next_iteration &#10003;"]
        r16_loop_gate --> r16_run_work
        r16_run_work --> r16_increment
        r16_increment --> r16_next_iteration
    end
    subgraph r17["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r17_show_iteration["show_iteration &#10003;"]
    end
    r16_run_work --> r17_show_iteration
    subgraph r18["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r18_loop_gate["loop_gate &#10003;"]
        r18_run_work["run_work &#10003;"]
        r18_increment["increment &#10003;"]
        r18_next_iteration["next_iteration &#10003;"]
        r18_loop_gate --> r18_run_work
        r18_run_work --> r18_increment
        r18_increment --> r18_next_iteration
    end
    subgraph r19["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r19_show_iteration["show_iteration &#10003;"]
    end
    r18_run_work --> r19_show_iteration
    subgraph r20["iterate [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration]"]
        r20_loop_gate["loop_gate &#10003;"]
        r20_run_work["run_work &#9675;"]
        r20_increment["increment &#9675;"]
        r20_next_iteration["next_iteration &#9675;"]
        r20_loop_gate --> r20_run_work
        r20_run_work --> r20_increment
        r20_increment --> r20_next_iteration
    end
    r18_next_iteration --> r20_loop_gate
    r16_next_iteration --> r18_loop_gate
    r14_next_iteration --> r16_loop_gate
    r13_next_iteration --> r14_loop_gate
    subgraph r21["do_work [4a697cbf-run_loop-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-next_iteration-run_work]"]
        r21_show_iteration["show_iteration &#10003;"]
    end
    r13_run_work --> r21_show_iteration
    r11_next_iteration --> r13_loop_gate
    r9_next_iteration --> r11_loop_gate
    r7_next_iteration --> r9_loop_gate
    r5_next_iteration --> r7_loop_gate
    r3_next_iteration --> r5_loop_gate
    r1_next_iteration --> r3_loop_gate
    r0_run_loop --> r1_loop_gate

    class r0_start completed
    class r0_run_loop completed
    class r0_finish completed
    class r1_loop_gate completed
    class r1_run_work completed
    class r1_increment completed
    class r1_next_iteration completed
    class r2_show_iteration completed
    class r3_loop_gate completed
    class r3_run_work completed
    class r3_increment completed
    class r3_next_iteration completed
    class r4_show_iteration completed
    class r5_loop_gate completed
    class r5_run_work completed
    class r5_increment completed
    class r5_next_iteration completed
    class r6_show_iteration completed
    class r7_loop_gate completed
    class r7_run_work completed
    class r7_increment completed
    class r7_next_iteration completed
    class r8_show_iteration completed
    class r9_loop_gate completed
    class r9_run_work completed
    class r9_increment completed
    class r9_next_iteration completed
    class r10_show_iteration completed
    class r11_loop_gate completed
    class r11_run_work completed
    class r11_increment completed
    class r11_next_iteration completed
    class r12_show_iteration completed
    class r13_loop_gate completed
    class r13_run_work completed
    class r13_increment completed
    class r13_next_iteration completed
    class r14_loop_gate completed
    class r14_run_work completed
    class r14_increment completed
    class r14_next_iteration completed
    class r15_show_iteration completed
    class r16_loop_gate completed
    class r16_run_work completed
    class r16_increment completed
    class r16_next_iteration completed
    class r17_show_iteration completed
    class r18_loop_gate completed
    class r18_run_work completed
    class r18_increment completed
    class r18_next_iteration completed
    class r19_show_iteration completed
    class r20_loop_gate completed
    class r20_run_work skipped
    class r20_increment skipped
    class r20_next_iteration skipped
    class r21_show_iteration completed

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
