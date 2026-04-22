# pipeline-full

```mermaid
flowchart TD
    subgraph r0["main [5ae79f53]"]
        r0_init["init &#10003;"]
        r0_scan["scan &#10003;"]
        r0_process_all["process_all &#10003;"]
        r0_report["report &#10003;"]
        r0_init --> r0_scan
        r0_scan --> r0_process_all
        r0_process_all --> r0_report
    end
    subgraph r1["process_target [5ae79f53-process_all-0]"]
        r1_prepare["prepare &#10003;"]
        r1_run_stages["run_stages &#10003;"]
        r1_finalize["finalize &#10003;"]
        r1_prepare --> r1_run_stages
        r1_run_stages --> r1_finalize
    end
    subgraph r2["stage_loop [5ae79f53-process_all-0-run_stages]"]
        r2_stage_gate["stage_gate &#10003;"]
        r2_validate["validate &#10003;"]
        r2_transform["transform &#10003;"]
        r2_verify["verify &#10003;"]
        r2_advance_stage["advance_stage &#10003;"]
        r2_next_stage["next_stage &#10003;"]
        r2_stage_gate --> r2_validate
        r2_validate --> r2_transform
        r2_transform --> r2_verify
        r2_verify --> r2_advance_stage
        r2_advance_stage --> r2_next_stage
    end
    subgraph r3["stage_loop [5ae79f53-process_all-0-run_stages-next_stage]"]
        r3_stage_gate["stage_gate &#10003;"]
        r3_validate["validate &#10003;"]
        r3_transform["transform &#10003;"]
        r3_verify["verify &#10003;"]
        r3_advance_stage["advance_stage &#10003;"]
        r3_next_stage["next_stage &#10003;"]
        r3_stage_gate --> r3_validate
        r3_validate --> r3_transform
        r3_transform --> r3_verify
        r3_verify --> r3_advance_stage
        r3_advance_stage --> r3_next_stage
    end
    subgraph r4["stage_loop [5ae79f53-process_all-0-run_stages-next_stage-next_stage]"]
        r4_stage_gate["stage_gate &#10003;"]
        r4_validate["validate &#10003;"]
        r4_transform["transform &#10003;"]
        r4_verify["verify &#10003;"]
        r4_advance_stage["advance_stage &#10003;"]
        r4_next_stage["next_stage &#10003;"]
        r4_stage_gate --> r4_validate
        r4_validate --> r4_transform
        r4_transform --> r4_verify
        r4_verify --> r4_advance_stage
        r4_advance_stage --> r4_next_stage
    end
    subgraph r5["stage_loop [5ae79f53-process_all-0-run_stages-next_stage-next_stage-next_stage]"]
        r5_stage_gate["stage_gate &#10003;"]
        r5_validate["validate &#10003;"]
        r5_transform["transform &#10003;"]
        r5_verify["verify &#10003;"]
        r5_advance_stage["advance_stage &#10003;"]
        r5_next_stage["next_stage &#10003;"]
        r5_stage_gate --> r5_validate
        r5_validate --> r5_transform
        r5_transform --> r5_verify
        r5_verify --> r5_advance_stage
        r5_advance_stage --> r5_next_stage
    end
    subgraph r6["stage_loop [5ae79f53-process_all-0-run_stages-next_stage-next_stage-next_stage-next_stage]"]
        r6_stage_gate["stage_gate &#10003;"]
        r6_validate["validate &#9675;"]
        r6_transform["transform &#9675;"]
        r6_verify["verify &#9675;"]
        r6_advance_stage["advance_stage &#9675;"]
        r6_next_stage["next_stage &#9675;"]
        r6_stage_gate --> r6_validate
        r6_validate --> r6_transform
        r6_transform --> r6_verify
        r6_verify --> r6_advance_stage
        r6_advance_stage --> r6_next_stage
    end
    r5_next_stage --> r6_stage_gate
    r4_next_stage --> r5_stage_gate
    r3_next_stage --> r4_stage_gate
    r2_next_stage --> r3_stage_gate
    r1_run_stages --> r2_stage_gate
    r0_process_all --> r1_prepare
    subgraph r7["process_target [5ae79f53-process_all-1]"]
        r7_prepare["prepare &#10003;"]
        r7_run_stages["run_stages &#10003;"]
        r7_finalize["finalize &#10003;"]
        r7_prepare --> r7_run_stages
        r7_run_stages --> r7_finalize
    end
    subgraph r8["stage_loop [5ae79f53-process_all-1-run_stages]"]
        r8_stage_gate["stage_gate &#10003;"]
        r8_validate["validate &#10003;"]
        r8_transform["transform &#10003;"]
        r8_verify["verify &#10003;"]
        r8_advance_stage["advance_stage &#10003;"]
        r8_next_stage["next_stage &#10003;"]
        r8_stage_gate --> r8_validate
        r8_validate --> r8_transform
        r8_transform --> r8_verify
        r8_verify --> r8_advance_stage
        r8_advance_stage --> r8_next_stage
    end
    subgraph r9["stage_loop [5ae79f53-process_all-1-run_stages-next_stage]"]
        r9_stage_gate["stage_gate &#10003;"]
        r9_validate["validate &#10003;"]
        r9_transform["transform &#10003;"]
        r9_verify["verify &#10003;"]
        r9_advance_stage["advance_stage &#10003;"]
        r9_next_stage["next_stage &#10003;"]
        r9_stage_gate --> r9_validate
        r9_validate --> r9_transform
        r9_transform --> r9_verify
        r9_verify --> r9_advance_stage
        r9_advance_stage --> r9_next_stage
    end
    subgraph r10["stage_loop [5ae79f53-process_all-1-run_stages-next_stage-next_stage]"]
        r10_stage_gate["stage_gate &#10003;"]
        r10_validate["validate &#10003;"]
        r10_transform["transform &#10003;"]
        r10_verify["verify &#10003;"]
        r10_advance_stage["advance_stage &#10003;"]
        r10_next_stage["next_stage &#10003;"]
        r10_stage_gate --> r10_validate
        r10_validate --> r10_transform
        r10_transform --> r10_verify
        r10_verify --> r10_advance_stage
        r10_advance_stage --> r10_next_stage
    end
    subgraph r11["stage_loop [5ae79f53-process_all-1-run_stages-next_stage-next_stage-next_stage]"]
        r11_stage_gate["stage_gate &#10003;"]
        r11_validate["validate &#10003;"]
        r11_transform["transform &#10003;"]
        r11_verify["verify &#10003;"]
        r11_advance_stage["advance_stage &#10003;"]
        r11_next_stage["next_stage &#10003;"]
        r11_stage_gate --> r11_validate
        r11_validate --> r11_transform
        r11_transform --> r11_verify
        r11_verify --> r11_advance_stage
        r11_advance_stage --> r11_next_stage
    end
    subgraph r12["stage_loop [5ae79f53-process_all-1-run_stages-next_stage-next_stage-next_stage-next_stage]"]
        r12_stage_gate["stage_gate &#10003;"]
        r12_validate["validate &#9675;"]
        r12_transform["transform &#9675;"]
        r12_verify["verify &#9675;"]
        r12_advance_stage["advance_stage &#9675;"]
        r12_next_stage["next_stage &#9675;"]
        r12_stage_gate --> r12_validate
        r12_validate --> r12_transform
        r12_transform --> r12_verify
        r12_verify --> r12_advance_stage
        r12_advance_stage --> r12_next_stage
    end
    r11_next_stage --> r12_stage_gate
    r10_next_stage --> r11_stage_gate
    r9_next_stage --> r10_stage_gate
    r8_next_stage --> r9_stage_gate
    r7_run_stages --> r8_stage_gate
    r0_process_all --> r7_prepare
    subgraph r13["process_target [5ae79f53-process_all-2]"]
        r13_prepare["prepare &#10003;"]
        r13_run_stages["run_stages &#10003;"]
        r13_finalize["finalize &#10003;"]
        r13_prepare --> r13_run_stages
        r13_run_stages --> r13_finalize
    end
    subgraph r14["stage_loop [5ae79f53-process_all-2-run_stages]"]
        r14_stage_gate["stage_gate &#10003;"]
        r14_validate["validate &#10003;"]
        r14_transform["transform &#10003;"]
        r14_verify["verify &#10003;"]
        r14_advance_stage["advance_stage &#10003;"]
        r14_next_stage["next_stage &#10003;"]
        r14_stage_gate --> r14_validate
        r14_validate --> r14_transform
        r14_transform --> r14_verify
        r14_verify --> r14_advance_stage
        r14_advance_stage --> r14_next_stage
    end
    subgraph r15["stage_loop [5ae79f53-process_all-2-run_stages-next_stage]"]
        r15_stage_gate["stage_gate &#10003;"]
        r15_validate["validate &#10003;"]
        r15_transform["transform &#10003;"]
        r15_verify["verify &#10003;"]
        r15_advance_stage["advance_stage &#10003;"]
        r15_next_stage["next_stage &#10003;"]
        r15_stage_gate --> r15_validate
        r15_validate --> r15_transform
        r15_transform --> r15_verify
        r15_verify --> r15_advance_stage
        r15_advance_stage --> r15_next_stage
    end
    subgraph r16["stage_loop [5ae79f53-process_all-2-run_stages-next_stage-next_stage]"]
        r16_stage_gate["stage_gate &#10003;"]
        r16_validate["validate &#10003;"]
        r16_transform["transform &#10003;"]
        r16_verify["verify &#10003;"]
        r16_advance_stage["advance_stage &#10003;"]
        r16_next_stage["next_stage &#10003;"]
        r16_stage_gate --> r16_validate
        r16_validate --> r16_transform
        r16_transform --> r16_verify
        r16_verify --> r16_advance_stage
        r16_advance_stage --> r16_next_stage
    end
    subgraph r17["stage_loop [5ae79f53-process_all-2-run_stages-next_stage-next_stage-next_stage]"]
        r17_stage_gate["stage_gate &#10003;"]
        r17_validate["validate &#10003;"]
        r17_transform["transform &#10003;"]
        r17_verify["verify &#10003;"]
        r17_advance_stage["advance_stage &#10003;"]
        r17_next_stage["next_stage &#10003;"]
        r17_stage_gate --> r17_validate
        r17_validate --> r17_transform
        r17_transform --> r17_verify
        r17_verify --> r17_advance_stage
        r17_advance_stage --> r17_next_stage
    end
    subgraph r18["stage_loop [5ae79f53-process_all-2-run_stages-next_stage-next_stage-next_stage-next_stage]"]
        r18_stage_gate["stage_gate &#10003;"]
        r18_validate["validate &#9675;"]
        r18_transform["transform &#9675;"]
        r18_verify["verify &#9675;"]
        r18_advance_stage["advance_stage &#9675;"]
        r18_next_stage["next_stage &#9675;"]
        r18_stage_gate --> r18_validate
        r18_validate --> r18_transform
        r18_transform --> r18_verify
        r18_verify --> r18_advance_stage
        r18_advance_stage --> r18_next_stage
    end
    r17_next_stage --> r18_stage_gate
    r16_next_stage --> r17_stage_gate
    r15_next_stage --> r16_stage_gate
    r14_next_stage --> r15_stage_gate
    r13_run_stages --> r14_stage_gate
    r0_process_all --> r13_prepare
    subgraph r19["process_target [5ae79f53-process_all-3]"]
        r19_prepare["prepare &#10003;"]
        r19_run_stages["run_stages &#10003;"]
        r19_finalize["finalize &#10003;"]
        r19_prepare --> r19_run_stages
        r19_run_stages --> r19_finalize
    end
    subgraph r20["stage_loop [5ae79f53-process_all-3-run_stages]"]
        r20_stage_gate["stage_gate &#10003;"]
        r20_validate["validate &#10003;"]
        r20_transform["transform &#10003;"]
        r20_verify["verify &#10003;"]
        r20_advance_stage["advance_stage &#10003;"]
        r20_next_stage["next_stage &#10003;"]
        r20_stage_gate --> r20_validate
        r20_validate --> r20_transform
        r20_transform --> r20_verify
        r20_verify --> r20_advance_stage
        r20_advance_stage --> r20_next_stage
    end
    subgraph r21["stage_loop [5ae79f53-process_all-3-run_stages-next_stage]"]
        r21_stage_gate["stage_gate &#10003;"]
        r21_validate["validate &#10003;"]
        r21_transform["transform &#10003;"]
        r21_verify["verify &#10003;"]
        r21_advance_stage["advance_stage &#10003;"]
        r21_next_stage["next_stage &#10003;"]
        r21_stage_gate --> r21_validate
        r21_validate --> r21_transform
        r21_transform --> r21_verify
        r21_verify --> r21_advance_stage
        r21_advance_stage --> r21_next_stage
    end
    subgraph r22["stage_loop [5ae79f53-process_all-3-run_stages-next_stage-next_stage]"]
        r22_stage_gate["stage_gate &#10003;"]
        r22_validate["validate &#10003;"]
        r22_transform["transform &#10003;"]
        r22_verify["verify &#10003;"]
        r22_advance_stage["advance_stage &#10003;"]
        r22_next_stage["next_stage &#10003;"]
        r22_stage_gate --> r22_validate
        r22_validate --> r22_transform
        r22_transform --> r22_verify
        r22_verify --> r22_advance_stage
        r22_advance_stage --> r22_next_stage
    end
    subgraph r23["stage_loop [5ae79f53-process_all-3-run_stages-next_stage-next_stage-next_stage]"]
        r23_stage_gate["stage_gate &#10003;"]
        r23_validate["validate &#10003;"]
        r23_transform["transform &#10003;"]
        r23_verify["verify &#10003;"]
        r23_advance_stage["advance_stage &#10003;"]
        r23_next_stage["next_stage &#10003;"]
        r23_stage_gate --> r23_validate
        r23_validate --> r23_transform
        r23_transform --> r23_verify
        r23_verify --> r23_advance_stage
        r23_advance_stage --> r23_next_stage
    end
    subgraph r24["stage_loop [5ae79f53-process_all-3-run_stages-next_stage-next_stage-next_stage-next_stage]"]
        r24_stage_gate["stage_gate &#10003;"]
        r24_validate["validate &#9675;"]
        r24_transform["transform &#9675;"]
        r24_verify["verify &#9675;"]
        r24_advance_stage["advance_stage &#9675;"]
        r24_next_stage["next_stage &#9675;"]
        r24_stage_gate --> r24_validate
        r24_validate --> r24_transform
        r24_transform --> r24_verify
        r24_verify --> r24_advance_stage
        r24_advance_stage --> r24_next_stage
    end
    r23_next_stage --> r24_stage_gate
    r22_next_stage --> r23_stage_gate
    r21_next_stage --> r22_stage_gate
    r20_next_stage --> r21_stage_gate
    r19_run_stages --> r20_stage_gate
    r0_process_all --> r19_prepare
    subgraph r25["process_target [5ae79f53-process_all-4]"]
        r25_prepare["prepare &#10003;"]
        r25_run_stages["run_stages &#10003;"]
        r25_finalize["finalize &#10003;"]
        r25_prepare --> r25_run_stages
        r25_run_stages --> r25_finalize
    end
    subgraph r26["stage_loop [5ae79f53-process_all-4-run_stages]"]
        r26_stage_gate["stage_gate &#10003;"]
        r26_validate["validate &#10003;"]
        r26_transform["transform &#10003;"]
        r26_verify["verify &#10003;"]
        r26_advance_stage["advance_stage &#10003;"]
        r26_next_stage["next_stage &#10003;"]
        r26_stage_gate --> r26_validate
        r26_validate --> r26_transform
        r26_transform --> r26_verify
        r26_verify --> r26_advance_stage
        r26_advance_stage --> r26_next_stage
    end
    subgraph r27["stage_loop [5ae79f53-process_all-4-run_stages-next_stage]"]
        r27_stage_gate["stage_gate &#10003;"]
        r27_validate["validate &#10003;"]
        r27_transform["transform &#10003;"]
        r27_verify["verify &#10003;"]
        r27_advance_stage["advance_stage &#10003;"]
        r27_next_stage["next_stage &#10003;"]
        r27_stage_gate --> r27_validate
        r27_validate --> r27_transform
        r27_transform --> r27_verify
        r27_verify --> r27_advance_stage
        r27_advance_stage --> r27_next_stage
    end
    subgraph r28["stage_loop [5ae79f53-process_all-4-run_stages-next_stage-next_stage]"]
        r28_stage_gate["stage_gate &#10003;"]
        r28_validate["validate &#10003;"]
        r28_transform["transform &#10003;"]
        r28_verify["verify &#10003;"]
        r28_advance_stage["advance_stage &#10003;"]
        r28_next_stage["next_stage &#10003;"]
        r28_stage_gate --> r28_validate
        r28_validate --> r28_transform
        r28_transform --> r28_verify
        r28_verify --> r28_advance_stage
        r28_advance_stage --> r28_next_stage
    end
    subgraph r29["stage_loop [5ae79f53-process_all-4-run_stages-next_stage-next_stage-next_stage]"]
        r29_stage_gate["stage_gate &#10003;"]
        r29_validate["validate &#10003;"]
        r29_transform["transform &#10003;"]
        r29_verify["verify &#10003;"]
        r29_advance_stage["advance_stage &#10003;"]
        r29_next_stage["next_stage &#10003;"]
        r29_stage_gate --> r29_validate
        r29_validate --> r29_transform
        r29_transform --> r29_verify
        r29_verify --> r29_advance_stage
        r29_advance_stage --> r29_next_stage
    end
    subgraph r30["stage_loop [5ae79f53-process_all-4-run_stages-next_stage-next_stage-next_stage-next_stage]"]
        r30_stage_gate["stage_gate &#10003;"]
        r30_validate["validate &#9675;"]
        r30_transform["transform &#9675;"]
        r30_verify["verify &#9675;"]
        r30_advance_stage["advance_stage &#9675;"]
        r30_next_stage["next_stage &#9675;"]
        r30_stage_gate --> r30_validate
        r30_validate --> r30_transform
        r30_transform --> r30_verify
        r30_verify --> r30_advance_stage
        r30_advance_stage --> r30_next_stage
    end
    r29_next_stage --> r30_stage_gate
    r28_next_stage --> r29_stage_gate
    r27_next_stage --> r28_stage_gate
    r26_next_stage --> r27_stage_gate
    r25_run_stages --> r26_stage_gate
    r0_process_all --> r25_prepare

    class r0_init completed
    class r0_scan completed
    class r0_process_all completed
    class r0_report completed
    class r1_prepare completed
    class r1_run_stages completed
    class r1_finalize completed
    class r2_stage_gate completed
    class r2_validate completed
    class r2_transform completed
    class r2_verify completed
    class r2_advance_stage completed
    class r2_next_stage completed
    class r3_stage_gate completed
    class r3_validate completed
    class r3_transform completed
    class r3_verify completed
    class r3_advance_stage completed
    class r3_next_stage completed
    class r4_stage_gate completed
    class r4_validate completed
    class r4_transform completed
    class r4_verify completed
    class r4_advance_stage completed
    class r4_next_stage completed
    class r5_stage_gate completed
    class r5_validate completed
    class r5_transform completed
    class r5_verify completed
    class r5_advance_stage completed
    class r5_next_stage completed
    class r6_stage_gate completed
    class r6_validate skipped
    class r6_transform skipped
    class r6_verify skipped
    class r6_advance_stage skipped
    class r6_next_stage skipped
    class r7_prepare completed
    class r7_run_stages completed
    class r7_finalize completed
    class r8_stage_gate completed
    class r8_validate completed
    class r8_transform completed
    class r8_verify completed
    class r8_advance_stage completed
    class r8_next_stage completed
    class r9_stage_gate completed
    class r9_validate completed
    class r9_transform completed
    class r9_verify completed
    class r9_advance_stage completed
    class r9_next_stage completed
    class r10_stage_gate completed
    class r10_validate completed
    class r10_transform completed
    class r10_verify completed
    class r10_advance_stage completed
    class r10_next_stage completed
    class r11_stage_gate completed
    class r11_validate completed
    class r11_transform completed
    class r11_verify completed
    class r11_advance_stage completed
    class r11_next_stage completed
    class r12_stage_gate completed
    class r12_validate skipped
    class r12_transform skipped
    class r12_verify skipped
    class r12_advance_stage skipped
    class r12_next_stage skipped
    class r13_prepare completed
    class r13_run_stages completed
    class r13_finalize completed
    class r14_stage_gate completed
    class r14_validate completed
    class r14_transform completed
    class r14_verify completed
    class r14_advance_stage completed
    class r14_next_stage completed
    class r15_stage_gate completed
    class r15_validate completed
    class r15_transform completed
    class r15_verify completed
    class r15_advance_stage completed
    class r15_next_stage completed
    class r16_stage_gate completed
    class r16_validate completed
    class r16_transform completed
    class r16_verify completed
    class r16_advance_stage completed
    class r16_next_stage completed
    class r17_stage_gate completed
    class r17_validate completed
    class r17_transform completed
    class r17_verify completed
    class r17_advance_stage completed
    class r17_next_stage completed
    class r18_stage_gate completed
    class r18_validate skipped
    class r18_transform skipped
    class r18_verify skipped
    class r18_advance_stage skipped
    class r18_next_stage skipped
    class r19_prepare completed
    class r19_run_stages completed
    class r19_finalize completed
    class r20_stage_gate completed
    class r20_validate completed
    class r20_transform completed
    class r20_verify completed
    class r20_advance_stage completed
    class r20_next_stage completed
    class r21_stage_gate completed
    class r21_validate completed
    class r21_transform completed
    class r21_verify completed
    class r21_advance_stage completed
    class r21_next_stage completed
    class r22_stage_gate completed
    class r22_validate completed
    class r22_transform completed
    class r22_verify completed
    class r22_advance_stage completed
    class r22_next_stage completed
    class r23_stage_gate completed
    class r23_validate completed
    class r23_transform completed
    class r23_verify completed
    class r23_advance_stage completed
    class r23_next_stage completed
    class r24_stage_gate completed
    class r24_validate skipped
    class r24_transform skipped
    class r24_verify skipped
    class r24_advance_stage skipped
    class r24_next_stage skipped
    class r25_prepare completed
    class r25_run_stages completed
    class r25_finalize completed
    class r26_stage_gate completed
    class r26_validate completed
    class r26_transform completed
    class r26_verify completed
    class r26_advance_stage completed
    class r26_next_stage completed
    class r27_stage_gate completed
    class r27_validate completed
    class r27_transform completed
    class r27_verify completed
    class r27_advance_stage completed
    class r27_next_stage completed
    class r28_stage_gate completed
    class r28_validate completed
    class r28_transform completed
    class r28_verify completed
    class r28_advance_stage completed
    class r28_next_stage completed
    class r29_stage_gate completed
    class r29_validate completed
    class r29_transform completed
    class r29_verify completed
    class r29_advance_stage completed
    class r29_next_stage completed
    class r30_stage_gate completed
    class r30_validate skipped
    class r30_transform skipped
    class r30_verify skipped
    class r30_advance_stage skipped
    class r30_next_stage skipped

    classDef completed fill:#d4edda,stroke:#28a745
    classDef failed fill:#f8d7da,stroke:#dc3545
    classDef skipped fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:5 5```
