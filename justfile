fuzz_targets := "FuzzRoundTrip FuzzParse FuzzUnmarshal FuzzUnitsMatchesParseDuration"

# List the recipes
default:
    @just --list

# Everything CI checks, except fuzzing
check: lint test vuln

# Format all Go files
fmt:
    gofmt -w .

# Regenerate wrapper_types.go from gen_wrapper_types.go
generate:
    go generate ./...

# gofmt, go vet, go mod tidy and golangci-lint
lint:
    @test -z "$(gofmt -l .)" || { gofmt -l .; echo "run 'just fmt'"; exit 1; }
    go vet ./...
    go mod tidy -diff
    golangci-lint run ./...

# Run the tests with the race detector
test:
    go test -race -count=1 ./...

# Show test coverage per function
cover:
    go test -count=1 -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Check for known vulnerabilities
vuln:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Fuzz each target for a while, e.g. `just fuzz 2m FuzzParse`
fuzz time="30s" *targets=fuzz_targets:
    for t in {{ targets }}; do go test -run='^$' -fuzz="^$t\$" -fuzztime={{ time }} . || exit 1; done

# Run the benchmarks
bench:
    go test -run='^$' -bench=. -benchmem ./...
