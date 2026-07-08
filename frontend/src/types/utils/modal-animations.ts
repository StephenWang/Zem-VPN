import type modalAnimations from '@/utils/modal-animations';

export interface ModalAnimationRule {
  [key: string]: ModalAnimationRuleDetail;
}

export interface ModalAnimationRuleDetail {
  in: ModalAnimationRuleParams;
  out?: ModalAnimationRuleParams;
}

export interface ModalAnimationRuleParams {
  from: {
    [key: string]: any;
  };
  to: {
    [key: string]: any;
  };
}

export type ModalAnimationTypes = keyof typeof modalAnimations;
