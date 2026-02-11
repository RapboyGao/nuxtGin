package runtime

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
)

func newGinEngine() *gin.Engine {
	engine := gin.New()
	engine.Use(gin.LoggerWithFormatter(ginGreenTagFormatter), gin.Recovery())
	return engine
}

func ginGreenTagFormatter(p gin.LogFormatterParams) string {
	statusCode := fmt.Sprintf("%d", p.StatusCode)
	method := p.Method
	if p.IsOutputColor() {
		statusCode = p.StatusCodeColor() + statusCode + p.ResetColor()
		method = p.MethodColor() + method + p.ResetColor()
	}

	tag := "[GIN]"
	if p.IsOutputColor() {
		tag = color.New(color.FgGreen).Sprint("[GIN]")
	}

	if p.Latency > 0 {
		return fmt.Sprintf("%s %v | %3s | %13v | %15s | %-7s %#v\n%s",
			tag,
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusCode,
			p.Latency,
			p.ClientIP,
			method,
			p.Path,
			p.ErrorMessage,
		)
	}

	return fmt.Sprintf("%s %v | %3s | %15s | %-7s %#v\n%s",
		tag,
		p.TimeStamp.Format("2006/01/02 - 15:04:05"),
		statusCode,
		p.ClientIP,
		method,
		p.Path,
		p.ErrorMessage,
	)
}
