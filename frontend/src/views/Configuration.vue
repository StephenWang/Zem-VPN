<template>
  <div class="view configuration">
    <section class="settings-card">
      <h2>代理设置</h2>
      <div class="form-group">
        <label for="proxy-port">HTTP/SOCKS 混合代理端口</label>
        <input
          id="proxy-port"
          v-model.number="proxyPort"
          type="number"
          min="1"
          max="65535"
          placeholder="7890"
          @keyup.enter="savePort"
        />
        <p class="hint">默认 7890，修改后重新连接订阅生效</p>
      </div>
      <button @click="savePort" :disabled="saving" class="btn-primary">
        {{ saving ? '保存中...' : '保存' }}
      </button>
    </section>

    <section class="settings-card">
      <h2>TUN 设置</h2>
      <div class="form-group">
        <label for="tun-address">TUN 地址</label>
        <input
          id="tun-address"
          v-model="tunAddress"
          placeholder="172.19.0.1/30"
        />
        <p class="hint">多个地址用逗号分隔，如：172.19.0.1/30,fdfe:dcba:9876::1/126</p>
      </div>
      <div class="form-group">
        <label for="tun-stack">TUN Stack</label>
        <select id="tun-stack" v-model="tunStack">
          <option value="mixed">mixed（推荐）</option>
          <option value="gvisor">gvisor</option>
          <option value="system">system</option>
        </select>
      </div>
      <div class="form-group">
        <label for="tun-mtu">MTU</label>
        <input
          id="tun-mtu"
          v-model.number="tunMtu"
          type="number"
          min="1280"
          max="65535"
        />
      </div>
      <div class="form-row">
        <label class="checkbox">
          <input type="checkbox" v-model="tunAutoRoute" />
          自动路由 (auto_route)
        </label>
        <label class="checkbox">
          <input type="checkbox" v-model="tunStrictRoute" />
          严格路由 (strict_route)
        </label>
        <label class="checkbox">
          <input type="checkbox" v-model="tunEndpointIndependentNAT" />
          Endpoint Independent NAT
        </label>
        <label class="checkbox">
          <input type="checkbox" v-model="tunGSO" />
          GSO
        </label>
      </div>
      <button @click="saveTun" :disabled="savingTun" class="btn-primary">
        {{ savingTun ? '保存中...' : '保存 TUN 设置' }}
      </button>
    </section>

    <section v-if="serviceAvailable" class="settings-card">
      <h2>Windows 服务模式</h2>
      <p class="hint">
        安装服务后，后台服务以管理员权限运行 sing-box 核心，GUI 无需每次弹 UAC。
      </p>
      <div class="service-status">
        <span>服务状态:</span>
        <span :class="['status-badge', serviceStatusClass]">
          {{ serviceStatusText }}
        </span>
      </div>
      <div class="form-row buttons">
        <button
          v-if="!serviceInstalled"
          @click="installService"
          :disabled="serviceLoading"
          class="btn-primary"
        >
          安装服务
        </button>
        <button
          v-else
          @click="uninstallService"
          :disabled="serviceLoading"
          class="btn-danger"
        >
          卸载服务
        </button>
        <button
          v-if="serviceInstalled && !serviceRunning"
          @click="startService"
          :disabled="serviceLoading"
          class="btn-secondary"
        >
          启动服务
        </button>
        <button
          v-if="serviceInstalled && serviceRunning"
          @click="stopService"
          :disabled="serviceLoading"
          class="btn-secondary"
        >
          停止服务
        </button>
      </div>
      <label class="checkbox">
        <input type="checkbox" v-model="serviceMode" @change="toggleServiceMode" />
        启用服务模式（优先通过后台服务连接）
      </label>
    </section>

    <section class="settings-card">
      <h2>关于</h2>
      <div class="info-row">
        <span>应用名称</span>
        <span>Zem</span>
      </div>
      <div class="info-row">
        <span>技术栈</span>
        <span>Go + Wails v2 + Vue 3 + sing-box</span>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import {
  GetProxyPort,
  SetProxyPort,
  GetTunSettings,
  SetTunSettings,
  IsServiceModeAvailable,
  IsServiceInstalled,
  IsServiceRunning,
  InstallService,
  UninstallService,
  StartService,
  StopService,
  GetServiceMode,
  SetServiceMode,
  RefreshServiceClient
} from '@wailsjs/go/main/App'

const proxyPort = ref(7890)
const saving = ref(false)

const tunAddress = ref('172.19.0.1/30')
const tunStack = ref('mixed')
const tunMtu = ref(9000)
const tunAutoRoute = ref(true)
const tunStrictRoute = ref(true)
const tunEndpointIndependentNAT = ref(true)
const tunGSO = ref(true)
const savingTun = ref(false)

const serviceAvailable = ref(false)
const serviceInstalled = ref(false)
const serviceRunning = ref(false)
const serviceLoading = ref(false)
const serviceMode = ref(false)

const emit = defineEmits(['error', 'success'])

onMounted(async () => {
  try {
    proxyPort.value = await GetProxyPort()
  } catch (e) {
    emit('error', '读取代理端口失败: ' + e)
  }

  try {
    const tun = await GetTunSettings()
    tunAddress.value = (tun.address || []).join(',')
    tunStack.value = tun.stack || 'mixed'
    tunMtu.value = tun.mtu || 9000
    tunAutoRoute.value = tun.auto_route !== false
    tunStrictRoute.value = tun.strict_route !== false
    tunEndpointIndependentNAT.value = tun.endpoint_independent_nat !== false
    tunGSO.value = tun.gso !== false
  } catch (e) {
    emit('error', '读取 TUN 设置失败: ' + e)
  }

  await refreshServiceStatus()
})

const serviceStatusText = computed(() => {
  if (!serviceInstalled.value) return '未安装'
  if (serviceRunning.value) return '运行中'
  return '已停止'
})

const serviceStatusClass = computed(() => {
  if (!serviceInstalled.value) return 'warn'
  if (serviceRunning.value) return 'ok'
  return 'warn'
})

const refreshServiceStatus = async () => {
  try {
    serviceAvailable.value = await IsServiceModeAvailable()
    if (serviceAvailable.value) {
      serviceInstalled.value = await IsServiceInstalled()
      serviceRunning.value = await IsServiceRunning()
      serviceMode.value = await GetServiceMode()
    }
  } catch (e) {
    console.error('refresh service status:', e)
  }
}

const savePort = async () => {
  const port = Number(proxyPort.value)
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    emit('error', '请输入 1-65535 之间的有效端口')
    return
  }
  saving.value = true
  try {
    await SetProxyPort(port)
    emit('success', '代理端口已保存')
  } catch (e) {
    emit('error', '保存失败: ' + e)
  } finally {
    saving.value = false
  }
}

const saveTun = async () => {
  const addresses = tunAddress.value
    .split(',')
    .map(s => s.trim())
    .filter(Boolean)

  const mtu = Number(tunMtu.value)
  if (!Number.isInteger(mtu) || mtu < 1280 || mtu > 65535) {
    emit('error', 'MTU 必须在 1280-65535 之间')
    return
  }
  if (addresses.length === 0) {
    emit('error', 'TUN 地址不能为空')
    return
  }

  savingTun.value = true
  try {
    await SetTunSettings({
      address: addresses,
      stack: tunStack.value,
      mtu: mtu,
      auto_route: tunAutoRoute.value,
      strict_route: tunStrictRoute.value,
      endpoint_independent_nat: tunEndpointIndependentNAT.value,
      gso: tunGSO.value
    })
    emit('success', 'TUN 设置已保存，重新连接后生效')
  } catch (e) {
    emit('error', '保存 TUN 设置失败: ' + e)
  } finally {
    savingTun.value = false
  }
}

const toggleServiceMode = async () => {
  try {
    await SetServiceMode(serviceMode.value)
    emit('success', serviceMode.value ? '已启用服务模式' : '已禁用服务模式')
  } catch (e) {
    emit('error', '设置服务模式失败: ' + e)
    serviceMode.value = !serviceMode.value
  }
}

const installService = async () => {
  serviceLoading.value = true
  try {
    await InstallService()
    await refreshServiceStatus()
    emit('success', '服务安装成功')
  } catch (e) {
    emit('error', '安装服务失败: ' + e)
  } finally {
    serviceLoading.value = false
  }
}

const uninstallService = async () => {
  serviceLoading.value = true
  try {
    await UninstallService()
    await refreshServiceStatus()
    emit('success', '服务已卸载')
  } catch (e) {
    emit('error', '卸载服务失败: ' + e)
  } finally {
    serviceLoading.value = false
  }
}

const startService = async () => {
  serviceLoading.value = true
  try {
    await StartService()
    await refreshServiceStatus()
    await RefreshServiceClient()
    emit('success', '服务已启动')
  } catch (e) {
    emit('error', '启动服务失败: ' + e)
  } finally {
    serviceLoading.value = false
  }
}

const stopService = async () => {
  serviceLoading.value = true
  try {
    await StopService()
    await refreshServiceStatus()
    emit('success', '服务已停止')
  } catch (e) {
    emit('error', '停止服务失败: ' + e)
  } finally {
    serviceLoading.value = false
  }
}
</script>

<style scoped>
.view {
  padding: 20px;
}

.settings-card {
  background: #16213e;
  border-radius: 8px;
  padding: 20px;
  margin-bottom: 20px;
}

.settings-card h2 {
  font-size: 18px;
  margin-bottom: 16px;
  color: #00d4aa;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 8px;
  font-size: 14px;
  color: #aaa;
}

.form-group input,
.form-group select {
  width: 100%;
  max-width: 320px;
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

.form-row {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 16px;
  align-items: center;
}

.form-row.buttons {
  gap: 10px;
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

.hint {
  margin-top: 6px;
  font-size: 12px;
  color: #666;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 10px 0;
  border-bottom: 1px solid #333;
  font-size: 14px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-row span:first-child {
  color: #888;
}

.service-status {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  font-size: 14px;
}

.status-badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge.ok {
  background: #00d4aa;
  color: #1a1a2e;
}

.status-badge.warn {
  background: #e94560;
  color: #fff;
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

.btn-secondary {
  background: #0f3460;
  color: #eee;
}

.btn-danger {
  background: #e94560;
  color: #fff;
}
</style>
