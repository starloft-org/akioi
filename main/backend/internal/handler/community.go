package handler

import (
	"strings"
	"time"
	"unicode/utf8"

	"akioi/internal/httpx"
	"akioi/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type solutionItem struct {
	ID        uint      `json:"id"`
	ProblemID uint      `json:"problem_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	CanEdit   bool      `json:"can_edit"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type discussionItem struct {
	ID             uint      `json:"id"`
	ProblemID      uint      `json:"problem_id"`
	ProblemTitle   string    `json:"problem_title,omitempty"`
	ProblemDisplay string    `json:"problem_display_id,omitempty"`
	UserID         uint      `json:"user_id"`
	Username       string    `json:"username"`
	Nickname       string    `json:"nickname"`
	Title          string    `json:"title"`
	Content        string    `json:"content,omitempty"`
	CommentCount   int       `json:"comment_count"`
	CanEdit        bool      `json:"can_edit"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type commentItem struct {
	ID           uint      `json:"id"`
	DiscussionID uint      `json:"discussion_id"`
	UserID       uint      `json:"user_id"`
	Username     string    `json:"username"`
	Nickname     string    `json:"nickname"`
	Content      string    `json:"content"`
	CanEdit      bool      `json:"can_edit"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type postRequest struct {
	ProblemID uint   `json:"problem_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
}

// ListSolutions 返回一道题目的公开题解。
func (h *Handler) ListSolutions(c *gin.Context) {
	problemID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, problemID).Error; err != nil || !h.problemAccessible(c, &p) {
		httpx.NotFound(c, "题目不存在")
		return
	}
	page, pageSize, offset := httpx.Pagination(c)
	query := h.DB.Model(&model.Solution{}).Where("problem_id = ? AND visible = ?", problemID, true)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询题解失败")
		return
	}
	var rows []model.Solution
	if err := query.Order("id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		httpx.ServerError(c, "查询题解失败")
		return
	}
	httpx.OK(c, httpx.List(total, page, pageSize, h.solutionItems(c, rows, false)))
}

func (h *Handler) GetSolution(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var row model.Solution
	if err := h.DB.Where("id = ? AND visible = ?", id, true).First(&row).Error; err != nil {
		httpx.NotFound(c, "题解不存在")
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, row.ProblemID).Error; err != nil || !h.problemAccessible(c, &p) {
		httpx.NotFound(c, "题解不存在")
		return
	}
	items := h.solutionItems(c, []model.Solution{row}, true)
	httpx.OK(c, gin.H{"solution": items[0]})
}

func (h *Handler) CreateSolution(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	problemID, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var p model.Problem
	if err := h.DB.First(&p, problemID).Error; err != nil || !h.problemAccessible(c, &p) {
		httpx.NotFound(c, "题目不存在")
		return
	}
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPost(req.Title, req.Content) {
		httpx.BadRequest(c, "标题需为 1-128 个字符，内容需为 1-50000 个字符")
		return
	}
	row := model.Solution{ProblemID: problemID, UserID: u.ID, Title: strings.TrimSpace(req.Title), Content: strings.TrimSpace(req.Content), Visible: true}
	if err := h.DB.Create(&row).Error; err != nil {
		httpx.ServerError(c, "发布题解失败")
		return
	}
	httpx.OK(c, gin.H{"id": row.ID})
}

func (h *Handler) UpdateSolution(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.Solution
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "题解不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能编辑自己的题解")
		return
	}
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPost(req.Title, req.Content) {
		httpx.BadRequest(c, "标题需为 1-128 个字符，内容需为 1-50000 个字符")
		return
	}
	if err := h.DB.Model(&row).Updates(map[string]any{"title": strings.TrimSpace(req.Title), "content": strings.TrimSpace(req.Content)}).Error; err != nil {
		httpx.ServerError(c, "保存题解失败")
		return
	}
	httpx.OK(c, gin.H{})
}

func (h *Handler) DeleteSolution(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.Solution
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "题解不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能删除自己的题解")
		return
	}
	if err := h.DB.Delete(&row).Error; err != nil {
		httpx.ServerError(c, "删除题解失败")
		return
	}
	httpx.OK(c, gin.H{})
}

// ListDiscussions 返回全站讨论列表，可按 problem_id 筛选。
func (h *Handler) ListDiscussions(c *gin.Context) {
	page, pageSize, offset := httpx.Pagination(c)
	query := h.DB.Model(&model.Discussion{}).Where("visible = ?", true)
	if problemID := httpx.QueryUint(c, "problem_id"); problemID > 0 {
		query = query.Where("problem_id = ?", problemID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		httpx.ServerError(c, "查询讨论失败")
		return
	}
	var rows []model.Discussion
	if err := query.Order("updated_at desc, id desc").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		httpx.ServerError(c, "查询讨论失败")
		return
	}
	httpx.OK(c, httpx.List(total, page, pageSize, h.discussionItems(c, rows, false)))
}

func (h *Handler) GetDiscussion(c *gin.Context) {
	id, ok := httpx.PathID(c, "id")
	if !ok {
		return
	}
	var row model.Discussion
	if err := h.DB.Where("id = ? AND visible = ?", id, true).First(&row).Error; err != nil {
		httpx.NotFound(c, "讨论不存在")
		return
	}
	var comments []model.DiscussionComment
	if err := h.DB.Where("discussion_id = ? AND visible = ?", id, true).Order("id asc").Find(&comments).Error; err != nil {
		httpx.ServerError(c, "查询评论失败")
		return
	}
	httpx.OK(c, gin.H{
		"discussion": h.discussionItems(c, []model.Discussion{row}, true)[0],
		"comments":   h.commentItems(c, comments),
	})
}

func (h *Handler) CreateDiscussion(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPost(req.Title, req.Content) {
		httpx.BadRequest(c, "标题需为 1-128 个字符，内容需为 1-50000 个字符")
		return
	}
	if req.ProblemID > 0 {
		var p model.Problem
		if err := h.DB.First(&p, req.ProblemID).Error; err != nil || !h.problemAccessible(c, &p) {
			httpx.BadRequest(c, "关联题目不存在")
			return
		}
	}
	row := model.Discussion{ProblemID: req.ProblemID, UserID: u.ID, Title: strings.TrimSpace(req.Title), Content: strings.TrimSpace(req.Content), Visible: true}
	if err := h.DB.Create(&row).Error; err != nil {
		httpx.ServerError(c, "发布讨论失败")
		return
	}
	httpx.OK(c, gin.H{"id": row.ID})
}

func (h *Handler) UpdateDiscussion(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.Discussion
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "讨论不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能编辑自己的讨论")
		return
	}
	var req postRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validPost(req.Title, req.Content) {
		httpx.BadRequest(c, "标题需为 1-128 个字符，内容需为 1-50000 个字符")
		return
	}
	if err := h.DB.Model(&row).Updates(map[string]any{"title": strings.TrimSpace(req.Title), "content": strings.TrimSpace(req.Content)}).Error; err != nil {
		httpx.ServerError(c, "保存讨论失败")
		return
	}
	httpx.OK(c, gin.H{})
}

func (h *Handler) DeleteDiscussion(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.Discussion
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "讨论不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能删除自己的讨论")
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("discussion_id = ?", row.ID).Delete(&model.DiscussionComment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
	if err != nil {
		httpx.ServerError(c, "删除讨论失败")
		return
	}
	httpx.OK(c, gin.H{})
}

type commentRequest struct {
	Content string `json:"content"`
}

func (h *Handler) CreateComment(c *gin.Context) {
	u := httpx.MustUser(c)
	discussionID, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var discussion model.Discussion
	if err := h.DB.Where("id = ? AND visible = ?", discussionID, true).First(&discussion).Error; err != nil {
		httpx.NotFound(c, "讨论不存在")
		return
	}
	var req commentRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validContent(req.Content, 5000) {
		httpx.BadRequest(c, "评论需为 1-5000 个字符")
		return
	}
	row := model.DiscussionComment{DiscussionID: discussionID, UserID: u.ID, Content: strings.TrimSpace(req.Content), Visible: true}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&discussion).Updates(map[string]any{
			"comment_count": gorm.Expr("comment_count + 1"), "updated_at": time.Now(),
		}).Error
	})
	if err != nil {
		httpx.ServerError(c, "发布评论失败")
		return
	}
	httpx.OK(c, gin.H{"id": row.ID})
}

func (h *Handler) UpdateComment(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.DiscussionComment
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "评论不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能编辑自己的评论")
		return
	}
	var req commentRequest
	if err := c.ShouldBindJSON(&req); err != nil || !validContent(req.Content, 5000) {
		httpx.BadRequest(c, "评论需为 1-5000 个字符")
		return
	}
	if err := h.DB.Model(&row).Update("content", strings.TrimSpace(req.Content)).Error; err != nil {
		httpx.ServerError(c, "保存评论失败")
		return
	}
	httpx.OK(c, gin.H{})
}

func (h *Handler) DeleteComment(c *gin.Context) {
	u := httpx.MustUser(c)
	id, ok := httpx.PathID(c, "id")
	if u == nil || !ok {
		return
	}
	var row model.DiscussionComment
	if err := h.DB.First(&row, id).Error; err != nil {
		httpx.NotFound(c, "评论不存在")
		return
	}
	if row.UserID != u.ID && !u.IsAdmin() {
		httpx.Forbidden(c, "只能删除自己的评论")
		return
	}
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		return tx.Model(&model.Discussion{}).Where("id = ? AND comment_count > 0", row.DiscussionID).
			UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error
	})
	if err != nil {
		httpx.ServerError(c, "删除评论失败")
		return
	}
	httpx.OK(c, gin.H{})
}

func (h *Handler) solutionItems(c *gin.Context, rows []model.Solution, withContent bool) []solutionItem {
	users := h.communityUsers(solutionUserIDs(rows))
	current := httpx.CurrentUser(c)
	items := make([]solutionItem, 0, len(rows))
	for _, row := range rows {
		u := users[row.UserID]
		item := solutionItem{ID: row.ID, ProblemID: row.ProblemID, UserID: row.UserID, Username: u.Username, Nickname: u.Nickname, Title: row.Title, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		item.CanEdit = current != nil && (current.ID == row.UserID || current.IsAdmin())
		if withContent {
			item.Content = row.Content
		}
		items = append(items, item)
	}
	return items
}

func (h *Handler) discussionItems(c *gin.Context, rows []model.Discussion, withContent bool) []discussionItem {
	userIDs := make([]uint, 0, len(rows))
	problemIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		if row.ProblemID > 0 {
			problemIDs = append(problemIDs, row.ProblemID)
		}
	}
	users := h.communityUsers(userIDs)
	problems := map[uint]model.Problem{}
	if len(problemIDs) > 0 {
		var found []model.Problem
		h.DB.Where("id IN ?", problemIDs).Find(&found)
		for _, p := range found {
			problems[p.ID] = p
		}
	}
	current := httpx.CurrentUser(c)
	items := make([]discussionItem, 0, len(rows))
	for _, row := range rows {
		u, p := users[row.UserID], problems[row.ProblemID]
		item := discussionItem{ID: row.ID, ProblemID: row.ProblemID, ProblemTitle: p.Title, ProblemDisplay: p.DisplayID, UserID: row.UserID, Username: u.Username, Nickname: u.Nickname, Title: row.Title, CommentCount: row.CommentCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		item.CanEdit = current != nil && (current.ID == row.UserID || current.IsAdmin())
		if withContent {
			item.Content = row.Content
		}
		items = append(items, item)
	}
	return items
}

func (h *Handler) commentItems(c *gin.Context, rows []model.DiscussionComment) []commentItem {
	userIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
	}
	users := h.communityUsers(userIDs)
	current := httpx.CurrentUser(c)
	items := make([]commentItem, 0, len(rows))
	for _, row := range rows {
		u := users[row.UserID]
		item := commentItem{ID: row.ID, DiscussionID: row.DiscussionID, UserID: row.UserID, Username: u.Username, Nickname: u.Nickname, Content: row.Content, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
		item.CanEdit = current != nil && (current.ID == row.UserID || current.IsAdmin())
		items = append(items, item)
	}
	return items
}

func (h *Handler) communityUsers(ids []uint) map[uint]model.User {
	users := map[uint]model.User{}
	if len(ids) == 0 {
		return users
	}
	var rows []model.User
	h.DB.Where("id IN ?", ids).Find(&rows)
	for _, row := range rows {
		users[row.ID] = row
	}
	return users
}

func solutionUserIDs(rows []model.Solution) []uint {
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	return ids
}

func validPost(title, content string) bool {
	title = strings.TrimSpace(title)
	return utf8.RuneCountInString(title) >= 1 && utf8.RuneCountInString(title) <= 128 && validContent(content, 50000)
}

func validContent(content string, max int) bool {
	content = strings.TrimSpace(content)
	n := utf8.RuneCountInString(content)
	return n >= 1 && n <= max
}
