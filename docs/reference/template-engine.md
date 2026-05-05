# Template Engine Reference

Markov uses [Pongo2 v6](https://github.com/flosch/pongo2/v6) for template rendering -- a Jinja2/Django-compatible template engine for Go. Templates are evaluated at runtime against the workflow context to produce dynamic values for step parameters, conditions, and variable assignments.

## Variable Substitution

```yaml
# Simple value
command: "echo {{ message }}"

# Nested field access
image: "{{ config.registry }}/{{ config.image_name }}"

# Array index access
first_item: "{{ items.0 }}"
also_works: "{{ items[0] }}"
```

| Syntax | Description |
|---|---|
| `{{ variable }}` | Substitute a simple value from context |
| `{{ variable.field }}` | Access a nested map field |
| `{{ array.0 }}` or `{{ array[0] }}` | Access an array element by index |

## Filters

Filters transform values using the pipe (`|`) operator.

```yaml
# Single filter
greeting: "{{ name | upper }}"

# Filter with argument
label: "{{ description | truncate:50 }}"

# Chained filters
slug: "{{ title | lower | cut:' ' }}"
```

| Syntax | Description |
|---|---|
| `{{ value \| filter }}` | Apply a single filter |
| `{{ value \| filter:arg }}` | Apply a filter with an argument |
| `{{ value \| filter1 \| filter2 }}` | Chain multiple filters left to right |

## Custom Filter: fromjson

Defined in `pkg/template/template.go`. Parses a JSON string into a native Go value (map, list, number, bool, etc.).

```yaml
vars:
  parsed_data: "{{ raw_json | fromjson }}"
```

**Input must be valid JSON.** If the input string is not valid JSON, the filter returns an error and the step fails.

### Special handling in set_fact

When a `set_fact` var value is **exactly** `{{ path | fromjson }}` (no surrounding text), the engine bypasses normal template rendering. Instead, it:

1. Resolves the dot-path directly from context
2. JSON-parses the raw string value
3. Stores the parsed object/array/value preserving its structure

This avoids the flattening that would occur if pongo2 rendered the value to a string first.

```yaml
# This preserves object structure (direct resolution path)
- name: parse_config
  type: set_fact
  vars:
    config: "{{ raw_config_json | fromjson }}"

# This does NOT get special handling (has surrounding text)
- name: broken
  type: set_fact
  vars:
    config: "prefix {{ raw_config_json | fromjson }} suffix"
```

The special path is triggered only when the entire value matches the pattern `{{ <path> | fromjson }}` with nothing before or after the braces.

## Built-in Pongo2 Filters

These are the most commonly useful filters from Pongo2's built-in set.

| Filter | Description | Example |
|---|---|---|
| `length` | Length of string, list, or map | `{{ items \| length }}` |
| `default` | Fallback if value is empty/nil | `{{ name \| default:"unknown" }}` |
| `upper` | Uppercase string | `{{ status \| upper }}` |
| `lower` | Lowercase string | `{{ name \| lower }}` |
| `join` | Join list elements with separator | `{{ tags \| join:", " }}` |
| `split` | Split string into list | `{{ csv_line \| split:"," }}` |
| `first` | First element of list/string | `{{ items \| first }}` |
| `last` | Last element of list/string | `{{ items \| last }}` |
| `add` | Add a number | `{{ count \| add:1 }}` |
| `truncate` | Truncate string to length | `{{ desc \| truncate:80 }}` |
| `title` | Title-case string | `{{ name \| title }}` |
| `capitalize` | Capitalize first character | `{{ word \| capitalize }}` |
| `striptags` | Remove HTML tags | `{{ html \| striptags }}` |
| `safe` | Mark as safe (no auto-escaping) | `{{ raw_html \| safe }}` |
| `escapejs` | Escape for JavaScript strings | `{{ val \| escapejs }}` |
| `floatformat` | Format float precision | `{{ price \| floatformat:2 }}` |
| `wordcount` | Count words in string | `{{ text \| wordcount }}` |
| `yesno` | Map true/false/nil to strings | `{{ flag \| yesno:"yes,no,maybe" }}` |
| `slice` | Slice a list | `{{ items \| slice:"1:3" }}` |
| `reverse` | Reverse a list or string | `{{ items \| reverse }}` |
| `pluralize` | Pluralize suffix based on count | `{{ count \| pluralize:"item,items" }}` |
| `center` | Center-pad a string | `{{ name \| center:20 }}` |
| `ljust` | Left-justify (right-pad) string | `{{ name \| ljust:20 }}` |
| `rjust` | Right-justify (left-pad) string | `{{ num \| rjust:10 }}` |
| `cut` | Remove all occurrences of a string | `{{ phone \| cut:"-" }}` |

## Control Structures

Pongo2 supports Jinja2-style control flow. These are available in any template context, though they are rarely needed because markov provides `for_each` and `when` at the step level.

```yaml
# Conditional
command: >
  {% if environment == "prod" %}
    deploy --replicas=3
  {% elif environment == "staging" %}
    deploy --replicas=1
  {% else %}
    echo "skipping deploy"
  {% endif %}

# Loop (prefer markov's for_each for step-level iteration)
script: >
  {% for host in servers %}
    echo "Checking {{ host }}"
  {% endfor %}

# Comments (stripped from output)
value: "{{ name }} {# this comment is removed #}"
```

## Where Templates Are Evaluated

Not every field in a step definition goes through template rendering. The following table shows which fields are template-evaluated.

| Field | Evaluated | Notes |
|---|---|---|
| `params` values | Yes | All string values in the params map |
| `vars` values | Yes | Both step-level and workflow-level |
| `when` | Yes | Evaluated as boolean expression |
| `artifacts[].path` | Yes | Artifact path field |
| `facts` values | Yes | Gate step fact values |
| `msg` | Yes | Assert step failure message |
| `that` expressions | Yes | Assert step conditions |
| `name` | No | Step name is a static identifier |
| `type` | No | Step type is resolved, not rendered |
| `register` | No | Register key is a literal context key |
| `for_each_key` | No | Field name, not a template |
| `for_each_sort` | No | Field name, not a template |
| `as` | No | Iterator variable name is literal |

## Boolean Evaluation

The `EvalBool` function (in `template.go:57`) evaluates a pongo2 expression and returns a boolean result. It works by wrapping the expression:

```
{% if <expr> %}true{% endif %}
```

If the rendered output (trimmed) equals `"true"`, the result is `true`. Otherwise, `false`.

**Used for:** `when` conditions, `assert` `that` expressions, and plain string values in `set_fact` vars (strings without `{{` or `{%` delimiters).

**Pongo2 truthiness rules:**

| Value | Truthy? |
|---|---|
| `nil` | false |
| `""` (empty string) | false |
| `0` (zero) | false |
| `false` | false |
| Empty list `[]` | false |
| Empty map `{}` | false |
| Everything else | true |

## Type Coercion

After template rendering in `set_fact` steps, string results are coerced to native Go types by `coerceString` (in `facts.go:146`). This ensures that computed values have the expected types downstream.

| Rendered String | Coerced Type | Coerced Value |
|---|---|---|
| `"true"` or `"True"` | `bool` | `true` |
| `"false"` or `"False"` | `bool` | `false` |
| `"None"` | `string` | `"None"` (kept as-is) |
| `""` (empty) | `string` | `""` (kept as-is) |
| `"42"` | `int` | `42` |
| `"3.14"` | `float64` | `3.14` |
| `'["a","b"]'` | `[]any` | Parsed JSON array |
| `'{"k":"v"}'` | `map[string]any` | Parsed JSON object |
| Anything else | `string` | Kept as-is |

Coercion order matters: integer parsing is attempted before float parsing. JSON parsing is only attempted if the string starts with `[` or `{`.
