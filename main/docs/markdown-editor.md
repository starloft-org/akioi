# Markdown 编辑器与正文图片

题面、输入/输出格式、提示、公告正文都用同一套 Markdown + LaTeX 渲染，编辑端提供带工具栏的可视化编辑器。

## 渲染规则

- 渲染实现：`main/frontend/src/utils/markdown.js`（`markdown-it` + `@traptitech/markdown-it-katex` + `DOMPurify`）。
- 原始 HTML 不渲染（`html: false`），避免正文里误粘的标签被执行；单换行生效（`breaks: true`）。
- 公式：`$...$` 行内、`$$...$$` 独立成块，错误公式按原文标红显示，不会整段渲染失败。
- 外链统一新窗口打开，并加 `rel="noopener noreferrer"`。
- 显示位置：题目详情（描述/输入格式/输出格式/提示）、首页公告详情。

## 编辑器

组件：`main/frontend/src/components/MarkdownEditor.vue`，用于

- 后台「题目管理 → 新建/编辑题目」（描述、输入格式、输出格式、提示）
- 后台「公告管理 → 新建/编辑公告」（内容）
- 「我的题目」新建/编辑弹窗（描述、输入格式、输出格式、提示）

功能：

| 操作 | 说明 |
| --- | --- |
| 编辑 / 分栏 / 预览 | 右上角切换；分栏左边写、右边实时预览，窄屏自动上下排布 |
| 加粗、斜体、行内代码、引用、列表、链接 | 工具栏按钮，快捷键 `Ctrl+B` / `Ctrl+I` / `Ctrl+K` |
| 公式 | 「公式」插入 `$...$`，「∑ 块」插入 `$$...$$` |
| 图片 | 点「图片」选文件上传，或直接在编辑器里 **粘贴截图**，都会插入 `![名字](/uploads/images/...)` |

## 图片上传接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/v1/uploads/images` | `multipart/form-data`，字段 `file`；需要登录；返回 `{ "url": "/uploads/images/202611/<随机名>.png", "size": 12345 }` |

- 大小上限 **5MB**；类型只收 png / jpg / jpeg / gif / webp，并按文件头校验确实是图片。
- 落盘目录 `uploads/images/YYYYMM/`，文件名 `用户ID-时间戳36进制-随机后缀.ext`，同名不会覆盖。
- 返回的 `url` 是站内相对路径，静态由后端 `/uploads` 直接提供（前端 nginx 反代 `/uploads/`），无需登录即可显示。
- 上传动作会写业务日志：`用户 xx 上传正文图片 ...`。
