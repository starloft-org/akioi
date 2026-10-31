package middleware

import (
	"strings"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
)

// Auth 强制登录，未登录返回 401。
func Auth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := auth.Authenticate(bearerToken(c))
		if err != nil {
			httpx.Unauthorized(c, err.Error())
			c.Abort()
			return
		}
		httpx.SetUser(c, u)
		c.Next()
	}
}

// OptionalAuth 解析令牌但不强制登录。
func OptionalAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := bearerToken(c); token != "" {
			if u, err := auth.Authenticate(token); err == nil {
				httpx.SetUser(c, u)
			}
		}
		c.Next()
	}
}

// RequireAdmin 要求管理员角色，超级管理员与管理员都可以通过。
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := httpx.CurrentUser(c)
		if u == nil || !u.IsAdmin() {
			httpx.Forbidden(c, "需要管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSuperAdmin 要求超级管理员角色，用于管理员管理与权限分配。
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := httpx.CurrentUser(c)
		if u == nil || !u.IsSuperAdmin() {
			httpx.Forbidden(c, "需要超级管理员权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePermission 要求管理员拥有指定后台权限项，超级管理员始终通过。
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := httpx.CurrentUser(c)
		if u == nil || !u.IsAdmin() {
			httpx.Forbidden(c, "需要管理员权限")
			c.Abort()
			return
		}
		if !u.HasAdminPermission(permission) {
			httpx.Forbidden(c, "当前管理员未被分配该模块的权限")
			c.Abort()
			return
		}
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}

// AccessLog 按类别记录 HTTP 访问日志，包含身份与 IP。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		user := "-"
		if u := httpx.CurrentUser(c); u != nil {
			user = u.Username
		}
		logger.Accessf("%s %s %s status=%d query=%q user=%s",
			c.ClientIP(), c.Request.Method, c.Request.URL.Path, c.Writer.Status(), c.Request.URL.RawQuery, user)
	}
}

// Recover 捕获 panic，记录错误日志并返回统一错误响应。
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("请求处理 panic: %v path=%s", r, c.Request.URL.Path)
				httpx.ServerError(c, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// RealIP 优先取反向代理透传的真实 IP，供访问日志与限流使用。
func RealIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch {
		case c.GetHeader("X-Real-IP") != "":
			c.Request.RemoteAddr = strings.TrimSpace(c.GetHeader("X-Real-IP"))
		case c.GetHeader("X-Forwarded-For") != "":
			fwd := c.GetHeader("X-Forwarded-For")
			if idx := strings.Index(fwd, ","); idx > 0 {
				c.Request.RemoteAddr = strings.TrimSpace(fwd[:idx])
			} else {
				c.Request.RemoteAddr = strings.TrimSpace(fwd)
			}
		}
		c.Next()
	}
}

// CORS 仅在配置了允许来源时启用跨域，供本地前端联调使用。
func CORS(origins func() []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		allowed := make(map[string]bool)
		for _, o := range origins() {
			allowed[o] = true
		}
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed["*"] || allowed[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
