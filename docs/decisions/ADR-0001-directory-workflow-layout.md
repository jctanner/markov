# ADR-0001: Directory Workflow File Layout

## Status

Proposed

## Context

Large Markov workflows become difficult to maintain as a single YAML document. Users need a consistent multi-file layout without losing the simplicity and portability of the existing single-file format.

## Decision

Add directory workflow support using required conventional filenames:

```text
meta.yaml
vars.yaml
rules.yaml
step_types.yaml
workflows/*.yaml
```

The loader will merge those files into the existing parser model:

- `meta.yaml` provides `entrypoint`, `namespace`, and `forks`.
- `vars.yaml` provides top-level `vars`.
- `rules.yaml` provides top-level `rules`.
- `step_types.yaml` provides top-level `step_types`.
- Each `workflows/*.yaml` file provides exactly one workflow object directly.

The existing single-file format remains supported.

## Rationale

Required filenames make the layout predictable for users and tooling. They also avoid ambiguous recursive YAML discovery and keep validation errors easy to explain.

A direct workflow object under `workflows/*.yaml` avoids a redundant wrapping key. The directory name already provides the category.

The engine should not know whether input came from one file or many files. Directory loading should happen before validation and execution, producing the same `WorkflowFile` structure already used by Markov.

## Consequences

Positive:

- Larger workflows become easier to organize and review.
- Existing engine execution behavior stays unchanged.
- Users get one standard project layout.
- Existing single-file workflows remain portable.

Negative:

- Directory mode introduces a second input format to document and test.
- File path resolution needs a clear contract.
- Required filenames are less flexible than arbitrary include patterns.

## Follow-Up

If users later need more flexibility, Markov can add optional category directories such as `rules/*.yaml` or `step_types/*.yaml` in a later ADR.

