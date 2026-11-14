<script setup lang="ts">
// Tenant branding: name, logo url and primary color (issue #11). The
// primary color feeds the CSS accent variables live.
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ApiError, fetchCurrentTenant, updateTenantBranding } from '../../api'
import { tenantStore } from '../../stores/tenant'

const form = reactive({ name: '', logo_url: '', primary_color: '' })
const saving = ref(false)
const loaded = ref(false)

onMounted(async () => {
  const tenant = await fetchCurrentTenant()
  form.name = tenant.name
  form.logo_url = tenant.logo_url ?? ''
  form.primary_color = tenant.theme?.primary_color ?? ''
  loaded.value = true
})

async function save() {
  saving.value = true
  try {
    const updated = await updateTenantBranding({
      name: form.name || undefined,
      logo_url: form.logo_url || undefined,
      theme: { primary_color: form.primary_color || undefined },
    })
    tenantStore.apply({
      name: updated.name,
      primary_color: updated.theme?.primary_color,
      logo_url: updated.theme?.logo_url ?? updated.logo_url,
    })
    ElMessage.success('品牌设置已保存')
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div>
    <h2 class="page__title">品牌设置</h2>
    <div v-if="loaded" class="block" style="max-width: 560px">
      <el-form label-position="top">
        <el-form-item label="租户名称">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="Logo 地址">
          <el-input v-model="form.logo_url" placeholder="https://…/logo.png" />
        </el-form-item>
        <el-form-item label="主题色">
          <div class="branding__color">
            <el-color-picker v-model="form.primary_color" />
            <span class="muted">{{ form.primary_color || '默认蓝' }}</span>
          </div>
        </el-form-item>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </el-form>
    </div>
  </div>
</template>

<style scoped>
.branding__color {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
</style>
