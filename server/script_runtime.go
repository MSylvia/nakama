package server

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"fmt"

	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type ScriptRuntime struct {
	logger        *zap.Logger
	modules       map[string]string //name to content
	luaLibs       map[string]lua.LGFunction
	snapshotState *lua.LState
}

func NewScriptRuntime(logger *zap.Logger, multiLogger *zap.Logger, datadir string) *ScriptRuntime {
	r := &ScriptRuntime{
		logger:  logger,
		modules: make(map[string]string),
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
	// Make module dir if not exists
	modulesPath := filepath.Join(datadir, "modules")
	os.MkdirAll(modulesPath, os.ModePerm)

	r.logger.Info("Loading modules", zap.String("path", modulesPath))

	// Accumulate all Lua modules paths
	modules := make([]string, 0)
	err := filepath.Walk(modulesPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			r.logger.Warn("Could not read module - skipping", zap.Error(err))
		} else if !f.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				r.logger.Warn("Could not read module - skipping", zap.Error(err))
			} else {
				modules = append(modules, path)
				modulePath := strings.TrimSuffix(path, filepath.Ext(path))
				r.modules[modulePath] = string(content)
			}
		}
		return nil
	})
	if err != nil {
		r.logger.Error("Failed to load modules", zap.Error(err))
	}

	multiLogger.Info("Evaluating modules", zap.Int("count", len(r.modules)), zap.Strings("modules", modules))

	l := r.NewState(r.modules)
	defer l.Close()
	for name, mod := range r.modules {

		// checked _LOADED to see if its already loaded or not

		err := l.DoString(mod)
		if err != nil {
			r.logger.Error("Failed to evaluate module", zap.String("name", name), zap.Error(err))
			delete(r.modules, name)
		}
	}

	multiLogger.Info("Loaded modules", zap.Int("count", len(r.modules)))

	r.snapshotState = r.NewState(r.modules)
}

func (r *ScriptRuntime) NewStateThread() (*lua.LState, context.CancelFunc) {
	//TODO(mo) use context to limit lua processing time
	return r.snapshotState.NewThread()
}

func (r *ScriptRuntime) NewState(modules map[string]string) *lua.LState {
	l := lua.NewState(lua.Options{
		CallStackSize:       1024,
		RegistrySize:        1024,
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true,
	})

	for name, lib := range r.luaLibs {
		l.Push(l.NewFunction(lib))
		l.Push(lua.LString(name))
		l.Call(1, 0)
	}

	env := l.Get(lua.EnvironIndex)
	l.SetField(env, "require", l.NewFunction(func(ls *lua.LState) int {
		name := l.CheckString(1)

		loaded := l.GetField(l.Get(lua.RegistryIndex), "_LOADED")
		lv := l.GetField(loaded, name)
		if lua.LVAsBool(lv) {
			fmt.Println("found in _LOADED")
			return 1
		}

		if mod, ok := modules[name]; ok {
			fmt.Println("loading again...")
			modasfunc, err := l.LoadString(mod)
			if err != nil {
				l.Error(lua.LString("require failed to load module "+name), 1)
				r.logger.Error("require failed to load module", zap.String("path", name), zap.Error(err))
				return 0
			}

			l.Push(modasfunc)
			l.Push(lua.LString(name))
			l.Call(1, 1)
			l.SetField(loaded, name, lua.LTrue)

			//TODO(mo) is this the best way to load a module?
			//preload := l.GetField(l.GetField(env, "package"), "preload")
			//l.SetField(preload, name, lf)

			//l.SetField(loaded, name, modasfunc)
			//l.Push(modasfunc)
			return 1
		}

		l.Error(lua.LString("require failed to find module "+name), 1)
		r.logger.Warn("require failed to find module", zap.String("path", name))
		return 0
	}))

	//preload := l.GetField(l.GetField(l.Get(lua.EnvironIndex), "package"), "preload")
	//for name, module := range r.modules {
	//	mod, err := l.LoadString(module)
	//	if err != nil {
	//		r.logger.Error("Failed to evaluate module", zap.String("name", name), zap.Error(err))
	//	} else {
	//		l.SetField(preload, name, mod)
	//	}
	//}

	return l
}
