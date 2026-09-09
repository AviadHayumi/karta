module example

go 1.26.3

require github.com/run-ai/karta/karta-wasm/sandbox v0.0.0

require (
	github.com/tetratelabs/wazero v1.11.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/run-ai/karta/karta-wasm/sandbox => ../
