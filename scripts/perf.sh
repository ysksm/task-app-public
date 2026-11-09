#!/usr/bin/env bash
# Performance check (NFR-03): seeds ~10,000 issues, starts a server and
# times the main queries. Usage: scripts/perf.sh [path/to/taskapp]
set -euo pipefail

BIN=${1:-bin/taskapp}
DATA=$(mktemp -d)
trap 'kill $PID 2>/dev/null; rm -rf "$DATA"' EXIT

echo "seeding (preset perf) ..."
start=$(date +%s)
"$BIN" seed --data "$DATA" --preset perf
echo "  $(( $(date +%s) - start ))s"

"$BIN" serve --data "$DATA" --port 0 --backup-interval 0 > "$DATA/out" 2> "$DATA/err" &
PID=$!
for _ in $(seq 50); do
  ADDR=$(sed -n 's/.*"addr":"\([^"]*\)".*/\1/p' "$DATA/out" | head -1)
  [[ -n $ADDR ]] && break
  sleep 0.1
done
BASE=http://$ADDR
AUTH="Authorization: Bearer tk_perf_owner"

enc() { python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1]))' "$1"; }

# Runs a request 5 times and prints the median time in ms.
measure() {
  local label=$1 url=$2 times=()
  curl -s -o /dev/null -H "$AUTH" "$url" # warm up
  for _ in 1 2 3 4 5; do
    times+=("$(curl -s -o /dev/null -w '%{time_total}' -H "$AUTH" "$url")")
  done
  local med
  med=$(printf '%s\n' "${times[@]}" | sort -n | sed -n 3p)
  local size
  size=$(curl -s -H "$AUTH" "$url" | wc -c | tr -d ' ')
  printf '%-46s %7.0f ms  %8s bytes\n' "$label" "$(echo "$med * 1000" | bc)" "$size"
}

echo
echo "issues: $(curl -s -H "$AUTH" "$BASE/api/v1/issues?limit=1" | python3 -c 'import json,sys;print(json.load(sys.stdin)["total"])')"
echo
measure "list project (2,560 issues, limit 2000)"   "$BASE/api/v1/issues?project=P1&limit=2000"
measure "list project for the map (limit 5000)"      "$BASE/api/v1/issues?project=P1&limit=5000"
measure "list all projects (limit 500, updated)"     "$BASE/api/v1/issues?limit=500"
measure "filter: assignee=me & open (all projects)"  "$BASE/api/v1/issues?limit=500&filter=$(enc '{"and":[{"field":"assignee","op":"eq","value":"me"},{"field":"state_category","op":"in","value":["backlog","unstarted","started"]}]}')"
measure "filter: label=frontend & type=task"         "$BASE/api/v1/issues?project=P2&limit=500&filter=$(enc '{"and":[{"field":"label","op":"eq","value":"frontend"},{"field":"type","op":"eq","value":"task"}]}')"
measure "filter: text contains (all projects)"       "$BASE/api/v1/issues?limit=500&filter=$(enc '{"field":"text","op":"contains","value":"3-4-2"}')"
measure "filter: descendants of an epic (tree)"      "$BASE/api/v1/issues?limit=500&include_ancestors=true&filter=$(enc '{"field":"ancestor","op":"eq","value":"P1-1"}')"
measure "issue detail"                               "$BASE/api/v1/issues/P1-1"
measure "acceptance queue"                           "$BASE/api/v1/acceptance"
measure "Jira search/jql (ORCA assigned filter)"     "$BASE/rest/api/3/search/jql?maxResults=30&jql=$(enc 'assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC')"
