# First-class Jev support

Implement named connections and decisions, a native Jev executor, durable inline conditions, and decision-backed gate facts. Validate with unit tests and a live CPU Kev-0.8B server, including authentication and pause/resume without inference replay.

- [x] Schema and client
- [x] Executor and durable decisions
- [x] Examples and documentation
- [x] Automated checks and live Kev verification

Decisions: credentials remain runtime environment/file references; gate decisions resolve before Grule; persisted decision receipts include input/config hashes and complete API responses. No automatic cross-provider fallback.

Verification (2026-09-27): `make test`, `make build`, and `make vet` passed; all 22 examples validate. All 13 changed/new Go files pass gofmt. Full `make lint` is blocked only by pre-existing formatting in pkg/callback/http.go, pkg/callback/jsonl.go, pkg/engine/artifacts.go, and pkg/engine/engine_callback_test.go.

Live run `abc6ecfe` used authenticated CPU Kev-0.8B: three inference requests, pause, then successful resume after stopping Kev. Missing/wrong keys returned 401, and Markov failed on a wrong key. No secret appeared in captured logs or the state database. See [validation report](../../notes/jev-kev-validation.md).

Implemented inline helpers in step when/assert, not general templates or direct Grule calls. Gate facts resolve before forward chaining. Optional directory definitions are supported. Existing direct primitive fan-out output collection is unchanged; use child workflows.
