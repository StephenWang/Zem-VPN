import type { ModalAnimationRule } from '@/types/utils/modal-animations';

const modalAnimations: ModalAnimationRule = {
  'scale-fade-in': {
    in: {
      from: {
        opacity: 0,
        scale: 0.5,
        translateY: 0,
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        opacity: 1,
        scale: 1,
        translateY: 0
      }
    }
  },
  'center-scale-fade-in': {
    in: {
      from: {
        opacity: 0,
        scale: 0.5,
        top: '50%',
        translateY: '-50%',
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        opacity: 1,
        scale: 1,
        top: '50%',
        translateY: '-50%'
      }
    }
  },
  'bottom-fade-in': {
    in: {
      from: {
        opacity: 0,
        translateY: '100%',
        scale: 0.5,
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        opacity: 1,
        translateY: 0,
        scale: 1
      }
    }
  },
  'right-skew-in': {
    in: {
      from: {
        transform: 'skewX(-30deg)',
        translateX: '150%'
      },
      to: {
        transform: 'skewX(0)',
        translateX: 0
      }
    }
  },
  'top-slide-in': {
    in: {
      from: {
        opacity: 0,
        translateY: '-100%',
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        opacity: 1,
        translateY: 0,
        scale: 1
      }
    }
  },
  'right-slide-in': {
    in: {
      from: {
        translateX: '100%',
        translateY: 0,
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        translateX: 0,
        translateY: 0
      }
    }
  },
  'bottom-slide-in': {
    in: {
      from: {
        opacity: 0,
        translateY: '100%',
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        opacity: 1,
        translateY: 0,
        scale: 1
      }
    }
  },
  'bottom-translate-in': {
    in: {
      from: {
        translateY: '100%',
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        translateY: 0,
        duration: 0.25
      }
    }
  },
  'left-slide-in': {
    in: {
      from: {
        translateX: '-100%',
        translateY: 0,
        ease: 'cubic-bezier(0, 0, 0.2, 1)'
      },
      to: {
        translateX: 0,
        translateY: 0
      }
    }
  }
};
export default modalAnimations;
