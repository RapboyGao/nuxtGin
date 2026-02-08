package utils

import (
	"fmt"

	"github.com/fatih/color"
)

var goLogPrefix = color.New(color.FgBlue).Sprint("[go]")

// Print prints a single log line with a blue [go] prefix.
func Print(v ...any) {
	if len(v) == 0 {
		fmt.Println(goLogPrefix)
		return
	}
	values := append([]any{goLogPrefix}, v...)
	fmt.Println(values...)
}

// PrintMulti prints multiple lines, each prefixed with a blue [go] tag.
func PrintMulti(lines ...string) {
	for _, line := range lines {
		Print(line)
	}
}
