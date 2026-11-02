import request from './request'

// 站点公开信息
export const getSiteInfo = () => request.get('/api/v1/site')
export const getAnnouncements = () => request.get('/api/v1/announcements')
// listTags 返回所有已经在用的题目标签，新建 / 编辑题目时用作候选项
export const listTags = () => request.get('/api/v1/tags')

export const getLanguages = () => request.get('/api/v1/languages')

// 鉴权
export const register = (data) => request.post('/api/v1/auth/register', data)
export const login = (data) => request.post('/api/v1/auth/login', data)
export const sendSmsCode = (data) => request.post('/api/v1/auth/sms/send', data)
export const sendEmailCode = (data) => request.post('/api/v1/auth/email/send', data)
export const getCaptchaConfig = (params) => request.get('/api/v1/site/captcha', { params })
export const resetPasswordBySms = (data) => request.post('/api/v1/auth/password/reset', data)
export const logout = () => request.post('/api/v1/auth/logout')
export const getMe = () => request.get('/api/v1/auth/me')
export const getWechatUrl = (params) =>
  request.get('/api/v1/auth/wechat/url', {
    params: typeof params === 'string' ? { redirect: params } : params
  })
export const wechatCallback = (params) => request.get('/api/v1/auth/wechat/callback', { params })
export const getWechatStatus = (state) => request.get('/api/v1/auth/wechat/status', { params: { state } })
export const unbindWechat = () => request.delete('/api/v1/auth/wechat/binding')

// 题库
export const listProblems = (params) => request.get('/api/v1/problems', { params })
export const getProblem = (id) => request.get(`/api/v1/problems/${id}`)

// 全站积分榜
export const listRankings = (params) => request.get('/api/v1/rankings', { params })

// 题解
export const listSolutions = (problemId, params) =>
  request.get(`/api/v1/problems/${problemId}/solutions`, { params })
export const getSolution = (id) => request.get(`/api/v1/solutions/${id}`)
export const createSolution = (problemId, data) =>
  request.post(`/api/v1/problems/${problemId}/solutions`, data)
export const updateSolution = (id, data) => request.put(`/api/v1/solutions/${id}`, data)
export const deleteSolution = (id) => request.delete(`/api/v1/solutions/${id}`)

// 讨论与评论
export const listDiscussions = (params) => request.get('/api/v1/discussions', { params })
export const getDiscussion = (id) => request.get(`/api/v1/discussions/${id}`)
export const createDiscussion = (data) => request.post('/api/v1/discussions', data)
export const updateDiscussion = (id, data) => request.put(`/api/v1/discussions/${id}`, data)
export const deleteDiscussion = (id) => request.delete(`/api/v1/discussions/${id}`)
export const createComment = (id, data) => request.post(`/api/v1/discussions/${id}/comments`, data)
export const updateComment = (id, data) => request.put(`/api/v1/comments/${id}`, data)
export const deleteComment = (id) => request.delete(`/api/v1/comments/${id}`)

// 提交
export const createSubmission = (data) => request.post('/api/v1/submissions', data)
export const listSubmissions = (params) => request.get('/api/v1/submissions', { params })
export const getSubmission = (id) => request.get(`/api/v1/submissions/${id}`)
export const getSubmissionStatus = (id) => request.get(`/api/v1/submissions/${id}/status`)

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
// 个人主页背景图，与头像共用上传大小限制
export const uploadCover = (file) => {
  const form = new FormData()
  form.append('file', file)
  return request.post('/api/v1/users/me/cover', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}
export const deleteCover = () => request.delete('/api/v1/users/me/cover')

// Markdown 编辑器上传正文图片（题面、公告），返回 { url, size }
export const uploadImage = (file) => {
  const form = new FormData()
  form.append('file', file)
  return request.post('/api/v1/uploads/images', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 60000
  })
}

// 管理后台
export const adminGetStats = () => request.get('/api/admin/v1/stats')
export const adminGetSettings = () => request.get('/api/admin/v1/settings')
export const adminUpdateSettings = (data) => request.put('/api/admin/v1/settings', data)

export const adminListUsers = (params) => request.get('/api/admin/v1/users', { params })
export const adminUpdateUser = (id, data) => request.put(`/api/admin/v1/users/${id}`, data)
export const adminImportUsers = (file) => {
  const form = new FormData()
  form.append('file', file)
  return request.post('/api/admin/v1/users/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export const adminListAdmins = () => request.get('/api/admin/v1/admins')
export const adminCreateAdmin = (data) => request.post('/api/admin/v1/admins', data)
export const adminUpdateAdmin = (id, data) => request.put(`/api/admin/v1/admins/${id}`, data)
export const adminDeleteAdmin = (id) => request.delete(`/api/admin/v1/admins/${id}`)

export const adminListProblems = (params) => request.get('/api/admin/v1/problems', { params })
export const adminMoveProblemsBank = (data) => request.post('/api/admin/v1/problems/move-bank', data)
export const adminGetProblem = (id) => request.get(`/api/admin/v1/problems/${id}`)
export const adminCreateProblem = (data) => request.post('/api/admin/v1/problems', data)

// 题库：题号由题库前缀与题号长度决定，题目归属某个题库
export const adminListProblemBanks = () => request.get('/api/admin/v1/problem-banks')
export const adminCreateProblemBank = (data) => request.post('/api/admin/v1/problem-banks', data)
export const adminUpdateProblemBank = (id, data) => request.put(`/api/admin/v1/problem-banks/${id}`, data)
export const adminDeleteProblemBank = (id) => request.delete(`/api/admin/v1/problem-banks/${id}`)

// 批量导入在后台执行，上传完返回 task_id，用这个接口轮询进度
export const adminGetImportTask = (taskId) => request.get(`/api/admin/v1/import-tasks/${taskId}`)

// 单个题目包上传：服务端立刻返回 task_id，由前端轮询进度，避免大包请求超时
export const adminImportProblemOne = (file, bankId, onUploadProgress, visibility) => {
  const form = new FormData()
  form.append('files', file)
  if (bankId) {
    form.append('bank_id', bankId)
  if (visibility) {
    form.append('visibility', visibility)
  }
  }
  return request.post('/api/admin/v1/problems/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 600000,
    onUploadProgress: (event) => {
      if (!onUploadProgress) {
        return
      }
      const total = event?.total || 0
      if (total > 0) {
        onUploadProgress(Math.min(100, Math.round((event.loaded / total) * 100)))
      }
    }
  })
}

// 个人题目批量导入：上传完立刻返回 task_id，再用 myGetImportTask 轮询进度
export const myGetImportTask = (taskId) => request.get(`/api/v1/my/import-tasks/${taskId}`)

export const myImportProblemsOne = (file, onUploadProgress) => {
  const form = new FormData()
  form.append('files', file)
  return request.post('/api/v1/my/problems/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 600000,
    onUploadProgress: (event) => {
      if (!onUploadProgress) {
        return
      }
      const total = event?.total || 0
      if (total > 0) {
        onUploadProgress(Math.min(100, Math.round((event.loaded / total) * 100)))
      }
    }
  })
}

export const adminImportProblem = (files, bankId, onProgress) => {
  const form = new FormData()
  // 导入的题目统一归入所选题库，未选择时后端归入主库
  if (bankId) {
    form.append('bank_id', String(bankId))
  }
  // files 为数组，支持一次导入多个题目包 zip
  for (const file of files) {
    form.append('files', file)
  }
  return request.post('/api/admin/v1/problems/import', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    // 只包含上传耗时，导入过程在后台执行并由任务接口轮询
    timeout: 600000,
    // 上传进度按已发送字节数回调，onProgress 收到 0-100 的整数
    onUploadProgress: (event) => {
      if (typeof onProgress !== 'function' || !event.total) {
        return
      }
      onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
    }
  })
}
export const adminUpdateProblem = (id, data) => request.put(`/api/admin/v1/problems/${id}`, data)
export const adminDeleteProblem = (id) => request.delete(`/api/admin/v1/problems/${id}`)

export const adminListTestdata = (id) => request.get(`/api/admin/v1/problems/${id}/testdata`)
export const adminSyncTestdata = (id) => request.post(`/api/admin/v1/problems/${id}/testdata/sync`)
export const adminJudgeNodesStatus = () => request.get('/api/admin/v1/judges/status')
export const adminJudgeSyncAll = () => request.post('/api/admin/v1/judges/sync-all', null, { timeout: 600000 })
export const adminUploadTestdata = (id, files, scores, subtasks) => {
  const form = new FormData()
  for (const f of files) {
    form.append('files', f)
  }
  if (scores) {
    form.append('scores', JSON.stringify(scores))
  }
  if (subtasks) {
    form.append('subtasks', subtasks)
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

// 数据库备份：文件由后台写入服务器 BACKUP_DIR，下载走携带鉴权头的 blob 请求
export const adminListBackups = () => request.get('/api/admin/v1/backups')
export const adminCreateBackup = () => request.post('/api/admin/v1/backups')
export const adminDownloadBackup = (name) =>
  request.get(`/api/admin/v1/backups/${encodeURIComponent(name)}`, { responseType: 'blob', timeout: 600000 })
export const adminDeleteBackup = (name) =>
  request.delete(`/api/admin/v1/backups/${encodeURIComponent(name)}`)

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

export const adminListSolutions = (params) =>
  request.get('/api/admin/v1/solutions', { params })
export const adminReviewSolution = (id, data) =>
  request.post(`/api/admin/v1/solutions/${id}/review`, data)
export const adminUpdateAnnouncement = (id, data) =>
  request.put(`/api/admin/v1/announcements/${id}`, data)
export const adminDeleteAnnouncement = (id) =>
  request.delete(`/api/admin/v1/announcements/${id}`)

export const adminGetLogs = (params) => request.get('/api/admin/v1/logs', { params })

// 团队
export const listTeams = (params) => request.get('/api/v1/teams', { params })
export const getTeam = (id) => request.get(`/api/v1/teams/${id}`)
export const createTeam = (data) => request.post('/api/v1/teams', data)
export const updateTeam = (id, data) => request.put(`/api/v1/teams/${id}`, data)
export const listTeamMembers = (id) => request.get(`/api/v1/teams/${id}/members`)
export const removeTeamMember = (id, uid) => request.delete(`/api/v1/teams/${id}/members/${uid}`)
export const applyTeam = (id, data) => request.post(`/api/v1/teams/${id}/applications`, data)
export const listTeamApplications = (id, params) =>
  request.get(`/api/v1/teams/${id}/applications`, { params })
export const reviewTeamApplication = (id, aid, data) =>
  request.post(`/api/v1/teams/${id}/applications/${aid}/review`, data)
export const listTeamProblems = (id) => request.get(`/api/v1/teams/${id}/problems`)
export const listTeamContests = (id) => request.get(`/api/v1/teams/${id}/contests`)
export const listTeamSets = (id) => request.get(`/api/v1/teams/${id}/sets`)
export const listMyTeams = () => request.get('/api/v1/my/teams')
export const convertMyProblems = (data) => request.post('/api/v1/my/problems/convert', data)
export const getTeamSet = (id, sid) => request.get(`/api/v1/teams/${id}/sets/${sid}`)
export const createTeamSet = (id, data) => request.post(`/api/v1/teams/${id}/sets`, data)
export const updateTeamSet = (id, sid, data) => request.put(`/api/v1/teams/${id}/sets/${sid}`, data)
export const deleteTeamSet = (id, sid) => request.delete(`/api/v1/teams/${id}/sets/${sid}`)

// 我的题目（团队题目 / 个人题目）与题单
export const listMyProblems = (params) => request.get('/api/v1/my/problems', { params })
export const getMyProblem = (id) => request.get(`/api/v1/my/problems/${id}`)
export const createMyProblem = (data) => request.post('/api/v1/my/problems', data)
export const updateMyProblem = (id, data) => request.put(`/api/v1/my/problems/${id}`, data)
export const deleteMyProblem = (id) => request.delete(`/api/v1/my/problems/${id}`)
export const listMyProblemTestdata = (id) => request.get(`/api/v1/my/problems/${id}/testdata`)
export const uploadMyProblemTestdata = (id, files, scores, subtasks) => {
  const form = new FormData()
  for (const f of files) {
    form.append('files', f)
  }
  if (scores) {
    form.append('scores', JSON.stringify(scores))
  }
  if (subtasks) {
    form.append('subtasks', subtasks)
  }
  return request.post(`/api/v1/my/problems/${id}/testdata`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 600000
  })
}
export const deleteMyProblemTestdata = (id) => request.delete(`/api/v1/my/problems/${id}/testdata`)
export const listMyProblemSets = () => request.get('/api/v1/my/problem-sets')
export const createMyProblemSet = (data) => request.post('/api/v1/my/problem-sets', data)
export const getMyProblemSet = (id) => request.get(`/api/v1/my/problem-sets/${id}`)
export const updateMyProblemSet = (id, data) => request.put(`/api/v1/my/problem-sets/${id}`, data)
export const deleteMyProblemSet = (id) => request.delete(`/api/v1/my/problem-sets/${id}`)
