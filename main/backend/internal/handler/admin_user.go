package handler

import (
	"encoding/csv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/model"
	"akioi/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminUserItem struct {
	ID              uint       `json:"id"`
	Username        string     `json:"username"`
	Nickname        string     `json:"nickname"`
	Phone           string     `json:"phone"`
	Role            string     `json:"role"`
	Status          int        `json:"status"`
	School          string     `json:"school"`
	Grade           string     `json:"grade"`
	CreatedAt       time.Time  `json:"created_at"`
	LastLoginAt     *time.Time `json:"last_login_at"`
	SubmissionCount int64      `json:"submission_count"`
}

// AdminListUsers 返回用户列表，支持关键字与角色筛选。
func (h *Handler) AdminListUsers(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)

	query := h.DB.Model(&model.User{})
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		like := "%" + kw + "%"
		query = query.Where("username LIKE ? OR nickname LIKE ? OR phone LIKE ?", like, like, like)
	}
	if role := strings.TrimSpace(c.Query("role")); role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询用户失败")
		return
	}
	var users []model.User
	if err := query.Order("id asc").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		httpx.ServerError(c, "查询用户失败")
		return
	}

	counts := map[uint]int64{}
	if len(users) > 0 {
		ids := make([]uint, 0, len(users))
		for i := range users {
			ids = append(ids, users[i].ID)
		}
		var rows []struct {
			UserID uint
			Total  int64
		}
		if err := h.DB.Model(&model.Submission{}).
			Select("user_id, count(*) as total").Where("user_id IN ?", ids).
			Group("user_id").Scan(&rows).Error; err != nil {
			httpx.ServerError(c, "查询用户失败")
			return
		}
		for _, r := range rows {
			counts[r.UserID] = r.Total
		}
	}

	items := make([]adminUserItem, 0, len(users))
	for i := range users {
		u := &users[i]
		items = append(items, adminUserItem{
			ID:              u.ID,
			Username:        u.Username,
			Nickname:        u.Nickname,
			Phone:           model.PhoneString(u.Phone),
			Role:            u.Role,
			Status:          u.Status,
			School:          u.School,
			Grade:           u.Grade,
			CreatedAt:       u.CreatedAt,
			LastLoginAt:     u.LastLoginAt,
			SubmissionCount: counts[u.ID],
		})
	}
	httpx.OK(c, httpx.List(total, page, pageSize, items))
}

type adminUpdateUserRequest struct {
	Role     *string `json:"role"`
	Status   *int    `json:"status"`
	Password *string `json:"password"`
	Nickname *string `json:"nickname"`
}

// AdminUpdateUser 修改用户的角色、状态、密码或昵称，仅更新请求中出现的字段。
func (h *Handler) AdminUpdateUser(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var u model.User
	if err := h.DB.First(&u, id).Error; err != nil {
		httpx.NotFound(c, "用户不存在")
		return
	}
	var req adminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}

	updates := map[string]any{}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if !model.ValidRole(role) {
			httpx.BadRequest(c, "角色取值非法")
			return
		}
		updates["role"] = role
	}
	if req.Status != nil {
		if *req.Status != model.UserStatusActive && *req.Status != model.UserStatusBanned {
			httpx.BadRequest(c, "状态取值非法")
			return
		}
		updates["status"] = *req.Status
	}
	if req.Nickname != nil {
		nickname := strings.TrimSpace(*req.Nickname)
		if len(nickname) > 32 {
			httpx.BadRequest(c, "昵称长度不能超过 32 个字符")
			return
		}
		updates["nickname"] = nickname
	}
	if req.Password != nil {
		if len(*req.Password) < 6 || len(*req.Password) > 64 {
			httpx.BadRequest(c, "密码长度需在 6 到 64 个字符之间")
			return
		}
		hash, err := h.Auth.HashPassword(*req.Password)
		if err != nil {
			httpx.ServerError(c, "更新用户失败")
			return
		}
		updates["password_hash"] = hash
		updates["auth_version"] = gorm.Expr("auth_version + 1")
	}

	if len(updates) > 0 {
		if err := h.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			httpx.ServerError(c, "更新用户失败")
			return
		}
	}
	var fresh model.User
	if err := h.DB.First(&fresh, u.ID).Error; err != nil {
		httpx.ServerError(c, "更新用户失败")
		return
	}
	httpx.OK(c, gin.H{"user": model.NewPublicUser(&fresh)})
}

// AdminImportUsers 从 CSV 批量导入用户，逐行创建并跳过已存在或不合格的行。
func (h *Handler) AdminImportUsers(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "缺少 CSV 文件")
		return
	}
	src, err := file.Open()
	if err != nil {
		httpx.BadRequest(c, "无法读取 CSV 文件")
		return
	}
	defer src.Close()

	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		httpx.BadRequest(c, "CSV 解析失败")
		return
	}

	imported, skipped := 0, 0
	for i, row := range records {
		if len(row) == 0 {
			skipped++
			continue
		}
		cell := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		username := cell(0)
		if i == 0 && strings.EqualFold(username, "username") {
			continue
		}
		password := cell(1)
		if username == "" || len(password) < 6 {
			skipped++
			continue
		}

		var exists int64
		if err := h.DB.Model(&model.User{}).Where("username = ?", username).Count(&exists).Error; err != nil {
			httpx.ServerError(c, "导入失败")
			return
		}
		if exists > 0 {
			skipped++
			continue
		}
		hash, err := h.Auth.HashPassword(password)
		if err != nil {
			httpx.ServerError(c, "导入失败")
			return
		}
		phone := cell(3)
		if phone != "" && !service.ValidPhone(phone) {
			skipped++
			continue
		}
		if phone != "" {
			var phoneExists int64
			if err := h.DB.Model(&model.User{}).Where("phone = ?", phone).Count(&phoneExists).Error; err != nil {
				httpx.ServerError(c, "导入失败")
				return
			}
			if phoneExists > 0 {
				skipped++
				continue
			}
		}
		role := cell(4)
		if !model.ValidRole(role) {
			role = model.RoleUser
		}
		nickname := cell(2)
		if nickname == "" {
			nickname = username
		}
		u := model.User{
			Username:     username,
			PasswordHash: hash,
			Nickname:     nickname,
			Phone:        model.PhonePointer(phone),
			Role:         role,
			Status:       model.UserStatusActive,
		}
		if err := h.DB.Create(&u).Error; err != nil {
			httpx.ServerError(c, "导入失败")
			return
		}
		imported++
	}
	httpx.OK(c, gin.H{"imported": imported, "skipped": skipped})
}
