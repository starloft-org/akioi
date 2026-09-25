import request from './request'

// 站点公开信息
export const getSiteInfo = () => request.get('/api/v1/site')
export const getAnnouncements = () => request.get('/api/v1/announcements')
export const getLanguages = () => request.get('/api/v1/languages')

// 鉴权
export const register = (data) => request.post('/api/v1/auth/register', data)
export const login = (data) => request.post('/api/v1/auth/login', data)
export const logout = () => request.post('/api/v1/auth/logout')
export const getMe = () => request.get('/api/v1/auth/me')
export const getWechatUrl = (redirect) =>
  request.get('/api/v1/auth/wechat/url', { params: { redirect } })
export const wechatCallback = (params) => request.get('/api/v1/auth/wechat/callback', { params })

// 题库
export const listProblems = (params) => request.get('/api/v1/problems', { params })
export const getProblem = (id) => request.get(`/api/v1/problems/${id}`)

// 提交
export const createSubmission = (data) => request.post('/api/v1/submissions', data)
export const listSubmissions = (params) => request.get('/api/v1/submissions', { params })
export const getSubmission = (id) => request.get(`/api/v1/submissions/${id}`)

// 比赛
export const listContests = (params) => request.get('/api/v1/contests', { params })
export const getContest = (id) => request.get(`/api/v1/contests/${id}`)
export const registerContest = (id, data) => request.post(`/api/v1/contests/${id}/register`, data)
export const getContestRank = (id) => request.get(`/api/v1/contests/${id}/rank`)
export const contestRankCsvUrl = (id) => `/api/v1/contests/${id}/rank.csv`
export const listContestSubmissions = (id, params) =>
  request.get(`/api/v1/contests/${id}/submissions`, { params })

// 用户
export const getUserProfile = (id) => request.get(`/api/v1/users/${id}`)
export const updateMe = (data) => request.put('/api/v1/users/me', data)
export const changePassword = (data) => request.put('/api/v1/users/me/password', data)
export const uploadAvatar = (file) => {
  const form = new FormData()
  form.append('file', file)
  return request.post('/api/v1/users/me/avatar', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

// 管理后台
export const adminGetStats = () => request.get('/api/admin/v1/stats')

export const adminListUsers = (params) => request.get('/api/admin/v1/users', { params })
export const adminUpdateUser = (id, data) => request.put(`/api/admin/v1/users/${id}`, data)
export const adminImportUsers = (file) => {
  const form = new FormData()
  form.append('file', file)
  return request.post('/api/admin/v1/users/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export const adminListProblems = (params) => request.get('/api/admin/v1/problems', { params })
export const adminGetProblem = (id) => request.get(`/api/admin/v1/problems/${id}`)
export const adminCreateProblem = (data) => request.post('/api/admin/v1/problems', data)
export const adminUpdateProblem = (id, data) => request.put(`/api/admin/v1/problems/${id}`, data)
export const adminDeleteProblem = (id) => request.delete(`/api/admin/v1/problems/${id}`)

export const adminListTestdata = (id) => request.get(`/api/admin/v1/problems/${id}/testdata`)
export const adminUploadTestdata = (id, files, scores) => {
  const form = new FormData()
  for (const f of files) {
    form.append('files', f)
  }
  if (scores) {
    form.append('scores', JSON.stringify(scores))
  }
  return request.post(`/api/admin/v1/problems/${id}/testdata`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 600000
  })
}
export const adminDeleteTestdata = (id) => request.delete(`/api/admin/v1/problems/${id}/testdata`)
export const adminDownloadTestdata = (id) =>
  request.get(`/api/admin/v1/problems/${id}/testdata/download`, { responseType: 'blob' })

// saveBlob 触发浏览器下载，用于需要携带鉴权头的文件下载。
export const saveBlob = (blob, filename) => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

export const adminListContests = (params) => request.get('/api/admin/v1/contests', { params })
export const adminGetContest = (id) => request.get(`/api/admin/v1/contests/${id}`)
export const adminCreateContest = (data) => request.post('/api/admin/v1/contests', data)
export const adminUpdateContest = (id, data) => request.put(`/api/admin/v1/contests/${id}`, data)
export const adminDeleteContest = (id) => request.delete(`/api/admin/v1/contests/${id}`)
export const adminSetContestProblems = (id, problems) =>
  request.put(`/api/admin/v1/contests/${id}/problems`, { problems })

export const adminListSubmissions = (params) => request.get('/api/admin/v1/submissions', { params })
export const adminRejudgeSubmission = (id) => request.post(`/api/admin/v1/submissions/${id}/rejudge`)
export const adminBatchRejudge = (data) => request.post('/api/admin/v1/rejudge', data)

export const adminListAnnouncements = (params) =>
  request.get('/api/admin/v1/announcements', { params })
export const adminCreateAnnouncement = (data) => request.post('/api/admin/v1/announcements', data)
export const adminUpdateAnnouncement = (id, data) =>
  request.put(`/api/admin/v1/announcements/${id}`, data)
export const adminDeleteAnnouncement = (id) =>
  request.delete(`/api/admin/v1/announcements/${id}`)

export const adminGetLogs = (params) => request.get('/api/admin/v1/logs', { params })
