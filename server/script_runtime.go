package server

import (
	"os"
	"path/filepath"

	"nakama/server/runtime_modules"

	"errors"

	"strings"

	"fmt"

	"github.com/satori/go.uuid"
	"github.com/yuin/gopher-lua"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	__nakamaReturnValue = "__nakama_return_flag__"
)

type BuiltinModule interface {
	Loader(l *lua.LState) int
}

type ScriptRuntime struct {
	logger         *zap.Logger
	multiLogger    *zap.Logger
	luaPath        string
	stdLibs        map[string]lua.LGFunction
	builtinModules map[string]BuiltinModule
	modules        []string
	snapshotState  *lua.LState
}

func NewScriptRuntime(logger *zap.Logger, multiLogger *zap.Logger, datadir string) *ScriptRuntime {
	r := &ScriptRuntime{
		logger:      logger,
		multiLogger: multiLogger,
		modules:     make([]string, 0),
		luaPath:     filepath.Join(datadir, "modules"),
		stdLibs: map[string]lua.LGFunction{
			lua.LoadLibName:      lua.OpenPackage,
			lua.BaseLibName:      lua.OpenBase,
			lua.TabLibName:       lua.OpenTable,
			lua.StringLibName:    lua.OpenString,
			lua.MathLibName:      lua.OpenMath,
			lua.ChannelLibName:   lua.OpenChannel,
			lua.CoroutineLibName: lua.OpenCoroutine,
		},
		builtinModules: map[string]BuiltinModule{
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

func (r *ScriptRuntime) InitModules() error {
	r.logger.Info("Initialising modules", zap.String("path", r.luaPath))
	err := filepath.Walk(r.luaPath, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			r.logger.Error("Could not read module ", zap.Error(err))
			return err
		} else if !f.IsDir() {
			r.modules = append(r.modules, path)
		}
		return nil
	})
	if err != nil {
		r.logger.Error("Failed to load modules", zap.Error(err))
		return err
	}

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

	for name, lib := range r.stdLibs {
		l.Push(l.NewFunction(lib))
		l.Push(lua.LString(name))
		l.Call(1, 0)
	}

	for name, mod := range r.builtinModules {
		l.PreloadModule(name, mod.Loader)
	}

	r.multiLogger.Info("Evaluating modules", zap.Int("count", len(r.modules)), zap.Strings("modules", r.modules))

	// `DoFile(..)` only parses and evaluates modules. Calling it multiple times, will load and eval the file multiple times.
	// So to make sure that we only load and evaluate modules once, regardless of whether there is dependency between files, we load them all into `preload`.
	// This is to make sure that modules are only loaded and evaluated once as `doFile()` does not (always) update _LOADED table.
	// Bear in mind two separate thoughts around the script runtime design choice:
	//
	// 1) This is only a problem if one module is dependent on another module.
	// This means that the global functions are evaluated once at system startup and then later on when the module is required through `require`.
	// We circumvent this by checking the _LOADED table to check if `require` had evaluated the module and avoiding double-eval.
	//
	// 2) Second item is that modules must be pre-loaded into the state for callback-func eval to work properly (in case of HTTP/RPC/etc invokes)
	// So we need to always load the modules into the system via `preload` so that they are always available in the LState.
	// We can't rely on `require` to have seen the module in case there is no dependency between the modules.

	//for _, mod := range r.modules {
	//	relPath, _ := filepath.Rel(r.luaPath, mod)
	//	moduleName := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	//
	//	// check to see if this module was loaded by `require` before executing it
	//	loaded := l.GetField(l.Get(lua.RegistryIndex), "_LOADED")
	//	lv := l.GetField(loaded, moduleName)
	//	if lua.LVAsBool(lv) {
	//		// Already evaluated module via `require(..)`
	//		continue
	//	}
	//
	//	if err = l.DoFile(mod); err != nil {
	//		failedModules++
	//		r.logger.Error("Failed to evaluate module - skipping", zap.String("path", mod), zap.Error(err))
	//	}
	//}

	preload := l.GetField(l.GetField(l.Get(lua.EnvironIndex), "package"), "preload")
	fns := make(map[string]*lua.LFunction)
	for _, path := range r.modules {
		f, err := l.LoadFile(path)
		if err != nil {
			r.logger.Error("Could not preload module", zap.String("name", path), zap.Error(err))
			return err
		} else {
			relPath, _ := filepath.Rel(r.luaPath, path)
			moduleName := strings.TrimSuffix(relPath, filepath.Ext(relPath))
			l.SetField(preload, moduleName, f)
			fns[moduleName] = f
		}
	}

	for name, fn := range fns {
		loaded := l.GetField(l.Get(lua.RegistryIndex), "_LOADED")
		lv := l.GetField(loaded, name)
		if lua.LVAsBool(lv) {
			// Already evaluated module via `require(..)`
			continue
		}

		l.Push(fn)
		fnErr := l.PCall(0, -1, nil)
		if fnErr != nil {
			r.logger.Error("Could not complete runtime invocation", zap.Error(fnErr))
			return fnErr
		}
	}

	r.snapshotState = l
	r.multiLogger.Info("Loaded all modules successfully")
	return nil
}

func (r *ScriptRuntime) NewStateThread() (*lua.LState, context.CancelFunc) {
	//TODO(mo) use context to limit lua processing time
	return r.snapshotState.NewThread()
}

func (r *ScriptRuntime) InvokeLuaFunction(fnType string, fnKey string, uid uuid.UUID, inputData map[string]interface{}) (map[interface{}]interface{}, error) {
	fn := runtime_modules.GetRegisteredFunction(fnType, fnKey)
	if fn == nil {
		r.logger.Error("Runtime function was not found", zap.String("key", fnType+fnKey))
		return nil, errors.New("Runtime function was not found")
	}

	l, _ := r.NewStateThread()
	defer l.Close()

	l.Push(lua.LString(__nakamaReturnValue))
	l.Push(fn)

	nargs := 0
	if inputData != nil {
		nargs = 1
		l.Push(r.convertMap(l, inputData))
	}

	err := l.PCall(nargs, -1, nil)
	if err != nil {
		r.logger.Error("Could not complete runtime invocation", zap.Error(err))
		return nil, err
	}

	var retValue *lua.LTable
	errMessage := "Runtime function returned invalid data. Only allowed one return value of type Lua Table"
	for i := 0; i < 3; i++ {
		ret := l.Get(-1)
		if ret.Type() == lua.LTTable && i == 0 { // If the first return value is Table, then decode it, otherwise raise error
			retValue, _ = ret.(*lua.LTable)
			l.Pop(1)
		} else if ret.Type() == lua.LTString {
			retString := lua.LVAsString(ret)
			if retString == __nakamaReturnValue {
				if retValue != nil {
					break //no longer process as we've received the end of lua stack
				}
				return nil, nil
			}

			r.logger.Error(errMessage, zap.String("function", fnKey))
			return nil, errors.New(errMessage)
		} else {
			r.logger.Error(errMessage, zap.String("function", fnKey))
			return nil, errors.New(errMessage)
		}
	}

	return r.convertLuaTable(retValue), nil
}

func (r *ScriptRuntime) convertMap(l *lua.LState, data map[string]interface{}) *lua.LTable {
	lt := l.NewTable()
	for k, v := range data {
		lt.RawSetString(k, r.convertValue(l, v))
	}

	return lt
}

func (r *ScriptRuntime) convertLuaTable(lv *lua.LTable) map[interface{}]interface{} {
	returnData, _ := r.convertLuaValue(lv).(map[interface{}]interface{})
	return returnData
}

func (r *ScriptRuntime) convertValue(l *lua.LState, val interface{}) lua.LValue {
	if val == nil {
		return lua.LNil
	}

	// types looked up from https://golang.org/pkg/encoding/json/#Unmarshal
	switch v := val.(type) {
	case bool:
		return lua.LBool(v)
	case string:
		return lua.LString(v)
	case float64:
		return lua.LNumber(v)
	case map[string]interface{}:
		return r.convertMap(l, v)
	case []interface{}:
		lt := l.NewTable()
		for k, v := range v {
			lt.RawSetInt(k, r.convertValue(l, v))
		}
		return lt
	default:
		return nil
	}
}

func (r *ScriptRuntime) convertLuaValue(lv lua.LValue) interface{} {
	// taken from https://github.com/yuin/gluamapper/blob/master/gluamapper.go#L79
	switch v := lv.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LString:
		return string(v)
	case lua.LNumber:
		return float64(v)
	case *lua.LTable:
		maxn := v.MaxN()
		if maxn == 0 { // table
			ret := make(map[interface{}]interface{})
			v.ForEach(func(key, value lua.LValue) {
				keystr := fmt.Sprint(r.convertLuaValue(key))
				ret[keystr] = r.convertLuaValue(value)
			})
			return ret
		} else { // array
			ret := make([]interface{}, 0, maxn)
			for i := 1; i <= maxn; i++ {
				ret = append(ret, r.convertLuaValue(v.RawGetInt(i)))
			}
			return ret
		}
	default:
		return v
	}
}
