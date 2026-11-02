package handler

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"akioi/internal/httpx"
	"akioi/internal/logger"

	"github.com/gin-gonic/gin"
)

// 题目题面、公告正文里插入的图片：5MB 以内，只收常见位图。
const maxContentImageBytes = 5 << 20

var allowedContentImageExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

// UploadImage 保存 Markdown 编辑器里插入的图片，返回可直接写进正文的地址。
// 登录用户都可以上传（自己要写题面、公告正文），文件按月份归档，名字随机避免覆盖。
func (h *Handler) UploadImage(c *gin.Context) {
	u := httpx.MustUser(c)
	if u == nil {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		httpx.BadRequest(c, "缺少图片文件")
		return
	}
	if file.Size <= 0 {
		httpx.BadRequest(c, "图片文件为空")
		return
	}
	if file.Size > maxContentImageBytes {
		httpx.BadRequest(c, "图片不能超过 5MB")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedContentImageExt[ext] {
		httpx.BadRequest(c, "图片仅支持 png/jpg/jpeg/gif/webp")
		return
	}

	src, err := file.Open()
	if err != nil {
		httpx.ServerError(c, "图片保存失败")
		return
	}
	head := make([]byte, 512)
	n, readErr := io.ReadFull(src, head)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		src.Close()
		httpx.ServerError(c, "图片保存失败")
		return
	}
	src.Close()
	if !strings.HasPrefix(http.DetectContentType(head[:n]), "image/") {
		httpx.BadRequest(c, "这个文件看起来不是图片")
		return
	}

	dir := filepath.Join(h.Cfg.UploadDir, "images", time.Now().Format("200601"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpx.ServerError(c, "图片保存失败")
		return
	}
	name := strconv.FormatUint(uint64(u.ID), 10) + "-" +
		strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + randomHex(4) + ext
	if err := c.SaveUploadedFile(file, filepath.Join(dir, name)); err != nil {
		httpx.ServerError(c, "图片保存失败")
		return
	}
	url := "/uploads/images/" + time.Now().Format("200601") + "/" + name
	logger.Businessf("用户 %s 上传正文图片 %s（%d 字节） ip=%s", u.Username, url, file.Size, c.ClientIP())
	httpx.OK(c, gin.H{"url": url, "size": file.Size})
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(buf)
}
