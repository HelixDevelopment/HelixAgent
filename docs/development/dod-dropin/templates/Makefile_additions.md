# Makefile additions — wire the gates

Append these targets to your project's root `Makefile`. They call the drop-in
scripts so the gate discipline is one `make ci-validate-all` away.

```make
# Definition of Done gates
.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

# Run a single module's demo: make demo-one MOD=path/to/module
demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

# Single entry point. Add your existing build/test/lint targets before the gates
# so gates run AFTER static checks.
ci-validate-all: fmt vet lint test no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"
```

## Graduation path

1. **Warm-up (month 1):** run `ci-validate-all` as above — everything warn-only.
   Capture the baseline skip count + NO-DEMO count. That is your backlog.
2. **Drain (months 2-3):** each PR reduces either number by at least one.
3. **Flip (when skip count is 0 and NO-DEMO count is 0):**
   ```make
   ci-validate-all: fmt vet lint test no-silent-skips demo-all
   ```
   The suffix `-warn` goes away. Now the gate has teeth.

Before you flip, run `make ci-validate-all` with strict targets locally — if it
passes, flip in the same PR. If it fails, fix the breakage first and retry.
