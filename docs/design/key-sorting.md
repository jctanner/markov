{% raw %}
# Feature Request: Stable `for_each` Iteration Keys

## Problem

`for_each` uses the array index as `ForEachKey` (`"0"`, `"1"`, `"2"`, …).
This means the identity of each sub-run is tied to the item's position in
the list, not to a domain-meaningful key. If the source list is reordered
between runs (or between a failure and a resume), the engine maps the
wrong sub-run state to the wrong item.

### Concrete scenario

The RFE pipeline creates RHOAIENG epics from a strategy and writes a
receipt artifact:

```yaml
epics:
  - key: RHOAIENG-19201
    title: "[Eng RHAISTRAT-1] ..."
    type: eng
  - key: RHOAIENG-19202
    title: "[QE RHAISTRAT-1] ..."
    type: qe
```

A downstream `for_each` fans out a sub-workflow per epic:

```yaml
- name: per_epic_work
  for_each: "receipt.frontmatter.epics"
  as: epic
  workflow: epic-breakdown
  vars:
    epic_key: "{{ epic.key }}"
```

Today the sub-run for `RHOAIENG-19201` gets `ForEachKey: "0"` and
`RHOAIENG-19202` gets `ForEachKey: "1"`. If the receipt were ever
regenerated with the items in a different order, resume would replay the
wrong items. We can mitigate this by never regenerating the receipt (the
workflow already gates on its existence), but the engine shouldn't require
upstream producers to guarantee list order — that's fragile.

## Proposed Solution

### `for_each_key` field

Add an optional `for_each_key` field to the step schema that names a
property on each item to use as the stable iteration key:

```yaml
- name: per_epic_work
  for_each: "receipt.frontmatter.epics"
  for_each_key: "key"          # ← use epic.key as ForEachKey
  as: epic
  workflow: epic-breakdown
```

When `for_each_key` is set:

1. For each item, extract `item[for_each_key]` and stringify it.
2. Use that value as `ForEachKey` in the sub-run record instead of the
   array index.
3. Validate uniqueness across items — fail the step if two items produce
   the same key.

When `for_each_key` is omitted, behavior is unchanged (array index).

### `for_each_sort` field

Add an optional `for_each_sort` field that sorts the list before
iteration:

```yaml
- name: per_epic_work
  for_each: "receipt.frontmatter.epics"
  for_each_key: "key"
  for_each_sort: "key"         # ← sort by epic.key ascending
  as: epic
  workflow: epic-breakdown
```

When `for_each_sort` is set:

1. Sort the resolved list by `item[for_each_sort]` (string comparison,
   ascending) before dispatching items.
2. Combined with `for_each_key`, this makes iteration order deterministic
   and key assignment stable regardless of how the source list was
   produced.

`for_each_sort` without `for_each_key` is still useful — it guarantees
deterministic execution order even when using index-based keys, as long
as the list contents don't change between runs.

### Implementation notes

**Parser** (`pkg/parser/types.go`):

```go
type Step struct {
    // ... existing fields ...
    ForEachKey  string `yaml:"for_each_key"`
    ForEachSort string `yaml:"for_each_sort"`
}
```

**Engine** (`pkg/engine/engine.go`, `executeForEach`):

```go
// After resolveForEachList(), before dispatching:

if step.ForEachSort != "" {
    sort.SliceStable(listVal, func(i, j int) bool {
        ki := fmt.Sprintf("%v", extractField(listVal[i], step.ForEachSort))
        kj := fmt.Sprintf("%v", extractField(listVal[j], step.ForEachSort))
        return ki < kj
    })
}

// In the goroutine, replace index-based key:
forEachKey := fmt.Sprintf("%d", idx)
if step.ForEachKey != "" {
    forEachKey = fmt.Sprintf("%v", extractField(itemVal, step.ForEachKey))
}
```

**State store**: No schema changes needed — `ForEachKey` is already a
string column. The only change is that values like `"RHOAIENG-19201"`
appear instead of `"0"`.

**Resume**: `GetChildRuns` already matches by `ForEachKey`. Using
domain keys makes resume correct even if the list order shifts. The
engine should match completed child runs by `ForEachKey` value (already
does), and only retry items whose key has no completed child run.

### Validation

- `for_each_key` specified but the named field is missing from an item →
  fail with: `for_each_key "key" not found on item at index N`
- Duplicate keys across items → fail with:
  `duplicate for_each_key "RHOAIENG-19201" at indices 0 and 3`
- `for_each_sort` field missing from an item → fail with:
  `for_each_sort "key" not found on item at index N`

## Example: Full Fan-Out with Stable Keys

```yaml
- name: load_epic_receipt
  type: load_artifact
  artifacts:
    receipt:
      path: "/app/artifacts/epic-receipts/{{ strat_key }}.md"
      format: markdown
      source: local

- name: per_epic_work
  for_each: "load_epic_receipt.artifacts.receipt.frontmatter.epics"
  for_each_key: "key"
  for_each_sort: "key"
  as: epic
  concurrency: 3
  workflow: epic-breakdown
  vars:
    epic_key: "{{ epic.key }}"
    epic_type: "{{ epic.type }}"
    strat_key: "{{ strat_key }}"
    model: "{{ model }}"
```

Sub-runs would be created with `ForEachKey` values like
`"RHOAIENG-19201"` and `"RHOAIENG-19202"` — stable across restarts,
resumes, and reordering.
{% endraw %}
