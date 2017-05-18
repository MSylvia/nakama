package runtime_modules

import (
	"sync"

	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

const (
	FUNC_TYPE_HTTP   = "http_"
	FUNC_TYPE_BEFORE = "pre_"
	FUNC_TYPE_AFTER  = "post_"
	FUNC_TYPE_RPC    = "rpc_"
)

type BuiltinModule interface {
	Loader(l *lua.LState) int
}

var (
	registeredFunctionsMutex = &sync.Mutex{}
	registeredFunctions      = make(map[string]*lua.LFunction)
)

func GetRegisteredFunction(fnType string, fnKey string) *lua.LFunction {
	return registeredFunctions[fnType+fnKey]
}

func PutRegisteredFunction(fnType string, fnKey string, fn *lua.LFunction) {
	registeredFunctionsMutex.Lock()
	registeredFunctions[fnType+fnKey] = fn
	registeredFunctionsMutex.Unlock()
}

type NakamaModule struct {
	logger *zap.Logger
}

func NewNakamaModule(logger *zap.Logger) *NakamaModule {
	return &NakamaModule{
		logger: logger,
	}
}

func (n *NakamaModule) Loader(l *lua.LState) int {
	mod := l.SetFuncs(l.NewTable(), map[string]lua.LGFunction{
		"register_before": n.registerBefore,
		"register_after":  n.registerAfter,
		"register_http":   n.registerHTTP,
		"register_rpc":    n.registerRPC,
	})

	l.Push(mod)
	return 1
}

func (n *NakamaModule) registerBefore(l *lua.LState) int {
	return 0
}

func (n *NakamaModule) registerAfter(l *lua.LState) int {
	return 0
}

func (n *NakamaModule) registerHTTP(l *lua.LState) int {
	fn := l.CheckFunction(1)
	path := l.CheckString(2)

	if path == "" {
		l.ArgError(2, "Expects HTTP endpoint")
		return 0
	}

	PutRegisteredFunction(FUNC_TYPE_HTTP, path, fn)
	n.logger.Info("Registered HTTP function invocation", zap.String("path", path))
	return 0
}

func (n *NakamaModule) registerRPC(l *lua.LState) int {
	return 0
}
