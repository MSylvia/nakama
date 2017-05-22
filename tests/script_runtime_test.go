package tests

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"nakama/server"
	"nakama/server/runtime_modules"

	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

const DATA_PATH = "/tmp/nakama/data/"

func newRuntime() *server.ScriptRuntime {
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
	return server.NewScriptRuntime(logger, logger, DATA_PATH)
}

func writeStatsModule() {
	writeFile("stats.lua", `
stats={}

-- Get the mean value of a table
function stats.mean( t )
  local sum = 0
  local count= 0

  for k,v in pairs(t) do
    if type(v) == 'number' then
      sum = sum + v
      count = count + 1
    end
  end

  return (sum / count)
end
print("Stats Module Loaded")
return stats`)
}

func writeTestModule() {
	writeFile("test.lua", `
test={}
-- Get the mean value of a table
function test.printWorld()
	print("Hello World")
	return {["message"]="Hello World"}
end

print("Test Module Loaded")
return test
`)
}

func writeFile(name, content string) {
	ioutil.WriteFile(filepath.Join(DATA_PATH, "/modules/"+name), []byte(content), 0644)
}

func TestRuntimeSampleScript(t *testing.T) {
	r := newRuntime()
	defer r.Stop()
	r.InitModules()

	l, _ := r.NewStateThread()
	defer l.Close()
	err := l.DoString(`
local example = "an example string"
for i in string.gmatch(example, "%S+") do
   print(i)
end`)

	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeDisallowStandardLibs(t *testing.T) {
	r := newRuntime()
	r.InitModules()
	defer r.Stop()

	l, _ := r.NewStateThread()
	defer l.Close()
	err := l.DoString(`
-- Return true if file exists and is readable.
function file_exists(path)
  local file = io.open(path, "r")
  if file then file:close() end
  return file ~= nil
end
file_exists "./"`)

	if err == nil {
		t.Error(errors.New("Successfully accessed IO package"))
	}
}

// This test will always pass.
// Have a look at the stdout messages to see if the module was loaded multiple times
// You should only see "Test Module Loaded" once
func TestRuntimeRequireEval(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeTestModule()
	writeFile("test-invoke.lua", `
local nakama = require("nakama")
local test = require("test")
test.printWorld()
`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeRequireFile(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeStatsModule()
	writeFile("local_test.lua", `
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
assert(stats.mean(t) > 0)
`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeRequirePreload(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeStatsModule()
	writeFile("states-invoke.lua", `
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
print(stats.mean(t))
`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeKeepChangesBetweenStates(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	varMod := []byte(`
var={}
var.count = 1
return var`)
	ioutil.WriteFile(filepath.Join(DATA_PATH, "/modules/var.lua"), varMod, 0644)
	r.InitModules()

	l, _ := r.NewStateThread()
	defer l.Close()

	err := l.DoString(`
local var = require("var")
var.count = 2`)

	if err != nil {
		t.Error(err)
	}

	err = l.DoString(`
local var = require("var")
assert(var.count == 2)`)

	if err != nil {
		t.Error(err)
	}

	l2, _ := r.NewStateThread()
	defer l2.Close()
	err = l2.DoString(`
local var = require("var")
assert(var.count == 2)`)

	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeRegisterHTTP(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeTestModule()
	writeFile("http-invoke.lua", `
local nakama = require("nakama")
local test = require("test")
nakama.register_http(test.printWorld, "/test/helloworld")
	`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}

	m, err := r.InvokeLuaFunction(runtime_modules.FUNC_TYPE_HTTP, "/test/helloworld", uuid.Nil, nil)
	if err != nil {
		t.Error(err)
	}

	msg := m["message"]
	if msg != "Hello World" {
		t.Error("Invocation failed. Return result not expected")
	}
}

func TestRuntimeRegisterHTTPNoResponse(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeFile("test.lua", `
test={}
-- Get the mean value of a table
function test.printWorld()
	print("Hello World")
end

print("Test Module Loaded")
return test
	`)
	writeFile("http-invoke.lua", `
local nakama = require("nakama")
local test = require("test")
nakama.register_http(test.printWorld, "/test/helloworld")
	`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}

	_, err = r.InvokeLuaFunction(runtime_modules.FUNC_TYPE_HTTP, "/test/helloworld", uuid.Nil, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeRegisterHTTPWithInputData(t *testing.T) {
	defer os.RemoveAll(DATA_PATH)
	r := newRuntime()
	defer r.Stop()

	writeFile("test.lua", `
test={}
-- Get the mean value of a table
function test.printWorld(inputData)
	print("Hello World")
	return inputData
end

print("Test Module Loaded")
return test
	`)
	writeFile("http-invoke.lua", `
local nakama = require("nakama")
local test = require("test")
nakama.register_http(test.printWorld, "/test/helloworld")
	`)

	err := r.InitModules()
	if err != nil {
		t.Error(err)
	}

	inputData := make(map[string]interface{})
	inputData["message"] = "Hello World"

	m, err := r.InvokeLuaFunction(runtime_modules.FUNC_TYPE_HTTP, "/test/helloworld", uuid.Nil, inputData)
	if err != nil {
		t.Error(err)
	}

	msg := m["message"]
	if msg != "Hello World" {
		t.Error("Invocation failed. Return result not expected")
	}
}
