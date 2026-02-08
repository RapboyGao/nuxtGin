package nuxtGin

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

var setupGinDebugPrinterOnce sync.Once

func setupGinDebugPrinter() {
	setupGinDebugPrinterOnce.Do(func() {
		blueTag := color.New(color.FgBlue).Sprint("[GIN-debug]")
		gin.DebugPrintFunc = func(format string, values ...any) {
			out := fmt.Sprintf(format, values...)
			out = strings.ReplaceAll(out, "[GIN-debug]", blueTag)
			fmt.Print(out)
		}
	})
}
