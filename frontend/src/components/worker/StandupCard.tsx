import JobCard, { type CardProps } from "./JobCard";
import { Block, Field } from "./ui";

// StandupCard is the stand-up job. Its Team PRs list and its list of PRs waiting
// for your review both use the review worker's team, so the card names that team.
export default function StandupCard(props: CardProps) {
  const { team } = props.api.cfg.reviewer;
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
          help="The stand-up uses the review worker's team. Change it under Which PRs on the review card."
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
