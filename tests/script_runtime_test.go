package tests

import (
	"nakama/server"
	"testing"

	"go.uber.org/zap"
)

func newRuntime() *server.ScriptRuntime {
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
	return server.NewScriptRuntime(logger, logger, "data")
}

func TestRuntimeSampleScript(t *testing.T) {
	r := newRuntime()
	l, _ := r.NewStateThread()
	defer l.Close()
	err := l.DoString(`
local example = "an example string"
for i in string.gmatch(example, "%S+") do
   print(i)
end
	`)

	if err != nil {
		t.Error(err)
	}
}

func TestRuntimeDisallowStandardLibs(t *testing.T) {
	r := newRuntime()
	l, _ := r.NewStateThread()
	defer l.Close()
	err := l.DoString(`
-- Return true if file exists and is readable.
function file_exists(path)
  local file = io.open(path, "r")
  if file then file:close() end
  return file ~= nil
end
file_exists "./"
	`)

	if err == nil {
		t.Error("Successfully accessed IO package")
	} else {
		t.Log(err)
	}
}

func TestRuntimeRequire(t *testing.T) {
	r := newRuntime()
	l := r.NewState(map[string]string{
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

return stats
	`,
	})
	defer l.Close()

	//
	//preload := l.GetField(l.GetField(l.Get(lua.EnvironIndex), "package"), "preload")
	//l.SetField(preload, "stats", statsModule)

	//if e != nil {
	//	t.Error(e)
	//}

	err := l.DoString(`
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
print(stats.mean(t))
		`)

	if err != nil {
		t.Error(err)
	}

	err = l.DoString(`
local stats = require("stats")
t = {[1]=5, [2]=7, [3]=8, [4]='Something else.'}
print(stats.mean(t))
		`)

	if err != nil {
		t.Error(err)
	}
}
