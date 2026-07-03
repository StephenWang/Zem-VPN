<template>
  <div class="app-tabs">
    <button
      v-for="tab in tabs"
      :key="tab.key"
      class="app-tab"
      :class="{ active: modelValue === tab.key }"
      @click="selectTab(tab.key)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>

<script setup>
const props = defineProps({
  tabs: {
    type: Array,
    required: true,
    default: () => []
  },
  modelValue: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue', 'change'])

const selectTab = (key) => {
  if (key === props.modelValue) return
  emit('update:modelValue', key)
  emit('change', key)
}
</script>

<style scoped>
.app-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.app-tab {
  padding: 6px 12px;
  border-radius: 6px;
  background: #16213e;
  color: #aaa;
  font-size: 13px;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
}

.app-tab:hover {
  background: #0f3460;
}

.app-tab.active {
  background: rgba(0, 212, 170, 0.15);
  color: #00d4aa;
  border-color: #00d4aa;
}
</style>
