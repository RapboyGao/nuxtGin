package endpoint

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

func registerEndpointHandlers(router gin.IRouter, endpoints []EndpointLike) error {
	for i := range endpoints {
		handler, method, path, err := buildGinHandler(endpoints[i])
		if err != nil {
			return fmt.Errorf("register endpoint[%d] failed: %w", i, err)
		}
		router.Handle(method, path, handler)
	}
	return nil
}

func buildGinHandler(e EndpointLike) (gin.HandlerFunc, string, string, error) {
	meta := e.EndpointMeta()
	if strings.TrimSpace(string(meta.Method)) == "" {
		return nil, "", "", errors.New("method is required")
	}
	if strings.TrimSpace(meta.Path) == "" {
		return nil, "", "", errors.New("path is required")
	}
	if !meta.Method.IsValid() {
		return nil, "", "", errors.New("invalid http method")
	}
	return e.GinHandler(), string(meta.Method), meta.Path, nil
}

func bindStructT[T any](bind func(any) error) (T, error) {
	var v T
	if isNoType(typeOf[T]()) {
		return v, nil
	}
	if err := bind(&v); err != nil {
		return v, err
	}
	return v, nil
}

func bindJSONStructT[T any](ctx *gin.Context) (T, error) {
	var v T
	if isNoType(typeOf[T]()) {
		return v, nil
	}
	if err := ctx.ShouldBindJSON(&v); err != nil {
		return v, err
	}
	return v, nil
}

func bindCookieStructT[T any](ctx *gin.Context) (T, error) {
	var v T
	if isNoType(typeOf[T]()) {
		return v, nil
	}
	cookies := map[string]string{}
	for _, c := range ctx.Request.Cookies() {
		cookies[c.Name] = c.Value
	}
	if err := mapstructure.Decode(cookies, &v); err != nil {
		return v, err
	}
	return v, nil
}

func isNoType(t reflect.Type) bool {
	if t == nil || t.Kind() == reflect.Invalid {
		return true
	}
	return t == reflect.TypeOf(NoParams{}) || t == reflect.TypeOf(NoBody{}) || t == reflect.TypeOf(NoMessage{})
}
