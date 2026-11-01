package handler

import (
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminAccountItem struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Role        string     `json:"role"`
	Status      int        `json:"status"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
}

func newAdminAccountItem(u *model.User) adminAccountItem {
	return adminAccountItem{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		Role:        u.Role,
		Status:      u.Status,
		Permissions: u.Permissions(),
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// AdminListAdmins 返回全部管理员及其权限，附带权限项清单供后台渲染。
func (h *Handler) AdminListAdmins(c *gin.Context) {
	var users []model.User
	if err := h.DB.Where("role IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).
		Order("id asc").Find(&users).Error; err != nil {
		httpx.ServerError(c, "查询管理员失败")
		return
	}
	items := make([]adminAccountItem, 0, len(users))
	for i := range users {
		items = append(items, newAdminAccountItem(&users[i]))
	}
	httpx.OK(c, gin.H{
		"items":              items,
		"total":              len(items),
		"permission_options": model.AdminPermissionOptions(),
	})
}

type adminCreateAdminRequest struct {
	UserID      uint     `json:"user_id"`
	Permissions []string `json:"permissions"`
}

// AdminCreateAdmin 把已有账号提升为管理员并分配权限，不创建新账号。
func (h *Handler) AdminCreateAdmin(c *gin.Context) {
	var req adminCreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	if req.UserID == 0 {
		httpx.BadRequest(c, "请选择要设为管理员的用户")
		return
	}
	if unknown := model.UnknownAdminPermissions(req.Permissions); len(unknown) > 0 {
		httpx.BadRequest(c, "存在无效的权限项："+strings.Join(unknown, "、"))
		return
	}
	var target model.User
	if err := h.DB.First(&target, req.UserID).Error; err != nil {
		httpx.NotFound(c, "用户不存在")
		return
	}
	if target.IsAdmin() {
		httpx.Conflict(c, "该用户已经是管理员")
		return
	}
	// 未指定权限时默认授予全部后台模块，之后可以在编辑里裁剪。
	permissions := req.Permissions
	if len(permissions) == 0 {
		permissions = model.AllAdminPermissions()
	}
	if err := h.DB.Model(&model.User{}).Where("id = ?", target.ID).Updates(map[string]any{
		"role":              model.RoleAdmin,
		"admin_permissions": strings.Join(model.NormalizeAdminPermissions(permissions), ","),
	}).Error; err != nil {
		httpx.ServerError(c, "设置管理员失败")
		return
	}
	if operator := httpx.CurrentUser(c); operator != nil {
		logger.Adminf("管理员 %s 把用户 %s (id=%d) 设为管理员 ip=%s", operator.Username, target.Username, target.ID, c.ClientIP())
	}
	var fresh model.User
	if err := h.DB.First(&fresh, target.ID).Error; err != nil {
		httpx.ServerError(c, "设置管理员失败")
		return
	}
	httpx.OK(c, gin.H{"admin": newAdminAccountItem(&fresh)})
}

type adminUpdateAdminRequest struct {
	Permissions *[]string `json:"permissions"`
	Nickname    *string   `json:"nickname"`
	Password    *string   `json:"password"`
	Status      *int      `json:"status"`
}

// AdminUpdateAdmin 调整管理员的权限、昵称、密码或状态，超级管理员账号不可修改。
func (h *Handler) AdminUpdateAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var target model.User
	if err := h.DB.First(&target, id).Error; err != nil {
		httpx.NotFound(c, "管理员不存在")
		return
	}
	if target.IsSuperAdmin() {
		httpx.BadRequest(c, "不能修改超级管理员")
		return
	}
	if target.Role != model.RoleAdmin {
		httpx.BadRequest(c, "该账号不是管理员")
		return
	}

	var req adminUpdateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	if req.Permissions != nil {
		if unknown := model.UnknownAdminPermissions(*req.Permissions); len(unknown) > 0 {
			httpx.BadRequest(c, "存在无效的权限项："+strings.Join(unknown, "、"))
			return
		}
		updates["admin_permissions"] = strings.Join(model.NormalizeAdminPermissions(*req.Permissions), ",")
	}
	if req.Nickname != nil {
		nickname := strings.TrimSpace(*req.Nickname)
		if nickname == "" {
			httpx.BadRequest(c, "昵称不能为空")
			return
		}
		if len([]rune(nickname)) > 32 {
			httpx.BadRequest(c, "昵称长度不能超过 32 个字符")
			return
		}
		updates["nickname"] = nickname
	}
	if req.Status != nil {
		if *req.Status != model.UserStatusActive && *req.Status != model.UserStatusBanned {
			httpx.BadRequest(c, "状态取值非法")
			return
		}
		updates["status"] = *req.Status
	}
	if req.Password != nil {
		if len(*req.Password) < 6 || len(*req.Password) > 64 {
			httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
			return
		}
		hash, err := h.Auth.HashPassword(*req.Password)
		if err != nil {
			httpx.ServerError(c, "更新管理员失败")
			return
		}
		updates["password_hash"] = hash
		updates["auth_version"] = gorm.Expr("auth_version + 1")
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.User{}).Where("id = ?", target.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新管理员失败")
			return
		}
	}
	var fresh model.User
	if err := h.DB.First(&fresh, target.ID).Error; err != nil {
		httpx.ServerError(c, "更新管理员失败")
		return
	}
	httpx.OK(c, gin.H{"admin": newAdminAccountItem(&fresh)})
}

// AdminDeleteAdmin 取消管理员身份并清空权限，账号本身保留为普通用户。
func (h *Handler) AdminDeleteAdmin(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var target model.User
	if err := h.DB.First(&target, id).Error; err != nil {
		httpx.NotFound(c, "管理员不存在")
		return
	}
	if target.IsSuperAdmin() {
		httpx.BadRequest(c, "不能取消超级管理员")
		return
	}
	if target.Role != model.RoleAdmin {
		httpx.BadRequest(c, "该账号不是管理员")
		return
	}
	if err := h.DB.Model(&model.User{}).Where("id = ?", target.ID).Updates(map[string]any{
		"role":              model.RoleUser,
		"admin_permissions": "",
		"auth_version":      gorm.Expr("auth_version + 1"),
	}).Error; err != nil {
		httpx.ServerError(c, "取消管理员失败")
		return
	}
	httpx.OK(c, gin.H{})
}
