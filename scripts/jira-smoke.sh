#!/usr/bin/env bash
# Smoke test of the Jira REST API v3 compatible layer, sending the same
# requests ORCA sends in Jira Cloud mode (Basic email:token, User-Agent Orca).
#
#   scripts/jira-smoke.sh [base_url] [email] [token] [project_key]
#
# Defaults match `make demo-fresh`: http://127.0.0.1:7700, owner@taskapp.local,
# tk_demo_owner, APP. It creates one issue and one comment in that project.
set -euo pipefail

BASE=${1:-http://127.0.0.1:7700}
EMAIL=${2:-owner@taskapp.local}
TOKEN=${3:-tk_demo_owner}
PROJECT=${4:-APP}
API=$BASE/rest/api/3
fail=0

call() { # method path [json] -> prints "status body"
  local method=$1 path=$2 data=${3:-}
  local args=(-s -o /tmp/jira-smoke.$$ -w '%{http_code}' -X "$method" -u "$EMAIL:$TOKEN"
    -H 'User-Agent: Orca' -H 'Accept: application/json' -H 'Content-Type: application/json')
  [[ -n $data ]] && args+=(-d "$data")
  local status
  status=$(curl "${args[@]}" "$path")
  echo "$status $(cat /tmp/jira-smoke.$$)"
  rm -f /tmp/jira-smoke.$$
}

check() { # label expected_status "status body" [jq filter]
  local label=$1 want=$2 got=$3 filter=${4:-}
  local status=${got%% *} body=${got#* }
  if [[ $status != "$want" ]]; then
    printf 'FAIL %-34s HTTP %s (want %s) %s\n' "$label" "$status" "$want" "${body:0:200}"
    fail=1
    return
  fi
  local summary=""
  if [[ -n $filter && -n $body ]]; then
    summary=$(jq -rc "$filter" <<<"$body" 2>/dev/null || echo "?")
  fi
  printf 'ok   %-34s HTTP %s %s\n' "$label" "$status" "$summary"
}

command -v jq >/dev/null || { echo "jq is required"; exit 2; }

check "myself" 200 "$(call GET "$API/myself")" '.displayName + " <" + .emailAddress + ">"'
check "serverInfo" 200 "$(call GET "$API/serverInfo")" '.deploymentType'
check "project/search" 200 "$(call GET "$API/project/search?maxResults=100&startAt=0")" '[.values[].key] | join(",")'

jql='assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC'
fields='["summary","project","issuetype","status","assignee","reporter","priority","labels","created","updated"]'
check "search/jql (assigned)" 200 \
  "$(call POST "$API/search/jql" "$(jq -nc --arg j "$jql" --argjson f "$fields" '{jql:$j,maxResults:30,fields:$f}')")" \
  '"\(.issues | length) issues: " + ([.issues[].key] | join(","))'
check "search/jql (bad JQL)" 400 "$(call POST "$API/search/jql" '{"jql":"assignee ="}')" '.errorMessages[0]'

check "createmeta issuetypes" 200 "$(call GET "$API/issue/createmeta/$PROJECT/issuetypes?maxResults=100&startAt=0")" '[.issueTypes[].name] | join(",")'
check "priority" 200 "$(call GET "$API/priority")" '[.[].name] | join(",")'
check "user/search" 200 "$(call GET "$API/user/search?maxResults=50&query=")" '[.[].displayName] | join(",")'

created=$(call POST "$API/issue" "$(jq -nc --arg p "$PROJECT" '{fields:{project:{key:$p},issuetype:{id:"10003"},summary:"jira-smoke: created via REST v3",
  description:{type:"doc",version:1,content:[{type:"paragraph",content:[{type:"text",text:"Created by scripts/jira-smoke.sh"}]}]}}}')")
check "create issue" 201 "$created" '.key'
KEY=$(jq -r '.key' <<<"${created#* }")

check "get issue" 200 "$(call GET "$API/issue/$KEY?fields=summary,description,status,attachment&expand=renderedFields")" '.fields.status.name'
check "edit issue" 204 "$(call PUT "$API/issue/$KEY" '{"fields":{"summary":"jira-smoke: edited","labels":["smoke"],"priority":{"id":"3"}}}')"
check "assign (unassign)" 204 "$(call PUT "$API/issue/$KEY/assignee" '{"accountId":null}')"
transitions=$(call GET "$API/issue/$KEY/transitions")
check "transitions" 200 "$transitions" '[.transitions[].name] | join(",")'
TID=$(jq -r '.transitions[] | select(.to.statusCategory.key == "indeterminate") | .id' <<<"${transitions#* }" | head -1)
check "transition → In Progress" 204 "$(call POST "$API/issue/$KEY/transitions" "{\"transition\":{\"id\":\"$TID\"}}")"
check "add comment" 201 "$(call POST "$API/issue/$KEY/comment" '{"body":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"jira-smoke comment"}]}]}}')" '.id'
check "list comments" 200 "$(call GET "$API/issue/$KEY/comment?maxResults=100&orderBy=created&startAt=0")" '"\(.total) comments"'
check "agile board" 200 "$(call GET "$BASE/rest/agile/1.0/board?projectKeyOrId=$PROJECT&maxResults=2")" '"total \(.total)"'

echo
if [[ $fail == 0 ]]; then
  echo "All checks passed. Created $KEY — open $BASE/browse/$KEY"
else
  echo "Some checks failed."
fi
exit $fail
