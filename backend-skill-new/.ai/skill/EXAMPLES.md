# Examples

Reference for good and bad engineering practices. Use as a guide when evaluating implementations.

---

## Good Practices

**Small changes** — Prefer focused implementations that modify only the requested scope.

**Reuse** — Reuse existing modules before introducing new implementations.

**Clear responsibilities** — Keep transport, business logic, and persistence separated.

**Consistent naming** — Use business terminology consistently across the project.

**Safe error handling** — Return client-safe errors while preserving useful diagnostic information internally.

**Testing** — Update tests whenever business behavior changes.

**Performance** — Optimize only after identifying measurable bottlenecks.

---

## Bad Practices

**Duplicate logic** — Creating a new implementation when an existing one can be reused.

**Mixed responsibilities** — Combining transport, business logic, and persistence in the same component.

**Premature optimization** — Optimizing code without measurable evidence.

**Hidden side effects** — Changing unrelated behavior while implementing a feature.

**Weak naming** — Using generic or ambiguous identifiers.

**Missing validation** — Trusting external input without validation.

**Ignored errors** — Suppressing failures instead of handling them consistently.

**Exposed secrets** — Logging or returning sensitive information.

**Dead code** — Leaving unused code after implementation.
