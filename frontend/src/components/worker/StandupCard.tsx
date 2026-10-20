import JobCard, { type CardProps } from "./JobCard";
import { standupRun } from "./runs";
import { Block, Field } from "./ui";

// StandupCard is the stand-up job. Its Team PRs list and its review queue use
// the team of the first PRs run that is on, else the first PRs run, so it
// names that team here. With no PRs run, there is no team.
export default function StandupCard(props: CardProps) {
  const run = standupRun(props.api.cfg.reviewer.runs);
  const team = run?.team ?? { enabled: false, maxAgeDays: 4, team: "", repos: [] };
  const { jiraSite } = props.api.cfg;
  const listsTeam = team.enabled && team.repos.length > 0;
  return (
    <JobCard
      {...props}
      jobKey="standup"
      name="Stand-up"
      about="A Slack DM with today's meetings, your tickets, PRs ready to merge and PRs waiting for your review."
      done="Done. Check your Slack DMs."
    >
      <Block title="Settings">
        <Field
          label="Team"
          help={
            run
              ? `The stand-up uses the team of the first PRs run that is on, now the ${run.name} run. Change it in that run on the review card.`
              : "The stand-up uses the team of the first PRs run that is on. There is no PRs run, so it has no team."
          }
        >
          <p className="pt-1 text-[11px] text-white/85">
            {team.team ? (
              <span className="font-mono">{team.team}</span>
            ) : (
              <span className="text-tertiary">No team set</span>
            )}
          </p>
          <p className="text-[10px] text-muted-foreground">
            {team.team
              ? `Each PR from ${team.team} waiting for your review gets a line. The rest are counted.`
              : "Every PR waiting for your review is listed."}{" "}
            {listsTeam
              ? `Team PRs lists the open PRs in ${team.repos.length === 1 ? "1 repo" : `${team.repos.length} repos`}.`
              : "There is no Team PRs list, because Team PRs is off or has no repos."}
          </p>
        </Field>
        <Field label="Tickets">
          <p className="pt-1 text-[11px] text-muted-foreground">
            {jiraSite ? (
              <>
                From <span className="font-mono text-white/85">{jiraSite}</span>
              </>
            ) : (
              "None, because no Jira site is set in General."
            )}
          </p>
        </Field>
      </Block>
    </JobCard>
  );
}
