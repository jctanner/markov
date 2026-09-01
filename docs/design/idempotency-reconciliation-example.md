# Proposed Idempotency and Reconciliation Workflow

> **Design example — not executable yet.** The `idempotency` block shown below
> is proposed syntax for the next feature; current Markov releases do not
> recognize or enforce it.

This example publishes a release through an HTTP API. It demonstrates the
unsafe retry case: the API may create the release successfully, but Markov may
crash or lose connectivity before it saves the step as `completed`. A normal
resume would issue the POST again.

```yaml
entrypoint: release

vars:
  repository: acme/payments
  git_sha: "abc123def"
  release_name: "payments-2026.09.01"

step_types:
  releases_api:
    base: http_request
    params:
      base_url: "https://releases.example.test/api"
      headers:
        Authorization: "Bearer {{ release_token }}"

workflows:
  - name: release
    steps:
      - name: verify-artifact
        type: assert
        that:
          - "git_sha != ''"

      # Proposed behavior:
      # 1. Persist this stable key before attempting the effect.
      # 2. On retry, GET the external release first.
      # 3. If it exists with this key, adopt it as this step's completed output.
      # 4. Otherwise POST once, forwarding the key in Idempotency-Key.
      - name: publish-release
        type: releases_api
        register: published_release
        idempotency:
          key: "release:{{ repository }}:{{ git_sha }}"
          request_header: Idempotency-Key
          reconcile:
            type: releases_api
            params:
              method: GET
              path: "/releases/{{ repository }}/{{ git_sha }}"
            found_when: >-
              status_code == 200 and
              body.metadata.markov_idempotency_key ==
              'release:{{ repository }}:{{ git_sha }}'
        params:
          method: POST
          path: "/releases"
          body:
            repository: "{{ repository }}"
            name: "{{ release_name }}"
            git_sha: "{{ git_sha }}"
            metadata:
              markov_idempotency_key: "release:{{ repository }}:{{ git_sha }}"

      - name: announce-release
        type: shell_exec
        params:
          command: >-
            echo "published {{ published_release.body.url }}"
```

## Resume outcomes

| Situation when `publish-release` resumes | Reconciliation result | Action |
|---|---|---|
| The original POST never reached the API | No release exists | Send the POST with the same `Idempotency-Key`. |
| The API created the release but Markov did not checkpoint it | Release exists and carries the same key | Adopt the external release as completed; do not POST again. |
| A release exists with a different key | Conflicting external state | Fail safely and require an operator decision. |
| The API cannot be queried | Reconciliation is inconclusive | Fail safely; do not blindly POST. |

## Required runtime contract

The proposed feature would persist an effect intent before performing the
external mutation, then save an effect receipt after it succeeds or is
reconciled. The key must be stable across retries and should be derived from
the logical effect (repository plus revision above), not from a random run ID.

`http_request` is the first useful executor because many APIs understand an
idempotency header. Other executors need explicit reconciliation adapters: a
Jira comment might query by a marker, and a `script_exec` step might run a
read-only check script. Markov must not claim generic retry safety for an
executor until it can establish one of the outcomes above.
