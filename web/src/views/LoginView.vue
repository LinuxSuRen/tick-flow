<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ApiError, login, setToken, type LoginResult } from '../api'
import { meStore } from '../stores/me'
import { tenantStore } from '../stores/tenant'

const router = useRouter()
const form = reactive({
  email: '',
  password: '',
})
const loading = ref(false)

async function submit() {
  if (!form.email || !form.password) {
    ElMessage.warning('请输入邮箱和密码')
    return
  }
  loading.value = true
  try {
    const result = await login(form.email, form.password)
    await afterLogin(result)
  } catch (err) {
    if (err instanceof ApiError) {
      ElMessage.error(err.message)
    } else {
      ElMessage.error('登录失败，请稍后重试')
    }
  } finally {
    loading.value = false
  }
}

async function afterLogin(result: LoginResult) {
  setToken(result.token)
  meStore.apply(result.me)
  tenantStore.apply({ name: result.me.tenant?.name })
  if (result.me.tenant) {
    router.replace('/')
  } else {
    ElMessage.info('请先选择要进入的租户')
    // tenant selection lands with the auth batch; the skeleton parks here
  }
}
</script>

<template>
  <div class="login">
    <div class="login__card">
      <div class="login__brand">
        <span class="login__logo">TF</span>
        <span class="login__name">TickFlow</span>
      </div>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="name@example.com" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="请输入密码"
            autocomplete="current-password"
            @keyup.enter="submit"
          />
        </el-form-item>
        <el-button class="login__submit" type="primary" native-type="submit" :loading="loading">
          登录
        </el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-base);
}

.login__card {
  width: 360px;
  background: var(--color-bg-panel);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-card);
  padding: var(--space-8) var(--space-6);
}

.login__brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  justify-content: center;
  margin-bottom: var(--space-6);
}

.login__logo {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  background: var(--color-accent-base);
  color: #fff;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.login__name {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-bright);
}

.login__submit {
  width: 100%;
  margin-top: var(--space-2);
}
</style>
