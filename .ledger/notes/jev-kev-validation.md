# Jev integration: local Kev verification

Date: 2026-09-27. Workflow: [examples/jev.yaml](../../examples/jev.yaml). Raw responses and server metadata: [jev-kev-validation.json](jev-kev-validation.json).

Reproduction: [CPU Kev setup and test guide](../../docs/guides/testing-with-kev.md) includes installation, pinned dependencies, credentials, server startup, and offline-resume commands.

## Stack

- AMD Ryzen 7 2700X, eight OpenMP/MKL threads, CPU-only PyTorch 2.8.0+cpu, FP32; GPU unused.
- Kev source: `5920c5fe4ca8e0970ed4209ac2c9b8e18bea5109`.
- Model: `jaredpalmer/kev-0.8b@9a45d25eb2ab761841196625383fa1dff0e56c1e`.
- Base: Qwen/Qwen3.5-0.8B-Base, revision `dc7cdfe2ee4154fa7e30f5b51ca41bfa40174e68`.
- Local service: `http://127.0.0.1:18009`, bearer authentication enabled using an ephemeral key.

## Results

Run `abc6ecfe` completed the inference portion in **2.313 seconds**, then paused for approval. This is one integration run after model loading, not a performance or accuracy benchmark. Kev may reuse shared prefixes between these requests.

1. Native `jev` step returned valid Choice, Noul, and Score answers through `register`.
2. Inline `jev.noul` condition executed and saved its decision.
3. Gate decision resolved to a numeric fact and was saved before the approval pause.
4. Server request counters confirmed **three inference requests**.
5. Kev was stopped; `markov resume ... --var approved=true` completed in **0.026 seconds** without a running inference server. Registered output and gate facts remained available to downstream assertions.
6. Missing and wrong bearer tokens returned **401**. A Markov run using a wrong key failed as expected.
7. The ephemeral secret was absent from captured verbose logs and the state database.

The test server remains stopped. Temporary logs/state are under `/tmp/markov-jev-live-hd9q8av8`; durable responses are attached above. This verifies compatibility with this Kev version, not hosted Jev access or decision accuracy.

## Automated checks

- `GOCACHE=/tmp/go-build make test`: passed all packages, including new client, schema, and engine tests.
- `make build` and `make vet`: passed.
- All **22** workflow examples validate.
- All **13** changed/new Go files pass gofmt; `git diff --check` passes.
- Full `make lint` reaches a pre-existing formatting failure in `pkg/callback/http.go`, `pkg/callback/jsonl.go`, `pkg/engine/artifacts.go`, and `pkg/engine/engine_callback_test.go`. Those files were unchanged.

New tests cover bearer credentials from environment/file, structured input, response validation, rejected authentication, bounded transient retry, cancellation, per-connection concurrency, directory definitions, persisted false conditions, shared gate decisions, restart/resume, changed-input/refresh invalidation, and propagation of decision errors.

An initial live-test harness invocation used an unsupported `resume --verbose` flag, and the first example used a Pongo2-unsupported list literal. Both were corrected before the clean run reported here.
