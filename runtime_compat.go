package nuxtGin

import (
	"github.com/RapboyGao/nuxtGin/runtime"
	"github.com/gin-gonic/gin"
)

// APIServerConfig is kept for backward compatibility.
type APIServerConfig = runtime.APIServerConfig

func DefaultAPIServerConfig() APIServerConfig {
	return runtime.DefaultAPIServerConfig()
}

func BuildServerFromConfig(cfg APIServerConfig) (*gin.Engine, error) {
	return runtime.BuildServerFromConfig(cfg)
}

func RunServerFromConfig(cfg APIServerConfig) error {
	return runtime.RunServerFromConfig(cfg)
}
