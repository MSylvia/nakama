package server

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

func InitRuntime(logger *zap.Logger, datadir string) {
	// Make module dir if not exists
	modpath := filepath.Join(datadir, "modules")
	os.MkdirAll(modpath, os.ModePerm)
	// Accumulate all Lua modules.
	modules := make([]string, 10)
	err := filepath.Walk(modpath, func(path string, f os.FileInfo, err error) error {
		fmt.Printf("Walked %s\n", path)
		if !f.IsDir() {
			modules = append(modules, path)
		}
		return nil
	})
	if err != nil {
		logger.Error("Failed to walk script module dir.", zap.Error(err))
	}
	fmt.Printf("%+v\n", modules)
	luavm := lua.NewState(lua.Options{
		CallStackSize:       1024,
		RegistrySize:        1024,
		SkipOpenLibs:        true,
		IncludeGoStackTrace: true})
	defer luavm.Close()
}
