<template>
  <button
    :class="rootClass"
    :disabled="disabled || loading"
    @click="handleClick"
  >
    <span v-if="loading && iconPosition === 'left'" class="e-site-button__loading-spinner"></span>
    <Icon v-if="icon && !loading && iconPosition === 'left'" :name="icon" class="e-site-button__icon" />
    <slot />
    <span v-if="loading && iconPosition === 'right'" class="e-site-button__loading-spinner"></span>
    <Icon v-if="icon && !loading && iconPosition === 'right'" :name="icon" class="e-site-button__icon" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  type: {
    type: String,
    default: 'primary',
    validator: (value: string) => ['primary', 'secondary', 'tertiary'].includes(value)
  },
  theme: {
    type: String,
    default: 'default',
    validator: (value: string) => ['default', 'teal', 'purple', 'blue', 'danger'].includes(value)
  },
  size: {
    type: String,
    default: 'medium',
    validator: (value: string) => ['small', 'medium', 'large'].includes(value)
  },
  disabled: {
    type: Boolean,
    default: false
  },
  loading: {
    type: Boolean,
    default: false
  },
  icon: String,
  iconPosition: {
    type: String,
    default: 'left',
    validator: (value: string) => ['left', 'right'].includes(value)
  }
})

const emit = defineEmits(['click'])

const rootClass = computed(() => ([
  'e-site-button',
  props.type,
  {
    [`theme-${props.theme}`]: props.theme,
    [`size-${props.size}`]: props.size,
    [`icon-position-${props.iconPosition}`]: props.iconPosition,
    'loading': props.loading,
    'disabled': props.disabled
  }
]))

const handleClick = (e: MouseEvent) => {
  if (!props.disabled && !props.loading) {
    emit('click', e)
  }
}
</script>

<style scoped>
.e-site-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-weight: 500;
  transition: all 0.2s ease;
  user-select: none;
}

.e-site-button:hover:not(.disabled):not(.loading) {
  opacity: 0.9;
  transform: translateY(-1px);
}

.e-site-button:active:not(.disabled):not(.loading) {
  transform: translateY(0);
}

.e-site-button.disabled,
.e-site-button.loading {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Size modifiers */
.e-site-button.size-small {
  padding: 6px 12px;
  font-size: 13px;
}

.e-site-button.size-medium {
  padding: 8px 16px;
  font-size: 14px;
}

.e-site-button.size-large {
  padding: 12px 24px;
  font-size: 16px;
}

/* Elements */
.e-site-button__loading-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid transparent;
  border-top-color: currentColor;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

.e-site-button__icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  color: currentColor;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Type + Theme combinations */

/* Primary type */
.e-site-button.primary.theme-default,
.e-site-button.primary.theme-teal {
  background: #00d4aa;
  color: #1a1a2e;
}

.e-site-button.primary.theme-purple {
  background: #9b59b6;
  color: #fff;
}

.e-site-button.primary.theme-blue {
  background: #3498db;
  color: #fff;
}

.e-site-button.primary.theme-danger {
  background: #e94560;
  color: #fff;
}

/* Secondary type - 没有背景色，主题控制文字颜色、图标颜色和边框颜色 */
.e-site-button.secondary {
  background: transparent;
  border-width: 1px;
  border-style: solid;
}

.e-site-button.secondary.theme-default {
  color: #eee;
  border-color: #333;
}

.e-site-button.secondary.theme-teal {
  color: #00d4aa;
  border-color: #00d4aa;
}

.e-site-button.secondary.theme-purple {
  color: #9b59b6;
  border-color: #9b59b6;
}

.e-site-button.secondary.theme-blue {
  color: #3498db;
  border-color: #3498db;
}

.e-site-button.secondary.theme-danger {
  color: #e94560;
  border-color: #e94560;
}

/* Tertiary type - 没有背景没有边框，主题控制文字颜色和图标颜色 */
.e-site-button.tertiary {
  background: transparent;
  border: none;
}

.e-site-button.tertiary.theme-default {
  color: #aaa;
}

.e-site-button.tertiary.theme-default:hover:not(.disabled):not(.loading) {
  background: rgba(255,255,255,0.05);
  color: #eee;
}

.e-site-button.tertiary.theme-teal {
  color: #00d4aa;
}

.e-site-button.tertiary.theme-teal:hover:not(.disabled):not(.loading) {
  background: rgba(0,212,170,0.1);
}

.e-site-button.tertiary.theme-purple {
  color: #9b59b6;
}

.e-site-button.tertiary.theme-purple:hover:not(.disabled):not(.loading) {
  background: rgba(155,89,182,0.1);
}

.e-site-button.tertiary.theme-blue {
  color: #3498db;
}

.e-site-button.tertiary.theme-blue:hover:not(.disabled):not(.loading) {
  background: rgba(52,152,219,0.1);
}

.e-site-button.tertiary.theme-danger {
  color: #e94560;
}

.e-site-button.tertiary.theme-danger:hover:not(.disabled):not(.loading) {
  background: rgba(233,69,96,0.1);
}
</style>
