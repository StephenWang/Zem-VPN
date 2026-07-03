<script setup lang="ts">
import { ref, watch, nextTick, inject, Ref } from "vue";
import {
  computePosition,
  autoUpdate,
  offset,
  flip,
  shift,
  arrow,
} from "@floating-ui/dom";

interface Props {
  target?: HTMLElement | null;
  visible: boolean;
  type?: string;
  message: string;
}

const props = defineProps<Props>();
const chatRootEl = inject<Ref<HTMLElement | null>>('chatRootEl');
const tooltipRef = ref<HTMLElement>();
const arrowRef = ref<HTMLElement>();

const x = ref(0);
const y = ref(0);

let cleanup: (() => void) | null = null;

const update = async () => {
  if (
      !props.target ||
      !tooltipRef.value ||
      !arrowRef.value
  ) {
    return;
  }

  const {
    x: posX,
    y: posY,
    middlewareData,
    placement,
  } = await computePosition(
      props.target,
      tooltipRef.value,
      {
        placement: "bottom",
        middleware: [
          offset(8),
          flip(),
          shift({
            padding: 8,
          }),
          arrow({
            element: arrowRef.value,
          }),
        ],
      }
  );

  x.value = posX;
  y.value = posY;

  const arrowData = middlewareData.arrow;

  if (arrowData) {
    Object.assign(arrowRef.value.style, {
      left:
          arrowData.x != null
              ? `${arrowData.x}px`
              : "",
      top:
          arrowData.y != null
              ? `${arrowData.y}px`
              : "",
    });
  }

  tooltipRef.value.setAttribute(
      "data-placement",
      placement
  );
};

watch(
    () => props.visible,
    async (visible) => {
      cleanup?.();

      if (!visible) return;

      await nextTick();

      void update();

      cleanup = autoUpdate(
          props.target!,
          tooltipRef.value!,
          update
      );
    }
);
</script>

<template>
  <Teleport :to="chatRootEl">
    <div
        v-if="visible"
        ref="tooltipRef"
        class="e-tooltip"
        :class="type"
        :style="{
        left: x + 'px',
        top: y + 'px',
      }"
    >
      {{ message }}
      <div
          ref="arrowRef"
          class="e-tooltip__arrow"
      />
    </div>
  </Teleport>
</template>

<style lang="scss">
.e-tooltip {
  $c: &;
  position: fixed;
  z-index: 9999999;
  padding: 8px 12px;
  background: #d93025;
  color: white;
  border-radius: 8px;

  max-width: 250px;

  pointer-events: none;
  &__arrow {
    position: absolute;

    width: 8px;
    height: 8px;

    background: inherit;

    transform: rotate(45deg);
  }
  &[data-placement^="top"] {
    #{$c}__arrow {
      bottom: -4px;
    }
  }
  &[data-placement^="bottom"] {
    #{$c}__arrow {
      top: -4px;
    }
  }
  &[data-placement^="left"] {
    #{$c}__arrow {
      right: -4px;
    }
  }
  &[data-placement^="right"] {
    #{$c}__arrow {
      left: -4px;
    }
  }
}
</style>