import { IconSize } from './site-icon';

export type SiteButtonType = 'primary' | 'secondary' | 'tertiary' | 'liquid';
export type SiteButtonSize = 's' | 'm' | 'l' | 'xl';
export interface SiteButtonProps {
  type?: SiteButtonType;
  theme?: string;
  href?: string;
  target?: string;
  defaultType?: SiteButtonType;
  size?: SiteButtonSize;
  prevIcon?: string;
  prevIconSize?: IconSize;
  icon?: string;
  iconSize?: IconSize;
  loading?: boolean;
  disabled?: boolean;
  borderStyle?: string;
}
