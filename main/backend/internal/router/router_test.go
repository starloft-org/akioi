package router

import (
	"testing"

	"akioi/internal/config"
	"akioi/internal/handler"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// TestSetupRegistersRoutes 验证路由装配不会因路径冲突 panic，且关键接口都已注册。
func TestSetupRegistersRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecret: "test-secret",
		Judge:     config.JudgeConfig{QueueKey: "test:queue"},
	}
	settings := service.NewStaticSettingService(service.DefaultRuntimeSettings())
	h := handler.New(nil, cfg, nil, settings)

	engine := Setup(h, cfg, nil, nil)

	registered := map[string]bool{}
	for _, r := range engine.Routes() {
		registered[r.Method+" "+r.Path] = true
	}

	expected := []string{
		"GET /healthz",
		"GET /readyz",
		"POST /api/v1/auth/register",
		"POST /api/v1/auth/login",
		"GET /api/v1/problems",
		"GET /api/v1/problems/:id",
		"POST /api/v1/submissions",
		"GET /api/v1/submissions/:id",
		"GET /api/v1/contests/:id/rank",
		"GET /api/v1/contests/:id/rank.csv",
		"POST /api/v1/contests/:id/register",
		"PUT /api/v1/users/me/password",
		"POST /api/v1/users/me/avatar",
		"POST /api/v1/users/me/cover",
		"DELETE /api/v1/users/me/cover",
		"GET /api/v1/my/problems",
		"POST /api/v1/my/problems",
		"GET /api/v1/my/problems/:id",
		"PUT /api/v1/my/problems/:id",
		"DELETE /api/v1/my/problems/:id",
		"POST /api/v1/my/problems/:id/testdata",
		"DELETE /api/v1/my/problems/:id/testdata",
		"GET /api/v1/my/problem-sets",
		"POST /api/v1/my/problem-sets",
		"GET /api/v1/my/problem-sets/:id",
		"PUT /api/v1/my/problem-sets/:id",
		"DELETE /api/v1/my/problem-sets/:id",
		"GET /api/v1/teams/:id/sets/:sid",
		"GET /api/admin/v1/problem-banks",
		"POST /api/admin/v1/problem-banks",
		"PUT /api/admin/v1/problem-banks/:id",
		"DELETE /api/admin/v1/problem-banks/:id",
		"GET /api/admin/v1/backups",
		"POST /api/admin/v1/backups",
		"GET /api/admin/v1/backups/:name",
		"DELETE /api/admin/v1/backups/:name",
		"GET /api/admin/v1/stats",
		"GET /api/admin/v1/settings",
		"PUT /api/admin/v1/settings",
		"GET /api/admin/v1/users",
		"PUT /api/admin/v1/users/:id",
		"POST /api/admin/v1/users/import",
		"GET /api/admin/v1/logs",
		"GET /api/admin/v1/admins",
		"POST /api/admin/v1/admins",
		"PUT /api/admin/v1/admins/:id",
		"DELETE /api/admin/v1/admins/:id",
		"POST /api/admin/v1/problems",
		"POST /api/admin/v1/problems/:id/testdata",
		"GET /api/admin/v1/problems/:id/testdata/download",
		"PUT /api/admin/v1/contests/:id/problems",
		"POST /api/admin/v1/rejudge",
		"POST /api/admin/v1/submissions/:id/rejudge",
	}
	for _, route := range expected {
		if !registered[route] {
			t.Errorf("路由未注册: %s", route)
		}
	}
}
