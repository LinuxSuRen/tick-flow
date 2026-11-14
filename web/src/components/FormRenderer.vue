<script setup lang="ts">
// Schema-driven form renderer (issue #17): maps every field type of the
// template pack schema onto Element Plus controls. No page is hand-written
// per scenario.
import { computed, reactive, watch } from 'vue'
import type { FormField, FormField as Field } from '../api'

const props = defineProps<{ fields: FormField[] }>()
const model = defineModel<Record<string, unknown>>({ required: true })

const emit = defineEmits<{ (e: 'validate', ok: boolean): void }>()

function initial(f: Field): unknown {
  switch (f.type) {
    case 'boolean':
      return false
    case 'multiselect':
    case 'attachments':
      return []
    case 'number':
      return undefined
    default:
      return ''
  }
}

// Initialize defaults once; re-run when the type changes.
function reset() {
  for (const f of props.fields) {
    if (model.value[f.key] === undefined) {
      model.value[f.key] = initial(f)
    }
  }
}
watch(() => props.fields, reset, { immediate: true })

// Local validation mirrors the backend rules (required / max_length /
// min / max / pattern / max_count); the backend remains authoritative.
interface Rule {
  required?: boolean
  message: string
  trigger: string
  validator?: (rule: unknown, value: unknown, cb: (err?: Error) => void) => void
}

function rulesFor(f: Field): Rule[] {
  const rules: Rule[] = []
  if (f.required) {
    rules.push({
      required: true,
      message: `请填写${f.label}`,
      trigger: ['string', 'textarea'].includes(f.type) ? 'blur' : 'change',
    })
  }
  if (f.type === 'string' || f.type === 'textarea') {
    if (f.max_length) {
      rules.push({
        validator: (_r, v, cb) =>
          typeof v === 'string' && v.length > f.max_length!
            ? cb(new Error(`${f.label}不能超过 ${f.max_length} 个字符`))
            : cb(),
        trigger: 'blur',
        message: '',
      })
    }
    if (f.pattern) {
      rules.push({
        validator: (_r, v, cb) =>
          typeof v === 'string' && v !== '' && !new RegExp(f.pattern!).test(v)
            ? cb(new Error(`${f.label}格式不正确`))
            : cb(),
        trigger: 'blur',
        message: '',
      })
    }
  }
  if (f.type === 'number') {
    rules.push({
      validator: (_r, v, cb) => {
        if (v === '' || v === undefined || v === null) return cb()
        const n = Number(v)
        if (Number.isNaN(n)) return cb(new Error(`${f.label}必须是数字`))
        if (f.min !== undefined && n < f.min) return cb(new Error(`${f.label}不能小于 ${f.min}`))
        if (f.max !== undefined && n > f.max) return cb(new Error(`${f.label}不能大于 ${f.max}`))
        cb()
      },
      trigger: 'blur',
      message: '',
    })
  }
  return rules
}

const rules = computed(() => {
  const out: Record<string, Rule[]> = {}
  for (const f of props.fields) {
    out[f.key] = rulesFor(f)
  }
  return out
})

const draftFiles = reactive<Record<string, File[]>>({})

function onFileChange(fieldKey: string, files: { raw?: File }[]) {
  draftFiles[fieldKey] = files.map((x) => x.raw!).filter(Boolean)
}

defineExpose({
  // Collected files per attachments field; the page uploads them after the
  // ticket exists (uploads need a ticket id).
  draftFiles,
})
</script>

<template>
  <el-form :model="model" :rules="rules" label-position="top">
    <el-form-item
      v-for="f in fields"
      :key="f.key"
      :prop="f.key"
      :label="f.label"
    >
      <el-input
        v-if="f.type === 'string'"
        v-model="model[f.key] as string"
        :placeholder="f.placeholder ?? ''"
        :maxlength="f.max_length"
        show-word-limit
      />
      <el-input
        v-else-if="f.type === 'textarea'"
        v-model="model[f.key] as string"
        type="textarea"
        :rows="4"
        :placeholder="f.placeholder ?? ''"
        :maxlength="f.max_length"
        show-word-limit
      />
      <el-input-number
        v-else-if="f.type === 'number'"
        v-model="model[f.key] as number"
        :min="f.min"
        :max="f.max"
        style="width: 100%"
      />
      <el-select
        v-else-if="f.type === 'select'"
        v-model="model[f.key] as string"
        :placeholder="f.placeholder ?? '请选择'"
        style="width: 100%"
      >
        <el-option v-for="o in f.options" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
      <el-select
        v-else-if="f.type === 'multiselect'"
        v-model="model[f.key] as string[]"
        multiple
        :placeholder="f.placeholder ?? '请选择'"
        style="width: 100%"
      >
        <el-option v-for="o in f.options" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
      <el-switch v-else-if="f.type === 'boolean'" v-model="model[f.key] as boolean" />
      <el-date-picker
        v-else-if="f.type === 'date'"
        v-model="model[f.key] as string"
        type="date"
        value-format="YYYY-MM-DD"
        placeholder="选择日期"
        style="width: 100%"
      />
      <el-date-picker
        v-else-if="f.type === 'datetime'"
        v-model="model[f.key] as string"
        type="datetime"
        value-format="YYYY-MM-DDTHH:mm:ssZ"
        placeholder="选择日期时间"
        style="width: 100%"
      />
      <el-upload
        v-else-if="f.type === 'attachments'"
        :auto-upload="false"
        :limit="f.max_count"
        :on-change="(files: { raw?: File }[]) => onFileChange(f.key, files)"
        multiple
      >
        <el-button>选择文件</el-button>
        <template #tip>
          <div class="el-upload__tip">
            {{ f.max_count ? `最多 ${f.max_count} 个文件，提交后自动上传` : '提交后自动上传' }}
          </div>
        </template>
      </el-upload>
    </el-form-item>
  </el-form>
</template>
