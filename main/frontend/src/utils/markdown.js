import MarkdownIt from 'markdown-it'
import katexPlugin from '@traptitech/markdown-it-katex'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'

// html: false 不放行原始 HTML，避免题面里误粘的标签被执行；
// breaks: true 让单个换行也生效，贴合 OJ 题面的书写习惯。
const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  typographer: false
})

// $...$ 与 $$...$$ 渲染为 KaTeX；公式写错时按原文显示，不抛异常打断整段渲染。
md.use(katexPlugin, { throwOnError: false, errorColor: '#f54a45' })

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
// KaTeX 会输出 MathML 与少量 SVG，因此放开这两个 profile，否则公式会被清洗掉。
export const renderMarkdown = (content) => {
  if (!content) {
    return ''
  }
  return DOMPurify.sanitize(md.render(String(content)), {
    USE_PROFILES: { html: true, mathMl: true, svg: true }
  })
}
