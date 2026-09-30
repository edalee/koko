import { Ban, Loader2, MessageSquare, Plus } from "lucide-react";
import type { main } from "../../wailsjs/go/models";

/** The tab that already has a conversation open. */
export interface ConversationHolder {
  tabId: string;
  label: string; // the tab's slug, or its name when it has none
  connected: boolean;
}

interface ConversationPickerProps {
  // null while loading. An empty list renders nothing at all.
  conversations: main.Conversation[] | null;
  // "" means start a new conversation. null means nothing is chosen yet.
  selected: string | null;
  onSelect: (uuid: string) => void;
  holders: Map<string, ConversationHolder>;
}

function timeAgo(ts: number): string {
  const seconds = Math.floor((Date.now() - ts) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** What choosing a held conversation does, in the words the row shows. */
export function holderAction(h: ConversationHolder): string {
  return h.connected ? `open in ${h.label}` : `reconnect ${h.label}`;
}

function Radio({ on }: { on: boolean }) {
  return (
    <span
      className={`size-3.5 shrink-0 rounded-full border flex items-center justify-center ${
        on ? "border-accent" : "border-white/25"
      }`}
    >
      {on && <span className="size-1.5 rounded-full bg-accent" />}
    </span>
  );
}

/**
 * Step 2 of the session dialog: start fresh, or reopen one of the
 * conversations Claude has stored for the chosen directory.
 */
export default function ConversationPicker({
  conversations,
  selected,
  onSelect,
  holders,
}: ConversationPickerProps) {
  if (conversations !== null && conversations.length === 0) return null;

  // Native radios inside labels: arrow keys move through the group, and the
  // row shows a focus ring when its hidden input has keyboard focus.
  const rowClass = (on: boolean) =>
    `w-full flex items-start gap-2.5 px-3 py-2 text-left rounded-md transition-colors cursor-pointer has-[:focus-visible]:ring-1 has-[:focus-visible]:ring-accent/50 ${
      on ? "bg-accent/10" : "hover:bg-white/[0.04]"
    }`;

  return (
    <fieldset className="space-y-1.5 min-w-0">
      <legend className="mb-1.5 text-xs font-medium text-muted-foreground uppercase tracking-wider">
        Conversations in this directory
      </legend>

      <div className="max-h-64 overflow-y-auto rounded-md border border-white/[0.06] bg-white/[0.02] p-1 space-y-0.5">
        <label className={rowClass(selected === "")}>
          <input
            type="radio"
            name="conversation"
            value=""
            checked={selected === ""}
            onChange={() => onSelect("")}
            className="sr-only"
          />
          <span className="pt-0.5">
            <Radio on={selected === ""} />
          </span>
          <Plus className="size-3.5 mt-0.5 shrink-0 text-muted-foreground" />
          <span className="text-[13px] text-white/90">Start a new conversation</span>
        </label>

        {conversations === null ? (
          <div className="flex items-center gap-2 px-3 py-2 text-[12px] text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" />
            Loading conversations...
          </div>
        ) : (
          conversations.map((c) => {
            const on = selected === c.uuid;
            const holder = holders.get(c.uuid);
            return (
              <label key={c.uuid} className={rowClass(on)}>
                <input
                  type="radio"
                  name="conversation"
                  value={c.uuid}
                  checked={on}
                  onChange={() => onSelect(c.uuid)}
                  className="sr-only"
                />
                <span className="pt-0.5">
                  <Radio on={on} />
                </span>
                <MessageSquare className="size-3.5 mt-0.5 shrink-0 text-muted-foreground" />
                <span className="flex-1 min-w-0">
                  <span className="flex items-baseline gap-2">
                    <span className="text-[13px] text-white/90 truncate">
                      {c.title || "Untitled conversation"}
                    </span>
                    <span className="ml-auto text-[10px] text-tertiary shrink-0">
                      {timeAgo(c.modifiedAt)}
                    </span>
                    {holder && <Ban className="size-3 shrink-0 text-warning" aria-hidden />}
                  </span>
                  {holder ? (
                    <span className="block text-[11px] text-warning truncate">
                      {holderAction(holder)}
                    </span>
                  ) : (
                    c.preview && (
                      <span className="block text-[11px] text-muted-foreground/60 italic truncate">
                        {c.preview}
                      </span>
                    )
                  )}
                </span>
              </label>
            );
          })
        )}
      </div>
    </fieldset>
  );
}
