# ADR-0002: Source Integrity Modes for Resumable Workflow Runs

## Status

Accepted

## Context

Markov resumes a run by reloading its workflow from the recorded source path. A
workflow source may be a directory containing YAML, rule files, custom step
types, and `scripts/` used by `script_exec`. If that source changes between a
failure and resume, Markov can otherwise execute different behavior under the
same run ID.

Saving a complete source snapshot with every run would preserve reproducibility,
but can be unnecessarily expensive for large workflow projects and is awkward
for the normal local edit-and-resume development loop.

## Decision

Markov will fingerprint the complete workflow source tree instead of storing a
source snapshot. The fingerprint includes every regular source file by relative
path, content, and executable mode, plus the target of each symlink. Entries are
sorted before deriving the tree digest. Repository metadata and Markov state
files are excluded from the source tree.

Directory workflows use the supplied directory as their source root. A
single-file workflow uses its containing directory, so sibling files available
to the workflow, including `scripts/`, are also covered.

The run record retains the original source-tree digest. Each resume attempt
records the digest it observed and whether it differs from the original. Markov
does not silently overwrite the original digest.

Source-integrity behavior is selectable:

| Mode | Behavior on resume when the source digest differs |
|---|---|
| `warn` | Continue with the current source and record/report source drift. This is the default for interactive development. |
| `strict` | Refuse to resume. Users must restore the original source tree or start a new run. Intended for CI and production automation. |
| `off` | Do not compare source digests. Intended only for compatibility cases. |

The initial implementation will expose the mode through CLI configuration, such
as `--source-integrity strict`. Status output and callbacks must make any
accepted source drift visible.

## Consequences

Positive:

- Production runs can prevent a resume from silently using changed YAML or
  scripts without paying snapshot-storage costs.
- Local users can intentionally edit a workflow and retain completed-step
  checkpoints while iterating.
- A run retains an audit trail of its original source and every drifted resume
  attempt.
- New file-backed workflow features are automatically included without a
  separate allowlist.

Negative:

- `warn` resumes are not fully reproducible; their run history must be treated
  as source-drifted.
- Hashing a large workflow tree adds I/O at run and resume time.
- Without snapshots, a strict resume requires the user to restore the exact
  original source tree; Markov cannot do that on its own.
- This decision protects source identity at resume, but does not by itself
  reconcile a partially completed external side effect.

## Follow-Up

Implement source-tree fingerprinting and persistence consistently for SQLite
and Postgres. Add tests for unchanged source, strict rejection after a YAML or
script edit, accepted drift in `warn` mode, and status/callback visibility.
