#!/bin/bash
# Focused Access Control Test Script
# Validates that users cannot access or operate on wallets they do not own.
# Independent from the comprehensive test_api.sh.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
SENDER_USER="testsender"
RECEIVER_USER="testreceiver"
PASSWORD="password123"

print() { echo -e "$1"; }
pass() { echo -e "${GREEN}PASS${NC} - $1"; }
fail() { echo -e "${RED}FAIL${NC} - $1"; exit 1; }
step() { echo -e "\n${YELLOW}==> $1${NC}"; }

expect_code() {
  local got=$1
  local expected=$2
  local message=$3
  if [ "$got" -eq "$expected" ]; then
    pass "$message (HTTP $got)"
  else
    fail "$message (expected $expected got $got)"
  fi
}

register_user() {
  local username=$1
  local payload="{\"username\": \"$username\", \"password\": \"$PASSWORD\"}"
  response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" -d "$payload")
  code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | head -n -1)
  if [ "$code" -eq 201 ]; then
    pass "Registered user $username"
  elif [ "$code" -eq 400 ] && echo "$body" | grep -qi "already exists"; then
    pass "User $username already exists (idempotent)"
  else
    fail "Registration failed for $username (code $code) body: $body"
  fi
}

login_user() {
  local username=$1
  local payload="{\"username\": \"$username\", \"password\": \"$PASSWORD\"}"
  response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" -d "$payload")
  code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | head -n -1)
  if [ "$code" -eq 200 ]; then
    token=$(echo "$body" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$token" ]; then
      fail "Token extraction failed for $username"
    fi
    echo "$token"
  else
    fail "Login failed for $username (code $code) body: $body"
  fi
}

get_or_create_wallet() {
  local token=$1
  # list wallets
  response=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" "$BASE_URL/wallets")
  code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | head -n -1)
  if [ "$code" -ne 200 ]; then
    fail "List wallets failed (code $code) body: $body"
  fi
  wallet_id=$(echo "$body" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2 || true)
  if [ -n "$wallet_id" ]; then
    echo "$wallet_id"
    return 0
  fi
  # create wallet
  response=$(curl -s -w "\n%{http_code}" -X POST -H "Authorization: Bearer $token" -H "Content-Type: application/json" "$BASE_URL/wallets")
  code=$(echo "$response" | tail -n1)
  body=$(echo "$response" | head -n -1)
  if [ "$code" -ne 201 ]; then
    fail "Create wallet failed (code $code) body: $body"
  fi
  wallet_id=$(echo "$body" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)
  if [ -z "$wallet_id" ]; then
    fail "Could not parse created wallet id"
  fi
  echo "$wallet_id"
}

#############################################
# EXECUTION
#############################################
print "${BLUE}ACCESS CONTROL TEST START${NC}"

step "Register users (idempotent)"
register_user "$SENDER_USER"
register_user "$RECEIVER_USER"

step "Login users"
SENDER_TOKEN=$(login_user "$SENDER_USER")
RECEIVER_TOKEN=$(login_user "$RECEIVER_USER")

step "Ensure wallets exist"
SENDER_WALLET_ID=$(get_or_create_wallet "$SENDER_TOKEN")
RECEIVER_WALLET_ID=$(get_or_create_wallet "$RECEIVER_TOKEN")
print "Sender wallet: $SENDER_WALLET_ID | Receiver wallet: $RECEIVER_WALLET_ID"

step "Receiver attempts to GET sender wallet (expect 403)"
r=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $RECEIVER_TOKEN" "$BASE_URL/wallets/$SENDER_WALLET_ID")
code=$(echo "$r" | tail -n1); body=$(echo "$r" | head -n -1)
expect_code "$code" 403 "Receiver forbidden from sender wallet"

step "Sender attempts to GET receiver wallet (expect 403)"
r=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $SENDER_TOKEN" "$BASE_URL/wallets/$RECEIVER_WALLET_ID")
code=$(echo "$r" | tail -n1); body=$(echo "$r" | head -n -1)
expect_code "$code" 403 "Sender forbidden from receiver wallet"

step "Receiver attempts sender transaction history (expect 403)"
r=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $RECEIVER_TOKEN" "$BASE_URL/wallets/$SENDER_WALLET_ID/transactions?limit=5&offset=0")
code=$(echo "$r" | tail -n1)
expect_code "$code" 403 "Receiver forbidden from sender transactions"

step "Sender attempts receiver transaction history (expect 403)"
r=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $SENDER_TOKEN" "$BASE_URL/wallets/$RECEIVER_WALLET_ID/transactions?limit=5&offset=0")
code=$(echo "$r" | tail -n1)
expect_code "$code" 403 "Sender forbidden from receiver transactions"

step "Sender attempts deposit into receiver wallet (expect 403)"
payload="{\"wallet_id\": $RECEIVER_WALLET_ID, \"amount\": 10.00, \"description\": \"Should fail\"}"
r=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/wallets/deposit" -H "Authorization: Bearer $SENDER_TOKEN" -H "Content-Type: application/json" -d "$payload")
code=$(echo "$r" | tail -n1)
expect_code "$code" 403 "Unauthorized deposit blocked"

step "Receiver attempts transfer from sender wallet (expect 403)"
payload="{\"from_wallet_id\": $SENDER_WALLET_ID, \"to_wallet_id\": $RECEIVER_WALLET_ID, \"amount\": 5.00, \"description\": \"Unauthorized attempt\"}"
r=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/wallets/transfer" -H "Authorization: Bearer $RECEIVER_TOKEN" -H "Content-Type: application/json" -d "$payload")
code=$(echo "$r" | tail -n1)
expect_code "$code" 403 "Unauthorized transfer blocked"

step "Control: valid transfer sender -> receiver (expect 200 if funds)"
payload="{\"from_wallet_id\": $SENDER_WALLET_ID, \"to_wallet_id\": $RECEIVER_WALLET_ID, \"amount\": 1.25, \"description\": \"Control transfer\"}"
r=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/wallets/transfer" -H "Authorization: Bearer $SENDER_TOKEN" -H "Content-Type: application/json" -d "$payload")
code=$(echo "$r" | tail -n1)
if [ "$code" -eq 200 ]; then
  pass "Control transfer succeeded (expected)"
else
  echo "(If 400 insufficient funds, top up and re-run)"
  fail "Control transfer unexpected code $code"
fi

step "Non-existent wallet should return 404 (not 403)"
r=$(curl -s -w "\n%{http_code}" -H "Authorization: Bearer $SENDER_TOKEN" "$BASE_URL/wallets/999999")
code=$(echo "$r" | tail -n1)
expect_code "$code" 404 "Non-existent wallet returns 404"

print "\n${GREEN}ACCESS CONTROL TESTS PASSED${NC}"
