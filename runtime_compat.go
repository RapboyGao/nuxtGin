package nuxtGin

import (
	"github.com/RapboyGao/nuxtGin/runtime"
	"github.com/gin-gonic/gin"
)

func BuildServerFromConfig(cfg runtime.APIServerConfig) (*gin.Engine, error) {
	return runtime.BuildServerFromConfig(cfg)
}

func RunServerFromConfig(cfg runtime.APIServerConfig) error {
	return runtime.RunServerFromConfig(cfg)
}
