# ADR-0003: Durable Jev decisions

Status: accepted, 2026-09-27.

Jev-compatible APIs are configured through named connections with runtime credential references. Native steps use the normal executor/register contract. Named decisions power inline step conditions/assertions and structured gate facts.

Inline and gate evaluations persist separate step receipts keyed by the containing step and a hash of the definition, connection configuration, state, and optional refresh token. This preserves answers across false conditions, pauses, and process restarts without changing state-store schemas. Internal receipt names use the reserved `__jev__/` prefix and are excluded from ordinary context restoration.

Gate facts resolve before Grule runs; rules only inspect ordinary values. No network calls occur during forward chaining. Identical decisions within a step share a receipt; different steps/items remain independent. A refresh token changes identity only when the containing step is reevaluated.

One client per engine shares connection concurrency limits across native and named decisions. API errors fail execution; there is no implicit negative judgment or cross-provider fallback. Calls can repeat after a crash before persistence, so this is not exactly-once inference. Hosted aliases and local checkpoint changes are external state; users should pin versions where supported.

Inline helpers are initially limited to `when` and `assert.that`. General template calls and direct Grule function syntax are deliberately outside this implementation. The full response remains in receipts even when an expression selects only a scalar.
