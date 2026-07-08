export type IconSize = 'xxxs' | 'xxs' | 'xs' | 's' | 'm' | 'l' | 'xl' | 'xxl';

export type IconField = {
  value?: {
    svgCode?: string | null;
    alt?: string | null;
  } | null;
} | null;

export interface IconProps {
  field?: IconField;
  name?: string;
  size?: IconSize | string | null;
  svg?: string | null;
}

export interface IconEmits {
  click: [event: MouseEvent];
}
