package router

import (
	"akioi/internal/config"
	"akioi/internal/handler"
	"akioi/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// registerAdmin 装配管理后台路由，路由前缀 /api/admin/v1。
func registerAdmin(r *gin.Engine, h *handler.Handler, _ *redis.Client, _ *config.Config) {
	admin := r.Group("/api/admin/v1", middleware.Auth(h.Auth), middleware.RequireAdmin())

	admin.GET("/stats", h.AdminStats)
	admin.GET("/logs", h.AdminLogs)
	admin.GET("/settings", h.AdminGetSettings)
	admin.PUT("/settings", h.AdminUpdateSettings)

	users := admin.Group("/users")
	{
		users.GET("", h.AdminListUsers)
		users.PUT("/:id", h.AdminUpdateUser)
		users.POST("/import", h.AdminImportUsers)
	}

	problems := admin.Group("/problems")
	{
		problems.GET("", h.AdminListProblems)
		problems.POST("", h.AdminCreateProblem)
		problems.GET("/:id", h.AdminGetProblem)
		problems.PUT("/:id", h.AdminUpdateProblem)
		problems.DELETE("/:id", h.AdminDeleteProblem)
		problems.GET("/:id/testdata", h.AdminListTestdata)
		problems.POST("/:id/testdata", h.AdminUploadTestdata)
		problems.DELETE("/:id/testdata", h.AdminDeleteTestdata)
		problems.GET("/:id/testdata/download", h.AdminDownloadTestdata)
	}

	contests := admin.Group("/contests")
	{
		contests.GET("", h.AdminListContests)
		contests.POST("", h.AdminCreateContest)
		contests.GET("/:id", h.AdminGetContest)
		contests.PUT("/:id", h.AdminUpdateContest)
		contests.DELETE("/:id", h.AdminDeleteContest)
		contests.PUT("/:id/problems", h.AdminSetContestProblems)
	}

	admin.GET("/submissions", h.AdminListSubmissions)
	admin.POST("/submissions/:id/rejudge", h.AdminRejudgeSubmission)
	admin.POST("/rejudge", h.AdminRejudge)

	announcements := admin.Group("/announcements")
	{
		announcements.GET("", h.AdminListAnnouncements)
		announcements.POST("", h.AdminCreateAnnouncement)
		announcements.PUT("/:id", h.AdminUpdateAnnouncement)
		announcements.DELETE("/:id", h.AdminDeleteAnnouncement)
	}
}
