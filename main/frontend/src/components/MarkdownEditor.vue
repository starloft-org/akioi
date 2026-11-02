<template>
  <div class="md-editor">
    <div class="md-editor__bar">
      <div class="md-editor__tools">
        <button type="button" class="md-tool" title="加粗（Ctrl+B）" @click="wrap('**', '**', '加粗文字')">
          <strong>B</strong>
        </button>
        <button type="button" class="md-tool" title="斜体（Ctrl+I）" @click="wrap('*', '*', '斜体文字')">
          <em>I</em>
        </button>
        <button type="button" class="md-tool" title="行内代码" @click="wrap('`', '`', 'code')">
          <span class="md-tool__code">&lt;/&gt;</span>
        </button>
        <button type="button" class="md-tool" title="引用" @click="prefixLines('> ')">❝</button>
        <button type="button" class="md-tool" title="列表" @click="prefixLines('- ')">≡</button>
        <button type="button" class="md-tool" title="链接" @click="insertLink">🔗</button>
        <button type="button" class="md-tool md-tool--wide" title="行内公式" @click="wrap('$', '$', 'x^2')">
          公式
        </button>
        <button type="button" class="md-tool md-tool--wide" title="独立公式" @click="wrapBlock('$$', '$$', '\\sum_{i=1}^{n} i')">
          ∑ 块
        </button>
        <button type="button" class="md-tool md-tool--wide" :disabled="uploading" title="上传图片并插入" @click="pickImage">
          {{ uploading ? '上传中…' : '图片' }}
        </button>
      </div>
      <div class="md-editor__modes">
        <button
          v-for="item in modes"
          :key="item.value"
          type="button"
          class="md-mode"
          :class="{ 'is-active': mode === item.value }"
          @click="mode = item.value"
        >
          {{ item.label }}
        </button>
      </div>
    </div>
    <div class="md-editor__body" :class="`is-${mode}`">
      <textarea
        v-if="mode !== 'preview'"
        ref="inputRef"
        class="md-editor__input"
        :value="modelValue"
        :placeholder="placeholder"
        :style="{ minHeight }"
        spellcheck="false"
        @input="onInput"
        @keydown="onKeydown"
        @paste="onPaste"
      ></textarea>
      <div
        v-if="mode !== 'edit'"
        class="md-editor__preview md-body"
        :style="{ minHeight }"
        v-html="previewHtml"
      ></div>
    </div>
    <div class="md-editor__foot">
      <span>支持 Markdown 与 LaTeX（<code>$...$</code>、<code>$$...$$</code>）</span>
      <span class="md-editor__hint">可以直接把图片粘贴进来，也可以点「图片」上传</span>
    </div>
    <input
      ref="fileRef"
      class="md-editor__file"
      type="file"
      accept="image/png,image/jpeg,image/gif,image/webp"
      @change="onFileChange"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { renderMarkdown } from '@/utils/markdown'
import { uploadImage } from '@/api'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: '' },
  rows: { type: Number, default: 10 },
  minHeight: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])

const modes = [
  { value: 'edit', label: '编辑' },
  { value: 'split', label: '分栏' },
  { value: 'preview', label: '预览' }
]
const mode = ref('split')
const inputRef = ref(null)
const fileRef = ref(null)
const uploading = ref(false)

const previewHtml = computed(() => renderMarkdown(props.modelValue || ''))
const minHeight = computed(() => props.minHeight || `${props.rows * 26}px`)

const emitValue = (value, start, end) => {
  emit('update:modelValue', value)
  if (start === undefined) return
  nextTick(() => {
    const el = inputRef.value
    if (!el) return
    el.focus()
    el.setSelectionRange(start, end)
  })
}

const onInput = (event) => {
  emit('update:modelValue', event.target.value)
}

// 在光标处包裹选中文本；没有选中就用占位词。
const wrap = (before, after, placeholder = '') => {
  const el = inputRef.value
  if (!el) return
  const value = props.modelValue || ''
  const start = el.selectionStart
  const end = el.selectionEnd
  const selected = value.slice(start, end) || placeholder
  const next = value.slice(0, start) + before + selected + after + value.slice(end)
  emitValue(next, start + before.length, start + before.length + selected.length)
}

// 独占一行的块（公式块、代码块）：确保前后各有一个空行。
const wrapBlock = (before, after, placeholder = '') => {
  const el = inputRef.value
  if (!el) return
  const value = props.modelValue || ''
  const start = el.selectionStart
  const end = el.selectionEnd
  const selected = value.slice(start, end) || placeholder
  const lead = start === 0 || value.slice(0, start).endsWith('\n\n') ? '' : value.slice(0, start).endsWith('\n') ? '\n' : '\n\n'
  const tail = end >= value.length || value.slice(end).startsWith('\n\n') ? '' : value.slice(end).startsWith('\n') ? '\n' : '\n\n'
  const block = `${before}\n${selected}\n${after}`
  const next = value.slice(0, start) + lead + block + tail + value.slice(end)
  emitValue(next, start + lead.length + before.length + 1, start + lead.length + before.length + 1 + selected.length)
}

const insert = (text) => {
  const el = inputRef.value
  if (!el) return
  const value = props.modelValue || ''
  const start = el.selectionStart
  const end = el.selectionEnd
  const next = value.slice(0, start) + text + value.slice(end)
  emitValue(next, start + text.length, start + text.length)
}

// 给选中的每一行加前缀（引用、列表）。
const prefixLines = (prefix) => {
  const el = inputRef.value
  if (!el) return
  const value = props.modelValue || ''
  let start = el.selectionStart
  let end = el.selectionEnd
  if (start === end) {
    const lineStart = value.lastIndexOf('\n', start - 1) + 1
    const lineEnd = value.indexOf('\n', end)
    start = lineStart
    end = lineEnd === -1 ? value.length : lineEnd
  }
  const block = value.slice(start, end) || ''
  const next = block
    .split('\n')
    .map((line) => prefix + line)
    .join('\n')
  const nextValue = value.slice(0, start) + next + value.slice(end)
  emitValue(nextValue, start, start + next.length)
}

const insertLink = () => {
  const el = inputRef.value
  const value = props.modelValue || ''
  const selected = value.slice(el.selectionStart, el.selectionEnd) || '链接文字'
  insert(`[${selected}](https://)`)
}

const insertImage = (url, name = '图片') => {
  const isBlock = !(props.modelValue || '').endsWith('\n\n')
  insert(`${isBlock ? '\n\n' : ''}![${name}](${url})\n\n`)
}

const pickImage = () => {
  if (uploading.value) return
  fileRef.value?.click()
}

const doUpload = async (file) => {
  if (!file) return
  if (file.size > 5 * 1024 * 1024) {
    ElMessage.warning('图片不能超过 5MB')
    return
  }
  uploading.value = true
  try {
    const data = await uploadImage(file)
    insertImage(data.url, file.name || '图片')
    ElMessage.success('图片已插入')
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || '图片上传失败')
  } finally {
    uploading.value = false
  }
}

const onFileChange = async (event) => {
  const file = event.target.files?.[0]
  event.target.value = ''
  await doUpload(file)
}

// 支持直接粘贴截图。
const onPaste = (event) => {
  const items = event.clipboardData?.items || []
  for (const item of items) {
    if (item.type && item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) {
        event.preventDefault()
        doUpload(file)
      }
      return
    }
  }
}

const onKeydown = (event) => {
  if (!(event.ctrlKey || event.metaKey)) return
  const key = event.key.toLowerCase()
  if (key === 'b') {
    event.preventDefault()
    wrap('**', '**', '加粗文字')
  } else if (key === 'i') {
    event.preventDefault()
    wrap('*', '*', '斜体文字')
  } else if (key === 'k') {
    event.preventDefault()
    insertLink()
  }
}
</script>

<style scoped>
.md-editor {
  border: 1px solid var(--border-color, #dcdfe6);
  border-radius: 10px;
  overflow: hidden;
  background: var(--bg-card, #fff);
}

.md-editor__bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
  padding: 6px 8px;
  border-bottom: 1px solid var(--border-color, #ebeef5);
  background: var(--bg-soft, #fafafa);
}

.md-editor__tools,
.md-editor__modes {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.md-tool {
  min-width: 28px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-primary, #303133);
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
}

.md-tool:hover {
  background: var(--bg-hover, #eef2f7);
  border-color: var(--border-color, #dcdfe6);
}

.md-tool:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.md-tool--wide {
  font-size: 12px;
}

.md-tool__code {
  font-family: monospace;
  font-size: 12px;
}

.md-mode {
  height: 26px;
  padding: 0 10px;
  border: 1px solid var(--border-color, #dcdfe6);
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary, #606266);
  font-size: 12px;
  cursor: pointer;
}

.md-mode.is-active {
  border-color: var(--primary-color, #1cd5c7);
  color: var(--primary-color, #1cd5c7);
  background: rgba(28, 213, 199, 0.08);
}

.md-editor__body {
  display: flex;
  min-height: 160px;
}

.md-editor__body.is-split {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}

.md-editor__input {
  flex: 1;
  width: 100%;
  min-height: 160px;
  padding: 12px 14px;
  border: 0;
  outline: none;
  resize: vertical;
  background: transparent;
  color: var(--text-primary, #303133);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.7;
  tab-size: 2;
}

.md-editor__preview {
  flex: 1;
  min-width: 0;
  padding: 12px 14px;
  overflow: auto;
  border-left: 1px solid var(--border-color, #ebeef5);
}

.md-editor__body.is-preview .md-editor__preview {
  border-left: 0;
}

.md-editor__foot {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  justify-content: space-between;
  padding: 6px 10px;
  border-top: 1px solid var(--border-color, #ebeef5);
  color: var(--text-muted, #909399);
  font-size: 12px;
}

.md-editor__hint {
  color: var(--text-muted, #b1b3b8);
}

.md-editor__file {
  display: none;
}

@media (max-width: 860px) {
  .md-editor__body.is-split {
    grid-template-columns: minmax(0, 1fr);
  }

  .md-editor__preview {
    border-left: 0;
    border-top: 1px solid var(--border-color, #ebeef5);
  }
}
</style>
