# Adding `fromjson` filter to markov's template engine

## Problem

Workflow `set_fact` steps need to parse JSON strings (e.g. stdout from `shell_exec`) into structured data. Pongo2 does not ship a `fromjson` filter, so expressions like `{{ fetch_tickets.stdout | fromjson }}` fail with "Filter 'fromjson' does not exist."

## Where to add it

`pkg/template/template.go` — register a custom pongo2 filter via `pongo2.RegisterFilter` in an `init()` function.

## Implementation

- Filter name: `fromjson`
- Input: a pongo2 `*Value` whose `.String()` is valid JSON
- Output: a pongo2 `*Value` wrapping the parsed Go value (`map[string]any`, `[]any`, etc.)
- Use `encoding/json.Unmarshal` into `any`, then wrap with `pongo2.AsValue()`
- On parse error, return a `*pongo2.Error` with sender `"filter:fromjson"`

## Usage in workflows

```yaml
- name: fetch_tickets
  type: shell_exec
  params:
    command: "python3 -c '...script that outputs JSON array to stdout...'"

- name: parse_tickets
  type: set_fact
  vars:
    tickets: "{{ fetch_tickets.stdout | fromjson }}"

- name: process_tickets
  for_each: "tickets"
  as: ticket
  workflow: per-ticket
  vars:
    issue: "{{ ticket.key }}"
```

## Usage notes

### `set_fact` requires a simple expression

For `set_fact` to preserve the parsed structure (array/object), the
template must be a simple `{{ path | fromjson }}` expression — no
surrounding text, no chained filters:

```yaml
# Works — engine detects the pattern and parses JSON directly
items: "{{ produce_json.stdout | fromjson }}"

# Works — nested context paths are supported
data: "{{ step_name.output.some_field | fromjson }}"

# Does NOT work as structured data — falls back to string rendering
items: "Got: {{ produce_json.stdout | fromjson }}"
```

The engine intercepts the `{{ path | fromjson }}` pattern in `set_fact`
and resolves the context path directly, bypassing pongo2's string
rendering. Without this, pongo2 would stringify the parsed value back
into Go's `fmt.Sprintf` representation (`[map[key:value ...]]`), which
is not valid JSON and cannot be recovered.

### In-template filters work normally

Inside template expressions (not `set_fact`), the parsed value is a
native pongo2 value, so filters like `length` work:

```yaml
command: "echo 'Found {{ items | length }} items'"
```

### Source value must be a JSON string

The context path must resolve to a string containing valid JSON. If the
value is already structured (e.g., from YAML vars), `fromjson` is not
needed — use the value directly.

## Testing

Validated with `examples/fromjson-test.yaml`:

```yaml
entrypoint: main
workflows:
  - name: main
    steps:
      - name: produce_json
        type: shell_exec
        params:
          command: "echo '[{\"key\":\"PROJ-100\"},{\"key\":\"PROJ-200\"}]'"
        register: produce_json

      - name: parse_it
        type: set_fact
        vars:
          items: "{{ produce_json.stdout | fromjson }}"

      - name: show_count
        type: shell_exec
        params:
          command: "echo 'Parsed {{ items | length }} items'"

      - name: process_items
        for_each: items
        for_each_key: "key"
        for_each_sort: "key"
        as: item
        concurrency: 1
        workflow: handle_item
        vars:
          item_key: "{{ item.key }}"

  - name: handle_item
    steps:
      - name: log_item
        type: shell_exec
        params:
          command: "echo 'Item {{ item_key }}'"
```

Run: `markov run examples/fromjson-test.yaml --verbose`
