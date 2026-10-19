// 站点主题色的默认值，与 styles/main.css 中的 :root 变量保持一致
export const DEFAULT_THEME_COLOR = '#1cd5c7'

const HEX_COLOR_RE = /^#[0-9a-fA-F]{6}$/

// applyTheme 把站点主色写入根节点；衍生色（hover、浅色阶、Element 组件色）
// 全部由 CSS color-mix 从 --akioi-primary 计算，这里只需设置一个变量。
// 非法或缺失的值回退到默认品牌青。
export function applyTheme(color) {
  const value = HEX_COLOR_RE.test(color || '') ? color : DEFAULT_THEME_COLOR
  document.documentElement.style.setProperty('--akioi-primary', value)
}
