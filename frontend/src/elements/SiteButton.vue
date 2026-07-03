<script setup lang="ts">
import { computed, useSlots } from "vue";
import {SiteButtonProps} from "../types/elements/site-button.ts";

const props = withDefaults(defineProps<SiteButtonProps>(), {
  target: '_blank',
  type: 'primary',
  theme: 'blue'
});
const slots = useSlots();
const rootClass = computed(() => ([
  'e-site-button',
  props.type,
  {
    [`theme-${props.theme}`]: props.theme,
    'only-icon': !slots.default
  }
]))
const rootStyle = computed(() => {
  return {
    '--border-style': props.borderStyle
  }
})
</script>

<template>
<component :class="rootClass" :style="rootStyle" :is="href?'a':'button'" :type="href?undefined:'button'" :href="href" :target="target" :disabled="disabled">
  <Icon class="e-site-button__icon" :name="prevIcon" size="s" />
  <slot />
  <Icon class="e-site-button__icon" :name="icon" size="s" />
</component>
</template>

<style lang="scss">
.e-site-button {
  font-size: 14px;
  font-weight: 500;
  border-radius: 666px;
  border: none;
  padding: 8px 16px;
  cursor: pointer;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 4px;
  &__icon {
    flex-shrink: 0;
  }
  &.only-icon {
    padding: 8px;
  }
  &.primary {
    &.theme {
      &-blue {
        background: #0831c2;
        color: #fff;
        transition: background 0.2s;
        &:hover:not(:disabled) {
          background: #2563eb;
        }
      }
    }
    &:disabled {
      background: #ccc;
      color: #949494;
      opacity: 0.6;
      cursor: not-allowed;
    }
  }
  &.secondary {
    border: 1px #0831c2;
    background: transparent;
    color: #0831c2;
    border-style: var(--border-style, solid);
    &:disabled {
      background: #ccc;
      color: #949494;
      opacity: 0.6;
      cursor: not-allowed;
    }
  }
  &.tertiary {
    color: #0831c2;
    padding: 8px 0;
    background: transparent;
    &:hover:not(:disabled) {
      color: #2563eb;
    }
    &:disabled {
      color: #949494;
      opacity: 0.6;
      cursor: not-allowed;
    }
    &.theme {
      &-blue {
        color: #0831c2;
        transition: all 0.2s;
        &:hover:not(:disabled) {
          color: #2563eb;
        }
      }
    }
  }
  &:disabled {
    color: #949494;
    opacity: 0.6;
    cursor: not-allowed;
  }
}
</style>