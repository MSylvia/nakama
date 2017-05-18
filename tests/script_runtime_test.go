package tests

import (
	"io/ioutil"
	"nakama/server"
	"os"
	"testing"

	"path/filepath"

	"nakama/server/runtime_modules"

	"github.com/satori/go.uuid"
	"go.uber.org/zap"
)

const DATA_PATH = "/tmp/nakama/data/"

func newRuntime() *server.ScriptRuntime {
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
	return server.NewScriptRuntime(logger, logger, DATA_PATH)
}

func TestRuntimeSampleScript(t *testing.T) {
	r := newRuntime()
	r.InitModules()
	defer r.Stop()

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
		t.Error("Successfully accessed IO package")
	} else {
		t.Log(err)
	}
}

func TestRuntimeRequireFile(t *testing.T) {
	r := newRuntime()
	defer r.Stop()

	statsMod := []byte(`
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

return stats`)
	ioutil.WriteFile(filepath.Join(DATA_PATH, "/modules/stats.lua"), statsMod, 0644)
	r.InitModules()

	l, _ := r.NewStateThread()
	defer l.Close()
	err := l.DoString(`
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
assert(stats.mean(t) > 0)`)

	if err != nil {
		t.Error(err)
	}

	os.RemoveAll(DATA_PATH)
}

func TestRuntimeRequirePreload(t *testing.T) {
	r := newRuntime()
	r.InitModules()
	defer r.Stop()

	r.AppendPreload(map[string]string{
		"stats": `
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

return stats`})

	l, _ := r.NewStateThread()
	defer l.Close()

	err := l.DoString(`
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
print(stats.mean(t))`)

	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeDiscardChangesBetweenStates(t *testing.T) {
	r := newRuntime()
	defer r.Stop()

	statsMod := []byte(`
stats={}
stats.count = 1
return stats`)
	ioutil.WriteFile(filepath.Join(DATA_PATH, "/modules/test.lua"), statsMod, 0644)
	r.InitModules()

	l, _ := r.NewStateThread()
	defer l.Close()

	err := l.DoString(`
local test = require("test")
test.count = 2`)

	if err != nil {
		t.Error(err)
	}

	err = l.DoString(`
local test = require("test")
assert(test.count == 2)`)

	if err != nil {
		t.Error(err)
	}

	l2, _ := r.NewStateThread()
	defer l2.Close()
	err = l2.DoString(`
local test = require("test")
assert(test.count == 1)`)

	if err != nil {
		t.Error(err)
	}

	os.RemoveAll(DATA_PATH)
}

func TestRuntimeRegisterHTTP(t *testing.T) {
	r := newRuntime()
	r.InitModules()
	defer r.Stop()

	r.AppendPreload(map[string]string{
		"test": `
test={}
-- Get the mean value of a table
function test.printWorld()
	print("Hello World")
end
return test`})

	l, _ := r.NewStateThread()
	err := l.DoString(`
local nakama = require("nakama")
local test = require("test")
nakama.register_http(test.printWorld, "/stats/increment")
`)

	if err != nil {
		t.Error(err)
	}
	l.Close()

	err = r.InvokeLuaFunction(runtime_modules.FUNC_TYPE_HTTP, "/stats/increment", uuid.Nil)
	if err != nil {
		t.Error(err)
	}
}
