module github.com/gobwas/cli/examples/complex

go 1.26

require (
	github.com/gobwas/cli v0.0.0-20201220175310-6874379ea65a
	github.com/gobwas/flagutil v0.9.0
)

require gopkg.in/yaml.v2 v2.2.8 // indirect

// Always exercise the checked out library, not a published version.
replace github.com/gobwas/cli => ../..
