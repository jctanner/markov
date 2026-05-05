# Bug: HTTP callback buffer overflow drops events

## Summary

When a workflow completes quickly (all steps finish within 1–2 seconds), the HTTP callback system drops most events with `callback http: buffer full, dropping event`. This causes markovd to have an incomplete picture of the run — missing steps and a permanently "running" status.

## Reproduction

Run the `deploy-pipeline.yaml` example via markovd:

```
POST /api/v1/runs
{"workflow_name": "deploy-pipeline", "vars": {}}
```

This workflow fans out to 3 environments with recursive retry loops, producing ~60 callback events. Because every step is a fast `echo` command, all events fire within ~2 seconds.

### Observed behavior

- markovd receives only 3 of ~60 step events (preflight, build, test_gate)
- All sub-workflow steps (deploy_env, retry_loop) are lost
- The `run_completed` / `run_failed` event is also lost
- The run is stuck as `status: "running"` permanently
- API container logs show repeated: `callback http: buffer full, dropping event`

### Expected behavior

All callback events should be delivered to the receiver, even if the workflow completes quickly.

## Root cause

The HTTP callback sender uses a buffered channel (or similar fixed-size queue) to decouple event production from HTTP POST delivery. When steps complete faster than the HTTP client can POST them, the buffer fills and events are silently dropped.

The `run_completed` / `run_failed` event is the most critical — losing it leaves the run in a zombie "running" state that never resolves.

## Impact

- **Zombie runs**: Any fast workflow will get stuck as "running" forever since the terminal event is dropped.
- **Missing steps**: markovd's step table and graph visualization show only a fraction of the actual execution.
- **Data loss**: No retry or persistence means dropped events are gone permanently.

## Suggested fixes

### Short term

1. **Make the terminal event blocking**: `run_completed` and `run_failed` should bypass the buffer and send synchronously (or flush the buffer first). These events must never be dropped.
2. **Increase the buffer size**: The current buffer is too small for fan-out workflows. A buffer of 1000+ events would handle most realistic pipelines.

### Medium term

3. **Back-pressure instead of dropping**: Block the workflow executor when the buffer is full rather than dropping events. A small delay in step execution is preferable to data loss.
4. **Retry with disk spill**: Write events to a local file (JSONL) when the HTTP buffer is full, then drain the file after the run completes.

### Long term

5. **Batch delivery**: Accumulate events over a short window (100ms) and POST them as an array. This reduces HTTP round-trips from N to N/batch_size.
6. **Acknowledge-based delivery**: The receiver responds with the last seen event ID; the sender retransmits anything unacknowledged.

## Workaround

For now, markovd operators can manually fix zombie runs by POSTing the terminal event:

```bash
curl -X POST http://localhost:8082/api/v1/events \
  -H "Authorization: Bearer $CALLBACK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"event_type":"run_failed","run_id":"<run_id>","workflow_name":"main","timestamp":"2026-04-24T01:02:32Z"}'
```

## Related

- `deploy-pipeline.yaml` — example that reliably triggers this bug
- markovd `internal/api/events.go` — the callback receiver (not at fault; it processes events fine)
- markov `internal/callback/http.go` (presumed) — the sender-side buffer that drops events
