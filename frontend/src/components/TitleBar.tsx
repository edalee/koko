import { ReactNode } from "react";

interface TitleBarProps {
  children?: ReactNode;
}

export default function TitleBar({ children }: TitleBarProps) {
  return (
    <div
      className="flex h-10 shrink-0 items-center bg-titlebar border-b border-border select-none"
      style={{ WebkitAppRegion: "drag" } as React.CSSProperties}
    >
      {/* macOS traffic light spacer (inset title bar) */}
      <div className="w-18 shrink-0" />
      {children}
    </div>
  );
}
