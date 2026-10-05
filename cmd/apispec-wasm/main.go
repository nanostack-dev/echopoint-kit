//go:build js && wasm

// Command apispec-wasm is the apispec engine for the browser. It defines
// globalThis.apispecHandle(requestJSON) -> responseJSON (see package bridge)
// and then calls globalThis.apispecReady() when the page defined it.
package main

import (
	"syscall/js"

	"github.com/nanostack-dev/echopoint-kit/apispec/bridge"
)

func main() {
	js.Global().Set("apispecHandle", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return string(bridge.Handle(nil))
		}
		return string(bridge.Handle([]byte(args[0].String())))
	}))
	if ready := js.Global().Get("apispecReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}
