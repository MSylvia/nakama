package server

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
)

type ScriptRuntime struct {
	logger  *zap.Logger
	modules map[string]string //name to content
	luaLibs map[string]lua.LGFunction
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
				r.modules[path] = string(content)
			}
		}
		return nil
	})
	if err != nil {
		r.logger.Error("Failed to load modules", zap.Error(err))
	}

	l := r.NewState()
	defer l.Close()
	for _, mod := range r.modules {
		l.DoString(mod)
	}

	multiLogger.Info("Loaded modules", zap.Int("count", len(modules)), zap.Strings("modules", modules))
}

func (r *ScriptRuntime) NewState() *lua.LState {
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

	for _, mod := range r.modules {
		l.LoadString(mod)
	}

	return l
}
