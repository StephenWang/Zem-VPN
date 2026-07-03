<template>
  <aside class="sidebar" :class="{ collapsed: isCollapsed }">
    <div class="sidebar-header">
      <h1 class="logo" v-show="!isCollapsed">Zem</h1>
      <button class="toggle-btn" @click="toggle" :title="isCollapsed ? '展开' : '收起'">
        <Icon v-if="isCollapsed" name="ChevronRight" />
        <Icon v-else name="ChevronLeft" />
      </button>
    </div>

    <nav class="menu">
      <router-link
        v-for="item in menus"
        :key="item.path"
        :to="item.path"
        class="menu-item"
        active-class="active"
      >
        <Icon :name="item.icon" class="menu-icon" />
        <span class="label" v-show="!isCollapsed">{{ item.label }}</span>
      </router-link>
    </nav>

    <TrafficChart :collapsed="isCollapsed" :data="trafficData" />
  </aside>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import Icon from '../elements/Icon.vue'
import TrafficChart from './TrafficChart.vue'

const props = defineProps({
  modelValue: Boolean,
  trafficData: {
    type: Array,
    default: () => []
  }
})
const emit = defineEmits(['update:modelValue'])

const isCollapsed = ref(props.modelValue)

const menus = [
  { path: '/servers', label: 'Servers', icon: 'Server' },
  { path: '/subscriptions', label: 'Subscriptions', icon: 'Subscriptions' },
  { path: '/configuration', label: 'Configuration', icon: 'Configuration' }
]

const toggle = () => {
  isCollapsed.value = !isCollapsed.value
  emit('update:modelValue', isCollapsed.value)
}

const handleResize = () => {
  if (window.innerWidth < BREAKPOINT) {
    isCollapsed.value = true
  } else {
    isCollapsed.value = false
  }
  emit('update:modelValue', isCollapsed.value)
}

watch(() => props.modelValue, (val) => {
  isCollapsed.value = val
})

onMounted(() => {
  handleResize()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.sidebar {
  position: sticky;
  top: 0;
  align-self: flex-start;
  width: 200px;
  height: 100vh;
  background: #16213e;
  border-right: 1px solid #333;
  display: flex;
  flex-direction: column;
  transition: width 0.25s ease;
  flex-shrink: 0;
  overflow-y: auto;
}

.sidebar.collapsed {
  width: 60px;
}

.sidebar-header {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  border-bottom: 1px solid #333;
}

.logo {
  font-size: 22px;
  color: #00d4aa;
  margin: 0;
}

.toggle-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: #888;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.toggle-btn:hover {
  background: #0f3460;
  color: #eee;
}

.toggle-btn :deep(svg) {
  width: 18px;
  height: 18px;
}

.menu {
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 6px;
  color: #aaa;
  text-decoration: none;
  transition: background 0.2s, color 0.2s;
  white-space: nowrap;
  overflow: hidden;
}

.menu-item:hover {
  background: #0f3460;
  color: #eee;
}

.menu-item.active {
  background: #00d4aa;
  color: #1a1a2e;
  font-weight: 600;
}

.menu-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.menu-icon :deep(svg) {
  width: 20px;
  height: 20px;
}

.label {
  font-size: 14px;
}
</style>
