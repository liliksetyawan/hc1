# Patterns

Recommended implementation patterns. Load the relevant section based on the task.

---

## Feature Implementation

**Responsibilities:** Understand requirements → reuse existing components → implement business behavior → preserve architecture → keep changes focused.

**Workflow:** Requirements → Impact Analysis → Review Existing → Design Solution → Write Failing Test (Red) → Implement Domain → Write Failing Test (Red) → Implement Application → Refactor → Review.

**MUST:** Understand the business objective; reuse existing implementations; respect project architecture; limit changes to the requested scope; keep implementation production-ready.
**SHOULD:** Keep changes small; minimize side effects; preserve backward compatibility.
**NEVER:** Rewrite existing features unnecessarily; mix unrelated changes; introduce dead code; break existing behavior.

---

## API Pattern

Request → Validation → Business Logic → Persistence → Response → Logging.

**MUST:** Validate all input; keep transport separate from business logic (transport calls application only); return consistent responses; handle expected errors; preserve API contracts; map DTOs to domain objects.
**SHOULD:** Keep endpoints focused; keep responses minimal; support pagination when applicable.
**NEVER:** Implement business logic in transport layer; access persistence from transport; return internal errors; expose sensitive information.

---

## Business Logic Pattern

Input → Business Validation (Invariants) → Domain Logic → Domain Events → Persistence (via Repository) → Result.

**MUST:** Keep business logic independent (no infrastructure deps); coordinate components without tight coupling; protect aggregate invariants; raise domain events for significant changes; use repository interfaces defined in domain layer.
**SHOULD:** Keep business rules reusable; separate orchestration from persistence; keep implementations deterministic.
**NEVER:** Access transport concerns; build persistence queries; format API responses; depend on infrastructure details.

---

## Persistence Pattern

Repository **interfaces** live in domain layer; **implementations** live in infrastructure layer.

**MUST:** Implement interfaces defined in domain layer; map domain objects to persistence models and back; handle storage failures; keep transactions minimal (single aggregate).
**SHOULD:** Reuse existing queries; optimize repeated operations; minimize storage access.
**NEVER:** Execute business rules; access transport concerns; call external systems; duplicate persistence logic.

---

## Integration Pattern

Validate request → Build integration request → Execute external operation → Handle timeout/retry → Validate response → Map response to domain → Return result.

**MUST:** Isolate integration logic; handle expected failures; configure timeouts; return domain-friendly results; protect sensitive information.
**SHOULD:** Retry transient failures; keep integrations replaceable; record significant failures.
**NEVER:** Mix business logic with integration logic; leak provider-specific details; expose sensitive data.

---

## Background Processing Pattern

Receive task → Validate payload → Execute business logic → Persist results → Retry if applicable → Record outcome.

**MUST:** Keep tasks idempotent; handle failures gracefully; preserve data consistency; record processing status.
**SHOULD:** Keep tasks independent; keep execution deterministic; limit retry attempts.
**NEVER:** Depend on execution order; block unrelated work; ignore failed executions.

---

## Domain Modeling

### Entity
Defined by identity. Has invariants and behaviors.

```go
type Order struct {
    id        OrderId
    customerId CustomerId
    items     []OrderItem
    status    OrderStatus
}
order.ID() == other.ID()  // identity comparison
```

### Value Object
Defined by attributes. Immutable. No identity.

```go
type Money struct {
    amount   Decimal
    currency Currency
}
// Returns new instance — no mutation
func (m Money) Add(other Money) Money { ... }
```

### Aggregate
Cluster of entities and value objects. All state changes go through the Aggregate Root.

```go
func (o *OrderAggregate) AddItem(product Product, qty Quantity) {
    // Invariants enforced here — external code never touches items[]
    o.validateItemLimit()
    o.items = append(o.items, OrderItem{product, qty})
}
```

### Domain Event
Something significant that happened. Immutable. Named in past tense.

```go
type OrderPlaced struct {
    orderID     OrderId
    customerID  CustomerId
    totalAmount Money
    occurredAt  time.Time
}
func (e *OrderPlaced) EventName() string { return "order.placed" }
```

**Workflow:** Extract Domain Concepts → Identify Entities → Identify Value Objects → Identify Aggregates → Define Invariants → Assign Behaviors → Define Domain Events → Write Domain Tests.

**MUST:** Model behavior alongside data (avoid anemic models); protect aggregate invariants within Aggregate Root; keep value objects immutable; raise domain events for significant changes.
**SHOULD:** Name entities and methods using domain language; keep aggregates small; test domain models through behaviors and invariants.
**NEVER:** Add behavior that doesn't belong to the entity; expose aggregate internals for external mutation; allow invariants to be violated; create mutable value objects.
