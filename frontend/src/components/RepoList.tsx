import { Plus, X } from "lucide-react";
import { useState } from "react";
import { cn } from "../lib/utils";

const inputClass =
  "px-2 py-1 text-xs bg-white/[0.04] border border-white/[0.06] rounded-md text-white outline-none focus:border-accent/40";

// RepoList adds and removes "owner/repo" names. A bare name means <org>/<name>,
// where org is the team's org. With no org, it needs owner/repo.
export default function RepoList({
  repos,
  org,
  empty,
  onChange,
}: {
  repos: string[];
  org: string;
  empty: string;
  onChange: (next: string[]) => void;
}) {
  const [draft, setDraft] = useState("");
  const [problem, setProblem] = useState<string | null>(null);

  const add = () => {
    const name = draft.trim();
    setProblem(null);
    if (!name) return;
    if (!/^[A-Za-z0-9._-]+(\/[A-Za-z0-9._-]+)?$/.test(name)) {
      setProblem(org ? "Use owner/repo, or a bare repo name" : "Use owner/repo");
      return;
    }
    if (!name.includes("/") && !org) {
      setProblem("Use owner/repo, or set the team first");
      return;
    }
    const full = name.includes("/") ? name : `${org}/${name}`;
    if (repos.some((r) => r.toLowerCase() === full.toLowerCase())) {
      setProblem("Already in the list");
      return;
    }
    onChange([...repos, full]);
    setDraft("");
  };

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1.5">
        {repos.map((repo) => (
          <span
            key={repo}
            className="inline-flex items-center gap-1 px-2 py-1 text-[11px] rounded-md bg-white/[0.05] border border-white/[0.08] text-white/85 font-mono"
          >
            {repo}
            <button
              type="button"
              onClick={() => onChange(repos.filter((r) => r !== repo))}
              className="opacity-50 hover:opacity-100 hover:text-error transition-opacity"
              title={`Remove ${repo}`}
            >
              <X className="size-3" />
            </button>
          </span>
        ))}
        {repos.length === 0 && <span className="text-[10px] text-tertiary">{empty}</span>}
      </div>
      <div className="flex gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              add();
            }
          }}
          placeholder={org ? `owner/repo, or repo for ${org}` : "owner/repo"}
          className={cn(inputClass, "flex-1 py-1.5 font-mono placeholder:text-tertiary")}
        />
        <button
          type="button"
          onClick={add}
          disabled={!draft.trim()}
          className="flex items-center gap-1 px-2.5 py-1.5 text-xs rounded-md border border-white/[0.08] hover:bg-white/[0.06] disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Plus className="size-3" />
          <span className="text-white">Add</span>
        </button>
      </div>
      {problem && <p className="text-[10px] text-error">{problem}</p>}
    </div>
  );
}
