import { useEffect, useState } from "react";
import { cn } from "../../lib/utils";
import RepoList from "../RepoList";
import Toggle from "../Toggle";
import { inputClass, type SummaryConfig } from "./config";
import { Block, Field } from "./ui";

// SummaryCard is the team summary's own settings: whose work it reports,
// in which repos, and whether it adds Jira tickets and Claude's paragraphs.
// The JobCard around it has the switch, days, times and Run.
export default function SummaryCard({
  summary,
  jiraSite,
  onSave,
}: {
  summary: SummaryConfig;
  jiraSite: string;
  onSave: (summary: SummaryConfig) => void;
}) {
  // The team saves when the box loses focus, and only as org/team-slug.
  const [team, setTeam] = useState(summary.team);
  const [problem, setProblem] = useState<string | null>(null);
  useEffect(() => setTeam(summary.team), [summary.team]);

  const saveTeam = () => {
    const next = team.trim();
    if (next && !/^[^/\s]+\/[^/\s]+$/.test(next)) {
      setProblem("Use org/team-slug");
      return;
    }
    setProblem(null);
    if (next !== summary.team) onSave({ ...summary, team: next });
  };

  return (
    <Block title="Settings">
      <Field
        label="Team"
        htmlFor="summary-team"
        help={
          problem ? (
            <span className="text-error">{problem}</span>
          ) : summary.team ? (
            "Each member gets a paragraph or a list. Members with no activity share one line."
          ) : (
            <span className="text-warning">The summary cannot run until a team is set.</span>
          )
        }
      >
        <input
          id="summary-team"
          type="text"
          value={team}
          onChange={(e) => setTeam(e.target.value)}
          onBlur={saveTeam}
          placeholder="org/team-slug"
          className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
        />
      </Field>
      <Field label="Repos">
        <RepoList
          repos={summary.repos}
          org={summary.team.split("/")[0].trim()}
          empty="No repos set. It covers every repo in the team's org."
          onChange={(repos) => onSave({ ...summary, repos })}
        />
      </Field>
      <Field
        label="Tickets"
        htmlFor="summary-jira"
        help={
          jiraSite
            ? "Lists the tickets the PRs name, with their status and whether they moved."
            : "Lists the tickets the PRs name. It needs a Jira site in General."
        }
      >
        <Toggle
          id="summary-jira"
          on={summary.jira}
          onClick={() => onSave({ ...summary, jira: !summary.jira })}
        />
      </Field>
      <Field
        label="Paragraphs"
        htmlFor="summary-prose"
        help="Claude writes a short paragraph per person. If it fails, the DM lists the facts."
      >
        <Toggle
          id="summary-prose"
          on={summary.prose}
          onClick={() => onSave({ ...summary, prose: !summary.prose })}
        />
      </Field>
    </Block>
  );
}
