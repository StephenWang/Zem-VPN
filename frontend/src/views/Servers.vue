<template>
  <div class="view servers">
    <div class="toolbar">
      <div class="toolbar-left">
        <label>排序</label>
        <select v-model="sortBy" class="sort-select">
          <option value="default">默认</option>
          <option value="country">国家/地区</option>
          <option value="ping">Ping</option>
        </select>
      </div>
      <div class="toolbar-right">
        <button @click="handleSpeedTestButton" :disabled="!currentSubID" class="btn-primary" :class="{ 'btn-danger': testing }">
          <Icon name="Zap" class="btn-icon" :class="{ spinning: testing }" />
          {{ testing ? '终止测速' : '测速' }}
        </button>
      </div>
    </div>

    <AppTabs
      v-if="tabItems.length > 0"
      v-model="activeTab"
      :tabs="tabItems"
      @change="onTabChange"
    />

    <div v-if="loading" class="empty">加载中...</div>
    <div v-else-if="servers.length === 0" class="empty">
      暂无服务器，请先在 Subscriptions 页面添加并连接订阅
    </div>
    <div v-else class="server-list">
      <div
        v-for="server in sortedServers"
        :key="server.tag"
        class="server-card"
        :class="{ active: selectedTag === server.tag }"
        @click="selectServer(server.tag)"
      >
        <div class="server-main">
          <div class="server-tag">{{ server.tag }}</div>
          <div class="server-meta">
            <span class="server-type">{{ server.type }}</span>
            <span class="server-country">{{ server.country }}</span>
          </div>
          <div class="server-address">{{ server.server }}:{{ server.server_port }}</div>
        </div>
        <div class="server-side">
          <div class="ping-badge" :class="pingClass(server.ping)">
            <span v-if="testingTag === server.tag" class="spinner"></span>
            <template v-else>{{ formatPing(server.ping) }}</template>
          </div>
          <div v-if="selectedTag === server.tag" class="selected-mark">✓</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, onActivated, onDeactivated } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '../elements/Icon.vue'
import AppTabs from '../elements/AppTabs.vue'
import {
  GetServers,
  GetGroups,
  GetSubscriptionConfig,
  SelectServer,
  SelectGroup,
  SpeedTestNodes,
  GetStatus,
  GetCurrentSubscriptionID,
  GetSelectedNode,
  GetSpeedTestCache,
  ListSubscriptions
} from '@wailsjs/go/main/App'

const servers = ref([])
const groups = ref([])
const selectedTag = ref('')
const currentGroup = ref('')
const activeTab = ref('All')
const currentSubID = ref('')
const loading = ref(false)
const testing = ref(false)
const testingTag = ref('')
const sortBy = ref('default')
const speedTestAbort = ref(false)

const pingCache = ref({})
let activatedOnce = false

const emit = defineEmits(['error', 'success'])
const route = useRoute()

const tabItems = computed(() => {
  const items = [{ key: 'All', label: 'All' }]
  for (const g of groups.value) {
    items.push({ key: g.tag, label: g.tag })
  }
  return items
})

const filteredServers = computed(() => {
  if (activeTab.value === 'All') {
    return servers.value
  }
  const group = groups.value.find(g => g.tag === activeTab.value)
  if (!group) return servers.value
  const tags = new Set(group.outbounds || [])
  return servers.value.filter(s => tags.has(s.tag))
})

const sortedServers = computed(() => {
  const list = [...filteredServers.value]
  if (sortBy.value === 'country') {
    list.sort((a, b) => a.country.localeCompare(b.country, 'zh-CN'))
  } else if (sortBy.value === 'ping') {
    list.sort((a, b) => {
      const pa = a.ping === -1 ? Infinity : a.ping
      const pb = b.ping === -1 ? Infinity : b.ping
      return pa - pb
    })
  }
  return list
})

const formatPing = (ping) => {
  if (ping === undefined || ping === null) return '-'
  if (ping === -1) return '超时'
  return `${ping}ms`
}

const pingClass = (ping) => {
  if (ping === undefined || ping === null || ping === -1) return 'unknown'
  if (ping < 100) return 'good'
  if (ping < 300) return 'medium'
  return 'bad'
}

const loadServers = async (preserveState = true) => {
  loading.value = true
  try {
    const subs = await ListSubscriptions()
    if (subs.length === 0) {
      servers.value = []
      return
    }

    // 优先使用 settings 中保存的当前订阅，其次使用已连接的订阅
    const savedID = await GetCurrentSubscriptionID()
    let targetSub = subs[0]
    if (savedID) {
      const found = subs.find(s => s.id === savedID)
      if (found) targetSub = found
    }

    // 如果订阅切换了，不保留旧状态
    const subChanged = currentSubID.value !== targetSub.id
    currentSubID.value = targetSub.id

    const [list, groupList, cache] = await Promise.all([
      GetServers(targetSub.id),
      GetGroups(targetSub.id),
      GetSpeedTestCache(targetSub.id)
    ])

    if (!preserveState || subChanged) {
      pingCache.value = {}
    }
    // 合并持久化测速缓存
    for (const tag in cache || {}) {
      pingCache.value[tag] = cache[tag]
    }

    servers.value = (list || []).map(s => ({
      ...s,
      ping: pingCache.value[s.tag] ?? undefined
    }))
    groups.value = groupList || []

    // 尝试从 selected selector 的 default 读取当前选中，不覆盖用户手动选择
    if (!selectedTag.value || subChanged || !servers.value.some(s => s.tag === selectedTag.value)) {
      await loadSelectedTag(targetSub.id)
    }
    await loadCurrentGroup(targetSub.id)
  } catch (e) {
    emit('error', '加载服务器失败: ' + e)
  } finally {
    loading.value = false
  }
}

const loadSelectedTag = async (subID) => {
  try {
    const configJSON = await GetSubscriptionConfig(subID)
    const cfg = JSON.parse(configJSON || '{}')
    // 优先读取用户持久化选择，其次读取 selector default
    const savedTag = await GetSelectedNode(subID)
    if (savedTag && servers.value.some(s => s.tag === savedTag)) {
      selectedTag.value = savedTag
      return
    }
    const selected = (cfg.outbounds || []).find(o => o.type === 'selector' && (o.tag === 'selected' || o.tag === 'select'))
    if (selected && selected.default && servers.value.some(s => s.tag === selected.default)) {
      selectedTag.value = selected.default
    } else if (servers.value.length > 0) {
      selectedTag.value = servers.value[0].tag
    }
  } catch {
    if (servers.value.length > 0) {
      selectedTag.value = servers.value[0].tag
    }
  }
}

const loadCurrentGroup = async (subID) => {
  try {
    const configJSON = await GetSubscriptionConfig(subID)
    const cfg = JSON.parse(configJSON || '{}')
    const final = cfg.route?.final || ''
    currentGroup.value = final
    activeTab.value = final && groups.value.some(g => g.tag === final) ? final : 'All'
  } catch {
    currentGroup.value = ''
    activeTab.value = 'All'
  }
}

const selectServer = async (tag) => {
  if (!currentSubID.value) return
  try {
    await SelectServer(currentSubID.value, tag)
    selectedTag.value = tag
    emit('success', `已切换至 ${tag}`)
  } catch (e) {
    emit('error', '切换服务器失败: ' + e)
  }
}

const onTabChange = async (tag) => {
  activeTab.value = tag
  if (tag === 'All') return
  if (!currentSubID.value) return
  try {
    await SelectGroup(currentSubID.value, tag)
    currentGroup.value = tag
  } catch (e) {
    emit('error', '切换分组失败: ' + e)
  }
}

const handleSpeedTestButton = () => {
  if (testing.value) {
    abortSpeedTest()
  } else {
    runSpeedTest()
  }
}

const runSpeedTest = async () => {
  if (!currentSubID.value) {
    emit('error', '请先添加订阅')
    return
  }
  testing.value = true
  speedTestAbort.value = false
  try {
    const batchSize = 20
    const tags = servers.value.map(s => s.tag)
    for (let i = 0; i < tags.length; i += batchSize) {
      if (speedTestAbort.value) break
      const batch = tags.slice(i, i + batchSize)
      testingTag.value = batch[0]
      try {
        const results = await SpeedTestNodes(currentSubID.value, batch)
        for (const server of servers.value) {
          if (results[server.tag] !== undefined) {
            server.ping = results[server.tag]
            pingCache.value[server.tag] = results[server.tag]
          }
        }
      } catch (e) {
        for (const tag of batch) {
          const server = servers.value.find(s => s.tag === tag)
          if (server) {
            server.ping = -1
            pingCache.value[tag] = -1
          }
        }
      }
    }
    if (!speedTestAbort.value) {
      // 自动选择延迟最低且未超时的节点
      const available = servers.value.filter(s => typeof s.ping === 'number' && s.ping >= 0)
      if (available.length > 0) {
        available.sort((a, b) => a.ping - b.ping)
        const best = available[0]
        if (best.tag !== selectedTag.value) {
          await selectServer(best.tag)
        }
      }
      emit('success', '测速完成')
    }
  } catch (e) {
    emit('error', '测速失败: ' + e)
  } finally {
    testing.value = false
    testingTag.value = ''
    speedTestAbort.value = false
  }
}

const abortSpeedTest = () => {
  speedTestAbort.value = true
}

onMounted(() => {
})

onActivated(() => {
  // keep-alive 激活时仅刷新连接状态和分组，不重新加载服务器列表
  refreshConnectionState()
})

onActivated(async () => {
  if (!activatedOnce) {
    activatedOnce = true
    await loadServers(false)
    return
  }
  await refreshConnectionState()
})

onDeactivated(() => {
  // 离开 Servers 页面时终止正在进行的测速
  abortSpeedTest()
})

onDeactivated(() => {
  abortSpeedTest()
})

const refreshConnectionState = async () => {
  try {
    const subs = await ListSubscriptions()
    if (subs.length === 0) {
      servers.value = []
      return
    }

    const currentStatus = await GetStatus()
    let targetSub = subs[0]
    // 优先使用 settings 中保存的当前订阅
    const currentID = await GetCurrentSubscriptionID()
    if (currentID) {
      const found = subs.find(s => s.id === currentID)
      if (found) targetSub = found
    }

    const subChanged = currentSubID.value !== targetSub.id
    if (subChanged) {
      // 订阅已切换，终止测速并清空缓存后重新加载
      abortSpeedTest()
      pingCache.value = {}
      currentSubID.value = targetSub.id
      await loadServers(false)
      return
    }

    // 仅更新选中状态和分组，保留 ping 和列表
    await loadSelectedTag(targetSub.id)
    await loadCurrentGroup(targetSub.id)
  } catch (e) {
    emit('error', '刷新状态失败: ' + e)
  }
}
</script>

<style scoped>
.servers {
  padding: 20px;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-left label {
  font-size: 14px;
  color: #aaa;
}

.sort-select {
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid #333;
  background: #1a1a2e;
  color: #eee;
  font-size: 14px;
}

.toolbar-right button {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.btn-icon :deep(svg) {
  width: 16px;
  height: 16px;
}

.btn-icon.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.spinner {
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.empty {
  text-align: center;
  padding: 60px 20px;
  color: #666;
  background: #16213e;
  border-radius: 8px;
}

.server-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.server-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  background: #16213e;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s, border 0.2s;
  border: 1px solid transparent;
}

.server-card:hover {
  background: #0f3460;
}

.server-card.active {
  border-color: #00d4aa;
  background: rgba(0, 212, 170, 0.1);
}

.server-tag {
  font-size: 15px;
  font-weight: 600;
  color: #eee;
  margin-bottom: 4px;
}

.server-meta {
  display: flex;
  gap: 8px;
  margin-bottom: 4px;
}

.server-type {
  font-size: 11px;
  padding: 2px 6px;
  background: #0f3460;
  border-radius: 4px;
  color: #aaa;
  text-transform: uppercase;
}

.server-country {
  font-size: 11px;
  padding: 2px 6px;
  background: #1a4a7a;
  border-radius: 4px;
  color: #eee;
}

.server-address {
  font-size: 12px;
  color: #888;
}

.server-side {
  display: flex;
  align-items: center;
  gap: 12px;
}

.ping-badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
  min-width: 48px;
  text-align: center;
}

.ping-badge.good {
  background: #00d4aa;
  color: #1a1a2e;
}

.ping-badge.medium {
  background: #f0a500;
  color: #1a1a2e;
}

.ping-badge.bad {
  background: #e94560;
  color: #fff;
}

.ping-badge.unknown {
  background: #333;
  color: #999;
}

.selected-mark {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #00d4aa;
  color: #1a1a2e;
  border-radius: 50%;
  font-size: 14px;
  font-weight: 600;
}

button {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: opacity 0.2s;
}

button:hover:not(:disabled) {
  opacity: 0.85;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: #00d4aa;
  color: #1a1a2e;
  font-weight: 600;
}

.btn-primary.btn-danger {
  background: #e94560;
  color: #fff;
}
</style>
