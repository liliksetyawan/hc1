# Backend AI Engineering Skill

A technology-agnostic engineering skill pack for AI coding assistants.

This repository provides a consistent engineering standard for generating production-ready backend solutions. It focuses on engineering practices rather than programming languages, frameworks, or libraries.

Project-specific technologies, frameworks, and architectures are determined by the project context and the user's request.

## Design Goals

- Technology agnostic
- Framework agnostic
- Production focused
- Maintainable
- Consistent
- Token efficient
- AI friendly

## Repository Structure

```text
backend-ai-skill/
├── README.md
├── SKILL.md
├── STANDARDS.md   (engineering standards & DDD model)
├── PATTERNS.md    (implementation patterns)
├── PLAYBOOKS.md   (workflows)
├── CHECKLISTS.md  (validation checklists)
└── EXAMPLES.md    (good & bad practices)
```

## Repository Overview

| File | Description |
|------|-------------|
| `STANDARDS.md` | Engineering standards and constraints that apply to every implementation. |
| `PATTERNS.md` | Recommended implementation patterns for common backend components. |
| `PLAYBOOKS.md` | Step-by-step workflows for common engineering tasks. |
| `CHECKLISTS.md` | Validation checklists used before completing a task. |
| `EXAMPLES.md` | Good and bad engineering practices for reference. |

## Engineering Principles

The repository is based on the following principles:

- Correctness over convenience
- Security by default
- Simplicity over complexity
- Reuse over duplication
- Consistency over personal preference
- Maintainability over cleverness

## Rule Priority

Rules are evaluated using the following priority:

1. MUST
2. SHOULD
3. MAY
4. NEVER

Higher-priority rules always override lower-priority rules.

## Recommended Loading Order

For each task, load only the documents that are relevant.

1. Read `SKILL.md`.
2. Load `STANDARDS.md`.
3. Load `PATTERNS.md`.
4. Load `PLAYBOOKS.md`.
5. Validate using `CHECKLISTS.md`.
6. Refer to `EXAMPLES.md` only when additional guidance is needed.

Avoid loading unnecessary documents to keep the context focused and efficient.

## Supported Engineering Tasks

This repository is intended to support tasks such as:

- Feature implementation
- API development
- Business logic
- Data persistence
- External integrations
- Background processing
- Bug fixing
- Refactoring
- Performance optimization
- Code review

## Expected Outcome

Every generated implementation should be:

- Correct
- Readable
- Maintainable
- Secure
- Testable
- Performant
- Consistent with the existing project