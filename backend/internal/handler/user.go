package handler

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

type userProfile struct {
	User              model.PublicUser `json:"user"`
	SolvedCount       int64            `json:"solved_count"`
	SubmissionCount   int64            `json:"submission_count"`
	AcceptedCount     int64            `json:"accepted_count"`
	RecentSubmissions []SubmissionItem `json:"recent_submissions"`
}

// GetUserProfile 返回用户公开资料与做题统计。
func (h *Handler) GetUserProfile(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var u model.User
	if err := h.DB.First(&u, id).Error; err != nil {
		httpx.NotFound(c, "用户不存在")
		return
	}

	var solved, total, accepted int64
	h.DB.Model(&model.Submission{}).Where("user_id = ?", u.ID).Count(&total)
	h.DB.Model(&model.Submission{}).Where("user_id = ? AND status = ?", u.ID, model.StatusAccepted).Count(&accepted)
	h.DB.Model(&model.Submission{}).Where("user_id = ? AND status = ?", u.ID, model.StatusAccepted).
		Distinct("problem_id").Count(&solved)

	var recent []model.Submission
	h.DB.Where("user_id = ?", u.ID).Order("id desc").Limit(10).Find(&recent)

	httpx.OK(c, userProfile{
		User:              model.NewPublicUser(&u),
		SolvedCount:       solved,
		SubmissionCount:   total,
		AcceptedCount:     accepted,
		RecentSubmissions: h.decorateSubmissions(recent),
	})
}

type updateProfileRequest struct {
	Nickname  *string `json:"nickname"`
	Signature *string `json:"signature"`
	School    *string `json:"school"`
	Grade     *string `json:"grade"`
	Email     *string `json:"email"`
}

func (h *Handler) UpdateMe(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	updates := map[string]any{}
	if req.Nickname != nil {
		v := strings.TrimSpace(*req.Nickname)
		if len(v) > 32 {
			httpx.BadRequest(c, "昵称长度不能超过 32 个字符")
			return
		}
		updates["nickname"] = v
	}
	if req.Signature != nil {
		v := strings.TrimSpace(*req.Signature)
		if len(v) > 255 {
			httpx.BadRequest(c, "签名长度不能超过 255 个字符")
			return
		}
		updates["signature"] = v
	}
	if req.School != nil {
		updates["school"] = strings.TrimSpace(*req.School)
	}
	if req.Grade != nil {
		updates["grade"] = strings.TrimSpace(*req.Grade)
	}
	if req.Email != nil {
		updates["email"] = strings.TrimSpace(*req.Email)
	}
	if len(updates) == 0 {
		httpx.OK(c, gin.H{"user": model.NewPublicUser(u)})
		return
	}
	if err := h.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		httpx.ServerError(c, "保存失败")
		return
	}
	var fresh model.User
	if err := h.DB.First(&fresh, u.ID).Error; err != nil {
		httpx.ServerError(c, "保存失败")
		return
	}
	httpx.OK(c, gin.H{"user": model.NewPublicUser(&fresh)})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "请求参数格式错误")
		return
	}
	if !h.Auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		httpx.BadRequest(c, "原密码不正确")
		return
	}
	if len(req.NewPassword) < 6 || len(req.NewPassword) > 64 {
		httpx.BadRequest(c, "新密码长度需在 6 到 64 个字符之间")
		return
	}
	hash, err := h.Auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.ServerError(c, "修改密码失败")
		return
	}
	if err := h.DB.Model(&model.User{}).Where("id = ?", u.ID).UpdateColumn("password_hash", hash).Error; err != nil {
		httpx.ServerError(c, "修改密码失败")
		return
	}
	logger.Businessf("用户 %s 修改密码 ip=%s", u.Username, c.ClientIP())
	httpx.OK(c, gin.H{})
}

var allowedAvatarExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// UploadAvatar 保存头像到本地静态目录并更新用户资料。
func (h *Handler) UploadAvatar(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "缺少头像文件")
		return
	}
	maxBytes := int64(h.Cfg.MaxAvatarMB) * 1024 * 1024
	if file.Size > maxBytes {
		httpx.BadRequest(c, "头像文件过大")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAvatarExt[ext] {
		httpx.BadRequest(c, "头像仅支持 png/jpg/jpeg/gif/webp")
		return
	}

	dir := filepath.Join(h.Cfg.UploadDir, "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpx.ServerError(c, "头像保存失败")
		return
	}
	name := strconv.FormatUint(uint64(u.ID), 10) + "-" + strconv.FormatInt(time.Now().Unix(), 10) + ext
	if err := c.SaveUploadedFile(file, filepath.Join(dir, name)); err != nil {
		httpx.ServerError(c, "头像保存失败")
		return
	}
	url := "/uploads/avatars/" + name
	if err := h.DB.Model(&model.User{}).Where("id = ?", u.ID).UpdateColumn("avatar", url).Error; err != nil {
		httpx.ServerError(c, "头像保存失败")
		return
	}
	httpx.OK(c, gin.H{"avatar": url})
}

// decorateSubmissions 为提交附加用户名与题目信息。
func (h *Handler) decorateSubmissions(subs []model.Submission) []SubmissionItem {
	items := make([]SubmissionItem, 0, len(subs))
	if len(subs) == 0 {
		return items
	}
	userIDs := make([]uint, 0, len(subs))
	problemIDs := make([]uint, 0, len(subs))
	for i := range subs {
		userIDs = append(userIDs, subs[i].UserID)
		problemIDs = append(problemIDs, subs[i].ProblemID)
	}
	users := map[uint]model.User{}
	var rows []model.User
	h.DB.Where("id IN ?", userIDs).Find(&rows)
	for i := range rows {
		users[rows[i].ID] = rows[i]
	}
	problems := map[uint]model.Problem{}
	var probs []model.Problem
	h.DB.Where("id IN ?", problemIDs).Find(&probs)
	for i := range probs {
		problems[probs[i].ID] = probs[i]
	}
	for i := range subs {
		item := SubmissionItem{
			ID:         subs[i].ID,
			UserID:     subs[i].UserID,
			ProblemID:  subs[i].ProblemID,
			ContestID:  subs[i].ContestID,
			Language:   subs[i].Language,
			Status:     subs[i].Status,
			Score:      subs[i].Score,
			TimeMS:     subs[i].TimeMS,
			MemoryKB:   subs[i].MemoryKB,
			CodeLength: subs[i].CodeLength,
			CreatedAt:  subs[i].CreatedAt,
			JudgedAt:   subs[i].JudgedAt,
		}
		if u, ok := users[subs[i].UserID]; ok {
			item.Username = u.Username
			item.Nickname = u.Nickname
		}
		if p, ok := problems[subs[i].ProblemID]; ok {
			item.ProblemDisplayID = p.DisplayID
			item.ProblemTitle = p.Title
		}
		items = append(items, item)
	}
	return items
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
