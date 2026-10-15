# Standards

Engineering standards and constraints for every implementation. Rule priority: **MUST** > SHOULD > MAY > NEVER.

---

## Engineering Principles

**MUST:**
- Write tests **first** (TDD). No implementation without a failing test.
- Maintain **100% code coverage** for all new and modified packages.
- Understand requirements before implementation.
- Preserve existing project architecture and conventions.
- Implement only the requested scope.
- Keep business logic independent from implementation details.
- Separate responsibilities clearly.
- Reuse existing implementations before creating new ones.
- Keep implementations simple, maintainable, and production-ready.
- Validate all external input.
- Handle all expected errors.
- Write deterministic implementations.

**SHOULD:**
- Prefer explicit behavior over implicit.
- Prefer reusable components over duplication.
- Prefer small, focused modules.
- Prefer composition over duplication.
- Prefer consistency over personal preference.
- Minimize dependencies.

**NEVER:**
- Write implementation code before the test.
- Merge code with failing tests or untested coverage.
- Duplicate business logic.
- Introduce unnecessary abstractions.
- Mix unrelated responsibilities.
- Generate unused code.
- Leave incomplete implementations.
- Hardcode sensitive configuration.

---

## Architecture

**MUST:**
- Follow existing project architecture (layer model: domain, application, infrastructure, transport).
- Keep dependencies flowing toward the business layer.
- Respect module boundaries; keep components replaceable.
- Keep business rules independent from external systems.

**SHOULD:**
- Organize code by feature or domain.
- Keep modules cohesive; dependency graphs simple.
- Isolate external integrations.

**NEVER:**
- Mix business logic with transport or persistence logic.
- Access infrastructure directly from the domain layer.
- Create circular dependencies.

---

## Code Organization (DDD)

```
contexts/{bounded-context}/
├── domain/
│   ├── entities/
│   ├── value-objects/
│   ├── aggregates/
│   ├── events/
│   ├── services/
│   └── repositories/    (interfaces only)
├── application/
│   ├── commands/
│   ├── queries/
│   └── services/
├── infrastructure/
│   ├── persistence/
│   └── integrations/
└── transport/
    ├── api/
    └── consumers/
shared/
├── domain/
└── application/
migrations/
```

**MUST:** Group related functionality; keep files focused; remove dead code.
**SHOULD:** Keep files reasonably small; functions focused; predictable directory structures.
**NEVER:** Mix unrelated features in the same module; scatter related logic across locations.

---

## Naming Conventions

**MUST:** Use meaningful, descriptive names; follow existing project conventions; name by business intent.
**SHOULD:** Prefer descriptive names over abbreviations; use domain terminology.
**NEVER:** Use ambiguous or generic names; mix naming styles; reuse the same name for different concepts.

---

## Database

**MUST:** Keep data access in persistence layer; use parameterized queries; validate input; keep transactions short.
**SHOULD:** Optimize based on measured bottlenecks; batch operations; keep queries readable.
**NEVER:** Access the database from presentation layer; build queries using string concatenation; expose database errors.

---

## API Design

**MUST:** Design around business capabilities; validate every request; return consistent responses; use appropriate status codes; keep contracts stable.
**SHOULD:** Keep payloads minimal; use pagination for large collections; document public interfaces.
**NEVER:** Expose internal implementation details; leak infrastructure errors; break contracts without approval.

---

## Error Handling

**MUST:** Handle every expected error; preserve meaningful context; separate business errors from infrastructure errors; return client-safe error messages.
**SHOULD:** Use reusable error definitions; log unexpected failures.
**NEVER:** Ignore errors; swallow exceptions silently; expose stack traces or infrastructure details.

---

## Logging & Observability

**MUST:** Log significant operations; use structured logs; include request correlation identifiers; protect sensitive information.
**SHOULD:** Record execution duration; log external dependency interactions; capture retry attempts.
**NEVER:** Log passwords, secrets, tokens, or sensitive personal data.

---

## Security

**MUST:** Validate all external input; apply least privilege; protect confidential information; verify authorization; handle secrets securely.
**SHOULD:** Validate at trust boundaries; use defense in depth; fail securely.
**NEVER:** Trust client-provided data; store secrets in source code; expose credentials; bypass authorization.

---

## Performance

**MUST:** Optimize only after identifying bottlenecks; minimize unnecessary allocations, network calls, and storage access; release resources promptly.
**SHOULD:** Prefer efficient algorithms; batch operations; cache frequently accessed data.
**NEVER:** Optimize without evidence; sacrifice readability for insignificant gains; introduce premature optimization.

---

## Testing

**TDD First — Tests must be written before any implementation code.**

```
Red  →  Write a failing test (describes the behavior)
Green →  Write the minimum implementation to pass
Refactor →  Improve quality while tests stay green
```

**MUST:**
- Write **all** tests **before** writing implementation code. No implementation without a failing test.
- Maintain **100% code coverage** for every new or modified package. Coverage is measured via `go test -cover`. Zero exceptions.
- Keep tests deterministic and isolated (no shared state, no external dependencies).
- Update tests when behavior changes.
- Every public function, method, and error path **must** have a corresponding test.

**SHOULD:** Cover critical paths, edge cases, and failure scenarios; name tests by the behavior they verify; keep execution fast; group related assertions per test.

**NEVER:** Write implementation code before the test; merge code with failing tests; depend on test execution order; add untested code to the codebase.

---

## Code Review

**MUST:** Verify correctness, scope, architecture, naming, error handling, security, test coverage, and maintainability.
**SHOULD:** Prefer simple solutions; encourage reuse; remove unnecessary complexity.
**NEVER:** Approve incomplete implementations; ignore security concerns or failing tests.

---

## DDD Model

### Layers

- **Domain** — Entities, value objects, aggregates, domain services, repository interfaces (no infrastructure deps).
- **Application** — Commands, queries, application services. Orchestrates domain objects; depends only on domain.
- **Infrastructure** — Repository implementations, external integrations. Implements interfaces defined in domain.
- **Transport** — HTTP handlers, message consumers. Receives requests, delegates to application layer, returns responses.

### Domain Layer

- Entity: defined by identity. Has invariants and behaviors.
- Value Object: defined by attributes, immutable.
- Aggregate: cluster of entities/value objects with one Aggregate Root controlling all state changes.
- Domain Event: significant past-tense occurrence; immutable.
- Repository interface: defined in domain; implemented in infrastructure.

**MUST:** Keep domain independent of infrastructure, transport, and persistence concerns.
**SHOULD:** Protect aggregate invariants; raise domain events for significant changes; keep value objects immutable.
**NEVER:** Access transport or persistence concerns from domain.

See `PATTERNS.md` for domain modeling patterns.
