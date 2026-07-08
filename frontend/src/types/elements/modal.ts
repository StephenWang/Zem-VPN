import { Ref } from 'vue';
import type { ModalAnimationTypes } from '@/types/utils/modal-animations';

export interface ModalProps {
  removeOnHidden?: boolean;
  closable?: boolean;
  closeLabel?: boolean;
  animation?: ModalAnimationTypes;
  theme?: string;
  fireGtm?: boolean;
  sticky?: boolean;
  isFull?: boolean;
  contentClass?: string | string[] | Record<string, boolean> | null;
  id?: string | null;
  name?: string | null;
  url?: string | null;
  displayName?: string | null;
}

export interface ModalAnimationFrame {
  from: Record<string, unknown>;
  to: Record<string, unknown>;
}

export interface ModalAnimation {
  in: ModalAnimationFrame;
  out?: ModalAnimationFrame;
}

export interface ModalAnimations {
  [key: string]: ModalAnimation;
}

export interface ModalRef {
  el: Ref<HTMLElement | null>;
  open(options?: Record<string, unknown>): Promise<void>;
  close(options?: Record<string, unknown>): Promise<void>;
  show: Ref<boolean>;
}

export interface ModalEmits {
  (e: 'opened'): void;
  (e: 'close-click'): void;
  (e: 'closed'): void;
  (e: 'mask-click'): void;
}
