# Checklists

Validation checklists used before completing a task.

---

## General

**Requirements:**
- [ ] Requirements understood.
- [ ] Requested scope is respected.
- [ ] Assumptions documented if requirements are incomplete.

**Architecture:**
- [ ] Existing architecture preserved.
- [ ] Existing project conventions followed.
- [ ] Responsibilities separated.

**Implementation:**
- [ ] Existing components reused where appropriate.
- [ ] No duplicate logic introduced.
- [ ] No unrelated code modified.

**Quality:**
- [ ] Code readable and maintainable.
- [ ] Dead code removed.
- [ ] Naming consistent.

**Reliability:**
- [ ] Errors handled.
- [ ] Input validated.
- [ ] Sensitive information protected.

**Validation:**
- [ ] Tests written before implementation (TDD).
- [ ] 100% code coverage achieved (`go test -cover`).
- [ ] All tests passing.
- [ ] All checklist items verified.
- [ ] Ready for production.

---

## New Feature

- [ ] Business requirements understood.
- [ ] Bounded context identified.
- [ ] Ubiquitous language defined.
- [ ] Domain model designed (entities, value objects, aggregates).
- [ ] Invariants identified and protected.
- [ ] Domain events defined.
- [ ] Repository interfaces defined in domain layer.
- [ ] Existing functionality preserved.
- [ ] Existing components reused.
- [ ] TDD cycle followed (Red → Green → Refactor).
- [ ] **All tests written before implementation.**
- [ ] **100% code coverage achieved.**
- [ ] Validation and error handling implemented.
- [ ] Logging added where appropriate.
- [ ] All tests passing.
- [ ] Documentation updated if required.

---

## Bug Fix

- [ ] Root cause identified.
- [ ] Smallest safe fix implemented.
- [ ] Existing behavior preserved.
- [ ] Regression test added (written before the fix — TDD).
- [ ] 100% code coverage maintained.
- [ ] No unrelated behavior changed.
- [ ] Logs reviewed if applicable.

---

## Refactor

- [ ] Public behavior preserved.
- [ ] Complexity reduced.
- [ ] Duplicate logic removed.
- [ ] Naming improved.
- [ ] Tests still pass.
- [ ] No unnecessary abstractions introduced.

---

## Performance

- [ ] Bottleneck identified.
- [ ] Performance measured.
- [ ] Optimization justified.
- [ ] Resource usage reduced.
- [ ] Correctness preserved.
- [ ] Maintainability preserved.

---

## Code Review

**Correctness:**
- [ ] Requirements satisfied.
- [ ] Edge cases considered.

**Engineering:**
- [ ] Standards followed.
- [ ] Patterns followed.
- [ ] Playbook completed.

**Security:**
- [ ] Authorization verified if applicable.
- [ ] Sensitive data protected.

**Testing:**
- [ ] Tests written before implementation (TDD).
- [ ] 100% code coverage verified.
- [ ] Tests updated.
- [ ] Existing tests unaffected.

**Final:**
- [ ] Production ready.
- [ ] No unnecessary changes.
