//go:build !js || !wasm

package main

import (
	"fmt"
)

//go:generate go run ./tools -task=codegen

func main() {
	fmt.Println("DeanPrice.com client is compiled for WebAssembly (GOOS=js GOARCH=wasm or tinygo).")
}
