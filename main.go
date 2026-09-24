//go:build js && wasm

package main

import (
	"syscall/js"

	"deanprice.com/internal/contact"
)

var (
	country   = "GB"
	rawHost   = ""
	keepAlive []js.Func
)

func getContact(this js.Value, args []js.Value) any {
	channel := "email"
	if len(args) > 0 && !args[0].IsUndefined() && !args[0].IsNull() {
		channel = args[0].String()
	}
	if channel == "" {
		channel = "email"
	}

	return contact.ResolveContact(channel, country, rawHost)
}

func initContext() {
	doc := js.Global().Get("document")
	meta := doc.Call("querySelector", "meta[name=cf-country]")

	loc := ""
	if !meta.IsNull() && !meta.IsUndefined() {
		loc = meta.Get("content").String()
	}

	country = contact.ResolveCountry(loc)
	rawHost = js.Global().Get("location").Get("hostname").String()
}

func main() {
	initContext()

	contactFunc := js.FuncOf(getContact)
	keepAlive = append(keepAlive, contactFunc)

	// Primary unified dispatcher
	js.Global().Set("getWasmContact", contactFunc)

	select {}
}
