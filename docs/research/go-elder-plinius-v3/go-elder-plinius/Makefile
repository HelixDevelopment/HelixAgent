.PHONY: all test help

MODULES := go-plinius-common go-autotemp go-obliteratus go-eos go-almeche \
           go-cl4r1t4s go-glossopetrae go-p4rs3lt0ngv3 go-st3gg \
           go-g0dm0d3 go-v3sp3r go-l1b3rt4s go-leakhub \
           go-v3r1t4s go-i-llm go-basilisktoken go-hypertune \
           go-autostorygen go-dioscuri go-ourobopus go-gitty \
           go-misc-prompthacks go-autoredteam

help:
	@echo "Targets: test, count, clean"

test:
	@for m in $(MODULES); do \
		echo "=== $$m ===" && \
		(cd $$m && go test ./... -v 2>&1 | head -30) || true; \
	done

count:
	@echo "Modules: $$(echo $(MODULES) | wc -w)"
	@echo "Go files: $$(find . -name '*.go' | wc -l)"
	@echo "Test files: $$(find . -name '*_test.go' | wc -l)"
	@echo "Total lines: $$(find . -name '*.go' -o -name '*.md' | xargs cat | wc -l)"

clean:
	find . -name "coverage.out" -delete
	find . -name "*.test" -delete
