package runtime

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
		greenTag := color.New(color.FgGreen).Sprint("[GIN-debug]")
		yellowWarn := color.New(color.FgYellow).Sprint("[GIN-Warning]")
		redError := color.New(color.FgRed).Sprint("[GIN-Error]")
		gin.DebugPrintFunc = func(format string, values ...any) {
			out := fmt.Sprintf(format, values...)
			out = strings.ReplaceAll(out, "[GIN-debug]", greenTag)
			out = strings.ReplaceAll(out, "[WARNING]", yellowWarn)
			out = strings.ReplaceAll(out, "[ERROR]", redError)
			fmt.Print(out)
		}
	})
}
