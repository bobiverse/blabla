module blaexample

go 1.26.5

require github.com/bobiverse/blabla v0.0.3

require gopkg.in/yaml.v3 v3.0.1 // indirect

// Build against the checkout, not the published module, so the example
// exercises the code in this repo.
replace github.com/bobiverse/blabla => ../
