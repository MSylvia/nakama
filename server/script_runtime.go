package server

import (
	"os"
	"path/filepath"

	"nakama/server/runtime_modules"

	"errors"

	"github.com/satori/go.uuid"
	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ScriptRuntime struct {
	logger         *zap.Logger
	multiLogger    *zap.Logger
	luaPath        string
	luaLibs        map[string]lua.LGFunction
	builtinModules map[string]runtime_modules.BuiltinModule
	modules        []string
	snapshotState  *lua.LState
}

func NewScriptRuntime(logger *zap.Logger, multiLogger *zap.Logger, datadir string) *ScriptRuntime {
	r := &ScriptRuntime{
		logger:      logger,
		multiLogger: multiLogger,
		modules:     make([]string, 0),
		luaPath:     filepath.Join(datadir, "modules"),
		luaLibs: map[string]lua.LGFunction{
			lua.LoadLibName:      lua.OpenPackage,
			lua.BaseLibName:      lua.OpenBase,
			lua.TabLibName:       lua.OpenTable,
			lua.StringLibName:    lua.OpenString,
			lua.MathLibName:      lua.OpenMath,
			lua.ChannelLibName:   lua.OpenChannel,
			lua.CoroutineLibName: lua.OpenCoroutine,
		},
		builtinModules: map[string]runtime_modules.BuiltinModule{
			"nakama": runtime_modules.NewNakamaModule(logger),
		},
	}

	if err := os.MkdirAll(r.luaPath, os.ModePerm); err != nil {
		multiLogger.Fatal("Could not start script runtime", zap.Error(err))
	}
	return r
}

func (r *ScriptRuntime) Stop() {
	r.snapshotState.Close()
}

func (r *ScriptRuntime) InitModules() {
	r.logger.Info("Initialising modules", zap.String("path", r.luaPath))
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
		return
	}

	r.multiLogger.Info("Evaluating modules", zap.Int("count", len(r.modules)), zap.Strings("modules", r.modules))

	l := r.newState()
	defer l.Close()
	failedModules := 0
	for _, mod := range r.modules {
		if err = l.DoFile(mod); err != nil {
			failedModules++
			r.logger.Error("Failed to evaluate module - skipping", zap.String("path", mod), zap.Error(err))
		}
	}

	r.multiLogger.Info("Loaded modules", zap.Int("count", len(r.modules)-failedModules))
	r.snapshotState = r.newState()
}

func (r *ScriptRuntime) newState() *lua.LState {
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

	for name, mod := range r.builtinModules {
		l.PreloadModule(name, mod.Loader)
	}

	return l
}

func (r *ScriptRuntime) AppendPreload(modules map[string]string) {
	l := r.snapshotState
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

func (r *ScriptRuntime) InvokeLuaFunction(fnType string, fnKey string, uid uuid.UUID) error {
	fn := runtime_modules.GetRegisteredFunction(fnType, fnKey)
	if fn == nil {
		r.logger.Error("Runtime function was not found", zap.String("key", fnType+fnKey))
		return errors.New("Runtime function was not found")
	}

	l, _ := r.NewStateThread()
	defer l.Close()

	l.Push(fn)

	//l.Push(data)
	//err := l.PCall(1, -1, nil)

	err := l.PCall(0, -1, nil)
	if err != nil {
		r.logger.Error("Could not complete runtime invocation", zap.Error(err))
	}

	return err
}
