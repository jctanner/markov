# Task: Add Headers and Auth Support to http_request

## Goal

Allow `http_request` steps to send custom HTTP headers, including standard auth patterns (Bearer token, Basic auth), so that authenticated API calls don't require falling back to `shell_exec` + `curl`.

## Context

The `http_request` executor (`pkg/executor/http_request.go`) currently only sets `Content-Type: application/json` when a body is present. There is no way to pass `Authorization`, `Accept`, or any other header.

This forces every authenticated API call (Jira basic auth, GitHub token auth, etc.) to use `shell_exec` with inline `curl`, losing the benefits of `http_request`:
- Structured output (`status_code`, `body` auto-parsed as JSON)
- Declarative step type composition via `base_url` / `path`
- No shell quoting or escaping concerns

The `ai-first-pipeline` end-to-end demo workflow (`var/demos/end-to-end/`) wants to define three custom step types based on `http_request`:

```yaml
# step_types.yaml
jira_api:
  base: http_request
  description: "Jira REST API call with basic auth"
  params:
    base_url: "{{ jira_base }}"
    basic_auth:
      username: admin
      password: admin

github_api:
  base: http_request
  description: "GitHub API call with token auth"
  params:
    base_url: "{{ github_base }}/api/v3"
    headers:
      Authorization: "token {{ github_api_token }}"

github_admin_api:
  base: http_request
  description: "GitHub admin API (unauthenticated)"
  params:
    base_url: "{{ github_base }}/api/v3/admin"
```

None of these work today because `headers` and `basic_auth` are silently ignored by the executor.

## Proposed Parameters

Add two new parameters to `http_request`:

| Param | Type | Description |
|-------|------|-------------|
| `headers` | `map[string]string` | Custom HTTP headers applied to the request. Merged after the automatic `Content-Type` header (custom `Content-Type` overrides the default). |
| `basic_auth` | `{username, password}` | Sets the `Authorization` header using HTTP Basic Auth. Convenience shorthand — equivalent to setting the header manually. |

If both `basic_auth` and an explicit `Authorization` header are provided, `basic_auth` takes precedence.

## Proposed Parameter: ignore_status

Additionally:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `ignore_status` | `[]int` or `bool` | `nil` | Status codes that should NOT cause step failure. `true` ignores all >= 400 codes. A list like `[404, 409, 422]` ignores only those. |

This is a common need: DELETE calls that should tolerate 404, org-create calls that should tolerate 422 (already exists). Today these require `shell_exec` + `curl` just to handle the status check.

## Acceptance Criteria

- [x] `http_request` accepts a `headers` map and applies all entries to the outgoing request.
- [x] `http_request` accepts `basic_auth` with `username` and `password` fields, setting the `Authorization` header via `req.SetBasicAuth()`.
- [x] Custom step types can pre-fill `headers` and `basic_auth` in their `params`, and steps using the type can add/override individual headers.
- [x] `Content-Type: application/json` is still set automatically when `body` is present, but can be overridden via `headers`.
- [x] Existing workflows with no `headers` or `basic_auth` are unaffected.
- [x] `ignore_status` prevents step failure for specified HTTP status codes while still populating output variables.
- [x] Reference docs updated: `docs/reference/step-types.md` (http_request section).
- [x] Custom step types docs updated: `docs/reference/custom-step-types.md` (add auth example).
- [x] Tests cover: headers applied, basic_auth applied, header override in step vs step_type, ignore_status behavior.

## Files Likely Involved

- `pkg/executor/http_request.go` — add `headers`, `basic_auth`, `ignore_status` param handling
- `pkg/executor/http_request_test.go` — new test cases
- `docs/reference/step-types.md` — update http_request parameter table and add examples
- `docs/reference/custom-step-types.md` — add authenticated step type example

## Implementation Notes

The change to `http_request.go` is small. The core of it:

```go
// After creating the request, before e.client.Do(req):

// Apply custom headers
if hdrs, ok := params["headers"].(map[string]any); ok {
    for k, v := range hdrs {
        if s, ok := v.(string); ok {
            req.Header.Set(k, s)
        }
    }
}

// Apply basic auth (overrides any manual Authorization header)
if auth, ok := params["basic_auth"].(map[string]any); ok {
    user, _ := auth["username"].(string)
    pass, _ := auth["password"].(string)
    req.SetBasicAuth(user, pass)
}

// Apply ignore_status before the >= 400 check
```

The `ignore_status` check goes where the current `resp.StatusCode >= 400` error return is, replacing it with a check against the ignore list.
