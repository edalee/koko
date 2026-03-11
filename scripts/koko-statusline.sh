#!/bin/bash
# Claude Code status line script for Koko.
# Writes context window data to a per-session file for Koko to read.
# Also outputs a status line for Claude Code's own display.
input=$(cat)
SESSION_ID=$(echo "$input" | jq -r '.session_id // empty')
USED_PCT=$(echo "$input" | jq -r '.context_window.used_percentage // 0')
REMAINING_PCT=$(echo "$input" | jq -r '.context_window.remaining_percentage // 100')
MODEL=$(echo "$input" | jq -r '.model.display_name // "Claude"')

if [ -n "$SESSION_ID" ]; then
  mkdir -p ~/.koko/context
  echo "$input" | jq '{
    usedPercentage: (.context_window.used_percentage // 0),
    remainingPercentage: (.context_window.remaining_percentage // 100),
    model: (.model.display_name // "Claude")
  }' > ~/.koko/context/"${SESSION_ID}".json
fi

echo "${MODEL} · ${USED_PCT}% context"
