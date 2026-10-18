// 判题状态与文案，与后端 model 常量保持一致。
export const JUDGE_STATUS = {
  Pending: { text: '等待评测', type: 'info' },
  Judging: { text: '评测中', type: 'warning' },
  Accepted: { text: '答案正确', type: 'success' },
  WrongAnswer: { text: '答案错误', type: 'danger' },
  TimeLimitExceeded: { text: '超出时间限制', type: 'danger' },
  MemoryLimitExceeded: { text: '超出内存限制', type: 'danger' },
  RuntimeError: { text: '运行时错误', type: 'danger' },
  CompileError: { text: '编译错误', type: 'danger' },
  SystemError: { text: '系统错误', type: 'info' },
  OutputLimitExceeded: { text: '输出超限', type: 'danger' }
}

export const statusText = (status) => JUDGE_STATUS[status]?.text || status || '-'

export const statusType = (status) => JUDGE_STATUS[status]?.type || 'info'

export const LANGUAGES = [
  { key: 'cpp', name: 'C++' },
  { key: 'c', name: 'C' }
]

export const languageName = (key) =>
  LANGUAGES.find((item) => item.key === key)?.name || key || '-'

export const DIFFICULTIES = ['入门', '简单', '中等', '困难', '挑战']

export const COMPARE_MODES = [
  { value: 'default', label: '忽略行尾空格与末尾空行' },
  { value: 'exact', label: '严格比对' },
  { value: 'float', label: '浮点数误差比对' }
]

export const contestStatusText = (status) =>
  ({ pending: '未开始', running: '进行中', ended: '已结束' }[status] || status)

export const contestStatusType = (status) =>
  ({ pending: 'info', running: 'success', ended: 'info' }[status] || 'info')

const pad = (n) => String(n).padStart(2, '0')

export const formatTime = (value) => {
  if (!value) {
    return '-'
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return '-'
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export const formatDuration = (ms) => {
  if (ms === null || ms === undefined) {
    return '-'
  }
  if (ms < 1000) {
    return `${ms}ms`
  }
  return `${(ms / 1000).toFixed(2)}s`
}

export const formatMemory = (kb) => {
  if (!kb) {
    return '0KB'
  }
  if (kb < 1024) {
    return `${kb}KB`
  }
  return `${(kb / 1024).toFixed(1)}MB`
}

export const formatFileSize = (bytes) => {
  if (!bytes) {
    return '0B'
  }
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(idx === 0 ? 0 : 1)}${units[idx]}`
}

// toLocalInput 把时间转换为 datetime-local 输入框需要的本地时间字符串。
export const toLocalInput = (value) => {
  if (!value) {
    return ''
  }
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) {
    return ''
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
