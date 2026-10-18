import axios from 'axios'
import { ElMessage } from 'element-plus'

const request = axios.create({
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('akioi_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 登录、注册等鉴权接口返回 401 属于业务校验失败（账号或密码错误），
// 不能按“登录态失效”处理，否则会被静默跳转、用户看不到任何提示。
const AUTH_PATHS = ['/auth/login', '/auth/register', '/auth/sms/send', '/auth/password/reset']

const isAuthRequest = (url = '') => AUTH_PATHS.some((path) => url.includes(path))

// 统一解包 {code, message, data}；失败时提示一次并抛出错误。
request.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) {
        return body.data
      }
      ElMessage.error(body.message || '请求失败')
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    return body
  },
  (error) => {
    const status = error.response?.status
    const body = error.response?.data
    const message = body?.message || error.message || '网络异常'
    const authRequest = isAuthRequest(error.config?.url)

    if (status === 401 && !authRequest) {
      localStorage.removeItem('akioi_token')
      const current = window.location.pathname + window.location.search
      if (!current.startsWith('/login')) {
        window.location.href = `/login?redirect=${encodeURIComponent(current)}`
      }
      return Promise.reject(error)
    }

    ElMessage.error(message)
    return Promise.reject(error)
  }
)

export default request
