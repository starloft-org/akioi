// Package httpx 统一 HTTP 响应格式、登录态上下文与请求参数解析，
// 供 handler 与 middleware 共用，避免二者互相依赖。
package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"akioi/internal/model"

	"github.com/gin-gonic/gin"
)

// 业务响应码，code 为 0 表示成功。
const (
	CodeOK             = 0
	CodeBadRequest     = 40001
	CodeUnauthorized   = 40100
	CodeForbidden      = 40300
	CodeNotFound       = 40400
	CodeConflict       = 40900
	CodeTooManyRequest = 42900
	CodeSubmitTooFast  = 42901
	CodeServerError    = 50000
)

// ContextUserKey 是登录用户在 gin.Context 中的键名。
const ContextUserKey = "akioi_user"

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// OK 返回成功响应，HTTP 状态码固定 200。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: CodeOK, Message: "ok", Data: data})
}

// Fail 返回失败响应，HTTP 状态码与业务语义对齐。
func Fail(c *gin.Context, status, code int, message string) {
	c.JSON(status, Response{Code: code, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, CodeBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, CodeNotFound, message)
}

func Conflict(c *gin.Context, message string) {
	Fail(c, http.StatusConflict, CodeConflict, message)
}

func TooMany(c *gin.Context, code int, message string) {
	Fail(c, http.StatusTooManyRequests, code, message)
}

func ServerError(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, CodeServerError, message)
}

// SetUser 写入当前登录用户。
func SetUser(c *gin.Context, u *model.User) {
	c.Set(ContextUserKey, u)
}

// CurrentUser 返回当前登录用户，未登录时为 nil。
func CurrentUser(c *gin.Context) *model.User {
	v, ok := c.Get(ContextUserKey)
	if !ok {
		return nil
	}
	u, ok := v.(*model.User)
	if !ok {
		return nil
	}
	return u
}

// MustUser 返回当前登录用户；缺失时写出 401 并返回 nil。
func MustUser(c *gin.Context) *model.User {
	u := CurrentUser(c)
	if u == nil {
		Unauthorized(c, "请先登录")
		return nil
	}
	return u
}

// PathID 解析路径中的无符号整型参数。
func PathID(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || v == 0 {
		BadRequest(c, "路径参数 "+name+" 非法")
		return 0, false
	}
	return uint(v), true
}

// QueryInt 读取整型查询参数，缺失或非法时返回默认值。
func QueryInt(c *gin.Context, name string, def int) int {
	v := strings.TrimSpace(c.Query(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// QueryUint 读取无符号整型查询参数，缺失或非法时返回 0。
func QueryUint(c *gin.Context, name string) uint {
	v := strings.TrimSpace(c.Query(name))
	if v == "" {
		return 0
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0
	}
	return uint(n)
}

// Pagination 统一分页参数，page 从 1 开始，page_size 限制在 1..100。
func Pagination(c *gin.Context) (page, pageSize, offset int) {
	page = QueryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize = QueryInt(c, "page_size", 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset = (page - 1) * pageSize
	return
}

type ListData struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Items    any   `json:"items"`
}

func List(total int64, page, pageSize int, items any) ListData {
	if items == nil {
		items = []any{}
	}
	return ListData{Total: total, Page: page, PageSize: pageSize, Items: items}
}
