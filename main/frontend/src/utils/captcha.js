import { getCaptchaConfig } from '@/api'

// 前端人机验证封装：动态加载 StarLoft 验证码 JS，取 ticket/randstr 交由服务端校验。
// 平台要求每次动态引入脚本，不能下载到本地缓存。

const JS_FALLBACK = 'https://service.starloft.cn/service/captcha/js'

const configCache = new Map()
let scriptLoading = null

// resetCaptchaCache 用于切换账号或配置变化后重新拉取配置
export function resetCaptchaCache() {
  configCache.clear()
  scriptLoading = null
}

async function loadConfig(scene) {
  if (configCache.has(scene)) {
    return configCache.get(scene)
  }
  const data = await getCaptchaConfig({ scene })
  const cfg = data || {}
  configCache.set(scene, cfg)
  return cfg
}

function loadScript(url) {
  if (window.TencentCaptcha) {
    return Promise.resolve()
  }
  if (scriptLoading) {
    return scriptLoading
  }
  scriptLoading = new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = url
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => {
      scriptLoading = null
      reject(new Error('验证码组件加载失败，请检查网络后重试'))
    }
    document.head.appendChild(script)
  })
  return scriptLoading
}

/**
 * verifyCaptcha 弹出人机验证，返回 { ticket, randstr }。
 * 后端未开启人机验证时返回空票据，调用方可照常提交。
 * 配置拉取失败（后端已开启但验证码平台不可用）、用户主动关闭验证时 reject，
 * 调用方应提示并中止本次操作。
 */
export async function verifyCaptcha(scene = 'account') {
  const cfg = await loadConfig(scene)
  if (!cfg?.enabled || !cfg?.captcha_app_id) {
    return { ticket: '', randstr: '' }
  }
  await loadScript(cfg.web_js_url || JS_FALLBACK)
  if (!window.TencentCaptcha) {
    return { ticket: '', randstr: '' }
  }

  return new Promise((resolve, reject) => {
    let settled = false
    const captcha = new window.TencentCaptcha(cfg.captcha_app_id, (res) => {
      if (settled) {
        return
      }
      settled = true
      if (res && res.ret === 0 && res.ticket) {
        resolve({ ticket: res.ticket, randstr: res.randstr })
      } else if (res?.ret === 2) {
        reject(new Error('已取消人机验证'))
      } else {
        reject(new Error('人机验证未通过，请重试'))
      }
    })
    captcha.show()
  })
}
