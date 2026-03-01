import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";
import type { ReactNode } from "react";
import type { PanelState } from "../hooks/usePanelState";

interface PanelDockProps {
  panels: PanelState[];
  onReorder: (ids: string[]) => void;
  renderPanel: (panel: PanelState) => ReactNode;
}

function SortablePanel({
  id,
  children,
}: {
  id: string;
  children: ReactNode;
}) {
  const { attributes, listeners, setNodeRef, transform, transition } =
    useSortable({ id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div ref={setNodeRef} style={style} className="relative group/sortable">
      <div
        {...attributes}
        {...listeners}
        className="absolute -left-1 top-0 bottom-0 w-4 flex items-start pt-2.5 cursor-grab opacity-0 group-hover/sortable:opacity-100 transition-opacity z-10"
      >
        <GripVertical className="h-3.5 w-3.5 text-muted-foreground" />
      </div>
      <div>{children}</div>
    </div>
  );
}

export default function PanelDock({
  panels,
  onReorder,
  renderPanel,
}: PanelDockProps) {
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;
    if (over && active.id !== over.id) {
      const ids = panels.map((p) => p.id);
      const oldIndex = ids.indexOf(active.id as string);
      const newIndex = ids.indexOf(over.id as string);
      const newIds = [...ids];
      newIds.splice(oldIndex, 1);
      newIds.splice(newIndex, 0, active.id as string);
      onReorder(newIds);
    }
  }

  return (
    <div className="w-80 shrink-0 bg-panel p-3 space-y-2 overflow-y-auto">
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext
          items={panels.map((p) => p.id)}
          strategy={verticalListSortingStrategy}
        >
          {panels.map((panel) => (
            <SortablePanel key={panel.id} id={panel.id}>
              {renderPanel(panel)}
            </SortablePanel>
          ))}
        </SortableContext>
      </DndContext>
    </div>
  );
}
