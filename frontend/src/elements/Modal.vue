<script setup lang="ts">
import {computed, inject, nextTick, ref } from "vue";
import type { Ref } from "vue";
import {ModalEmits, ModalProps, ModalRef} from "../types/elements/modal.ts";
import modalAnimations from "../utils/modal-animations.ts";
import gsap from 'gsap';

const props = withDefaults(defineProps<ModalProps>(), {
  aniKey: 'center-scale-fade-in',
  closable: true,
  sticky: true
});
const emit = defineEmits<ModalEmits>();
defineSlots<{
  default():void;
  header():void;
}>();
const chatRootEl = inject<Ref<HTMLElement | null>>('chatRootEl');
const chatPanelEl = inject<Ref<HTMLElement | null>>('chatPanelEl');
const teleportTarget = computed(() => {
  const target = props.container ?? chatPanelEl?.value ?? chatRootEl?.value;
  return target || 'body';
});
const rootEl = ref<HTMLDivElement>();
const maskEl = ref<HTMLDivElement>();
const contentEl = ref<HTMLDivElement>();
const show = ref(false);
const rootClass = computed(() => {
  const result: string[] = ['e-modal'];
  if (teleportTarget.value instanceof HTMLElement) {
    if(teleportTarget.value.classList.contains('ai-chatbot-root')) {
      result.push('universal');
    }
  }
  return result;
})
const open = (options?: Record<string, unknown>): Promise<void> => {
  show.value = true;
  const aniKey = props.aniKey as keyof typeof modalAnimations;
  const animations = modalAnimations[aniKey];
  return new Promise((resolve) => {
    nextTick(() => {
      if (!contentEl.value || !maskEl.value) {
        resolve();
        return;
      }
      gsap.killTweensOf([contentEl.value, maskEl.value]);
      const tl = gsap.timeline({
        paused: true
      });
      tl.to(
          maskEl.value,
          {
            opacity: 1,
            onComplete() {
              resolve();
              emit('opened');
            }
          },
          'first'
      );
      tl.fromTo(contentEl.value, animations.in.from, { ...(options || {}), ...animations.in.to }, 'first').play();
    });
  });
};

const close = (options?: Record<string, unknown>): Promise<void> => {
  console.log('close called 1')
  return new Promise((resolve) => {
    if (!contentEl.value || !maskEl.value) {
      resolve();
      return;
    }
    console.log('close called 2')
    const aniKey = props.aniKey as keyof typeof modalAnimations;
    const animations = modalAnimations[aniKey];
    console.log('animations', aniKey, animations);
    const outAnimations = animations.out ?? {
      from: animations.in.to,
      to: animations.in.from
    };
    gsap.killTweensOf([contentEl.value, maskEl.value]);
    const tl = gsap.timeline({
      paused: true
    });
    tl.fromTo(contentEl.value, outAnimations.from, { ...(options || {}), ...outAnimations.to }, 'first');
    tl.to(
        maskEl.value,
        {
          opacity: 0,
          onComplete() {
            show.value = false;
            resolve();
            emit('closed');
          }
        },
        'first'
    );
    tl.play();
  });
};

const onMaskClick = () => {
  emit('mask-click');
  if (!props.sticky) {
    emit('close-click');
    close();
  }
};

const onClose = () => {
  emit('close-click');
  close();
};

defineExpose<ModalRef>({
  el: rootEl,
  open,
  close,
  show
});
</script>

<template>
  <teleport :to="teleportTarget">
    <div :class="rootClass" v-bind="$attrs" ref="rootEl" v-if="show">
      <div class="e-modal__mask" @click="onMaskClick" ref="maskEl" />
      <div class="e-modal__content" ref="contentEl">
        <div class="e-modal__header" v-if="$slots.header || closable">
          <div class="e-modal__header-main">
            <slot name="header" />
          </div>
          <div class="e-modal__close" @click="onClose" v-if="closable">
            <icon name="smart-close" size="s" />
          </div>
        </div>
        <div class="e-modal__body">
          <slot />
        </div>
      </div>
    </div>
  </teleport>
</template>

<style lang="scss">
.e-modal {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 999999;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding-bottom: 120px;
  &__mask {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.1);
    backdrop-filter: blur(10px);
  }
  &__header {
    flex-shrink: 0;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    &-main {
      flex: 1;
    }
  }
  &__close {
    flex-shrink: 0;
    line-height: 0;
    cursor: pointer;
  }
  &__content {
    position: relative;
    z-index: 1;
    width: 96%;
    max-width: 680px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    border: 1px solid #fff;
    border-radius: 16px;
    color: #000;
    background:linear-gradient(189deg, rgba(255, 255, 255, 0.60) -7.38%, #FFF 48.14%), conic-gradient(from 211deg at 52.88% 53.33%, rgba(243, 255, 246, 0.50) 0deg, rgba(149, 187, 241, 0.50) 144.0000021457672deg, rgba(189, 245, 206, 0.50) 270deg, rgba(240, 253, 255, 0.50) 360deg), radial-gradient(65.92% 88.67% at 49.73% 50%, rgba(194, 255, 198, 0.50) 0%, rgba(97, 205, 176, 0.43) 50%, rgba(41, 98, 255, 0.35) 100%), #FFF;
    box-shadow: 0 1px 11.4px rgba(118, 144, 236, 0.37);
  }
  &__body {
    max-height: 72%;
    overflow: auto;
  }
  h1,
  h2,
  h3,
  h4,
  h5,
  p {
    margin: 0;
  }
  &.universal {
    position: fixed;
  }
}
.full {
  .e-modal__content {
    position: absolute;
    top: 50%;
    transform: translateY(-50%) !important;;
  }
}
</style>