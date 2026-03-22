package runtime

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/RapboyGao/nuxtGin/endpoint"
	"github.com/gin-gonic/gin"
)

type runtimeTestResponse struct {
	Message string `json:"message"`
}

func resetRuntimeStateForTest() {
	loadConfigOnce = sync.Once{}
	loadConfigErr = nil
	*GetConfig = ServerRuntimeConfig{}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
		resetRuntimeStateForTest()
	})
}

func writeServerConfigFile(t *testing.T, dir string, config ServerRuntimeConfig) {
	t.Helper()
	bytes, err := stdjson.Marshal(config)
	if err != nil {
		t.Fatalf("marshal server config failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.config.json"), bytes, 0o644); err != nil {
		t.Fatalf("write server.config.json failed: %v", err)
	}
}

func buildTestEndpoints() []endpoint.EndpointLike {
	return []endpoint.EndpointLike{
		endpoint.NewEndpointNoParams[runtimeTestResponse, runtimeTestResponse](
			"Ping",
			endpoint.HTTPMethodPost,
			"/ping",
			func(req runtimeTestResponse, _ *gin.Context) (runtimeTestResponse, error) {
				return runtimeTestResponse{Message: req.Message}, nil
			},
		),
	}
}

func TestBuildServerFromConfig_AllowsExplicitServerConfigWithoutFile(t *testing.T) {
	tempDir := t.TempDir()
	withWorkingDir(t, tempDir)
	gin.SetMode(gin.ReleaseMode)

	engine, err := BuildServerFromConfig(APIServerConfig{
		Server: ServerRuntimeConfig{
			GinPort:  18099,
			NuxtPort: 13000,
			BaseUrl:  "/app",
		},
		GinMode: gin.ReleaseMode,
		ServerAPI: endpoint.ServerAPI{
			BasePath:  "/api-go",
			GroupPath: "/v1",
			Endpoints: buildTestEndpoints(),
		},
	})
	if err != nil {
		t.Fatalf("BuildServerFromConfig returned error: %v", err)
	}
	if engine == nil {
		t.Fatal("expected gin engine, got nil")
	}
	if GetConfig.BaseUrl != "/app" {
		t.Fatalf("expected active config baseUrl /app, got %q", GetConfig.BaseUrl)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "vue", "composables", "auto-generated-api.ts")); !os.IsNotExist(err) {
		t.Fatalf("BuildServerFromConfig should not export TS, stat err=%v", err)
	}
}

func TestBuildServerFromConfig_UsesServerConfigFileAsFallback(t *testing.T) {
	tempDir := t.TempDir()
	withWorkingDir(t, tempDir)
	gin.SetMode(gin.ReleaseMode)
	writeServerConfigFile(t, tempDir, ServerRuntimeConfig{
		GinPort:  18091,
		NuxtPort: 13001,
		BaseUrl:  "/fallback",
	})

	engine, err := BuildServerFromConfig(APIServerConfig{
		GinMode: gin.ReleaseMode,
		ServerAPI: endpoint.ServerAPI{
			BasePath:  "/api-go",
			GroupPath: "/v1",
			Endpoints: buildTestEndpoints(),
		},
	})
	if err != nil {
		t.Fatalf("BuildServerFromConfig returned error: %v", err)
	}
	if engine == nil {
		t.Fatal("expected gin engine, got nil")
	}
	if GetConfig.GinPort != 18091 || GetConfig.BaseUrl != "/fallback" {
		t.Fatalf("expected fallback config to be active, got %+v", *GetConfig)
	}
}

func TestBuildServerFromConfig_FailsWhenConfigIncompleteAndNoFile(t *testing.T) {
	tempDir := t.TempDir()
	withWorkingDir(t, tempDir)
	gin.SetMode(gin.ReleaseMode)

	_, err := BuildServerFromConfig(APIServerConfig{
		Server: ServerRuntimeConfig{
			GinPort: 18092,
		},
		GinMode: gin.ReleaseMode,
	})
	if err == nil {
		t.Fatal("expected error for incomplete config without server.config.json")
	}
}

func TestResolveGinMode_PrefersExplicitThenEnvThenFilesystem(t *testing.T) {
	tempDir := t.TempDir()
	withWorkingDir(t, tempDir)

	if err := os.Setenv("NUXT_GIN_MODE", "release"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	if err := os.Setenv("GIN_MODE", "debug"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Unsetenv("NUXT_GIN_MODE")
		_ = os.Unsetenv("GIN_MODE")
	})

	if mode := ResolveGinMode("debug"); mode != gin.DebugMode {
		t.Fatalf("expected explicit debug mode, got %q", mode)
	}
	if mode := ResolveGinMode(""); mode != gin.ReleaseMode {
		t.Fatalf("expected NUXT_GIN_MODE to win with release, got %q", mode)
	}
	if err := os.Unsetenv("NUXT_GIN_MODE"); err != nil {
		t.Fatalf("unsetenv failed: %v", err)
	}
	if mode := ResolveGinMode(""); mode != gin.DebugMode {
		t.Fatalf("expected GIN_MODE to win with debug, got %q", mode)
	}
}

func TestExportTypesFromConfig_WritesExpectedFiles(t *testing.T) {
	tempDir := t.TempDir()
	withWorkingDir(t, tempDir)
	gin.SetMode(gin.ReleaseMode)

	cfg := APIServerConfig{
		Server: ServerRuntimeConfig{
			GinPort:  18100,
			NuxtPort: 13002,
			BaseUrl:  "/app",
		},
		ServerAPI: endpoint.ServerAPI{
			BasePath:  "/api-go",
			GroupPath: "/v1",
			Endpoints: buildTestEndpoints(),
		},
		ServerTSPath:  "vue/composables/http.ts",
		ExportTSOnRun: false,
	}

	if err := ExportTypesFromConfig(cfg); err != nil {
		t.Fatalf("ExportTypesFromConfig returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "vue", "composables", "http.ts")); err != nil {
		t.Fatalf("expected exported TS file, got err=%v", err)
	}
}
