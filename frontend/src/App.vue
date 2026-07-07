<template>
  <div class="app-layout">
    <Sidebar v-model="sidebarCollapsed" :traffic-data="trafficHistory" />

    <div class="main-wrapper" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
      <header>
        <div class="header-left">
          <h2>{{ currentTitle }}</h2>
          <select v-model="proxyMode" class="mode-select" @change="onModeChange">
            <option value="rule">规则</option>
            <option value="direct">直连</option>
            <option value="system">系统代理</option>
          </select>
        </div>
        <div class="header-right">
          <span class="platform-badge" @click="showPlatformInfo = true">
            {{ platformInfo.os }} {{ platformInfo.arch }}
          </span>
          <span class="admin-badge" :class="isAdmin ? 'ok' : 'warn'">
            {{ isAdmin ? '管理员' : '非管理员' }}
          </span>
          <div class="status-badge" :class="status">
            {{ status === 'connected' ? '已连接' : '未连接' }}
          </div>
        </div>
      </header>

      <!-- 平台信息弹窗 -->
      <div v-if="showPlatformInfo" class="modal" @click="showPlatformInfo = false">
        <div class="modal-content" @click.stop>
          <h3>平台信息</h3>
          <div class="info-grid">
            <div v-for="(value, key) in platformInfo" :key="key">
              <label>{{ key }}:</label>
              <span>{{ value }}</span>
            </div>
          </div>
          <button @click="showPlatformInfo = false">关闭</button>
        </div>
      </div>

      <main>
        <!-- 非管理员警告 -->
        <div v-if="!isAdmin" class="warning-banner">
          ⚠️ 需要管理员/root权限才能使用 TUN 模式，请重启程序以管理员身份运行
        </div>

        <!-- Linux TUN 警告 -->
        <div v-if="platformInfo.os === 'linux' && platformInfo.tun === 'false'" class="warning-banner">
          ⚠️ TUN 设备不可用，请运行: sudo modprobe tun
        </div>

        <router-view v-slot="{ Component }">
          <keep-alive include="Servers">
            <component :is="Component" @error="showError" @success="showSuccess" />
          </keep-alive>
        </router-view>
      </main>
    </div>

    <!-- 提示 -->
    <Transition name="toast">
      <div v-if="error" class="toast error" @click="error = ''">
        <span class="toast-message">{{ error }}</span>
        <span class="toast-close">×</span>
      </div>
    </Transition>
    <Transition name="toast">
      <div v-if="success" class="toast success" @click="success = ''">
        <span class="toast-message">{{ success }}</span>
        <span class="toast-close">×</span>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import Sidebar from './components/Sidebar.vue'
import { IsAdmin, GetPlatformInfo, GetStatus, GetTrafficStats, GetProxyMode, SetProxyMode } from '@wailsjs/go/main/App'

const route = useRoute()
const sidebarCollapsed = ref(false)

const isAdmin = ref(false)
const platformInfo = ref({})
const status = ref('disconnected')
const showPlatformInfo = ref(false)
const error = ref('')
const success = ref('')
const trafficHistory = ref([])
const proxyMode = ref('rule')

let lastUp = 0
let lastDown = 0

const currentTitle = computed(() => {
  return route.name || 'Zem'
})

const showError = (msg) => {
  error.value = msg
  setTimeout(() => error.value = '', 3000)
}

const showSuccess = (msg) => {
  success.value = msg
  setTimeout(() => success.value = '', 3000)
}

onMounted(async () => {
  isAdmin.value = await IsAdmin()
  platformInfo.value = await GetPlatformInfo()
  proxyMode.value = await GetProxyMode()
  await refreshStatus()
  const statusTimer = setInterval(refreshStatus, 2000)
  const trafficTimer = setInterval(refreshTraffic, 1000)
  onUnmounted(() => {
    clearInterval(statusTimer)
    clearInterval(trafficTimer)
  })
})

const refreshStatus = async () => {
  status.value = await GetStatus()
}

const refreshTraffic = async () => {
  try {
    const stats = await GetTrafficStats()
    if (!stats) return
    const up = stats.up || 0
    const down = stats.down || 0
    const upRate = Math.max(0, up - lastUp)
    const downRate = Math.max(0, down - lastDown)
    lastUp = up
    lastDown = down
    trafficHistory.value.push({ up: upRate, down: downRate })
    if (trafficHistory.value.length > 60) {
      trafficHistory.value.shift()
    }
  } catch (e) {
    // 未连接时忽略
  }
}

const onModeChange = async () => {
  try {
    await SetProxyMode(proxyMode.value)
    showSuccess(`已切换为${modeLabel(proxyMode.value)}模式`)
  } catch (e) {
    showError('切换模式失败: ' + e)
    proxyMode.value = await GetProxyMode()
  }
}

const modeLabel = (mode) => {
  switch (mode) {
    case 'rule': return '规则'
    case 'direct': return '直连'
    case 'system': return '系统代理'
    default: return mode
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background: #1a1a2e;
  color: #eee;
}

.app-layout {
  display: flex;
  min-height: 100vh;
}

.main-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

header {
  height: 60px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  border-bottom: 1px solid #333;
  background: #1a1a2e;
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mode-select {
  padding: 6px 10px;
  border-radius: 6px;
  border: 1px solid #333;
  background: #16213e;
  color: #eee;
  font-size: 14px;
  cursor: pointer;
  outline: none;
}

.mode-select:hover {
  border-color: #00d4aa;
}

.mode-select option {
  background: #16213e;
  color: #eee;
}

header h2 {
  font-size: 18px;
  color: #00d4aa;
}

.header-right {
  display: flex;
  gap: 10px;
  align-items: center;
}

.platform-badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  background: #0f3460;
  color: #aaa;
  cursor: pointer;
}

.platform-badge:hover {
  background: #1a4a7a;
}

.admin-badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.admin-badge.ok {
  background: #00d4aa;
  color: #1a1a2e;
}

.admin-badge.warn {
  background: #e94560;
  color: #fff;
}

.status-badge {
  padding: 6px 16px;
  border-radius: 20px;
  font-size: 14px;
  font-weight: 500;
}

.status-badge.disconnected {
  background: #333;
  color: #999;
}

.status-badge.connected {
  background: #00d4aa;
  color: #1a1a2e;
}

main {
  flex: 1;
  overflow-y: auto;
  background: #1a1a2e;
}

.warning-banner {
  background: #e94560;
  color: #fff;
  padding: 12px 16px;
  border-radius: 8px;
  margin: 20px 20px 0;
  font-size: 14px;
}

/* Modal */
.modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: #16213e;
  padding: 24px;
  border-radius: 12px;
  min-width: 300px;
}

.modal-content h3 {
  margin-bottom: 16px;
  color: #00d4aa;
}

.info-grid {
  display: grid;
  gap: 8px;
  margin-bottom: 16px;
}

.info-grid div {
  display: flex;
  justify-content: space-between;
  font-size: 14px;
}

.info-grid label {
  color: #888;
}

.modal-content button {
  padding: 8px 16px;
  border: none;
  border-radius: 6px;
  background: #00d4aa;
  color: #1a1a2e;
  font-weight: 600;
  cursor: pointer;
}

.toast {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 20px;
  border-radius: 8px;
  font-size: 14px;
  cursor: pointer;
  animation: slideUp 0.3s ease;
  z-index: 1000;
  max-width: 80vw;
  max-height: 80vh;
  overflow-y: auto;
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.toast-message {
  word-break: break-word;
  white-space: pre-wrap;
  line-height: 1.5;
}

.toast-close {
  flex-shrink: 0;
  font-size: 18px;
  font-weight: bold;
  line-height: 1;
  margin-top: -2px;
  opacity: 0.8;
}

.toast-close:hover {
  opacity: 1;
}

.toast.error {
  background: #e94560;
  color: #fff;
}

.toast.success {
  background: #00d4aa;
  color: #1a1a2e;
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from,
.toast-leave-to {
  transform: translateX(-50%) translateY(20px);
  opacity: 0;
}

.toast-enter-to,
.toast-leave-from {
  transform: translateX(-50%) translateY(0);
  opacity: 1;
}

@keyframes slideUp {
  from {
    transform: translateX(-50%) translateY(20px);
    opacity: 0;
  }
  to {
    transform: translateX(-50%) translateY(0);
    opacity: 1;
  }
}
</style>
