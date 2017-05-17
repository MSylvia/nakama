package server

import (
	"os"
	"path/filepath"

	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ScriptRuntime struct {
	logger        *zap.Logger
	luaPath       string
	luaLibs       map[string]lua.LGFunction
	modules       []string
	snapshotState *lua.LState
}

func NewScriptRuntime(logger *zap.Logger, multiLogger *zap.Logger, datadir string) *ScriptRuntime {
	r := &ScriptRuntime{
		logger:  logger,
		modules: make([]string, 0),
		luaLibs: map[string]lua.LGFunction{
			lua.LoadLibName:      lua.OpenPackage,
			lua.BaseLibName:      lua.OpenBase,
			lua.TabLibName:       lua.OpenTable,
			lua.StringLibName:    lua.OpenString,
			lua.MathLibName:      lua.OpenMath,
			lua.ChannelLibName:   lua.OpenChannel,
			lua.CoroutineLibName: lua.OpenCoroutine,
		},
	}

	r.loadRuntimeModules(multiLogger, datadir)
	return r
}

func (r *ScriptRuntime) loadRuntimeModules(multiLogger *zap.Logger, datadir string) {
	r.luaPath = filepath.Join(datadir, "modules")
	os.MkdirAll(r.luaPath, os.ModePerm)

	r.logger.Info("Loading modules", zap.String("path", r.luaPath))
	err := filepath.Walk(r.luaPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			r.logger.Warn("Could not read module - skipping", zap.Error(err))
		} else if !f.IsDir() {
			r.modules = append(r.modules, path)
		}
		return nil
	})
	if err != nil {
		r.logger.Error("Failed to load modules", zap.Error(err))
	}
	multiLogger.Info("Evaluating modules", zap.Int("count", len(r.modules)), zap.Strings("modules", r.modules))

	// first time create a blank state
	// and evaluate all lua code for all register_XXX calls
	// then discard, and keep reference in memory for later
	l := r.NewState()
	defer l.Close()
	failedModules := 0
	for _, mod := range r.modules {
		if err = l.DoFile(mod); err != nil {
			failedModules++
			r.logger.Error("Failed to evaluate module", zap.String("path", mod), zap.Error(err))
		}
	}

	multiLogger.Info("Loaded modules", zap.Int("count", len(r.modules)-failedModules))

	// final state to keep in memory
	r.snapshotState = r.NewState()
}

func (r *ScriptRuntime) NewState() *lua.LState {
	l := lua.NewState(lua.Options{
		CallStackSize:       1024,
		RegistrySize:        1024,
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})

	// override before Package library is invoked.
	lua.LuaLDir = r.luaPath
	lua.LuaPathDefault = lua.LuaLDir + "/?.lua;" + lua.LuaLDir + "/?/init.lua"
	os.Setenv(lua.LuaPath, lua.LuaPathDefault)

	for name, lib := range r.luaLibs {
		l.Push(l.NewFunction(lib))
		l.Push(lua.LString(name))
		l.Call(1, 0)
	}
	return l
}

func (r *ScriptRuntime) PreloadModules(l *lua.LState, modules map[string]string) {
	preload := l.GetField(l.GetField(l.Get(lua.EnvironIndex), "package"), "preload")

	for name, mod := range modules {
		f, err := l.LoadString(mod)
		if err != nil {
			r.logger.Error("Could not preload module", zap.String("name", name), zap.Error(err))
		} else {
			l.SetField(preload, name, f)
		}
	}
}

func (r *ScriptRuntime) NewStateThread() (*lua.LState, context.CancelFunc) {
	//TODO(mo) use context to limit lua processing time
	return r.snapshotState.NewThread()
}
