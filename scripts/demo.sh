#!/usr/bin/env bash
# retries a payment against ledsure and shows that money moves exactly once
# usage: make up -> ./scripts/demo.sh

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
PAUSE="${DEMO_PAUSE:-1}"
BANK_CAD="00000000-0000-7000-8000-000000000001"
RUN="$(date +%s)"

bold=$'\e[1m'; dim=$'\e[2m'; green=$'\e[32m'; red=$'\e[31m'; reset=$'\e[0m'

step() { printf '\n%s▸ %s%s\n' "$bold" "$1" "$reset"; sleep "$PAUSE"; }
show() { printf '%s$ %s%s\n' "$dim" "$1" "$reset"; }
id_of() { grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4; }
balance_of() { curl -s "$BASE_URL/v1/accounts/$1/balance" | grep -o '"balance":-\?[0-9]*' | cut -d: -f2; }

post() {
	local tmp status
	tmp="$(mktemp)"
	status="$(curl -s -o "$tmp" -w '%{http_code}' -X POST "$BASE_URL$1" -H "Idempotency-Key: $2" -d "$3")"
	printf '%s %s\n' "$status" "$(cat "$tmp")"
	rm -f "$tmp"
}

#step 1
step "open a customer account for Alice"

show "POST /v1/accounts  {\"name\":\"alice\",\"currency\":\"CAD\"}"
alice="$(post /v1/accounts "open-alice-$RUN" '{"name":"alice","currency":"CAD"}' | cut -d' ' -f2- | id_of)"
echo "alice = $alice   balance = $(balance_of "$alice")"

deposit="{\"description\":\"deposit\",\"entries\":[{\"account_id\":\"$BANK_CAD\",\"amount\":-10000},{\"account_id\":\"$alice\",\"amount\":10000}]}"

step "deposit \$100.00: client times out and retries same one 3 more tiems"

show "POST /v1/transactions  Idempotency-Key: deposit-$RUN   (x4)"
first=""
for attempt in 1 2 3 4; do
	result="$(post /v1/transactions "deposit-$RUN" "$deposit")"
	status="${result%% *}"; body="${result#* }"
	[[ -z "$first" ]] && first="$body"
	if [[ "$body" == "$first" ]]; then same="${green}identical to attempt 1${reset}"; else same="${red}DIFFERENT${reset}"; fi
	printf 'attempt %d → %s  transaction %s  %s\n' "$attempt" "$status" "$(id_of <<<"$body")" "$same"
	sleep 0.3
done

step "check balance: 4 requests, 1 deposit"

show "GET /v1/accounts/$alice/balance"
printf 'alice = %s%s%s cents\n' "$green" "$(balance_of "$alice")" "$reset"

step "reuse the key for different amount: rejected"

show "POST /v1/transactions  Idempotency-Key: deposit-$RUN  (amount 20000)"
post /v1/transactions "deposit-$RUN" "${deposit//10000/20000}"

step "try to withdraw more than Alice has: rejected"

show "POST /v1/transactions  alice -50000 → bank"
post /v1/transactions "withdraw-$RUN" "{\"entries\":[{\"account_id\":\"$alice\",\"amount\":-50000},{\"account_id\":\"$BANK_CAD\",\"amount\":50000}]}"
printf 'alice = %s%s%s cents\n' "$green" "$(balance_of "$alice")" "$reset"
