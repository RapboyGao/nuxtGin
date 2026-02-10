package endpoint

import "github.com/gin-gonic/gin"

func shouldExportTSInCurrentEnv() bool {
	return gin.Mode() == gin.DebugMode
}

