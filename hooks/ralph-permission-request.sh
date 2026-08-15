#!/bin/bash
# ralph plugin: pre-approve Bash/Monitor invocations of this plugin's own
# bundled scripts (ralph-start.sh/ralph-monitor.sh/ralph-status.sh/
# ralph-cancel.sh under ${CLAUDE_PLUGIN_ROOT}/scripts/), so users never hit
# an interactive permission prompt for them regardless of where the plugin
# is installed (the resolved ${CLAUDE_PLUGIN_ROOT} path — a nix store hash,
# a local checkout, whatever — differs per install/version, so a static
# settings.json allow-rule keyed to one exact path can't cover every user;
# this hook matches the invocation *shape* instead).
#
# Fails toward asking, not toward allowing: anything that doesn't exactly
# match one of our own script invocations gets no opinion at all (silent,
# no output) — the normal permission prompt/settings still apply untouched.
# Registered on PermissionRequest (not PreToolUse — PreToolUse cannot
# suppress the interactive dialog, only PermissionRequest can), matcher
# "Bash|Monitor" since Monitor's underlying command execution is
# documented to reuse Bash's own permission rules but isn't confirmed to
# report tool_name "Bash" to hooks — matching both names is the safe way
# to cover whichever one it turns out to be.
#
# Uses jq, matching Claude Code's own documented hook examples (which
# assume jq); if jq isn't on PATH this just no-ops (safe degradation — the
# interactive prompt still appears, nothing breaks).
set -uo pipefail

INPUT="$(cat)"

command -v jq >/dev/null 2>&1 || exit 0

TOOL_NAME="$(jq -r '.tool_name // empty' <<<"$INPUT" 2>/dev/null)"
case "$TOOL_NAME" in
Bash | Monitor) ;;
*) exit 0 ;;
esac

CMD="$(jq -r '.tool_input.command // empty' <<<"$INPUT" 2>/dev/null)"
[[ -n "$CMD" ]] || exit 0

# Defense in depth: never allow a command containing shell chaining or
# substitution metacharacters, even if the shape check below has a gap —
# a legitimate invocation of our own scripts never needs any of these, so
# their presence means something is trying to smuggle extra commands in
# behind a familiar-looking prefix.
if [[ "$CMD" == *';'* || "$CMD" == *'|'* || "$CMD" == *'&'* || "$CMD" == *'`'* || "$CMD" == *'$('* || "$CMD" == *$'\n'* ]]; then
  exit 0
fi

# Must be exactly one of our own script invocations, in the exact shape our
# own commands/*.md construct them — not merely contain that substring
# somewhere in a longer/different command.
if [[ "$CMD" =~ ^bash\ \"[^\"]+/scripts/ralph-monitor\.sh\"\ \"[^\"]*\"$ ]] ||
  [[ "$CMD" =~ ^bash\ \"[^\"]+/scripts/ralph-(status|cancel)\.sh\"$ ]] ||
  [[ "$CMD" =~ ^bash\ \"[^\"]+/scripts/ralph-start\.sh\"(\ [A-Za-z0-9_./:=-]+|\ \"[^\"]*\")*$ ]]; then
  printf '{"hookSpecificOutput":{"hookEventName":"PermissionRequest","permissionDecision":"allow","permissionDecisionReason":"ralph plugin: pre-approved invocation of its own bundled script"}}\n'
fi
exit 0
