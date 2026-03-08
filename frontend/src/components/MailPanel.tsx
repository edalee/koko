import { Circle, Mail } from "lucide-react";

interface MailItem {
  id: string;
  sender: string;
  subject: string;
  time: string;
  unread: boolean;
  type: "github" | "jira" | "deploy";
}

const mockMails: MailItem[] = [
  {
    id: "1",
    sender: "GitHub",
    subject: "PR #342 approved: Add retry logic to claim worker",
    time: "5m ago",
    unread: true,
    type: "github",
  },
  {
    id: "2",
    sender: "Jira",
    subject: "PDAC-891 assigned to you: Fix duplicate claims",
    time: "23m ago",
    unread: true,
    type: "jira",
  },
  {
    id: "3",
    sender: "Deploy Bot",
    subject: "drumstick2 v1.4.2 deployed to production",
    time: "1h ago",
    unread: false,
    type: "deploy",
  },
  {
    id: "4",
    sender: "GitHub",
    subject: "Review requested: Migrate to AlloyDB vector search",
    time: "2h ago",
    unread: false,
    type: "github",
  },
];

function typeColor(type: MailItem["type"]) {
  switch (type) {
    case "github":
      return "from-accent to-accent-dark";
    case "jira":
      return "from-blue-500 to-blue-600";
    case "deploy":
      return "from-purple-500 to-purple-600";
  }
}

export default function MailPanel() {
  return (
    <div className="p-4 space-y-2">
      {mockMails.map((mail) => (
        <div
          key={mail.id}
          className="p-3 bg-white/[0.06] glass-card rounded-xl border border-border inset-highlight hover:bg-white/[0.09] hover:border-white/[0.12] transition-all cursor-pointer"
        >
          <div className="flex items-start gap-3">
            <div
              className={`size-8 rounded bg-gradient-to-r ${typeColor(mail.type)} flex items-center justify-center shrink-0`}
            >
              <Mail className="size-3.5 text-white" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <p className="text-sm text-white truncate flex-1">{mail.subject}</p>
                {mail.unread && <Circle className="size-2 fill-accent text-accent shrink-0" />}
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-muted-foreground">{mail.sender}</span>
                <span className="text-xs text-muted-foreground/50">·</span>
                <span className="text-xs text-muted-foreground">{mail.time}</span>
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function useMailCount() {
  return mockMails.filter((m) => m.unread).length;
}
