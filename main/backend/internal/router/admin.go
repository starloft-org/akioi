package router

import (
	"akioi/internal/config"
	"akioi/internal/handler"
	"akioi/internal/middleware"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// registerAdmin 装配管理后台路由，路由前缀 /api/admin/v1。
// 除概览外，各模块都要求管理员被分配了对应权限项，超级管理员始终通过。
func registerAdmin(r *gin.Engine, h *handler.Handler, _ *redis.Client, _ *config.Config) {
	admin := r.Group("/api/admin/v1", middleware.Auth(h.Auth), middleware.RequireAdmin())

	admin.GET("/stats", h.AdminStats)

	admin.GET("/logs", middleware.RequirePermission(model.AdminPermissionLogs), h.AdminLogs)
	admin.GET("/settings", middleware.RequirePermission(model.AdminPermissionSettings), h.AdminGetSettings)
	admin.PUT("/settings", middleware.RequirePermission(model.AdminPermissionSettings), h.AdminUpdateSettings)

	// 用户列表对全部管理员开放，供题目归属等选择器使用；修改与导入需要用户管理权限。
	admin.GET("/users", h.AdminListUsers)
	users := admin.Group("/users", middleware.RequirePermission(model.AdminPermissionUsers))
	{
		users.PUT("/:id", h.AdminUpdateUser)
		users.POST("/import", h.AdminImportUsers)
	}

	// 管理员管理与权限分配只对超级管理员开放。
	admins := admin.Group("/admins", middleware.RequireSuperAdmin())
	{
		admins.GET("", h.AdminListAdmins)
		admins.POST("", h.AdminCreateAdmin)
		admins.PUT("/:id", h.AdminUpdateAdmin)
		admins.DELETE("/:id", h.AdminDeleteAdmin)
	}

	problems := admin.Group("/problems", middleware.RequirePermission(model.AdminPermissionProblems))
	{
		problems.GET("", h.AdminListProblems)
		problems.POST("", h.AdminCreateProblem)
		problems.POST("/import", h.AdminImportProblem)
		problems.POST("/move-bank", h.AdminMoveProblemsBank)
		problems.GET("/:id", h.AdminGetProblem)
		problems.PUT("/:id", h.AdminUpdateProblem)
		problems.DELETE("/:id", h.AdminDeleteProblem)
		problems.GET("/:id/testdata", h.AdminListTestdata)
		problems.POST("/:id/testdata", h.AdminUploadTestdata)
		problems.POST("/:id/testdata/sync", h.AdminSyncTestdata)
		problems.DELETE("/:id/testdata", h.AdminDeleteTestdata)
		problems.GET("/:id/testdata/download", h.AdminDownloadTestdata)
	}

	// 批量导入的任务进度单独挂一层，避免与 /problems/:id 的通配段冲突
	admin.GET("/import-tasks/:id", middleware.RequirePermission(model.AdminPermissionProblems), h.AdminProblemImportStatus)

	banks := admin.Group("/problem-banks", middleware.RequirePermission(model.AdminPermissionProblemBanks))
	{
		banks.GET("", h.AdminListProblemBanks)
		banks.POST("", h.AdminCreateProblemBank)
		banks.PUT("/:id", h.AdminUpdateProblemBank)
		banks.DELETE("/:id", h.AdminDeleteProblemBank)
	}

	backups := admin.Group("/backups", middleware.RequirePermission(model.AdminPermissionBackups))
	{
		backups.GET("", h.AdminListBackups)
		backups.POST("", h.AdminCreateBackup)
		backups.GET("/:name", h.AdminDownloadBackup)
		backups.DELETE("/:name", h.AdminDeleteBackup)
	}

	judges := admin.Group("/judges", middleware.RequirePermission(model.AdminPermissionJudges))
	judges.GET("/status", h.AdminJudgeNodes)
	judges.POST("/sync-all", h.AdminJudgeSyncAll)

	contests := admin.Group("/contests", middleware.RequirePermission(model.AdminPermissionContests))
	{
		contests.GET("", h.AdminListContests)
		contests.POST("", h.AdminCreateContest)
		contests.GET("/:id", h.AdminGetContest)
		contests.PUT("/:id", h.AdminUpdateContest)
		contests.DELETE("/:id", h.AdminDeleteContest)
		contests.PUT("/:id/problems", h.AdminSetContestProblems)
	}

	submissions := admin.Group("/submissions", middleware.RequirePermission(model.AdminPermissionSubmissions))
	submissions.GET("", h.AdminListSubmissions)
	admin.POST("/submissions/:id/rejudge", middleware.RequirePermission(model.AdminPermissionSubmissions), h.AdminRejudgeSubmission)
	admin.POST("/rejudge", middleware.RequirePermission(model.AdminPermissionSubmissions), h.AdminRejudge)

	announcements := admin.Group("/announcements", middleware.RequirePermission(model.AdminPermissionAnnouncements))
	{
		announcements.GET("", h.AdminListAnnouncements)
		announcements.POST("", h.AdminCreateAnnouncement)
		announcements.PUT("/:id", h.AdminUpdateAnnouncement)
		announcements.DELETE("/:id", h.AdminDeleteAnnouncement)
	}
}
