package handler

import (
	"os"

	"akioi/internal/httpx"
	"akioi/internal/logger"

	"github.com/gin-gonic/gin"
)

// AdminListBackups 返回备份目录里的备份文件列表。
func (h *Handler) AdminListBackups(c *gin.Context) {
	items, err := h.Backup.List()
	if err != nil {
		httpx.ServerError(c, "读取备份列表失败")
		return
	}
	httpx.OK(c, gin.H{"items": items, "dir": h.Backup.Dir()})
}

// AdminCreateBackup 立即生成一份数据库备份。
func (h *Handler) AdminCreateBackup(c *gin.Context) {
	file, err := h.Backup.Create()
	if err != nil {
		logger.Errorf("生成数据库备份失败: %v", err)
		httpx.ServerError(c, "生成备份失败")
		return
	}
	if u := httpx.CurrentUser(c); u != nil {
		logger.Adminf("管理员 %s 生成数据库备份 %s（%d 字节）ip=%s", u.Username, file.Name, file.Size, c.ClientIP())
	}
	httpx.OK(c, gin.H{"file": file})
}

// AdminDownloadBackup 下载指定备份文件。
func (h *Handler) AdminDownloadBackup(c *gin.Context) {
	name := c.Param("name")
	path, err := h.Backup.Path(name)
	if err != nil {
		httpx.BadRequest(c, err.Error())
		return
	}
	if !fileExists(path) {
		httpx.NotFound(c, "备份文件不存在")
		return
	}
	c.FileAttachment(path, name)
}

// fileExists 判断文件是否存在。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// AdminDeleteBackup 删除指定备份文件。
func (h *Handler) AdminDeleteBackup(c *gin.Context) {
	name := c.Param("name")
	if err := h.Backup.Delete(name); err != nil {
		if err.Error() == "备份文件不存在" {
			httpx.NotFound(c, "备份文件不存在")
			return
		}
		httpx.BadRequest(c, err.Error())
		return
	}
	if u := httpx.CurrentUser(c); u != nil {
		logger.Adminf("管理员 %s 删除数据库备份 %s ip=%s", u.Username, name, c.ClientIP())
	}
	httpx.OK(c, gin.H{"deleted": true})
}
