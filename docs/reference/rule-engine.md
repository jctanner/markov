# Rule Engine Reference

Markov includes a forward-chaining rule engine powered by [Grule](https://github.com/hyperjumptech/grule-rule-engine). Rules are defined at the top level of the workflow file and evaluated by `gate` steps. The engine translates Python/Jinja2-like condition syntax into Grule's GRL format, so rule authors do not need to learn GRL directly.

## Rule Definition

Rules are defined in the top-level `rules:` block of the workflow file.

```yaml
rules:
  - name: approve_high_confidence
    description: "Auto-approve when confidence is high"
    salience: 10
    when: "confidence > 0.9 and risk == 'low'"
    action: continue
    set_fact:
      approved: true
      approval_reason: "auto-approved: high confidence, low risk"

  - name: reject_critical
    description: "Reject critical severity items"
    salience: 20
    when: "severity == 'critical' and not override"
    action: skip
    set_fact:
      approved: false
      rejection_reason: "blocked: critical severity"
```

### Rule fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | Yes | -- | Unique identifier for the rule. Used to reference it from gate steps. |
| `description` | string | No | `""` | Human-readable description. Used as the GRL rule description. |
| `salience` | int | No | `0` | Priority. Higher values fire first. When multiple rules fire, the action from the highest-salience rule is used. |
| `when` | string | Yes | -- | Condition expression in Python/Jinja2-like syntax (see Condition Syntax below). |
| `action` | string | No | `"continue"` | What happens when this rule fires: `continue`, `skip`, or `pause`. |
| `set_fact` | map | No | -- | Variables to set when the rule fires. |
| `file` | string | No | -- | Load rules from an external YAML file instead of defining inline. |

### Actions

| Action | Description |
|---|---|
| `continue` | Proceed with the workflow (default). |
| `skip` | Skip remaining steps. |
| `pause` | Pause execution (logged but not yet fully implemented; currently continues). |

## Rule Condition Syntax

The `when` field uses Python/Jinja2-like syntax that gets tokenized and translated to GRL format by the translator in `grltranslate.go`.

### Supported tokens

| Token Type | Examples | Description |
|---|---|---|
| Identifiers | `severity`, `count`, `auto_approved` | Bare words (letters, digits, underscores) |
| Strings | `'value'` or `"value"` | Single or double quoted |
| Numbers | `42`, `3.14`, `-1` | Integers and decimals, including negative |
| Comparison operators | `==`, `!=`, `<`, `>`, `<=`, `>=` | Standard comparisons |
| Logical operators | `and`, `or`, `not` | Boolean logic |
| Parentheses | `(`, `)` | Grouping |
| Literals | `None`, `true`, `True`, `false`, `False` | Special values |

### Translation rules

The translator converts YAML expressions to GRL calls on the FactStore. Here is the complete mapping:

| YAML Expression | GRL Translation | Description |
|---|---|---|
| `severity` | `Facts.IsTrue("severity")` | Bare identifier -- truthiness check |
| `not auto_approved` | `!Facts.IsTrue("auto_approved")` | Negated truthiness |
| `severity == 'high'` | `Facts.GetStr("severity") == "high"` | String comparison |
| `severity != 'low'` | `Facts.GetStr("severity") != "low"` | String inequality |
| `count > 5` | `Facts.GetNum("count") > 5` | Numeric comparison |
| `count <= 100` | `Facts.GetNum("count") <= 100` | Numeric less-than-or-equal |
| `confidence == None` | `Facts.IsNil("confidence")` | Null check |
| `confidence != None` | `!Facts.IsNil("confidence")` | Not-null check |
| `approved == true` | `Facts.IsTrue("approved")` | Boolean true check |
| `approved == false` | `!Facts.IsTrue("approved")` | Boolean false check |
| `approved != true` | `!Facts.IsTrue("approved")` | Negated boolean true |
| `approved != false` | `Facts.IsTrue("approved")` | Negated boolean false |
| `a == b` (ident == ident) | `Facts.GetStr("a") == Facts.GetStr("b")` | Two-identifier string comparison |
| `a > b` (ident > ident) | `Facts.GetNum("a") > Facts.GetNum("b")` | Two-identifier numeric comparison |
| `x and y` | `x && y` | Logical AND |
| `x or y` | `x \|\| y` | Logical OR |
| `(a or b) and c` | `(a \|\| b) && c` | Grouped expression |

**Key detail for identifier-vs-identifier comparisons:** When comparing two identifiers, `==` and `!=` use `GetStr` (string comparison), while `<`, `>`, `<=`, `>=` use `GetNum` (numeric comparison).

### Combining conditions

```yaml
when: "severity == 'critical' and (count > 10 or override == true)"
when: "not auto_approved and confidence > 0.8"
when: "status != None and status != 'pending'"
```

## FactStore

The FactStore (`factstore.go`) wraps a `map[string]any` and provides typed accessor methods. These methods are callable from GRL rule expressions and are the bridge between your YAML conditions and the Grule engine.

| Method | Signature | Return | Description |
|---|---|---|---|
| `Get` | `Get(key string)` | `any` | Raw value from the map |
| `GetStr` | `GetStr(key string)` | `string` | String value. Returns `""` if missing or nil. Non-strings are converted via `fmt.Sprintf`. |
| `GetNum` | `GetNum(key string)` | `float64` | Numeric value. Returns `0` if missing, nil, or not a number. Parses numeric strings. Handles int, int32, int64, float32, float64. |
| `IsTrue` | `IsTrue(key string)` | `bool` | Truthiness check. Returns `false` for: missing key, nil, `""`, `0`, `false`. Returns `true` for everything else. |
| `IsNil` | `IsNil(key string)` | `bool` | Returns `true` if the key is missing or its value is nil. |
| `Set` | `Set(key string, val any)` | -- | Set any value |
| `SetStr` | `SetStr(key string, val string)` | -- | Set a string value |
| `SetBool` | `SetBool(key string, val bool)` | -- | Set a boolean value |
| `SetNum` | `SetNum(key string, val float64)` | -- | Set a float64 value |
| `MarkFired` | `MarkFired(name string)` | -- | Mark a rule as fired (internal use) |
| `HasFired` | `HasFired(name string)` | `bool` | Check if a rule has fired (internal use) |

### IsTrue truthiness table

| Value | `IsTrue` returns |
|---|---|
| Missing key | `false` |
| `nil` | `false` |
| `""` (empty string) | `false` |
| `0` (int, int64, float64) | `false` |
| `false` (bool) | `false` |
| Non-empty string | `true` |
| Non-zero number | `true` |
| `true` (bool) | `true` |
| Any other type (maps, lists, etc.) | `true` |

## Gate Evaluation Flow

The gate evaluation process (`evaluateGate` in `gate.go`) follows these steps:

1. **Resolve rules** -- Look up each rule name from the gate's `rules:` list in the file-level `rules:` definitions.

2. **Build evaluation context** -- Copy the entire runtime context (`runCtx`), then template-render and merge the gate's `facts:` values on top.

3. **Create FactStore** -- Initialize a FactStore from the evaluation context map.

4. **Compile rules to GRL** -- Each YAML rule is translated to GRL format via the tokenizer/translator. The compiled GRL includes the condition, set_fact assignments, a `MarkFired` call, a `Changed` call, and a `Retract` call (each rule fires at most once).

5. **Load into Grule** -- GRL source is loaded into a Grule knowledge base (name: `"gate"`, version: `"1.0.0"`).

6. **Execute** -- The Grule engine runs with `MaxCycle=100` (forward chaining, up to 100 rule evaluation cycles).

7. **Collect results** -- Iterate over rules to find which ones fired. The action from the highest-salience fired rule becomes the gate's action.

8. **Merge facts** -- `set_fact` values from fired rules and gate-level `facts:` are merged back into `runCtx`.

## Gate Step Configuration

```yaml
- name: quality_gate
  type: gate
  rules:
    - rule_approve
    - rule_reject
  facts:
    score: "{{ analysis.score }}"
    severity: "{{ ticket.severity }}"
```

### Gate fields

| Field | Type | Required | Description |
|---|---|---|---|
| `rules` | list of strings | Yes | Names of rules to evaluate (must exist in top-level `rules:`) |
| `facts` | map | No | Additional variables to set in the evaluation context. Values are template-rendered against `runCtx`. |

### Facts scoping

Rules see these variables during evaluation:

- Global vars (file-level `vars:`)
- Workflow vars
- CLI `--var` overrides
- Values set by prior `set_fact` steps
- Registered step outputs
- Gate-level `facts:` (rendered and merged last, so they can override anything above)

Rules do **not** see step register outputs unless they are explicitly mapped through `facts:`.

```yaml
# Without facts mapping, the gate cannot see api_result:
- name: check_api
  type: http_request
  register: api_result
  params:
    url: "https://api.example.com/status"

# Map the value you need into the gate's facts:
- name: api_gate
  type: gate
  rules: [rule_api_healthy]
  facts:
    api_status: "{{ api_result.status_code }}"
```

## set_fact in Rules

When a rule fires, its `set_fact` values are compiled into GRL `then` block assignments. The compilation maps Go types to FactStore setter methods:

| Go Type | GRL Call | Example |
|---|---|---|
| `bool` | `Facts.SetBool("key", true)` | `approved: true` |
| `string` | `Facts.SetStr("key", "value")` | `reason: "auto-approved"` |
| `int` | `Facts.SetNum("key", N.0)` | `retries: 3` compiles to `SetNum("retries", 3.0)` |
| `float64` | `Facts.SetNum("key", N)` | `threshold: 0.85` |
| Other | `Facts.SetStr("key", fmt.Sprintf("%v", val))` | Fallback string conversion |

After execution, the engine reads back the set_fact values from the FactStore and merges them into the runtime context.

## External Rule Files

Rules can be loaded from external YAML files using the `file` field instead of inline definition.

```yaml
rules:
  - file: "rules/common-gates.yaml"
  - name: inline_rule
    when: "status == 'ready'"
    action: continue
```

The external file must contain a YAML document with a top-level `rules:` key:

```yaml
# rules/common-gates.yaml
rules:
  - name: require_approval
    description: "Block unapproved items"
    salience: 5
    when: "approved != true"
    action: skip
    set_fact:
      gate_status: "blocked"

  - name: pass_approved
    description: "Allow approved items through"
    salience: 10
    when: "approved == true"
    action: continue
    set_fact:
      gate_status: "passed"
```

Rules from external files are expanded inline during parsing. They behave identically to rules defined directly in the workflow file.

## Example: Complete Gate with Rules

This example shows a full workflow with two rules at different priorities, a `set_fact` step to prepare data, and a gate that evaluates the rules.

```yaml
entrypoint: review_pipeline
namespace: default
forks: 5

vars:
  min_score: 70

rules:
  - name: rule_approve
    description: "Approve if score meets threshold and no blockers"
    salience: 10
    when: "score >= 70 and blocker_count == 0"
    action: continue
    set_fact:
      decision: "approved"
      approved: true

  - name: rule_reject
    description: "Reject critical severity regardless of score"
    salience: 20
    when: "severity == 'critical'"
    action: skip
    set_fact:
      decision: "rejected"
      approved: false
      rejection_reason: "critical severity items require manual review"

workflows:
  - name: review_pipeline
    vars:
      score:
      severity:
      blocker_count: 0
    steps:
      - name: prepare_facts
        type: set_fact
        vars:
          score: "{{ score }}"
          severity: "{{ severity }}"
          blocker_count: "{{ blocker_count }}"

      - name: quality_gate
        type: gate
        rules:
          - rule_approve
          - rule_reject
        facts:
          score: "{{ score }}"
          severity: "{{ severity }}"
          blocker_count: "{{ blocker_count }}"

      - name: report_decision
        type: shell_exec
        params:
          command: "echo 'Decision: {{ decision }}'"
```

**Execution scenarios:**

- `--var score=85 --var severity=low` -- Both rules evaluate. `rule_approve` fires (score >= 70, blockers = 0). `rule_reject` does not fire (severity is not critical). Gate action: `continue`. Decision: `approved`.

- `--var score=85 --var severity=critical` -- Both rules evaluate. Both fire. `rule_reject` has salience 20 vs salience 10, so its action wins. Gate action: `skip`. Decision: `rejected`.

- `--var score=50 --var severity=low` -- Neither rule fires (score < 70, severity not critical). Gate action: `continue` (default). No decision facts are set.
