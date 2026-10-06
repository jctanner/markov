# Task: Step source positions in events and errors

## Status

Pending

## Summary

Decode through `yaml.Node` to record each step's file and line; add `file` and `line` to step events and name both lines in the duplicate-step-name error. See plan 001 section 9.

## Verification

Parser tests for single-file and directory layouts; callback event tests.
