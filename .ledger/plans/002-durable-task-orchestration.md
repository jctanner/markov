# Durable external tasks and orchestration reliability

Created 2026-10-11 from the Breadboard strategy workflow evaluation.
This plan owns the proposed Markov architecture and language work.
Markovd event projection and downstream submission APIs may require
coordinated changes in their own repositories.

Evaluation context: [strategy workflow findings](../../../../.ledger/plans/strat-workflow-eval-plan.md).

**Status: proposed.** Engine/DSL changes have not been authorized or implemented.
Checked current Markov parser/state/executors and Markov/markovd docs before
identifying gaps. Prioritize execution guarantees over expanding the DSL.

1. **Durable external tasks and resource ownership.** Persist a task intent
   and stable operation ID before job submission; an adapter submits or finds
   that operation, records its remote identity, then polls/reattaches. Resume
   reattaches instead of blindly submitting again. Cancellation consults the
   task/resource record, including submission still being reconciled. Existing
   Kubernetes deterministic names are useful but current executor Create does
   not reconcile AlreadyExists, and dashboard POST plus subsequent wait leaves
   a known ownership-registration window. Full guarantees require downstream
   idempotency support or an adapter capable of trustworthy reconciliation;
   a Markov key alone cannot make arbitrary Jira POST exactly once.
2. **Engine-side result and artifact contracts.** Validate parsed outputs or
   artifact files against schemas and expected per-item identities before
   marking a step complete. Keep orchestration schemas out of nested agent
   prompts unless explicitly requested; engine validation should not recreate
   S1's inherited StructuredOutput leak. A result containing nested JSON in
   `recorded`, an unexpected strategy key, or missing required review evidence
   should fail with a precise boundary error. Current assert/failed_when and
   script validators can already express these checks; a reusable contract
   reduces repeated plumbing, rather than inventing validation capability.
3. **Explicit retry and reconciliation policy.** Distinguish reattach/status
   polling, transient transport failure, definitive task failure, malformed
   output and business verdicts such as revise. Declare bounded attempts,
   backoff and elapsed-time/cost budgets; replay side effects only when their
   adapter establishes safe reuse. Resubmitting an agent costs money and must
   be distinct from rechecking an already completed agent's artifacts. Current
   timeouts/cleanup/error predicates provide pieces, not this general policy.
4. **Durable lifecycle events and projection recovery.** Commit lifecycle
   events alongside state transitions, assign sequence/event IDs, retain an
   outbox until acknowledged, and deduplicate/replay at markovd. Rebuild the UI
   projection after an outage from durable state/events. The current callback
   queue is in-memory with three transport/server-error attempts; delivery can
   be dropped and "flush" means send attempts finished, not durable receiver
   acknowledgment. Permanent/authentication rejection needs visible recovery
   handling rather than an infinite retry loop.
5. **Shared concurrency and budget controls.** Extend local for_each limits
   and Claude-step limits with durable named pools/leases across runs, e.g.
   model-account concurrency, Jira resource ownership and batch-wide agent
   spend/request budgets. A workflow's local parallelism setting cannot by
   itself prevent two runs exhausting the same upstream account. Counters
   must include child jobs/agents; actual-cost enforcement requires suitable
   telemetry rather than assuming one CLI's reported amount is authoritative.
6. **Evidence as a completion contract.** Allow tasks/finalizers to declare
   required artifacts and capture logs/traces on success, failure and cancel.
   Store large evidence as private artifact references with identity, hashes,
   capture status and retention information; avoid loading raw traces into
   template/model context. Gate destructive cleanup on verified preservation.
   The current host collector and archive guards implement this for the
   benchmark; a native integration should retain those semantics and make
   archival success separately visible from execution or business success.

No new primitive is needed just to run one pipeline per strategy: existing
`for_each_key`, sub-workflows, per-loop concurrency, `ignore_errors`,
`rescue`/`always`, assert/failed_when, and checkpoint/resume provide the base.
Use expected-item manifests and explicit aggregate assertions to ensure every
selected key has a terminal result. Generic first-class batch completion
policies could improve ergonomics later, but should not precede durable task
ownership and output validation. Current source-integrity/definition-hash
checks already cover source drift; evaluating changed input/artifact freshness
would be an extension, not a newly invented source-pinning feature.

Recommended first slice: a durable submit/wait/reconcile adapter plus
engine-side output contracts, tested with crashes at each submission/state
boundary, dropped responses, reordered/retried batch items, malformed outputs,
and cancellation during submission. These tests can use stub jobs and APIs
without expensive model runs. Preserve S1/S2 as evaluation controls.
