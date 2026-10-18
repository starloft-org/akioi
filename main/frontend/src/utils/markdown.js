import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

// html: false 不放行原始 HTML，避免题面里误粘的标签被执行；
// breaks: true 让单个换行也生效，贴合 OJ 题面的书写习惯。
const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  typographer: false
})

// 外链统一新窗口打开，并加 noopener 防止反向跳转。
const defaultLinkOpen =
  md.renderer.rules.link_open ||
  ((tokens, idx, options, env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('target', '_blank')
  tokens[idx].attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpen(tokens, idx, options, env, self)
}

// renderMarkdown 把 Markdown 文本渲染为可安全插入的 HTML。
export const renderMarkdown = (content) => {
  if (!content) {
    return ''
  }
  return DOMPurify.sanitize(md.render(String(content)))
}
