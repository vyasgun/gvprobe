#!/usr/bin/env bash
# gvprobe smoke test — runs every headless gvprobe command against a real
# gvproxy, once per datapath backend, plus probes for known upstream
# regressions (gvisor-tap-vsock issues are named per step).
#
# Each backend gets a full lifecycle: start, attach session, L2/L3 traces
# (DHCP, ARP, raw frame), DNS queries through gvproxy's resolver, teardown.
# Steps are judged by exit code (run) or by output content (expect — some
# bugs only show up in the reply line, e.g. NXDOMAIN where NOERROR belongs,
# or a wedged datapath behind a live PID).
#
# The attach session is held for the whole cycle, not for coverage's sake
# only: gvproxy on qemu/bess can exit when a direct client disconnects, so
# traces route through the attach proxy (attach.sock) while it runs.
#
# Intended to run from CI (e.g. against a gvisor-tap-vsock checkout):
# build gvproxy there, put it on PATH, build gvprobe, then run this script.
#
# Environment:
#   BIN        gvprobe binary        (default: ./bin/gvprobe)
#   BACKENDS   space-separated list of datapath backends to cover
#              (default: OS-appropriate — vfkit on macOS, bess qemu on Linux)
#   KEEP       "1" to leave gvproxy running after the run (default: stop)
#
# Usage:   ./scripts/smoke.sh
# Exits:   0 = all steps passed, 1 = at least one failure

set -u -o pipefail

BIN=${BIN:-./bin/gvprobe}
if [[ -n ${BACKENDS:-} ]]; then
  BACKENDS=($BACKENDS)
elif [[ $(uname -s) == Darwin ]]; then
  BACKENDS=(vfkit)
else
  BACKENDS=(bess qemu)
fi
LOG=$(mktemp)
PASS=0
FAIL=0
FAILED_STEPS=()

step() { printf '\n── %s ──\n' "$1"; }
ok()   { PASS=$((PASS + 1)); echo ">> PASS: $1"; }
bad()  { FAIL=$((FAIL + 1)); FAILED_STEPS+=("$1"); echo ">> FAIL: $1"; }

# run <description> <gvprobe args...>
# Streams the command's output live and judges it by exit code.
run() {
  local desc=$1; shift
  step "$desc"
  if "$BIN" "$@" 2>&1 | tee "$LOG"; then
    ok "$desc"
  else
    bad "$desc"
  fi
}

# expect <description> <egrep pattern> -- <gvprobe args...>
# Runs the command quietly and judges it by its OUTPUT, not its exit
# code — several upstream bugs only show up in the reply line (NXDOMAIN
# where NOERROR belongs, a wedged datapath behind a live PID). On
# failure the captured output is printed.
expect() {
  local desc=$1 pattern=$2; shift 2; shift # drop the -- separator
  step "$desc"
  "$BIN" "$@" >"$LOG" 2>&1
  if grep -qE "$pattern" "$LOG"; then
    ok "$desc"
  else
    bad "$desc"
    cat "$LOG"
  fi
}

# ── preflight ──────────────────────────────────────────────────────────

step "preflight"
if [[ ! -x "$BIN" ]]; then
  step "build gvprobe"
  if make >/dev/null; then ok "build gvprobe"; else bad "build gvprobe"; exit 1; fi
fi
if ! command -v gvproxy >/dev/null 2>&1; then
  echo "gvproxy not found on PATH — build it first (see gvisor-tap-vsock/cmd/gvproxy)"
  exit 1
fi
ok "gvprobe=$BIN backends=${BACKENDS[*]} gvproxy=$(command -v gvproxy)"

run "version" version

# Leave no previous run behind (stop is a no-op if none is running).
"$BIN" stop >/dev/null 2>&1

# ── per-backend cycles ─────────────────────────────────────────────────

for backend in "${BACKENDS[@]}"; do
  step "backend: $backend"

  run "[$backend] start gvproxy"          start --backend "$backend"
  run "[$backend] status reports running" status

  "$BIN" attach >/dev/null 2>&1 &
  attach_pid=$!
  sleep 1
  if kill -0 "$attach_pid" 2>/dev/null; then
    ok "[$backend] attach session held"
  else
    bad "[$backend] attach session exited early"
  fi

  # L2/L3 datapath: DHCP lease, gateway ARP, raw frames.
  run "[$backend] DHCP discover (x3)"     trace dhcp --count 3
  run "[$backend] ARP resolve gateway"    trace arp
  run "[$backend] raw 64-byte frame"      trace frame --size 64
  run "[$backend] raw frame + listen"     trace frame --size 64 --listen

  # DNS through gvproxy's resolver. Asserted on output: the reply line
  # carries the rcode and answer count, so a resolver that answers wrong
  # (NXDOMAIN where records belong) fails here, not just one that is silent.
  expect "[$backend] DNS A resolves" 'NOERROR, [1-9][0-9]* answer' -- trace dns example.com
  expect "[$backend] DNS AAAA resolves" 'NOERROR, [1-9][0-9]* answer' -- trace dns --type AAAA example.com
  expect "[$backend] DNS NXDOMAIN for bogus name" 'NXDOMAIN' -- trace dns no-such-domain.gvprobe.test

  # ── known-regression probes ────────────────────────────────────────
  # Each step encodes the CORRECT behavior for an upstream issue; a
  # regression fails the step and names the issue.
  #
  # #780 (open): a frame shorter than an Ethernet header must not wedge
  # the switch. The process stays alive when wedged, so the assertion is
  # functional: the datapath must still answer ARP afterwards.
  run   "[$backend] #780 short frame (10B)" trace frame --size 10
  expect "[$backend] #780 datapath alive after short frame" 'ARP handshake successful' -- trace arp

  # #772 (open): MX for an existing name with no MX records must be
  # NOERROR/NODATA — NXDOMAIN claims the whole name doesn't exist.
  expect "[$backend] #772 MX NODATA, not NXDOMAIN" 'NOERROR, 0 answer' -- trace dns --type MX raw.githubusercontent.com

  # #612 (fixed by #696): SOA and PTR queries must be forwarded upstream
  # and answered, not intercepted and dropped as empty NOERROR.
  expect "[$backend] #612 SOA forwarded" 'NOERROR, [1-9][0-9]* answer' -- trace dns --type SOA example.com
  expect "[$backend] #612 PTR forwarded" 'NOERROR, [1-9][0-9]* answer' -- trace dns --type PTR 8.8.8.8.in-addr.arpa

  # #563/#562 (fixed by #561): long and multiple TXT records must come
  # back as answers. gmail.com carries several long TXT records.
  expect "[$backend] #563/#562 TXT records" 'NOERROR, [1-9][0-9]* answer' -- trace dns --type TXT gmail.com

  kill "$attach_pid" 2>/dev/null
  wait "$attach_pid" 2>/dev/null
  ok "[$backend] attach session released"

  if [[ ${KEEP:-0} != "1" ]]; then
    run "[$backend] stop gvproxy"           stop
    run "[$backend] status reports stopped" status
  fi
done

# ── summary ────────────────────────────────────────────────────────────

printf '\n───────────────────────────────\n'
printf 'smoke: %d passed, %d failed\n' "$PASS" "$FAIL"
if (( FAIL > 0 )); then
  printf 'failed steps: %s\n' "${FAILED_STEPS[*]}"
  exit 1
fi
exit 0

# ── not covered ────────────────────────────────────────────────────────
# agent dhcp    needs Vertex AI credentials — not smoke-testable headlessly
