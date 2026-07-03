<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'

defineOptions({
  inheritAttrs: false,
})

type IconField = {
  value?: {
    svgCode?: string | null
    alt?: string | null
  } | null
} | null

const props = withDefaults(defineProps<{
  field?: IconField
  name?: string
  size?: string | null
  svg?: string | null
}>(), {
  field: null,
  name: undefined,
  size: null,
  svg: null,
})

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const iconModules = import.meta.glob('../icons/*.vue', {
  eager: true,
  import: 'default',
}) as Record<string, Component>

function toPascalCase(value: string) {
  return value
    .replace(/^Icon(?=[A-Z0-9_-]|$)/, '')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map(part => `${part.charAt(0).toUpperCase()}${part.slice(1).toLowerCase()}`)
    .join('')
}

const localIconName = computed(() => {
  if (!props.name) return null

  const normalized = toPascalCase(props.name)
  return normalized ? `Icon${normalized}` : null
})

const LocalIcon = computed(() => {
  if (!localIconName.value) return null
  return iconModules[`../icons/${localIconName.value}.vue`] ?? null
})

const renderedSvg = computed(() => {
  if (props.svg?.trim()) return props.svg

  const { svgCode, alt } = props.field?.value ?? {}
  if (alt && /^<svg(.|\s)*<\/svg>$/i.test(alt.trim())) return alt
  return svgCode || null
})

const iconClass = computed(() => [
  'e-icon',
  props.size ? `size-${props.size}` : null,
])

function handleClick(event: MouseEvent) {
  emit('click', event)
}
</script>

<template>
  <component
    :is="LocalIcon"
    v-if="LocalIcon"
    v-bind="$attrs"
    :class="iconClass"
    aria-hidden="true"
    focusable="false"
    @click="handleClick"
  />
  <span
    v-else-if="renderedSvg"
    v-bind="$attrs"
    :class="iconClass"
    aria-hidden="true"
    @click="handleClick"
    v-html="renderedSvg"
  ></span>
</template>
