# 个人资料与外观

用户在「个人资料」（`/profile`）里可以维护昵称、签名、学校、年级，并自定义头像、主题色与个人主页背景。这三项外观设置各自独立，只影响本人，不需要管理员介入。

## 头像

| 项 | 说明 |
| --- | --- |
| 接口 | `POST /api/v1/users/me/avatar`，`multipart/form-data`，字段名 `file` |
| 格式 | png / jpg / jpeg / gif / webp |
| 大小 | 受后台「访问与上传限制 → 头像大小上限」（`max_avatar_mb`，默认 2 MB）限制 |
| 存放 | `UPLOAD_DIR/avatars/<用户 ID>-<时间戳>.<扩展名>`，通过 `/uploads/avatars/...` 访问 |
| 返回值 | `{"avatar": "/uploads/avatars/..."}` |

每次上传都会写新文件，旧文件名按时间戳区分，不会覆盖。

## 主题色

| 项 | 说明 |
| --- | --- |
| 字段 | `users.theme_color`，`#RRGGBB` 或空字符串 |
| 接口 | `PUT /api/v1/users/me`，`{"theme_color": "#409EFF"}` |
| 校验 | 必须是 `#RRGGBB`，其他格式返回「主题色需为 #RRGGBB 格式」 |
| 清除 | 传空字符串，界面表现为「跟随站点主题色」 |
| 生效范围 | 仅本人浏览器。前端把用户主题色写入 `--akioi-primary`，覆盖管理员配置的站点主色；未自定义时沿用站点主色 |

主题色不会通过公开资料接口下发给其他用户，其他人访问该用户主页看到的仍是自己的主题色。

## 个人主页背景

| 项 | 说明 |
| --- | --- |
| 字段 | `users.cover_image`，图片地址或空字符串 |
| 上传 | `POST /api/v1/users/me/cover`，`multipart/form-data`，字段名 `file` |
| 清除 | `DELETE /api/v1/users/me/cover` |
| 格式 | png / jpg / jpeg / webp（不接收 gif，避免动图干扰阅读） |
| 大小 | 与头像共用 `max_avatar_mb` 上限 |
| 存放 | `UPLOAD_DIR/covers/<用户 ID>-<时间戳>.<扩展名>` |
| 展示 | 个人主页（`/users/:id`）顶部横幅，桌面端 180px、窄屏 120px 高，按 `cover` 居中裁切 |

上传后接口返回 `{"cover_image": "/uploads/covers/..."}`，前端直接用于横幅背景；清除后横幅整块隐藏，页面回到默认外观。

## 公开资料接口

`GET /api/v1/users/:id` 返回的 `user` 中含 `avatar` 与 `cover_image`，用于渲染其他人的主页；`theme_color` 只在 `GET /api/v1/users/me` 与登录响应等本人视角出现。

## 常见问题

| 现象 | 原因 |
| --- | --- |
| `头像文件过大` / `背景图文件过大` | 超过 `max_avatar_mb`，可在后台「访问与上传限制」调整 |
| `背景图仅支持 png/jpg/jpeg/webp` | 上传了 gif、heic、svg 等格式，先转成 png/jpg |
| `主题色需为 #RRGGBB 格式` | 提交的不是 6 位十六进制颜色 |
| 换色后页面没变 | 前端在登录信息里缓存主题色，刷新页面即可；后台未被自定义的账户不受影响 |
| 上传成功但主页看不到背景 | 需要查看的是 `/users/:id` 页面，个人资料页只展示预览 |
