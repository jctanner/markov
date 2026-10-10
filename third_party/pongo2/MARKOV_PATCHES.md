# Markov's copy of pongo2

`github.com/flosch/pongo2/v6` v6.0.0 (MIT, see LICENSE), used through a
`replace` directive in Markov's `go.mod`. Upstream tests, docs and template
fixtures are left out.

## Patches

1. `variable.go`, `parseVariableOrLiteral`: continue parsing after a
   `[subscript]`. Upstream stops there, so `tiers[tier].tests` and
   `a[b][c]` fail with "'}}' expected". With the patch, any mix of `.field`,
   `.0` and `[expr]` works.
