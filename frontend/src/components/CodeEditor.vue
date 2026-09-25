<template>
  <div ref="editorRef" class="code-editor"></div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { basicSetup } from 'codemirror'
import { cpp } from '@codemirror/lang-cpp'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'

const props = defineProps({
  modelValue: { type: String, default: '' },
  language: { type: String, default: 'cpp' },
  readonly: { type: Boolean, default: false }
})

const emit = defineEmits(['update:modelValue', 'submit'])

const editorRef = ref(null)
const readonlyCompartment = new Compartment()
let view = null

// Ctrl+Enter 触发提交事件
const submitKeymap = keymap.of([
  {
    key: 'Ctrl-Enter',
    run: () => {
      emit('submit')
      return true
    }
  }
])

const readonlyExtensions = (readonly) => [
  EditorView.editable.of(!readonly),
  EditorState.readOnly.of(readonly)
]

onMounted(() => {
  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue || '',
      extensions: [
        basicSetup,
        cpp(),
        submitKeymap,
        readonlyCompartment.of(readonlyExtensions(props.readonly)),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            emit('update:modelValue', update.state.doc.toString())
          }
        })
      ]
    }),
    parent: editorRef.value
  })
})

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})

// 外部改动时同步回编辑器，避免覆盖用户正在编辑的内容
watch(
  () => props.modelValue,
  (value) => {
    if (!view) {
      return
    }
    const current = view.state.doc.toString()
    if ((value || '') !== current) {
      view.dispatch({ changes: { from: 0, to: current.length, insert: value || '' } })
    }
  }
)

watch(
  () => props.readonly,
  (value) => {
    view?.dispatch({ effects: readonlyCompartment.reconfigure(readonlyExtensions(value)) })
  }
)
</script>
