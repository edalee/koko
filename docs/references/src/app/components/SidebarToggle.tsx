import { PanelRightOpen } from "lucide-react";
import { Button } from "./ui/button";

type SidebarToggleProps = {
  onClick: () => void;
};

export function SidebarToggle({ onClick }: SidebarToggleProps) {
  return (
    <Button
      onClick={onClick}
      size="sm"
      variant="ghost"
      className="text-white/80 hover:bg-white/10 hover:text-white"
      title="Toggle Right Sidebar"
    >
      <PanelRightOpen className="size-4" />
    </Button>
  );
}
