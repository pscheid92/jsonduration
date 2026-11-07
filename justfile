fuzz_targets := "FuzzRoundTrip FuzzParse FuzzParseMatchesReference FuzzUnmarshal FuzzUnitsMatchesParseDuration"

default:
    just --list

test:
    go test -race -count=1 ./...

# Run the tests as a 32-bit program, without -race, which 386 lacks. Linux only: macOS cannot run 32-bit programs.
test-32bit:
    GOARCH=386 go test -count=1 ./...

vet:
    go vet ./...

lint:
    golangci-lint run ./...

# Format all Go files in place.
fmt:
    gofmt -w .

# Apply the Go toolchain's automated fixes and modernizers.
fix:
    go fix ./...

# Fail when go fix, gofmt, or go mod tidy would change something; used by check and CI.
fix-check:
    go fix -diff ./...
    @test -z "$(gofmt -l .)" || { gofmt -d .; exit 1; }
    go mod tidy -diff

coverage:
    go test -count=1 -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Scan for known vulnerabilities in the code that is actually reached.
vuln:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Fuzz every target for a while, or some, as in `just fuzz 2m FuzzParse`.
fuzz time="30s" *targets=fuzz_targets:
    for t in {{ targets }}; do go test -run='^$' -fuzz="^$t\$" -fuzztime={{ time }} . || exit 1; done

bench:
    go test -run='^$' -bench=. -benchmem ./...

check: fix-check vet lint test vuln
