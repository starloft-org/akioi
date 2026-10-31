<template>
  <div class="panel">
    <div class="toolbar">
      <el-button type="primary" @click="openCreate">新建题库</el-button>
      <el-button @click="load">刷新</el-button>
      <span class="muted">题号 = 题库前缀 + 题号长度的序号，例如前缀 A、长度 4 时从 A0000 开始自动递增。</span>
    </div>

    <el-table v-loading="loading" :data="banks" border>
      <el-table-column prop="id" label="ID" width="70" />
      <el-table-column prop="name" label="题库名称" min-width="160" />
      <el-table-column prop="prefix" label="前缀" width="90" />
      <el-table-column prop="code_length" label="题号长度" width="100" />
      <el-table-column prop="sample_display_id" label="题号示例" width="120" />
      <el-table-column prop="problem_count" label="题目数" width="90" />
      <el-table-column prop="remark" label="备注" min-width="180">
        <template #default="{ row }">
          <span v-if="row.remark">{{ row.remark }}</span>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" @click="removeBank(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑题库' : '新建题库'" width="520px">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
        <el-form-item label="题库名称" prop="name">
          <el-input v-model="form.name" placeholder="例如 基础题、提高题" />
        </el-form-item>
        <el-form-item label="题库前缀" prop="prefix">
          <el-input
            v-model="form.prefix"
            :disabled="editing"
            placeholder="字母开头，可带数字，例如 A"
            style="width: 160px"
          />
        </el-form-item>
        <el-form-item label="题号长度" prop="code_length">
          <el-input-number v-model="form.code_length" :min="1" :max="10" :disabled="editing" />
          <span class="muted" style="margin-left: 8px">序号左补零到该位数</span>
        </el-form-item>
        <el-form-item label="题号示例">
          <span>{{ sampleDisplayID || '—' }}</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" maxlength="255" show-word-limit />
        </el-form-item>
      </el-form>

      <p v-if="editing" class="muted">前缀与题号长度决定历史题号格式，创建后不可修改。</p>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  adminCreateProblemBank,
  adminDeleteProblemBank,
  adminListProblemBanks,
  adminUpdateProblemBank
} from '@/api'

const loading = ref(false)
const saving = ref(false)
const banks = ref([])
const dialogVisible = ref(false)
const editing = ref(false)
const editingID = ref(0)
const formRef = ref(null)

const form = reactive({
  name: '',
  prefix: '',
  code_length: 4,
  remark: ''
})

const rules = {
  name: [{ required: true, message: '请填写题库名称', trigger: 'blur' }],
  prefix: [
    { required: true, message: '请填写题库前缀', trigger: 'blur' },
    { pattern: /^[A-Za-z][A-Za-z0-9]{0,7}$/, message: '以字母开头，只含字母或数字', trigger: 'blur' }
  ]
}

// 按当前填写的前缀与长度预览题号，与后端生成规则一致
const sampleDisplayID = computed(() => {
  if (!/^[A-Za-z][A-Za-z0-9]{0,7}$/.test(form.prefix)) {
    return ''
  }
  const width = Number(form.code_length) || 4
  return form.prefix + '0'.repeat(Math.max(width, 1))
})

const load = async () => {
  loading.value = true
  try {
    const data = await adminListProblemBanks()
    banks.value = data.items || []
  } catch (e) {
    banks.value = []
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  editing.value = false
  editingID.value = 0
  form.name = ''
  form.prefix = ''
  form.code_length = 4
  form.remark = ''
  dialogVisible.value = true
}

const openEdit = (row) => {
  editing.value = true
  editingID.value = row.id
  form.name = row.name
  form.prefix = row.prefix
  form.code_length = row.code_length
  form.remark = row.remark || ''
  dialogVisible.value = true
}

const save = async () => {
  if (!formRef.value) {
    return
  }
  try {
    await formRef.value.validate()
  } catch (e) {
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await adminUpdateProblemBank(editingID.value, {
        name: form.name,
        remark: form.remark
      })
    } else {
      await adminCreateProblemBank({
        name: form.name,
        prefix: form.prefix,
        code_length: form.code_length,
        remark: form.remark
      })
    }
  } catch (e) {
    return
  } finally {
    saving.value = false
  }
  ElMessage.success('已保存')
  dialogVisible.value = false
  load()
}

const removeBank = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除题库「${row.name}」？题库下仍有题目时无法删除。`,
      '删除确认',
      { type: 'warning' }
    )
  } catch (e) {
    return
  }
  try {
    await adminDeleteProblemBank(row.id)
  } catch (e) {
    return
  }
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>
