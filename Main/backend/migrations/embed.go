// Package migrations 以 embed 方式内置版本化 SQL 迁移文件，
// 供 internal/migrate 在服务启动时自动执行未应用的版本。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
