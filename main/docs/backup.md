# 数据库备份

后台「数据备份」页面可以一键导出整库、下载备份文件、删除旧备份。备份由后端用纯 Go 实现，不依赖容器里的 `mysqldump`。

## 一、权限与入口

- 入口：管理后台 → 系统 → 数据备份（`/admin/backups`）。
- 权限项：`backups`（Label「数据备份」，分组「系统」）。超级管理员默认拥有，普通管理员需要在「管理员管理」里勾选。
- 接口（前缀 `/api/admin/v1/backups`）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/admin/v1/backups` | 备份文件列表，返回 `items[{name,size,created_at}]` 与 `dir` |
| POST | `/api/admin/v1/backups` | 立即生成一份备份 |
| GET | `/api/admin/v1/backups/:name` | 下载指定备份 |
| DELETE | `/api/admin/v1/backups/:name` | 删除指定备份 |

## 二、备份内容

- 文件：`BACKUP_DIR` 下的 `akioi-YYYYmmdd-HHMMSS.sql.gz`（UTC+8 本地时间命名，gzip 压缩的 SQL 文本）。
- 内容：整库所有表的结构（`SHOW CREATE TABLE`）与数据（分批 `INSERT`）。
- 含软删除记录与题目测试点清单，因此恢复后已删除的内容也能回来；测试数据文件本体在 `data/testdata`，不在 SQL 备份里，需要单独备份。
- 二进制列（`BLOB` 系列、`BINARY`/`VARBINARY`、`BIT`）按 `0x…` 十六进制写出，避免字符集转换；其他文本按 MySQL 转义规则（`\\`、`\'`、`\n`、`\r`、`\0`、`\Z`）写出。
- 备份开头包含 `SET NAMES utf8mb4;` 与 `FOREIGN_KEY_CHECKS=0`，结尾恢复原值，因此可以直接导入空库。

## 三、目录与保留

- 目录由 `BACKUP_DIR` 指定，默认 `/app/backups`，compose 里挂载到 `./data/backups`，容器重建不会丢失。
- 备份不会自动清理，需要定期在页面上删除旧文件；也可以写宿主机 cron 清理，例如保留最近 7 份：

```bash
ls -1t /opt/akioi/main/data/backups/akioi-*.sql.gz | tail -n +8 | xargs -r rm -f
```

## 四、恢复

```bash
# 1. 解压并导入（会覆盖同名表，建议先导到一个临时库验证）
gunzip -c akioi-20240101-120000.sql.gz | mysql -h 127.0.0.1 -P 3306 -u root -p akioi

# 2. 如果导入到别的库，先建库并指定库名
mysql -h 127.0.0.1 -u root -p -e "CREATE DATABASE akioi_restore CHARACTER SET utf8mb4"
gunzip -c akioi-20240101-120000.sql.gz | mysql -h 127.0.0.1 -u root -p akioi_restore
```

在 docker 部署里，宿主机没有 mysql 客户端时可以进容器执行：

```bash
docker compose -f main/docker-compose.yml exec -T mysql \
  sh -c 'gunzip -c /tmp/xxx.sql.gz | mysql -uroot -p"$MYSQL_ROOT_PASSWORD" akioi'
```

## 五、注意事项

- 备份文件里包含 `system_settings` 中的密钥行（登录密钥、微信、短信、邮件、验证码、评测同步密钥），下载后请妥善保管，不要提交到仓库或公开分享。
- 生成备份会读取整库，数据量大时耗时较长；生成期间页面会一直等待，请不要重复点击。
- 备份文件名固定为时间戳格式，同一秒内重复生成会覆盖同名文件，页面上以最新一份为准。
- 恢复后如果登录密钥与恢复前不同，所有用户需要重新登录。
