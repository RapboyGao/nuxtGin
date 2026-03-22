package runtimeutil

import (
	"os"

	"github.com/arduino/go-paths-helper"
)

// Dir creates nested directories under the current working directory and returns the final path.
func Dir(names ...string) *paths.Path {
	workingDir, _ := os.Getwd()
	dir := paths.New(workingDir)
	dir.ToAbs()
	for _, name := range names {
		dir = dir.Join(name)
		dir.Mkdir()
	}
	return dir
}
