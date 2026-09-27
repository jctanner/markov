{% raw %}
# Jev decisions

Markov supports hosted Jev and compatible servers such as Kev through `POST /v1/systemone`. No Python SDK or model runtime is required in Markov; run the model server separately.

## Connections and definitions

```yaml
connections:
  local_kev:
    type: jev
    base_url: http://127.0.0.1:18009
    model: kev-latest
    api_key_env: KEV_API_KEY
    timeout: 120
    concurrency: 1

decisions:
  billing:
    connection: local_kev
    version: "1"
    type: noul
    instructions: Is this about a billing problem?
```

`base_url` is the service root, without `/v1/systemone`. For hosted Jev use `https://api.typesafe.ai`, model `jev-latest`, and a credential reference such as `api_key_env: TYPESAFE_API_KEY`.

Connections accept `type`, `base_url`, `model`, `api_key_env`, `api_key_file`, `timeout`, and `concurrency`. Model defaults to `jev-latest`, timeout to 120 seconds, and concurrency to one. Timeout includes queuing and retries. The concurrency limit is shared across calls using that connection in one engine process, including fan-outs; it is not a distributed limit across separate Markov processes.

Use at most one credential reference. An omitted reference means no Authorization header. A configured empty/missing credential is an error. File credentials are trimmed and paths are relative to the Markov process working directory; an absolute path is recommended. Secret values are resolved when calling the API and never added to workflow vars or request metadata. In Kubernetes, inject the environment variable or mount the credential file into the **Markov runner**, not just its child Jobs. Redirects are not followed.

Directory workflows may add optional `connections.yaml` and `decisions.yaml` files. Each contains the corresponding named map, without the outer `connections:` or `decisions:` key. Named definitions are static; template the supplied state instead. Version strings let authors identify changes to a decision rubric.

## Native step

```yaml
- name: triage
  type: jev
  timeout: 120
  register: result
  params:
    connection: local_kev
    state: "{{ ticket }}"
    questions:
      billing:
        type: noul
        instructions: Is this about a billing problem?
      team:
        type: choice
        instructions: Which team should handle this?
        criteria:
          billing: Payments and refunds
          technical: Software problems
      urgency:
        type: score
        instructions: How urgent is this?
        criteria: [Routine, Soon, Immediate]
```

`state` can be text, an object, or an array. Exact variable expressions preserve structured state. Questions follow the Jev Choice/Noul/Score schema. Register stores the full response: `answers`, `model`, `usage`, and `metadata` containing connection, requested model, request ID, elapsed seconds, attempt count, and request fingerprint. For example, `result.answers.billing.noul` is a number and `result.answers.team.choice` is a string.

Use existing `step_types` to reuse native Jev step defaults. Completed steps use ordinary Markov checkpoint semantics and are not rerun on resume, even if vars change. For batch output collection, use a keyed `for_each` over a child workflow whose Jev step has `register`; direct primitive fan-out has existing output-collection limitations.

## Inline conditions and assertions

```yaml
when: "jev.noul('billing', ticket) >= 0.7"
```

`jev.noul(name, state)`, `jev.choice(name, state)`, and `jev.score(name, state)` are available in step `when` and `assert.that`. The function must match the named decision's type. They return the corresponding scalar, not the complete response. Use an explicit numeric comparison for Noul; Pongo2 treats nonzero numbers as truthy. Thresholds are application policy, not guarantees of correctness.

An optional third argument is a refresh token:

```yaml
when: "jev.noul('billing', ticket, decision_revision) >= 0.7"
```

Changing that token triggers a new evaluation if the surrounding step is evaluated again. These functions are not available in arbitrary string templates, `set_fact` expressions, or Grule rule conditions. Use registered responses or decision-backed gate facts there.

## Decision-backed gate facts

```yaml
rules:
  - name: review_billing
    when: "billing_probability >= 0.7 and approved != true"
    action: pause

# Within a workflow's steps:
- name: review
  type: gate
  timeout: 120
  facts:
    billing_probability:
      decision: billing
      state: "{{ ticket }}"
      select: noul
      refresh: "{{ decision_revision }}"
  rules: [review_billing]
```

Resolve facts before Grule executes. All facts see the same incoming workflow context. `select` defaults to the decision type (`noul`, `choice`, or `score`); Choice/Score also support `confidence` and `probabilities`. Rules consume ordinary resolved facts, so forward chaining performs no network calls. Resolved facts are retained in the gate receipt; rule assignments take precedence over input facts with the same name.

`refresh` is an optional identity token, not an unconditional refresh switch: reusing the same value reuses the same decision. Gate `pause` remains durable and requires explicit resume input. Gate `skip` still only records the decision; use downstream `when` conditions to control execution.

## Persistence and failures

Inline and gate decisions are persisted as internal `__jev__/...` step receipts with the full response, named definition version, parent step, and SHA-256 fingerprint. Identity includes the containing step, decision definition, connection configuration, state, and refresh token; credentials themselves are excluded. Identical calls within one step share a receipt. Separate steps and fan-out items have separate identities.

Decisions are saved even when a condition is false or a gate pauses. A restarted Markov process reuses saved answers for identical inputs. Changing input/configuration/refresh produces a new receipt when that step is reevaluated. This is not exactly-once execution: a crash after the server replies but before persistence may repeat the request. Model aliases can change remotely; use fixed model versions where supported and record local Kev checkpoint identity separately.

Malformed responses, missing answers, invalid ranges/distributions, authentication failures, and timeouts fail execution. They do not become false judgments. HTTP 429/502/503/504 receive at most two retries within the deadline. Other status codes and transport failures fail immediately. Server error bodies are not copied into errors. Decision receipts emit ordinary step lifecycle callbacks; outputs and state are application data and follow the existing state-store/logging access controls.

For a fresh local server, follow the [CPU Kev setup guide](../guides/testing-with-kev.md).

## Try it

See `examples/jev.yaml`. With Kev serving on localhost:18009 and matching `KEV_API_KEY` in Markov's environment:

```bash
markov validate examples/jev.yaml
markov run examples/jev.yaml --state-store /tmp/markov-jev.db
markov status RUN_ID --steps --state-store /tmp/markov-jev.db
markov resume RUN_ID --var approved=true --state-store /tmp/markov-jev.db
```

The example runs a three-question Jev step, an inline condition, and a decision-backed gate. It deliberately requires human approval regardless of the model's prediction so pause/resume is reproducible.
{% endraw %}
