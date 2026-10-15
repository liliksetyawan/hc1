# Playbooks

Step-by-step workflows for common engineering tasks.

---

## Create Feature (TDD + DDD)

> **Rule: Tests first. 100% coverage. No exceptions.**

1. Understand requirements; define the bounded context.
2. Establish ubiquitous language (domain vocabulary).
3. Identify affected modules; review existing implementations.
4. Design the solution: domain model, aggregates, events, repository interfaces.
5. **Write all domain tests (Red) — every function, every error path.**
6. **Verify coverage: `go test -cover`. If not 100%, write more tests until it is.**
7. Implement domain layer (entities, value objects, aggregates).
8. **Write all application tests (Red) — every function, every error path.**
9. **Verify coverage: `go test -cover`. If not 100%, write more tests until it is.**
10. Implement application layer (commands, queries, services).
11. Implement infrastructure layer.
12. Implement transport layer.
13. **Verify coverage across all layers: `go test -cover`.**
14. **Refactor while keeping all tests green (Refactor).**
15. **Run `go test -cover` one final time — must be 100%.**
16. Review; verify production readiness.

**TDD Cycle:** Red → Green → Refactor (at every layer). **No code without a failing test first.**

**Coverage Gate:** `go test ./... -cover` must report **100%** before the feature is complete. Any uncovered line is a blocker.

**MUST:** Preserve existing architecture; reuse existing components; limit scope; write tests before implementation; keep domain layer independent of infrastructure.

---

## Fix Bug

1. Reproduce the issue.
2. Identify the root cause.
3. Review affected components.
4. Implement the smallest safe fix.
5. Verify the fix.
6. Add regression tests.
7. Review side effects.

**MUST:** Fix the root cause; preserve existing behavior; prevent regressions.

---

## Refactor

1. Understand current implementation.
2. Identify improvement opportunities.
3. Preserve public behavior.
4. Simplify implementation.
5. Verify functionality.
6. Update tests if required.

**MUST:** Preserve behavior; improve maintainability; keep changes incremental.

---

## Optimize

1. Identify the bottleneck.
2. Measure current performance.
3. Select the smallest effective optimization.
4. Implement the optimization.
5. Measure the result.
6. Verify correctness.

**MUST:** Optimize only measured bottlenecks; preserve correctness and maintainability.

---

## Code Review

1. Review requirements.
2. Review implementation.
3. Verify architecture, standards, and tests.
4. Review maintainability.
5. Approve or request changes.

**MUST:** Review correctness, security, maintainability, and testing.
**NEVER:** Approve incomplete implementations; ignore security concerns or failing tests.

---

## Investigate Production Issue

1. Collect evidence.
2. Review logs and metrics.
3. Identify affected components.
4. Identify root cause.
5. Propose mitigation.
6. Recommend permanent fix.

**MUST:** Base conclusions on evidence; separate facts from assumptions; preserve system stability.
