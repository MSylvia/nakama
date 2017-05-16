package tests

import (
	"nakama/server"
	"testing"

	"go.uber.org/zap"
)

func newRuntime() *server.ScriptRuntime {
	logger, _ := zap.NewDevelopment(zap.AddStacktrace(zap.ErrorLevel))
	return server.NewScriptRuntime(logger, logger, "data/modules")
}

func TestRuntimeSampleScript(t *testing.T) {
	r := newRuntime()
	l := r.NewState()
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
	l := r.NewState()
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

func TestLoadModules(t *testing.T) {

}
