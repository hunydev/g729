//go:build !js || !wasm

package main

import "fmt"

func main() {
	fmt.Println("g729wasm is intended for GOOS=js GOARCH=wasm builds")
}
