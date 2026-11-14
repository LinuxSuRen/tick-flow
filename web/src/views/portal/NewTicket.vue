<script setup lang="ts">
// Schema-driven submission form (issue #17). Files collected by attachment
// fields upload right after the ticket is created (uploads need a ticket
// id); failures surface but never lose the created ticket.
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ApiError,
  createTicket,
  fetchTicketType,
  uploadAttachment,
  type TicketTypeView,
} from '../../api'
import FormRenderer from '../../components/FormRenderer.vue'

const route = useRoute()
const router = useRouter()

const type = ref<TicketTypeView | null>(null)
const model = ref<Record<string, unknown>>({})
const title = ref('')
const content = ref('')
const submitting = ref(false)
const renderer = ref<InstanceType<typeof FormRenderer> | null>(null)

onMounted(async () => {
  type.value = await fetchTicketType(Number(route.params.typeId))
})

async function submit() {
  if (!type.value) return
  if (!title.value.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  submitting.value = true
  try {
    const created = await createTicket({
      ticket_type_id: type.value.id,
      title: title.value,
      content: content.value,
      custom_fields: model.value,
    })
    // Upload collected files per attachments field; ids are not written
    // back into custom_fields — the detail view reads the attachment list.
    const drafts = renderer.value?.draftFiles ?? {}
    for (const files of Object.values(drafts)) {
      for (const file of files) {
        try {
          await uploadAttachment(created.id, file)
        } catch (err) {
          if (err instanceof ApiError) {
            ElMessage.error(`附件 ${file.name} 上传失败：${err.message}`)
          }
        }
      }
    }
    router.replace(`/success/${created.id}`)
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.fields) {
        const first = Object.values(err.fields)[0]
        ElMessage.error(first ?? err.message)
      } else {
        ElMessage.error(err.message)
      }
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div v-if="type">
    <h2 class="page__title">{{ type.view_config?.portal?.title || type.name }}</h2>
    <p v-if="type.view_config?.portal?.description" class="muted newticket__desc">
      {{ type.view_config.portal.description }}
    </p>
    <div class="block">
      <el-form label-position="top">
        <el-form-item label="标题" required>
          <el-input v-model="title" placeholder="用一句话概括问题" maxlength="200" show-word-limit />
        </el-form-item>
        <el-form-item label="补充说明">
          <el-input v-model="content" type="textarea" :rows="3" placeholder="更多背景信息，有助于更快处理" />
        </el-form-item>
      </el-form>
      <FormRenderer ref="renderer" v-model="model" :fields="type.form_schema?.fields ?? []" />
      <div class="newticket__submit">
        <el-button @click="router.back()">返回</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">提交</el-button>
      </div>
    </div>
  </div>
  <div v-else class="block muted">加载中</div>
</template>

<style scoped>
.newticket__desc {
  margin: 0 0 var(--space-2);
}

.newticket__submit {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
</style>
