<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ApiError,
  login,
  selectTenant,
  setToken,
  type LoginResult,
  type TenantBrief,
} from '../api'
import { meStore } from '../stores/me'
import { tenantStore } from '../stores/tenant'

const router = useRouter()
const form = reactive({
  email: '',
  password: '',
})
const loading = ref(false)
const tenantChoice = ref<{ visible: boolean; options: TenantBrief[] }>({
  visible: false,
  options: [],
})
const choosing = ref(false)

async function submit() {
  if (!form.email || !form.password) {
    ElMessage.warning('请输入邮箱和密码')
    return
  }
  loading.value = true
  try {
    const result = await login(form.email, form.password)
    if (result.me.tenants.length > 1) {
      // multiple tenants: let the user pick before switching the token
      meStore.apply({ ...result.me, tenant: null })
      tenantChoice.value = { visible: true, options: result.me.tenants }
      return
    }
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
  if (!result.me.tenant) {
    const single = result.me.tenants[0]
    if (!single) {
      ElMessage.error('账号尚未加入任何工作区，请联系管理员')
      return
    }
    const switched = await selectTenant(single.id)
    await settle(switched)
    return
  }
  await settle(result)
}

async function settle(result: LoginResult) {
  setToken(result.token)
  meStore.apply(result.me)
  tenantStore.apply({ name: result.me.tenant?.name })
  router.replace('/')
}

async function choose(t: TenantBrief) {
  choosing.value = true
  try {
    const switched = await selectTenant(t.id)
    tenantChoice.value.visible = false
    await settle(switched)
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  } finally {
    choosing.value = false
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
      <div class="login__hint">
        <router-link to="/setup">首次使用？前往初始化</router-link>
      </div>
    </div>

    <el-dialog v-model="tenantChoice.visible" title="选择工作区" width="360">
      <div class="login__tenants">
        <el-button
          v-for="t in tenantChoice.options"
          :key="t.id"
          class="login__tenant"
          :loading="choosing"
          @click="choose(t)"
        >
          {{ t.name }}
        </el-button>
      </div>
    </el-dialog>
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

.login__hint {
  text-align: center;
  margin-top: var(--space-4);
  font-size: 13px;
}

.login__tenants {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.login__tenant {
  justify-content: flex-start;
  width: 100%;
}
</style>
