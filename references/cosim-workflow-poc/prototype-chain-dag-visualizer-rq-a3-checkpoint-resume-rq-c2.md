# Prototype Documentation: RQ-A3 & RQ-C2

**Author:** Sam (Prototype Engineer)  
**Date:** 2026-04-21  
**Status:** Built — both prototypes committed

---

## Prototype 1: Chain DAG Visualizer (RQ-A3)

**Repo:** `chain-dag-visualizer`  
**Commit:** `4b55707f`  
**Stack:** Flask + React Flow (CDN) + Flask-SocketIO + Dagre layout

### What It Proves

1. **React Flow maps cleanly to ChainResult** — the existing data model converts 1:1 to React Flow nodes/edges. Zero data model changes needed.
2. **WebSocket status streaming is trivial** — Flask-SocketIO pushes step transitions in real-time. Each event is `{run_id, step_name, status, output, duration_ms}`.
3. **Custom node rendering per step type works** — different visual treatment for llm_invoke (blue/purple), human_gate (orange with approve/reject buttons), parallel (dashed purple border with fan-out/fan-in), rule_evaluation (green).
4. **Human gate interaction works inline** — approve/reject buttons in the DAG node send WebSocket events back to the controller.
5. **No build step needed for PoC** — React Flow via CDN keeps the prototype dependency-free.

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Layout engine | Manual positioning (Dagre in production) | Avoid JS build toolchain for PoC |
| Transport | WebSocket (SocketIO) | Bidirectional needed for gate approval |
| Rendering | Custom HTML nodes, not SVG | Easier to embed buttons, forms |
| State mgmt | Server-side (in-memory) | PoC simplicity |

### Files

- `app.py` — Flask server, WebSocket handlers, REST API
- `chain_converter.py` — ChainResult → React Flow nodes/edges conversion
- `demo_chains.py` — Sample chains + simulated execution with delays
- `templates/index.html` — Single-page UI with DAG canvas, side panel, execution log

### Limitations

- Single-user, in-memory state
- Simulated execution (stub modules)
- CDN-loaded React Flow (~200KB, no tree-shaking)
- Manual layout, not true Dagre (production would use dagre-d3)

---

## Prototype 2: Checkpoint/Resume (RQ-C2)

**Repo:** `yaml-agent-chain-poc`  
**Commit:** `ee5ef6ab`  
**Stack:** SQLite + Python stdlib (zero new deps beyond sqlite3)

### What It Proves

1. **Checkpoint-per-step to SQLite works** — each step's output serialized to JSON, stored with metadata (attempt, status, TMS policy, side effect class). Schema identical to what Postgres JSONB would use.
2. **Resume skips completed steps** — on `--resume <run_id>`, steps 1-2 are loaded from cache, execution starts at the failed step 3.
3. **Context reconstruction works** — registered variables from completed steps are deserialized and injected into executor context before resuming.
4. **Run lineage is tracked** — resumed runs link to their parent via `resumed_from` foreign key. Full audit trail.
5. **DBOS pattern validated** — "workflows should be Postgres rows" works. SQLite here, Postgres in production. Same schema.

### Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Checkpoint store | SQLite (dev) / Postgres (prod) | DBOS-proven pattern, zero new infra |
| Granularity | Per-step (not per-substep) | Matches Temporal's activity-boundary model |
| Serialization | JSON (stdlib) | Step outputs are already dicts |
| Resume strategy | Skip-completed + replay-from-failed | Simplest correct behavior |
| Architecture | Wrapper (ResumableExecutor) | Non-invasive to base executor |

### Files

- `checkpoint_store.py` — SQLite-backed store with `chain_runs` + `checkpoints` tables
- `resumable_executor.py` — Wraps ChainExecutor with checkpoint hooks
- `examples/resume_demo.yaml` — 5-step chain with injectable failure
- `examples/resume_demo_test.py` — End-to-end test: fail → checkpoint → resume → complete

### Schema

```sql
chain_runs: (run_id, chain_name, status, started_at, completed_at, resumed_from, context_json)
checkpoints: (chain_run_id, step_name, attempt, status, output_json, error, duration_ms, tms_policy)
```

### Resume Flow

```
Run 1: step1 ✓ → step2 ✓ → step3 ✗ (fail)
        ↓ checkpoint   ↓ checkpoint   ↓ checkpoint (failed)

Run 2 (--resume run1):
        step1 [cached] → step2 [cached] → step3 ✓ → step4 ✓ → step5 ✓
```

### Four Resume Strategies Supported

1. **Auto-retry** — `retry_strategy` in YAML, automatic backoff
2. **Manual resume** — `--resume <run_id>` CLI flag
3. **Checkpoint fork** — `store.fork_run()` creates new run with copied checkpoints + modified context
4. **Selective re-run** — mark specific completed steps for re-execution (via API)

---

## Next Steps

- **A3**: Integrate with real `yaml-agent-chain-poc` executor (replace simulated execution)
- **C2**: Add `--resume` flag to `runner.py` CLI
- **C2**: Test TMS stale-flagging when resumed step output differs from original
- **B+C**: Once Raj lands RQ-B1 (K8s architecture decision), build controller-as-Job prototype with checkpoint PVC mount
