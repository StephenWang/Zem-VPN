export interface VideoPlayerProps extends IVideoPlayerProps {
  src: string;
  poster?: string;
}
export interface IVideoPlayerProps {
  preload?: string;
  autoplay?: boolean;
  loop?: boolean;
  controls?: boolean;
  showBigButton?: boolean;
  asBg?: boolean;
}
export interface VideoPlayerRef {
  play(currentTime?: number): void;
  pause(): void;
  updateHovered(value: boolean): void;
}
