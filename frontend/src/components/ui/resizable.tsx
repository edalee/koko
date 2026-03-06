import { Group, Panel, Separator } from "react-resizable-panels";
import { cn } from "../../lib/utils";

function ResizablePanelGroup({ className, ...props }: React.ComponentProps<typeof Group>) {
  return (
    <Group
      className={cn("flex h-full w-full data-[orientation=vertical]:flex-col", className)}
      {...props}
    />
  );
}

function ResizablePanel({ ...props }: React.ComponentProps<typeof Panel>) {
  return <Panel {...props} />;
}

function ResizableHandle({ className, ...props }: React.ComponentProps<typeof Separator>) {
  return (
    <Separator
      className={cn(
        "relative flex w-px items-center justify-center bg-border hover:bg-accent/50 transition-colors data-[orientation=vertical]:h-px data-[orientation=vertical]:w-full after:absolute after:inset-y-0 after:-inset-x-1 after:content-[''] data-[orientation=vertical]:after:inset-x-0 data-[orientation=vertical]:after:-inset-y-1",
        className,
      )}
      {...props}
    />
  );
}

export { ResizablePanelGroup, ResizablePanel, ResizableHandle };
