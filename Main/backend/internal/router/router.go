// Package router 集中装配 HTTP 路由与中间件。
package router

import (
	"net/http"

	"akioi/internal/config"
	"akioi/internal/handler"
	"akioi/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Setup 装配全部路由并返回 gin 引擎。
func Setup(h *handler.Handler, cfg *config.Config, db *gorm.DB, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	r.Use(middleware.RealIP())
	r.Use(middleware.Recover())
	r.Use(middleware.AccessLog())
	r.Use(middleware.CORS(cfg.CORSAllowOrigins))
	r.MaxMultipartMemory = 64 << 20

	auth := h.Auth
	optional := middleware.OptionalAuth(auth)
	required := middleware.Auth(auth)

	// 探针不参与限流，供容器健康检查使用。
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", func(c *gin.Context) {
		if err := db.Exec("SELECT 1").Error; err != nil {
			c.String(http.StatusServiceUnavailable, "database unavailable")
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.String(http.StatusServiceUnavailable, "redis unavailable")
			return
		}
		c.String(http.StatusOK, "ok")
	})
	r.Static("/uploads", cfg.UploadDir)

	limiter := middleware.RateLimit(rdb, cfg.RateLimitPerMinute)

	api := r.Group("/api/v1")
	api.Use(limiter)
	{
		api.GET("/site", h.SiteInfo)
		api.GET("/languages", h.ListLanguages)
		api.GET("/announcements", h.ListAnnouncements)

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", h.Register)
			authGroup.POST("/login", h.Login)
			authGroup.POST("/logout", required, h.Logout)
			authGroup.GET("/me", required, h.Me)
			authGroup.GET("/wechat/url", h.WeChatURL)
			authGroup.GET("/wechat/callback", h.WeChatCallback)
		}

		problems := api.Group("/problems")
		{
			problems.GET("", optional, h.ListProblems)
			problems.GET("/:id", optional, h.GetProblem)
		}

		submissions := api.Group("/submissions")
		submissions.Use(required)
		{
			submissions.POST("", h.CreateSubmission)
			submissions.GET("", h.ListSubmissions)
			submissions.GET("/:id", h.GetSubmission)
		}

		contests := api.Group("/contests")
		{
			contests.GET("", optional, h.ListContests)
			contests.GET("/:id", optional, h.GetContest)
			contests.GET("/:id/rank", optional, h.ContestRank)
			contests.GET("/:id/rank.csv", optional, h.ContestRankCSV)
			contests.GET("/:id/submissions", required, h.ContestSubmissions)
			contests.POST("/:id/register", required, h.RegisterContest)
		}

		users := api.Group("/users")
		users.Use(required)
		{
			users.GET("/:id", h.GetUserProfile)
			users.PUT("/me", h.UpdateMe)
			users.PUT("/me/password", h.ChangePassword)
			users.POST("/me/avatar", h.UploadAvatar)
		}
	}

	registerAdmin(r, h, rdb, cfg)

	return r
}
