# DoD drop-in package

Portable Definition of Done enforcement for any project. Originally extracted
from HelixAgent after the 2026-04-24 "tests pass, nothing works" triage
session (see `docs/development/definition-of-done.md` for the long-form
rationale).

## Contents

```
dod-dropin/
├── README.md                  ← you are here
├── APPLY.md                   ← 5-minute install procedure
├── scripts/
│   ├── no-silent-skips.sh     ← gate #4: skips must be annotated
│   ├── no-mocks-above-unit.sh ← gate #3: non-unit tests must hit real infra (no httptest/sqlmock/etc)
│   └── demo-all.sh            ← gate #6: every CLAUDE.md acceptance demo must pass
└── templates/
    ├── CLAUDE_md_clause.md      ← paste into each CLAUDE.md
    └── Makefile_additions.md    ← wire the gates
```

## Core premise

High test coverage and green suites do not prove a product works. They prove
the code agrees with itself. The only evidence of "working" is pasted output
from a real run of the real system in the same session as the change.

Everything in this drop-in exists to enforce that premise.

## Install

See `APPLY.md`. Five minutes to install. Skip and demo gates are warn-only
by default and graduate to strict when their backlogs hit zero. The
mocks-above-unit gate is **strict from day one** via a ratchet — running
`make no-mocks-above-unit-update-allowlist` once at install captures the
current state; thereafter the gate fails the build on any new in-process
fake outside that allowlist. Drain the allowlist over time; it should only
ever shrink.
