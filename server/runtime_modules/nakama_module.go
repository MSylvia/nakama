package runtime_modules

import (
	"github.com/yuin/gopher-lua"
)

const (
	FUNC_INVOKE_HTTP   = "http_"
	FUNC_INVOKE_BEFORE = "pre_"
	FUNC_INVOKE_AFTER  = "post_"
	FUNC_INVOKE_RPC    = "rpc_"
)

var (
	RegisteredFunctions = make(map[string]string)
)

func NakamaModule(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"register_before": registerBefore,
		"register_after":  registerAfter,
		"register_http":   registerHTTP,
		"register_rpc":    registerRPC,
	})

	L.Push(mod)
	return 1
}

func registerBefore(l *lua.LState) int {
	return 0
}

func registerAfter(l *lua.LState) int {
	return 0
}

func registerHTTP(l *lua.LState) int {
	return 0
}

func registerRPC(l *lua.LState) int {
	return 0
}
