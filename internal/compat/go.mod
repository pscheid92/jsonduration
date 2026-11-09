module github.com/pscheid92/jsonduration/internal/compat

go 1.27

require (
	github.com/go-json-experiment/json v1.27.2
	github.com/pscheid92/jsonduration v0.2.0
)

// The tests check the jsonduration next to them, not a release.
replace github.com/pscheid92/jsonduration => ../..
