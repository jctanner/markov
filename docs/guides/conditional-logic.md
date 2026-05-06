{% raw %}
# Conditional Logic

Markov provides three mechanisms for conditional execution: `when`, `gate`, and `assert`. This guide explains when to use each.

## when — Simple Step Conditions

The `when` field on any step takes a Pongo2 boolean expression. If false, the step is skipped:

```yaml
- name: deploy
  type: shell_exec
  when: "environment == 'production'"
  params:
    command: "./deploy.sh"

- name: notify
  type: shell_exec
  when: "needs_review and not auto_approved"
  params:
    command: "echo 'Flagged for review'"
```

Skipped steps are marked with status "skipped" in the state store — they don't fail the workflow and don't block downstream steps.

### When expression syntax

Expressions use Pongo2 (Jinja2-compatible) syntax:

```yaml
when: "count > 5"                          # Numeric comparison
when: "status == 'ready'"                  # String comparison
when: "items | length > 0"                 # Filter usage
when: "enabled and score >= threshold"     # Logical operators
when: "not error_flag"                     # Negation
when: "result.exit_code == 0"              # Nested field access
```

## gate — Rule-Based Decision Routing

A gate step evaluates named rules and sets facts based on which rules fire. Use gates when you need:

- Multiple conditions evaluated together with priority ordering
- Side effects (setting variables for downstream steps via `set_fact`)
- Forward-chaining logic (rules that fire based on other rules' output)

```yaml
rules:
  - name: auto_approve
    salience: 100
    when: "severity == 'critical' and confidence > 0.95"
    action: continue
    set_fact:
      auto_approved: true
      approval_reason: "High confidence critical fix"

  - name: flag_for_review
    salience: 50
    when: "severity == 'high' and not auto_approved"
    action: continue
    set_fact:
      needs_review: true

workflows:
  - name: main
    steps:
      - name: triage
        type: gate
        rules:
          - auto_approve
          - flag_for_review

      - name: run_analysis
        type: shell_exec
        when: "not auto_approved"
        params:
          command: "echo 'Running manual analysis'"

      - name: notify_reviewer
        type: shell_exec
        when: "needs_review"
        params:
          command: "echo 'Review needed'"
```

The gate evaluates all rules, fires matching ones in salience order, sets their facts, and the highest-salience fired rule's action determines the gate outcome.

## assert — Fail-Fast Validation

Use `assert` to validate preconditions and fail immediately if they aren't met:

```yaml
- name: check_preconditions
  type: assert
  that:
    - "status == 'ready'"
    - "score >= 4"
    - "items | length > 0"
  msg: "Preconditions not met"
```

The workflow fails on the first false expression. Use assert for conditions that should never be false at that point in execution — hard errors, not routing decisions.

## Decision Tree: Which to Use

```
Need to fail the workflow if a condition is false?
  → assert

Need to skip a single step?
  → when

Need to set variables based on conditions?
  → gate with set_fact

Need priority-ordered multi-condition evaluation?
  → gate with salience

Need to control looping/recursion?
  → gate controlling a recursive workflow
```

### Rules of Thumb

- **`when`** is for simple skip/run decisions on individual steps
- **`gate`** is for complex decision routing that affects multiple downstream steps
- **`assert`** is for validating invariants that should never be violated

## Combining Techniques

A common pattern uses a gate to set decision variables, then `when` conditions on downstream steps to act on those decisions:

```yaml
rules:
  - name: approve
    salience: 100
    when: "score >= 8"
    action: continue
    set_fact:
      approved: true

  - name: reject
    salience: 50
    when: "score < 4"
    action: continue
    set_fact:
      rejected: true

workflows:
  - name: main
    steps:
      - name: decision_gate
        type: gate
        rules: [approve, reject]
        facts:
          score: "{{ analysis.result.score }}"

      - name: deploy
        when: "approved"
        type: shell_exec
        params:
          command: "./deploy.sh"

      - name: notify_rejection
        when: "rejected"
        type: shell_exec
        params:
          command: "echo 'Rejected: score too low'"

      - name: manual_review
        when: "not approved and not rejected"
        type: shell_exec
        params:
          command: "echo 'Score in review range'"
```

## What's Next

- [Looping with Gates](looping-with-gates.md) — recursive workflows and retry loops
- [Rule Engine Reference](../reference/rule-engine.md) — complete rule syntax
- [Template Engine Reference](../reference/template-engine.md) — expression syntax for when conditions
{% endraw %}
