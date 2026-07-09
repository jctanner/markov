# Bug: Template Map Values Render as Go Strings in HTTP Bodies

## Status

Fixed

## Summary

When a workflow passes a map-like value through a template expression inside an HTTP request body, Markov can render the value using Go/Pongo's debug string form instead of preserving it as structured JSON. The receiving API gets a string such as `<map[string]interface {} Value>` instead of a JSON object.

## Reproduction

Use a templated map value in an HTTP POST body:

```yaml
entrypoint: main
vars:
  my_map: '{"foo": "bar"}'

step_types:
  agent_job:
    base: http_request
    params:
      base_url: http://example.test
      path: /jobs
      method: POST

workflows:
  - name: main
    steps:
      - name: call_api
        type: agent_job
        params:
          body:
            name: my-job
            args:
              some_map: "{{ my_map }}"
```

The same problem can appear when a parent workflow passes a JSON-looking value into a sub-workflow variable and the sub-workflow later embeds that value in an HTTP body:

```yaml
- name: do_work
  workflow: sub-workflow
  vars:
    config: '{"key": "value"}'
```

## Expected

The HTTP request body should preserve the structured map value:

```json
{
  "name": "my-job",
  "args": {
    "some_map": {
      "foo": "bar"
    }
  }
}
```

Markov should either preserve map/list values automatically when a template field is exactly a single value expression, or provide and document an explicit filter such as `to_json`:

```yaml
some_map: "{{ my_map | to_json }}"
```

## Actual

The HTTP request body can contain a string representation of the Go/Pongo value instead of an object:

```json
{
  "name": "my-job",
  "args": {
    "some_map": "<map[string]interface {} Value>"
  }
}
```

## Suspected Cause

Relevant paths:

- `pkg/template/template.go`
- `pkg/engine/facts.go`
- `pkg/engine/engine.go`
- `pkg/executor/http_request.go`

`RenderMap` recursively renders strings with Pongo2 and returns rendered strings. `coerceString` can parse JSON-looking strings into native values in some engine paths, including sub-workflow variable passing, but a native map rendered later through `{{ config }}` is converted back through Pongo's string rendering path. That loses structure before `http_request` JSON-encodes the body.

## Impact

HTTP APIs that expect nested object arguments receive garbage strings. This affects `agent_job`-style step types that POST to an orchestrator and can break dashboard job submission, skill execution, and sub-workflow-driven request bodies.

## Acceptance Criteria

- Exact single-value template expressions that resolve to maps or lists can be used in HTTP request bodies without becoming Go debug strings.
- Sub-workflow variables containing JSON objects can be passed into a child workflow and used in an HTTP body as structured JSON.
- If a `to_json` filter is added, it is documented and covered by tests.
- Regression tests cover map and list values in nested `http_request` body parameters.
- Existing string interpolation behavior still works for mixed strings such as `"prefix {{ value }} suffix"`.

## Fix

- Added `to_json` and `tojson` filters for JSON serialization.
- Added `from_json` as an alias for the existing `fromjson` filter.
- Updated exact single-expression rendering so `{{ path }}` preserves native map/list values in structured fields.
- JSON-looking strings that begin with `{` or `[` are parsed for exact single-expression fields, which preserves HTTP body structure.
- Mixed templates continue to render as strings; use `to_json` when JSON text is needed inside a larger string.

## Verification

- `GOCACHE=/tmp/go-build go test ./pkg/template ./pkg/engine ./pkg/executor`
