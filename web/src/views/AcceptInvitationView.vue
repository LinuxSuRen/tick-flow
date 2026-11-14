<script setup lang="ts">
// Accept an invitation: set the first password, then log in manually (the
// token is consumed on success).
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { acceptInvitation, ApiError } from '../api'

const route = useRoute()
const router = useRouter()
const form = reactive({ password: '', confirm: '' })
const submitting = ref(false)
const done = ref(false)

async function submit() {
  const token = String(route.query.token ?? '')
  if (!token) {
    ElMessage.error('邀请链接无效')
    return
  }
  if (form.password.length < 8) {
    ElMessage.warning('密码至少 8 位')
    return
  }
  if (form.password !== form.confirm) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  submitting.value = true
  try {
    await acceptInvitation(token, form.password)
    done.value = true
    ElMessage.success('密码已设置，请登录')
    setTimeout(() => router.replace('/login'), 1200)
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="accept">
    <div class="accept__card">
      <h2 class="accept__title">设置密码</h2>
      <p class="muted">你被邀请加入 TickFlow 工作区，请设置登录密码。</p>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="密码" required>
          <el-input v-model="form.password" type="password" show-password placeholder="至少 8 位" />
        </el-form-item>
        <el-form-item label="确认密码" required>
          <el-input v-model="form.confirm" type="password" show-password placeholder="再输入一次" />
        </el-form-item>
        <el-button class="accept__submit" type="primary" native-type="submit" :loading="submitting" :disabled="done">
          {{ done ? '已设置，正在跳转登录' : '设置密码' }}
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.accept {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-base);
}

.accept__card {
  width: 380px;
  background: var(--color-bg-panel);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: var(--space-8) var(--space-6);
}

.accept__title {
  margin: 0 0 var(--space-1);
}

.accept__submit {
  width: 100%;
  margin-top: var(--space-2);
}
</style>
