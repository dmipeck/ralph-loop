# Shared helpers for the ralph plugin's scripts. Sourced, not directly
# executed — no shebang, no execute bit needed.

RALPH_DIR=".claude/ralph"

ralph_state_path() { echo "$RALPH_DIR/state"; }

ralph_require_repo() {
  if [[ ! -e "$PWD/.git" ]]; then
    echo "No .git found directly in $PWD. ralph-loop (and this plugin) do" >&2
    echo "not search parent directories — run this from the exact repo root." >&2
    return 1
  fi
}

# Mirrors cmd/root.go's own default: <OS cache dir>/ralph-loop/<repo-basename>.
# Kept in sync deliberately — this is the SAME path the bare CLI already
# defaults to, not a new plugin-invented location, so a plain `ralph-loop`
# run and a `/ralph` run against the same repo share history/logs.
ralph_default_log_dir() {
  local cache="${XDG_CACHE_HOME:-}"
  if [[ -z "$cache" ]]; then
    case "$(uname)" in
      Darwin) cache="$HOME/Library/Caches" ;;
      *) cache="$HOME/.cache" ;;
    esac
  fi
  echo "$cache/ralph-loop/$(basename "$PWD")"
}

ralph_ensure_exclude() {
  local excl=".git/info/exclude"
  if command -v git >/dev/null 2>&1; then
    local gd
    gd=$(git rev-parse --git-common-dir 2>/dev/null) && excl="$gd/info/exclude"
  fi
  mkdir -p "$(dirname "$excl")" 2>/dev/null || return 0
  grep -qxF '/.claude/ralph/' "$excl" 2>/dev/null || printf '%s\n' '/.claude/ralph/' >>"$excl"
}

# KEY FILE -> value (empty if absent). Only the FIRST matching line counts,
# so callers can rewrite a key by appending a fresh KEY=value line rather
# than editing in place, if that's ever simpler than the atomic-rewrite
# path ralph-start.sh already uses.
ralph_state_get() {
  grep "^$1=" "$2" 2>/dev/null | head -1 | cut -d= -f2-
}

# Protocol line emitted by every script as its last output: RALPH_KEY=value.
ralph_emit() {
  printf 'RALPH_%s=%s\n' "$1" "$2"
}

# PID -> 0 if alive AND its command line looks like ralph-loop (a cheap
# guard against PID reuse by an unrelated process).
ralph_pid_alive() {
  kill -0 "$1" 2>/dev/null && ps -p "$1" -o command= 2>/dev/null | grep -q 'ralph-loop'
}

ralph_mtime() {
  stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null
}

# Resolves a usable ralph-loop binary: PATH, then a prebuilt bundled copy,
# then (dev checkout only) an mtime-gated `go build`. Prints the resolved
# path on stdout; prints an actionable error to stderr and returns 1 if
# nothing usable was found.
ralph_find_binary() {
  if command -v ralph-loop >/dev/null 2>&1; then
    command -v ralph-loop
    return 0
  fi
  if [[ -n "${CLAUDE_PLUGIN_ROOT:-}" && -x "$CLAUDE_PLUGIN_ROOT/bin/ralph-loop" ]]; then
    echo "$CLAUDE_PLUGIN_ROOT/bin/ralph-loop"
    return 0
  fi
  if [[ -n "${CLAUDE_PLUGIN_ROOT:-}" && -f "$CLAUDE_PLUGIN_ROOT/go.mod" ]] && command -v go >/dev/null 2>&1; then
    local bin="$CLAUDE_PLUGIN_ROOT/ralph-loop"
    if [[ ! -x "$bin" ]] || [[ -n "$(find "$CLAUDE_PLUGIN_ROOT" -name '*.go' -newer "$bin" 2>/dev/null | head -1)" ]]; then
      (cd "$CLAUDE_PLUGIN_ROOT" && go build -o ralph-loop .) >&2 || return 1
    fi
    echo "$bin"
    return 0
  fi
  echo "ralph-loop not on PATH, not bundled at \$CLAUDE_PLUGIN_ROOT/bin/ralph-loop, and no" >&2
  echo "Go source + 'go' toolchain available to build it. Install ralph-loop or Go, then retry." >&2
  return 1
}

# STDOUT_LOG -> STOPPED|INTERRUPTED|CRASHED, based on the loop's own final
# lines (see internal/loop/loop.go for the exact strings this matches).
ralph_classify_terminal() {
  local last
  last=$(tail -n 5 "$1" 2>/dev/null)
  if grep -q '^Ralph loop stopped after' <<<"$last"; then
    echo STOPPED
    return
  fi
  if grep -qE '^Interrupted (before|after) iteration' <<<"$last"; then
    echo INTERRUPTED
    return
  fi
  echo CRASHED
}
