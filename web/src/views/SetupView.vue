<script setup lang="ts">
// First-boot setup: creates the first tenant and admin (issue #11). The
// page only works on a fresh system; afterwards it shows a hint.
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError, setupFirstTenant } from '../api'
import { meStore } from '../stores/me'
import { tenantStore } from '../stores/tenant'

const router = useRouter()
const form = reactive({
  tenant_name: '',
  slug: '',
  admin_name: '',
  admin_email: '',
  admin_password: '',
})
const submitting = ref(false)
const alreadyDone = ref(false)

async function submit() {
  if (!form.tenant_name || !form.slug || !form.admin_name || !form.admin_email) {
    ElMessage.warning('请填写完整')
    return
  }
  if (form.admin_password.length < 8) {
    ElMessage.warning('密码至少 8 位')
    return
  }
  submitting.value = true
  try {
    const result = await setupFirstTenant(form)
    meStore.apply(result.me)
    tenantStore.apply({ name: result.me.tenant?.name })
    localStorage.setItem('tickflow_token', result.token)
    ElMessage.success('初始化完成，欢迎使用 TickFlow')
    router.replace('/')
  } catch (err) {
    if (err instanceof ApiError) {
      if (err.code === 'NOT_FOUND') {
        alreadyDone.value = true
        return
      }
      ElMessage.error(err.fields ? Object.values(err.fields).join('；') : err.message)
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="setup">
    <div class="setup__card">
      <div class="setup__brand">
        <img src="/favicon.svg" alt="TickFlow" class="setup__logo" />
      </div>
      <h2 class="setup__title">欢迎使用 TickFlow</h2>
      <p class="muted">首次使用请创建工作区与管理员账号，之后此页面自动失效。</p>
      <el-alert
        v-if="alreadyDone"
        type="warning"
        :closable="false"
        show-icon
        title="系统已完成初始化，请直接登录"
        style="margin-bottom: 16px"
      />
      <el-form v-else label-position="top" @submit.prevent="submit">
        <el-form-item label="工作区名称" required>
          <el-input v-model="form.tenant_name" placeholder="例如：某某科技" />
        </el-form-item>
        <el-form-item label="工作区标识" required>
          <el-input v-model="form.slug" placeholder="小写字母、数字或连字符，例如 some-tech" />
        </el-form-item>
        <el-form-item label="管理员姓名" required>
          <el-input v-model="form.admin_name" placeholder="你的名字" />
        </el-form-item>
        <el-form-item label="管理员邮箱" required>
          <el-input v-model="form.admin_email" placeholder="name@example.com" />
        </el-form-item>
        <el-form-item label="管理员密码" required>
          <el-input v-model="form.admin_password" type="password" show-password placeholder="至少 8 位" />
        </el-form-item>
        <el-button class="setup__submit" type="primary" native-type="submit" :loading="submitting">
          完成初始化
        </el-button>
      </el-form>
      <div class="setup__login">
        <router-link to="/login">已有账号？直接登录</router-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.setup {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-base);
  padding: var(--space-6) 0;
}

.setup__card {
  width: 420px;
  background: var(--color-bg-panel);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: var(--space-8) var(--space-6);
}

.setup__brand {
  display: flex;
  justify-content: center;
  margin-bottom: var(--space-2);
}

.setup__logo {
  width: 44px;
  height: 44px;
  object-fit: contain;
}

.setup__title {
  margin: 0 0 var(--space-1);
  text-align: center;
}

.setup__submit {
  width: 100%;
  margin-top: var(--space-2);
}

.setup__login {
  text-align: center;
  margin-top: var(--space-4);
  font-size: 13px;
}
</style>
