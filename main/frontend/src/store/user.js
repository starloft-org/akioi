import { defineStore } from 'pinia'
import * as api from '@/api'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('akioi_token') || '',
    user: null,
    wechatBound: false,
    site: {
      app_name: 'AKIOI',
      register_enabled: true,
      wechat_enabled: false,
      sms_enabled: false
    },
    languages: []
  }),
  getters: {
    isLogin: (state) => !!state.user,
    // 超级管理员与管理员都能进入后台，模块可见性由 hasPermission 决定
    isAdmin: (state) => !!state.user && (state.user.role === 'admin' || state.user.role === 'superadmin'),
    isSuperAdmin: (state) => !!state.user && state.user.role === 'superadmin',
    hasPermission: (state) => (key) => {
      if (!state.user) {
        return false
      }
      if (state.user.role === 'superadmin') {
        return true
      }
      if (state.user.role !== 'admin') {
        return false
      }
      return (state.user.permissions || []).includes(key)
    }
  },
  actions: {
    setToken(token) {
      this.token = token || ''
      if (token) {
        localStorage.setItem('akioi_token', token)
      } else {
        localStorage.removeItem('akioi_token')
      }
    },
    async loadSite() {
      try {
        this.site = await api.getSiteInfo()
      } catch (e) {
        // 站点信息拉取失败时沿用默认值
      }
    },
    async loadMe() {
      if (!this.token) {
        return null
      }
      try {
        const data = await api.getMe()
        this.user = data.user
        this.wechatBound = !!data.wechat_bound
        return this.user
      } catch (e) {
        this.setToken('')
        this.user = null
        this.wechatBound = false
        return null
      }
    },
    async login(payload) {
      const data = await api.login(payload)
      this.setToken(data.token)
      this.user = data.user
      return this.user
    },
    async register(payload) {
      const data = await api.register(payload)
      this.setToken(data.token)
      this.user = data.user
      return this.user
    },
    async logout() {
      try {
        await api.logout()
      } catch (e) {
        // 忽略登出接口异常，本地状态照常清理
      }
      this.setToken('')
      this.user = null
    }
  }
})
