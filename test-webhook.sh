#!/usr/bin/env bash
set -euo pipefail

URL="http://localhost:5500/api/v1/integrations/github/webhook"
SECRET="${GITHUB_WEBHOOK_SECRET}"

cat > /tmp/jtd-push.json <<'EOF'
{
  "ref": "refs/heads/master",
  "before": "0000000000000000000000000000000000000000",
  "after": "c17b1ae6721c87fc3462bfe709e670a85d5a8ff9",
  "repository": {
    "id": 123456789,
    "name": "jittrippin",
    "full_name": "nxrmqlly/jittrippin",
    "default_branch": "master"
  },
  "sender": {
    "login": "nxrmqlly"
  }
}
EOF

PAYLOAD="/tmp/jtd-push.json"

SIGNATURE="$(
    openssl dgst -sha256 -hmac "$SECRET" "$PAYLOAD" |
    awk '{print "sha256=" $2}'
)"

echo "Signature: $SIGNATURE"
echo

curl -i \
    -X POST \
    "$URL" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: push" \
    -H "X-Hub-Signature-256: $SIGNATURE" \
    --data-binary "@$PAYLOAD"