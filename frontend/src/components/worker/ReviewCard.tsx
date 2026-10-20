import { useEffect, useState } from "react";
import { cn } from "../../lib/utils";
import RepoList from "../RepoList";
import Toggle from "../Toggle";
import { formatWhen, inputClass, reviewerSetUp } from "./config";
import JobCard, { type CardProps } from "./JobCard";
import { Block, Choice, Disclosure, Field } from "./ui";

const days = (value: string) => Number.parseInt(value, 10) || 0;

// ReviewCard is the review worker. Its Reviewer group sets how reviews run, and
// starts closed if a reviewer is set up. Its Which PRs group sets which PRs get a review.
export default function ReviewCard(props: CardProps) {
  const { cfg, loaded, status, save, setReviewer, saveReviewer } = props.api;
  const r = cfg.reviewer;
  const setUp = reviewerSetUp(r);
  // The Reviewer group starts open if no reviewer is set up when worker.json loads.
  // That choice is made once, so typing the first repo does not close the group.
  const [open, setOpen] = useState<boolean | null>(null);
  const shown = open ?? !setUp;
  useEffect(() => {
    if (loaded) setOpen((o) => o ?? !setUp);
  }, [loaded]);

  const summary = !setUp
    ? "Not set up"
    : !r.command.includes("{reviewer}")
      ? `Runs ${r.command.trim()}`
      : r.source === "managed"
        ? `Managed clone of ${r.repo}, branch ${r.branch || "main"}`
        : `Local CLI at ${r.path}`;

  return (
    <JobCard
      {...props}
      jobKey="review"
      name="Review worker"
      about="Reviews each PR once and posts the review as comments on the PR, as you. One Slack line says each review is posted. Test prints the comments instead."
      multi
      done="Done. The reviews are on the PRs."
    >
      <Block title="Settings">
        <div className="space-y-3">
          <Disclosure
            open={shown}
            onToggle={() => setOpen(!shown)}
            controls="reviewer-body"
            title={<span className="block text-xs text-white/85 font-medium">Reviewer</span>}
            summary={shown ? "How the review runs" : summary}
          />
          <div
            id="reviewer-body"
            hidden={!shown}
            // A field in use keeps the group open, even before the config loads.
            onFocusCapture={() => setOpen((o) => o ?? true)}
            className="space-y-3"
          >
            <p className="text-[10px] text-tertiary leading-relaxed">
              Any CLI that follows the reviewer contract (plans 030 and 031). It reviews one PR and
              never posts. The worker posts its comments. If there is nothing to report, it posts
              LGTM 😃⭐😸.
            </p>
            <Field
              label="Command"
              htmlFor="reviewer-command"
              help={
                <>
                  {"{reviewer}"} is the CLI set below. {"{pr}"}, {"{repo}"}, {"{url}"} and {"{sha}"}{" "}
                  are filled in for each PR, and {"{claude}"} is a claude that cannot post.
                  NAME=value words at the start set environment variables.
                </>
              }
            >
              <input
                id="reviewer-command"
                type="text"
                value={r.command}
                onChange={(e) => setReviewer({ command: e.target.value })}
                onBlur={() => save(cfg)}
                placeholder="{reviewer} {pr}"
                className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
              />
            </Field>
            <Field
              label="Source"
              help="Where {reviewer} comes from. Managed: the worker keeps its own clone, so the branch you have checked out never changes what runs."
            >
              <Choice
                value={r.source}
                options={[
                  { value: "managed", label: "Managed" },
                  { value: "local", label: "Local path" },
                ]}
                onPick={(source) =>
                  // A local source saves once it has a path, from the path
                  // box below. Saved without one, it would fail reviews.
                  source === "local" && !r.path.trim()
                    ? setReviewer({ source })
                    : saveReviewer({ source })
                }
              />
            </Field>
            {r.source === "managed" ? (
              <>
                <Field label="Repo" htmlFor="reviewer-repo">
                  <input
                    id="reviewer-repo"
                    type="text"
                    value={r.repo}
                    onChange={(e) => setReviewer({ repo: e.target.value })}
                    onBlur={() => save(cfg)}
                    placeholder="owner/repo"
                    className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
                  />
                </Field>
                <Field label="Branch" htmlFor="reviewer-branch">
                  <input
                    id="reviewer-branch"
                    type="text"
                    value={r.branch}
                    onChange={(e) => setReviewer({ branch: e.target.value })}
                    onBlur={() => save(cfg)}
                    placeholder="main"
                    className={cn(inputClass, "w-36 font-mono placeholder:text-tertiary")}
                  />
                </Field>
                <Field
                  label="Auto-update"
                  htmlFor="reviewer-update"
                  help={
                    <>
                      Update to the newest commit on the branch before each run.{" "}
                      {status?.reviewer?.commit
                        ? `Last run used ${status.reviewer.commitBranch} @ ${status.reviewer.commit}${
                            status.reviewer.updated
                              ? `, updated ${formatWhen(status.reviewer.updated)}`
                              : ""
                          }.`
                        : "Not cloned yet. The next review run clones it."}
                    </>
                  }
                >
                  <div className="pt-0.5">
                    <Toggle
                      id="reviewer-update"
                      on={r.autoUpdate}
                      onClick={() => saveReviewer({ autoUpdate: !r.autoUpdate })}
                    />
                  </div>
                </Field>
              </>
            ) : (
              <Field label="Path" htmlFor="reviewer-path">
                <input
                  id="reviewer-path"
                  type="text"
                  value={r.path}
                  onChange={(e) => setReviewer({ path: e.target.value })}
                  onBlur={() => r.path.trim() && save(cfg)}
                  placeholder="~/repos/reviewer/review"
                  className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
                />
              </Field>
            )}
            <Field
              label="Logs folder"
              htmlFor="reviewer-logs"
              help="Off: the reviewer writes its comments as JSON to $REVIEW_RESULT. A folder: the worker reads the review from the reviewer's logs there instead."
            >
              <input
                id="reviewer-logs"
                type="text"
                value={r.logs}
                onChange={(e) => setReviewer({ logs: e.target.value })}
                onBlur={() => save(cfg)}
                placeholder="Off"
                className={cn(inputClass, "w-full font-mono placeholder:text-tertiary")}
              />
            </Field>
            <Field
              label="Marker"
              htmlFor="reviewer-marker"
              help={
                <>
                  Each comment starts with {"<!-- marker:"}. A PR with a comment behind the marker
                  counts as reviewed.
                </>
              }
            >
              <input
                id="reviewer-marker"
                type="text"
                value={r.markerPrefix}
                onChange={(e) => setReviewer({ markerPrefix: e.target.value })}
                onBlur={() => save(cfg)}
                placeholder="acme"
                className={cn(inputClass, "w-36 font-mono placeholder:text-tertiary")}
              />
            </Field>
          </div>
        </div>

        <div className="space-y-3 pt-3 border-t border-white/[0.05]">
          <h5 className="text-xs text-white/85 font-medium">Which PRs</h5>
          <Field
            label="Orgs"
            help="Only PRs in these orgs and accounts are reviewed, yours and the team's. Add your own login to keep your personal repos."
          >
            <RepoList
              owners
              repos={r.orgs}
              org=""
              empty="No orgs set. PRs in every org are reviewed."
              onChange={(orgs) => saveReviewer({ orgs })}
            />
          </Field>
          <Field
            label="Excluded repos"
            help="PRs in these repos are never reviewed, even in one of the orgs above."
          >
            <RepoList
              repos={r.excludeRepos}
              org=""
              empty="No repos excluded."
              onChange={(excludeRepos) => saveReviewer({ excludeRepos })}
            />
          </Field>
          <Field label="My PRs" htmlFor="review-mine" help="Your open PRs, outside archived repos.">
            <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
              <Toggle
                id="review-mine"
                on={r.mine.enabled}
                onClick={() => saveReviewer({ mine: { ...r.mine, enabled: !r.mine.enabled } })}
              />
              {r.mine.enabled && (
                <>
                  <span>opened in the last</span>
                  <input
                    type="number"
                    min={1}
                    aria-label="My PRs age in days"
                    value={r.mine.maxAgeDays}
                    onChange={(e) =>
                      saveReviewer({ mine: { ...r.mine, maxAgeDays: days(e.target.value) } })
                    }
                    className={cn(inputClass, "w-12 tabular-nums")}
                  />
                  <span>days</span>
                </>
              )}
            </div>
          </Field>
          <Field
            label="Team PRs"
            htmlFor="review-team"
            help="PRs that ask for your review, and every open PR in these repos. The stand-up uses this team too."
          >
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2 text-[10px] text-tertiary">
                <Toggle
                  id="review-team"
                  on={r.team.enabled}
                  onClick={() => saveReviewer({ team: { ...r.team, enabled: !r.team.enabled } })}
                />
                {r.team.enabled && (
                  <>
                    <span>opened in the last</span>
                    <input
                      type="number"
                      min={1}
                      aria-label="Team PRs age in days"
                      value={r.team.maxAgeDays}
                      onChange={(e) =>
                        saveReviewer({ team: { ...r.team, maxAgeDays: days(e.target.value) } })
                      }
                      className={cn(inputClass, "w-12 tabular-nums")}
                    />
                    <span>days</span>
                  </>
                )}
              </div>
              {/* The team box shows with Team PRs off too, because the stand-up uses the team. */}
              <div className="flex items-center gap-2 text-[10px] text-tertiary">
                <span className="shrink-0">Opened by</span>
                <input
                  type="text"
                  aria-label="Team"
                  value={r.team.team}
                  onChange={(e) => setReviewer({ team: { ...r.team, team: e.target.value } })}
                  onBlur={() => save(cfg)}
                  placeholder="org/team-slug"
                  className={cn(inputClass, "flex-1 min-w-0 font-mono placeholder:text-tertiary")}
                />
              </div>
              {r.team.enabled && (
                <RepoList
                  repos={r.team.repos}
                  org={r.team.team.split("/")[0].trim()}
                  empty="No repos yet. Only PRs that ask for your review are reviewed."
                  onChange={(repos) => saveReviewer({ team: { ...r.team, repos } })}
                />
              )}
            </div>
          </Field>
        </div>
      </Block>
    </JobCard>
  );
}
