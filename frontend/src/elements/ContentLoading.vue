<script setup lang="ts">
import Spinner from "./Spinner.vue";
import {ContentLoadingProps} from "../types/elements/content-loading.ts";

defineProps<ContentLoadingProps>();
</script>

<template>
<div class="e-content-loading" :class="[{ loading }]">
  <div class="e-content-loading__loading" v-show="loading">
    <spinner v-if="loading" />
  </div>
  <slot />
</div>
</template>

<style lang="scss">
.e-content-loading {
  position: relative;
  &__loading {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    display: flex;
    justify-content: center;
    align-items: center;
    background: rgba(255, 255, 255, 0.6);
  }
  &.loading {
    min-height: 120px;
  }
}
</style>