export interface TooltipState {
  show: boolean;
  text: string;
}

export interface ToolTipShowData {
  text: string;
  target: HTMLElement;
}

export interface TooltipProps {
  target?: HTMLElement | null;
  visible: boolean;
  type?: string;
  message: string;
}
