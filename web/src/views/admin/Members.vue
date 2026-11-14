<script setup lang="ts">
// Member management: list, invite, role change, removal (issue #11).
// Invitation links without SMTP come from the response for copy-paste.
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  ApiError,
  inviteMember,
  listMembers,
  removeMember,
  updateMemberRole,
  type MemberView,
} from '../../api'

const items = ref<MemberView[]>([])
const total = ref(0)
const loading = ref(false)
const page = reactive({ page: 1, page_size: 20 })

const roleCopy: Record<string, string> = {
  tenant_admin: '管理员',
  agent: '处理人',
  requester: '提交人',
}
const roleType: Record<string, 'danger' | 'primary' | 'info'> = {
  tenant_admin: 'danger', agent: 'primary', requester: 'info',
}

async function load() {
  loading.value = true
  try {
    const result = await listMembers(page.page, page.page_size)
    items.value = result.data ?? []
    total.value = result.total
  } finally {
    loading.value = false
  }
}

const inviteForm = reactive({ visible: false, email: '', role: 'agent', display_name: '' })
const inviting = ref(false)

async function doInvite() {
  if (!inviteForm.email.includes('@')) {
    ElMessage.warning('请输入正确的邮箱')
    return
  }
  inviting.value = true
  try {
    const result = await inviteMember({
      email: inviteForm.email,
      role: inviteForm.role,
      display_name: inviteForm.display_name || undefined,
    })
    inviteForm.visible = false
    inviteForm.email = ''
    inviteForm.display_name = ''
    const link = `${window.location.origin}/accept-invitation?token=${result.invite_token}`
    ElMessage.success('邀请已创建')
    // The link is handed to the admin when mail is not configured.
    navigator.clipboard?.writeText(link).then(
      () => ElMessage.info('邀请链接已复制，请转交给对方'),
      () => ElMessage.info(`邀请链接：${link}`),
    )
    await load()
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  } finally {
    inviting.value = false
  }
}

async function onRoleChange(member: MemberView, role: string) {
  try {
    await updateMemberRole(member.id!, role)
    ElMessage.success('角色已更新')
    await load()
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  }
}

async function onRemove(member: MemberView) {
  try {
    await removeMember(member.id!)
    ElMessage.success('已移除')
    await load()
  } catch (err) {
    if (err instanceof ApiError) ElMessage.error(err.message)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page__title-line">
      <h2 class="page__title">成员管理</h2>
      <el-button type="primary" @click="inviteForm.visible = true">邀请成员</el-button>
    </div>
    <div class="block">
      <el-table v-loading="loading" :data="items">
        <el-table-column prop="display_name" label="姓名" width="140" />
        <el-table-column prop="email" label="邮箱" min-width="200" />
        <el-table-column label="角色" width="150">
          <template #default="{ row }">
            <el-tag :type="roleType[row.role] ?? 'info'" effect="plain">
              {{ roleCopy[row.role] ?? row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            {{ row.status === 'invited' ? '待激活' : '正常' }}
          </template>
        </el-table-column>
        <el-table-column label="加入时间" width="170">
          <template #default="{ row }">
            {{ row.created_at ? new Date(row.created_at).toLocaleString('zh-CN', { hour12: false }) : '—' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220">
          <template #default="{ row }">
            <el-select
              :model-value="row.role"
              size="small"
              style="width: 110px"
              @change="(v: string) => onRoleChange(row, v)"
            >
              <el-option label="管理员" value="tenant_admin" />
              <el-option label="处理人" value="agent" />
              <el-option label="提交人" value="requester" />
            </el-select>
            <el-button size="small" type="danger" plain style="margin-left: 8px" @click="onRemove(row)">
              移除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination
          v-model:current-page="page.page"
          :page-size="page.page_size"
          :total="total"
          layout="total, prev, pager, next"
          @current-change="load"
        />
      </div>
    </div>

    <el-dialog v-model="inviteForm.visible" title="邀请成员" width="420">
      <el-form label-position="top">
        <el-form-item label="邮箱" required>
          <el-input v-model="inviteForm.email" placeholder="name@example.com" />
        </el-form-item>
        <el-form-item label="姓名">
          <el-input v-model="inviteForm.display_name" placeholder="留空则使用邮箱前缀" />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-radio-group v-model="inviteForm.role">
            <el-radio value="agent">处理人</el-radio>
            <el-radio value="requester">提交人</el-radio>
            <el-radio value="tenant_admin">管理员</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="inviteForm.visible = false">取消</el-button>
        <el-button type="primary" :loading="inviting" @click="doInvite">邀请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page__title-line {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-3);
}
</style>
