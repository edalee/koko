import {
  Code2,
  Settings,
  Sparkles,
} from "lucide-react";
import { Button } from "./ui/button";

export function Toolbar() {
  return (
    <div className="flex items-center justify-between px-4 py-2 bg-[#4A1A33] border-b border-white/10">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Code2 className="size-5 text-[#1FF2AB]" />
          <span className="text-sm text-white">KõKõ</span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            className="text-white/80 hover:bg-white/10 hover:text-white"
          >
            <Sparkles className="size-4 mr-1" />
            Claude
          </Button>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button
          variant="ghost"
          size="sm"
          className="text-white/80 hover:bg-white/10 hover:text-white"
        >
          <Settings className="size-4 mr-1" />
          Settings
        </Button>
      </div>
    </div>
  );
}