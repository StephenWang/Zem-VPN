<template>
  <div class="view subscriptions">
    <section class="sub-list">
      <h2>订阅列表</h2>
      <div v-if="subscriptions.length === 0" class="empty">
        暂无订阅，请添加
      </div>
      <div
        v-for="sub in subscriptions"
        :key="sub.id"
        class="sub-card"
        :class="{ active: currentSub === sub.id, selected: selectedId === sub.id }"
        @click="selectSub(sub.id)"
      >
        <div class="sub-info">
          <h3>{{ sub.name }}</h3>
          <p class="url">{{ sub.url }}</p>
          <p class="update">更新: {{ sub.lastUpdate }}</p>
          <div v-if="hasOptions(sub.options)" class="options-tags">
            <span v-if="sub.options.user_agent" class="tag">自定义 UA</span>
            <span v-if="sub.options.cookie" class="tag">Cookie</span>
            <span v-if="sub.options.preprocess" class="tag">预处理: {{ sub.options.preprocess }}</span>
            <span v-if="sub.options.skip_tls" class="tag warn">跳过 TLS</span>
          </div>
        </div>
        <div class="sub-actions">
          <SiteButton 
            @click.stop="updateSub(sub.id)" 
            :loading="updatingIds.includes(sub.id)" 
            type="secondary"
            theme="teal"
          >
            更新
          </SiteButton>
          <SiteButton @click.stop="editOptions(sub)" type="secondary" theme="blue">
            选项
          </SiteButton>
          <SiteButton 
            @click.stop="deleteSub(sub.id)" 
            type="primary" 
            theme="danger"
            :disabled="deletingIds.includes(sub.id)"
          >
            删除
          </SiteButton>
        </div>
      </div>
    </section>

    <section class="add-sub">
      <h2>添加订阅</h2>
      <div class="form-group">
        <label>订阅地址</label>
        <input
          v-model="newSubUrl"
          placeholder="https://example.com/clash.yaml 或 ss://..."
          @keyup.enter="addSub"
        />
      </div>
      <div class="form-group">
        <label>订阅名称</label>
        <input
          v-model="newSubName"
          placeholder="自定义名称"
          @keyup.enter="addSub"
        />
      </div>

      <div class="advanced-toggle" @click="showAdvanced = !showAdvanced">
        {{ showAdvanced ? '收起高级选项' : '高级选项' }}
      </div>

      <div v-if="showAdvanced" class="advanced-options">
        <div class="form-group">
          <label>User-Agent</label>
          <input v-model="newSubOptions.user_agent" placeholder="ClashforWindows/0.20.39" />
        </div>
        <div class="form-group">
          <label>Cookie</label>
          <input v-model="newSubOptions.cookie" placeholder="session=xxx" />
        </div>
        <div class="form-group">
          <label>预处理</label>
          <select v-model="newSubOptions.preprocess">
            <option value="">无</option>
            <option value="base64">Base64 解码</option>
          </select>
        </div>
        <label class="checkbox">
          <input type="checkbox" v-model="newSubOptions.skip_tls" />
          跳过 TLS 证书校验
        </label>
      </div>

      <SiteButton @click="addSub" :loading="adding" type="primary" theme="teal">
        {{ adding ? '添加中...' : '添加订阅' }}
      </SiteButton>
    </section>

    <section class="profile-list">
      <h2>Profiles</h2>
      <div v-if="profiles.length === 0" class="empty">
        暂无 Profile，可将多个订阅合并为一个 Profile
      </div>
      <div v-for="p in profiles" :key="p.id" class="sub-card">
        <div class="sub-info">
          <h3>{{ p.name }}</h3>
          <p class="url">模式: {{ p.merge_mode }} | 包含 {{ p.subscription_ids.length }} 个订阅</p>
        </div>
        <div class="sub-actions">
          <SiteButton
            @click="connectProfile(p.id)"
            :disabled="!isAdmin || (currentSub === 'profile:' + p.id && status === 'connected')"
            type="primary"
            theme="teal"
            :loading="connectingProfileId === p.id"
          >
            {{ currentSub === 'profile:' + p.id && status === 'connected' ? '已连接' : '连接' }}
          </SiteButton>
          <SiteButton @click="editProfile(p)" type="secondary" theme="blue">
            编辑
          </SiteButton>
          <SiteButton 
            @click="deleteProfile(p.id)" 
            type="primary" 
            theme="danger"
            :disabled="deletingProfileIds.includes(p.id)"
          >
            删除
          </SiteButton>
        </div>
      </div>

      <div class="add-profile">
        <h3>新建 Profile</h3>
        <div class="form-group">
          <input v-model="newProfileName" placeholder="Profile 名称" />
        </div>
        <div class="form-group">
          <label>合并模式</label>
          <select v-model="newProfileMode">
            <option value="union">合并所有节点 (union)</option>
            <option value="select">每个订阅一个 selector (select)</option>
          </select>
        </div>
        <div class="form-group">
          <label>选择订阅</label>
          <div class="checkbox-list">
            <label v-for="sub in subscriptions" :key="sub.id" class="checkbox">
              <input
                type="checkbox"
                :value="sub.id"
                v-model="selectedProfileSubs"
              />
              {{ sub.name }}
            </label>
          </div>
        </div>
        <SiteButton @click="createProfile" :loading="creatingProfile" type="primary" theme="teal">
          {{ creatingProfile ? '创建中...' : '创建 Profile' }}
        </SiteButton>
      </div>
    </section>

    <!-- 选项编辑弹窗 -->
    <div v-if="editingSub" class="modal" @click="editingSub = null">
      <div class="modal-content" @click.stop>
        <h3>编辑订阅选项</h3>
        <div class="form-group">
          <label>User-Agent</label>
          <input v-model="editOptionsData.user_agent" />
        </div>
        <div class="form-group">
          <label>Cookie</label>
          <input v-model="editOptionsData.cookie" />
        </div>
        <div class="form-group">
          <label>预处理</label>
          <select v-model="editOptionsData.preprocess">
            <option value="">无</option>
            <option value="base64">Base64 解码</option>
          </select>
        </div>
        <label class="checkbox">
          <input type="checkbox" v-model="editOptionsData.skip_tls" />
          跳过 TLS 证书校验
        </label>
        <div class="modal-actions">
          <SiteButton @click="saveOptions" type="primary" theme="teal">
            保存
          </SiteButton>
          <SiteButton @click="editingSub = null" type="secondary" theme="teal">
            取消
          </SiteButton>
        </div>
      </div>
    </div>

    <!-- Profile 编辑弹窗 -->
    <div v-if="editingProfile" class="modal" @click="editingProfile = null">
      <div class="modal-content" @click.stop>
        <h3>编辑 Profile</h3>
        <div class="form-group">
          <label>名称</label>
          <input v-model="editProfileData.name" />
        </div>
        <div class="form-group">
          <label>合并模式</label>
          <select v-model="editProfileData.merge_mode">
            <option value="union">合并所有节点 (union)</option>
            <option value="select">每个订阅一个 selector (select)</option>
          </select>
        </div>
        <div class="form-group">
          <label>选择订阅</label>
          <div class="checkbox-list">
            <label v-for="sub in subscriptions" :key="sub.id" class="checkbox">
              <input type="checkbox" :value="sub.id" v-model="editProfileData.subscription_ids" />
              {{ sub.name }}
            </label>
          </div>
        </div>
        <div class="modal-actions">
          <SiteButton @click="saveProfile" type="primary" theme="teal">
            保存
          </SiteButton>
          <SiteButton @click="editingProfile = null" type="secondary" theme="teal">
            取消
          </SiteButton>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, onActivated } from 'vue'
import SiteButton from '../elements/SiteButton.vue'
import {
  AddSubscription,
  AddSubscriptionWithOptions,
  UpdateSubscriptionOptions,
  UpdateSubscription,
  DeleteSubscription,
  ListSubscriptions,
  ConnectSubscription,
  Disconnect,
  GetStatus,
  GetCurrentSubscriptionID,
  IsAdmin,
  CreateProfile,
  UpdateProfile,
  DeleteProfile,
  ListProfiles,
  ConnectProfile
} from '@wailsjs/go/main/App'

const subscriptions = ref([])
const profiles = ref([])
const currentSub = ref('')
const status = ref('disconnected')
const selectedId = ref('')
const isAdmin = ref(false)
const newSubUrl = ref('')
const newSubName = ref('')
const adding = ref(false)
const showAdvanced = ref(false)
const creatingProfile = ref(false)
const newProfileName = ref('')
const newProfileMode = ref('union')
const selectedProfileSubs = ref([])

// Loading 状态管理
const updatingIds = ref([])
const deletingIds = ref([])
const deletingProfileIds = ref([])
const connectingProfileId = ref(null)

const newSubOptions = ref({
  user_agent: '',
  cookie: '',
  preprocess: '',
  skip_tls: false
})

const editingSub = ref(null)
const editOptionsData = ref({
  user_agent: '',
  cookie: '',
  preprocess: '',
  skip_tls: false
})

const editingProfile = ref(null)
const editProfileData = ref({
  id: '',
  name: '',
  subscription_ids: [],
  merge_mode: 'union'
})

const emit = defineEmits(['error', 'success'])
let statusTimer = null
let activatedOnce = false

const showError = (msg) => emit('error', msg)
const showSuccess = (msg) => emit('success', msg)

const hasOptions = (opts) => {
  if (!opts) return false
  return opts.user_agent || opts.cookie || opts.preprocess || opts.skip_tls
}

const refreshList = async () => {
  try {
    subscriptions.value = await ListSubscriptions()
  } catch (e) {
    showError('获取订阅列表失败: ' + e)
  }
}

const refreshProfiles = async () => {
  try {
    profiles.value = await ListProfiles()
  } catch (e) {
    showError('获取 Profile 列表失败: ' + e)
  }
}

const refreshStatus = async () => {
  try {
    status.value = await GetStatus()
    const id = await GetCurrentSubscriptionID()
    if (id && id.startsWith('profile:')) {
      currentSub.value = id
    } else if (id) {
      currentSub.value = id
    } else if (status.value !== 'connected') {
      currentSub.value = ''
    }
    // 如果仍在连接中但 GetCurrentSubscriptionID 返回空，保持 currentSub 不变
  } catch (e) {
    console.error('refreshStatus error:', e)
  }
}

const addSub = async () => {
  if (!newSubUrl.value.trim()) {
    showError('请输入订阅地址')
    return
  }
  adding.value = true
  try {
    const opts = {
      user_agent: newSubOptions.value.user_agent || '',
      cookie: newSubOptions.value.cookie || '',
      preprocess: newSubOptions.value.preprocess || '',
      skip_tls: !!newSubOptions.value.skip_tls
    }
    await AddSubscriptionWithOptions(
      newSubUrl.value,
      newSubName.value || '未命名',
      opts
    )
    newSubUrl.value = ''
    newSubName.value = ''
    newSubOptions.value = { user_agent: '', cookie: '', preprocess: '', skip_tls: false }
    await refreshList()
    showSuccess('订阅添加成功')
  } catch (e) {
    showError('添加失败: ' + e)
  } finally {
    adding.value = false
  }
}

const selectSub = async (id) => {
  selectedId.value = id
  await connect(id)
  selectedId.value = ''
}

const connect = async (id) => {
  if (!isAdmin.value) {
    showError('需要管理员权限')
    return
  }
  try {
    await ConnectSubscription(id)
    currentSub.value = id
    status.value = 'connected'
    showSuccess('连接成功')
  } catch (e) {
    await refreshStatus()
    showError('连接失败: ' + e)
  }
}

const connectProfile = async (id) => {
  if (!isAdmin.value) {
    showError('需要管理员权限')
    return
  }
  connectingProfileId.value = id
  try {
    await ConnectProfile(id)
    currentSub.value = 'profile:' + id
    status.value = 'connected'
    showSuccess('Profile 连接成功')
  } catch (e) {
    await refreshStatus()
    showError('Profile 连接失败: ' + e)
  } finally {
    connectingProfileId.value = null
  }
}

const updateSub = async (id) => {
  updatingIds.value.push(id)
  const wasConnected = currentSub.value === id
  try {
    await UpdateSubscription(id)
    await refreshList()
    if (wasConnected) {
      // 更新后如果之前正在使用该订阅，自动重新连接
      await connect(id)
    }
    showSuccess('更新成功')
  } catch (e) {
    showError('更新失败: ' + e)
  } finally {
    updatingIds.value = updatingIds.value.filter(i => i !== id)
  }
}

const deleteSub = async (id) => {
  if (!confirm('确定删除此订阅?')) return
  deletingIds.value.push(id)
  try {
    await DeleteSubscription(id)
    if (currentSub.value === id || currentSub.value === 'profile:' + id) {
      currentSub.value = ''
      status.value = 'disconnected'
      try { await Disconnect() } catch {}
    }
    await refreshList()
    await refreshProfiles()
    showSuccess('删除成功')
  } catch (e) {
    showError('删除失败: ' + e)
  } finally {
    deletingIds.value = deletingIds.value.filter(i => i !== id)
  }
}

const editOptions = (sub) => {
  editingSub.value = sub
  editOptionsData.value = {
    user_agent: sub.options?.user_agent || '',
    cookie: sub.options?.cookie || '',
    preprocess: sub.options?.preprocess || '',
    skip_tls: !!sub.options?.skip_tls
  }
}

const saveOptions = async () => {
  try {
    await UpdateSubscriptionOptions(editingSub.value.id, {
      user_agent: editOptionsData.value.user_agent,
      cookie: editOptionsData.value.cookie,
      preprocess: editOptionsData.value.preprocess,
      skip_tls: !!editOptionsData.value.skip_tls
    })
    editingSub.value = null
    await refreshList()
    showSuccess('选项已保存')
  } catch (e) {
    showError('保存选项失败: ' + e)
  }
}

const createProfile = async () => {
  if (!newProfileName.value.trim()) {
    showError('请输入 Profile 名称')
    return
  }
  if (selectedProfileSubs.value.length === 0) {
    showError('请至少选择一个订阅')
    return
  }
  creatingProfile.value = true
  try {
    await CreateProfile(newProfileName.value, selectedProfileSubs.value, newProfileMode.value)
    newProfileName.value = ''
    selectedProfileSubs.value = []
    newProfileMode.value = 'union'
    await refreshProfiles()
    showSuccess('Profile 创建成功')
  } catch (e) {
    showError('创建失败: ' + e)
  } finally {
    creatingProfile.value = false
  }
}

const editProfile = (p) => {
  editingProfile.value = p
  editProfileData.value = {
    id: p.id,
    name: p.name,
    subscription_ids: [...p.subscription_ids],
    merge_mode: p.merge_mode
  }
}

const saveProfile = async () => {
  try {
    await UpdateProfile(
      editProfileData.value.id,
      editProfileData.value.name,
      editProfileData.value.subscription_ids,
      editProfileData.value.merge_mode
    )
    editingProfile.value = null
    await refreshProfiles()
    showSuccess('Profile 已更新')
  } catch (e) {
    showError('更新失败: ' + e)
  }
}

const deleteProfile = async (id) => {
  if (!confirm('确定删除此 Profile?')) return
  deletingProfileIds.value.push(id)
  try {
    await DeleteProfile(id)
    if (currentSub.value === 'profile:' + id) {
      currentSub.value = ''
      status.value = 'disconnected'
      try { await Disconnect() } catch {}
    }
    await refreshProfiles()
    showSuccess('Profile 已删除')
  } catch (e) {
    showError('删除失败: ' + e)
  } finally {
    deletingProfileIds.value = deletingProfileIds.value.filter(i => i !== id)
  }
}

const refreshPageState = async () => {
  await refreshList()
  await refreshProfiles()
  await refreshStatus()
}

onMounted(async () => {
  isAdmin.value = await IsAdmin()
  await refreshPageState()
  statusTimer = setInterval(refreshStatus, 2000)
})

onActivated(async () => {
  if (!activatedOnce) {
    activatedOnce = true
    return
  }
  await refreshPageState()
})

onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer)
})
</script>

<style scoped>
.view {
  padding: 20px;
}

.sub-list h2,
.add-sub h2,
.profile-list h2 {
  font-size: 18px;
  margin-bottom: 15px;
  color: #00d4aa;
}

.empty {
  text-align: center;
  padding: 40px;
  color: #666;
  background: #16213e;
  border-radius: 8px;
}

.sub-card {
  background: #16213e;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  border: 2px solid transparent;
  transition: border-color 0.2s, background 0.2s;
}

.sub-card:hover {
  background: #1a2a4d;
}

.sub-card.selected {
  border-color: #00d4aa;
  background: #1a2a4d;
}

.sub-card.active {
  border-color: #00d4aa;
  background: #0f3460;
  box-shadow: 0 0 0 2px rgba(0, 212, 170, 0.25);
}

.sub-card.active h3 {
  color: #00d4aa;
}

.sub-info h3 {
  font-size: 16px;
  margin-bottom: 4px;
}

.sub-info .url {
  font-size: 12px;
  color: #888;
  max-width: 400px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sub-info .update {
  font-size: 12px;
  color: #666;
  margin-top: 4px;
}

.options-tags {
  display: flex;
  gap: 6px;
  margin-top: 8px;
  flex-wrap: wrap;
}

.tag {
  font-size: 11px;
  padding: 2px 8px;
  background: #0f3460;
  border-radius: 10px;
  color: #aaa;
}

.tag.warn {
  background: #e94560;
  color: #fff;
}

.sub-actions {
  display: flex;
  gap: 8px;
}

.add-sub,
.profile-list {
  background: #16213e;
  border-radius: 8px;
  padding: 20px;
  margin-top: 20px;
}

.add-profile {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid #333;
}

.add-profile h3 {
  font-size: 16px;
  margin-bottom: 12px;
  color: #00d4aa;
}

.form-group {
  margin-bottom: 15px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  color: #aaa;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #333;
  border-radius: 6px;
  background: #1a1a2e;
  color: #eee;
  font-size: 14px;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: #00d4aa;
}

.advanced-toggle {
  color: #00d4aa;
  font-size: 14px;
  cursor: pointer;
  margin-bottom: 12px;
  user-select: none;
}

.advanced-options {
  background: #1a1a2e;
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #ccc;
  cursor: pointer;
}

.checkbox input {
  width: auto;
}

.checkbox-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 200px;
  overflow-y: auto;
  padding: 10px;
  background: #1a1a2e;
  border-radius: 6px;
}

.modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: #16213e;
  padding: 24px;
  border-radius: 12px;
  min-width: 320px;
  max-width: 500px;
  width: 90%;
}

.modal-content h3 {
  margin-bottom: 16px;
  color: #00d4aa;
}

.modal-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-end;
  margin-top: 20px;
}
</style>
