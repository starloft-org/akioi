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
	r.Use(middleware.CORS(func() []string { return h.Settings.Current().CORSAllowOrigins }))
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

	limiter := middleware.RateLimit(rdb, func() int { return h.Settings.Current().RateLimitPerMinute })

	api := r.Group("/api/v1")
	api.Use(limiter)
	{
		api.GET("/site", h.SiteInfo)
		api.GET("/site/captcha", h.SiteCaptchaConfig)
		api.GET("/languages", h.ListLanguages)
		// 评测节点回传结果，使用同步密钥鉴权，不挂用户鉴权中间件
		api.POST("/internal/judgements", h.InternalJudgement)
		api.POST("/internal/judge-progress", h.InternalJudgeProgress)
		api.GET("/announcements", h.ListAnnouncements)
		api.GET("/rankings", h.GlobalRanking)

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", h.Register)
			authGroup.POST("/login", h.Login)
			authGroup.POST("/sms/send", h.SendSMSCode)
			authGroup.POST("/email/send", h.SendEmailCode)
			authGroup.POST("/password/reset", h.ResetPassword)
			authGroup.POST("/logout", required, h.Logout)
			authGroup.GET("/me", required, h.Me)
			authGroup.GET("/wechat/url", optional, h.WeChatURL)
			authGroup.GET("/wechat/callback", h.WeChatCallback)
			authGroup.GET("/wechat/status", h.WeChatStatus)
			authGroup.DELETE("/wechat/binding", required, h.WeChatUnbind)
		}

		problems := api.Group("/problems")
		{
			problems.GET("", optional, h.ListProblems)
			problems.GET("/:id", optional, h.GetProblem)
			problems.GET("/:id/solutions", optional, h.ListSolutions)
			problems.POST("/:id/solutions", required, h.CreateSolution)
		}

		solutions := api.Group("/solutions")
		{
			solutions.GET("/:id", optional, h.GetSolution)
			solutions.PUT("/:id", required, h.UpdateSolution)
			solutions.DELETE("/:id", required, h.DeleteSolution)
		}

		discussions := api.Group("/discussions")
		{
			discussions.GET("", optional, h.ListDiscussions)
			discussions.POST("", required, h.CreateDiscussion)
			discussions.GET("/:id", optional, h.GetDiscussion)
			discussions.PUT("/:id", required, h.UpdateDiscussion)
			discussions.DELETE("/:id", required, h.DeleteDiscussion)
			discussions.POST("/:id/comments", required, h.CreateComment)
		}

		comments := api.Group("/comments")
		{
			comments.PUT("/:id", required, h.UpdateComment)
			comments.DELETE("/:id", required, h.DeleteComment)
		}

		submissions := api.Group("/submissions")
		submissions.Use(required)
		{
			submissions.POST("", h.CreateSubmission)
			submissions.GET("", h.ListSubmissions)
			submissions.GET("/:id", h.GetSubmission)
			submissions.GET("/:id/status", h.GetSubmissionStatus)
		}

		teams := api.Group("/teams")
		{
			teams.GET("", optional, h.ListTeams)
			teams.GET("/:id", optional, h.GetTeam)
			teams.POST("", required, h.CreateTeam)
			teams.PUT("/:id", required, h.UpdateTeam)
			teams.GET("/:id/members", required, h.ListTeamMembers)
			teams.DELETE("/:id/members/:uid", required, h.RemoveTeamMember)
			teams.POST("/:id/applications", required, h.ApplyTeam)
			teams.GET("/:id/applications", required, h.ListTeamApplications)
			teams.POST("/:id/applications/:aid/review", required, h.ReviewTeamApplication)
			teams.GET("/:id/problems", required, h.ListTeamProblems)
			teams.GET("/:id/contests", required, h.ListTeamContests)
			teams.GET("/:id/sets", required, h.ListTeamSets)
			teams.GET("/:id/sets/:sid", required, h.GetTeamSet)
			teams.POST("/:id/sets", required, h.CreateTeamSet)
			teams.PUT("/:id/sets/:sid", required, h.UpdateTeamSet)
			teams.DELETE("/:id/sets/:sid", required, h.DeleteTeamSet)
		}

		// 用户自建的团队题目、个人题目与题单
		my := api.Group("/my")
		my.Use(required)
		{
			my.GET("/problems", h.ListMyProblems)
			my.POST("/problems", h.CreateMyProblem)
			my.GET("/problems/:id", h.GetMyProblem)
			my.PUT("/problems/:id", h.UpdateMyProblem)
			my.DELETE("/problems/:id", h.DeleteMyProblem)
			my.GET("/problems/:id/testdata", h.ListMyProblemTestdata)
			my.POST("/problems/:id/testdata", h.UploadMyProblemTestdata)
			my.DELETE("/problems/:id/testdata", h.DeleteMyProblemTestdata)
			// 用户端批量导入个人题目：上传完返回任务号，再轮询进度
			my.POST("/problems/import", h.MyImportProblem)
			my.POST("/problems/convert", h.ConvertMyProblems)
			my.GET("/teams", h.ListMyTeams)
			my.GET("/import-tasks/:id", h.MyProblemImportStatus)
			my.GET("/problem-sets", h.ListMyProblemSets)
			my.POST("/problem-sets", h.CreateMyProblemSet)
			my.GET("/problem-sets/:id", h.GetMyProblemSet)
			my.PUT("/problem-sets/:id", h.UpdateMyProblemSet)
			my.DELETE("/problem-sets/:id", h.DeleteMyProblemSet)
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
			users.POST("/me/cover", h.UploadCover)
			users.DELETE("/me/cover", h.DeleteCover)
		}
	}

	registerAdmin(r, h, rdb, cfg)

	return r
}
