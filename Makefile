# PGTransfer development tasks.
#
# Benchmarks are part of the normal workflow: `make bench` runs the full suite,
# and `make bench-save` captures a baseline that `make bench-compare` checks
# future work against.

BENCH      ?= .
BENCHTIME  ?= 1s
BENCHDIR   ?= .bench
BASELINE   ?= $(BENCHDIR)/baseline.txt

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build          Build the pgtransfer binary"
	@echo "  test           Run unit tests with the race detector"
	@echo "  test-integration  Run integration tests (needs the Docker stack)"
	@echo "  lint           gofmt + go vet"
	@echo "  bench          Run all benchmarks"
	@echo "  bench-csv      Run only the CSV pipeline benchmarks"
	@echo "  bench-save     Record a baseline in $(BASELINE)"
	@echo "  bench-compare  Compare the current tree against the saved baseline"
	@echo "  bench-clean    Remove recorded benchmark data"

.PHONY: build
build:
	go build -o bin/pgtransfer .

.PHONY: test
test:
	go test -race ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration -v ./internal/io/...

.PHONY: lint
lint:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-formatted:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...

# -run '^$$' skips tests so only benchmarks execute.
.PHONY: bench
bench:
	go test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) ./...

.PHONY: bench-csv
bench-csv:
	go test -run '^$$' -bench 'BenchmarkFormatCSVValue|BenchmarkCSVWriteRows|BenchmarkExportCSVPipeline|BenchmarkSplitCSVColumns' \
		-benchmem -benchtime=$(BENCHTIME) ./internal/io/

.PHONY: bench-save
bench-save:
	@mkdir -p $(BENCHDIR)
	go test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) ./... | tee $(BASELINE)
	@echo "Baseline written to $(BASELINE)"

.PHONY: bench-compare
bench-compare:
	@test -f $(BASELINE) || { echo "no baseline; run 'make bench-save' first"; exit 1; }
	go test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) ./... | tee $(BENCHDIR)/current.txt
	@command -v benchstat >/dev/null 2>&1 || { \
		echo "install benchstat for a comparison: go install golang.org/x/perf/cmd/benchstat@latest"; exit 0; }
	benchstat $(BASELINE) $(BENCHDIR)/current.txt

.PHONY: bench-clean
bench-clean:
	rm -rf $(BENCHDIR)
