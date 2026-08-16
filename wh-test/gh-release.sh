#!/usr/bin/env bash
set -euo pipefail

URL="http://jt.ritam.cc/api/v1/integrations/github/webhook"
SECRET="${GITHUB_WEBHOOK_SECRET}"

cat > /tmp/jtd-release.json <<'EOF'
{
  "action": "published",
  "release": {
    "id": 1,
    "tag_name": "v0.1.0",
    "target_commitish": "master",
    "draft": false,
    "prerelease": false
  },
  "repository": {
    "id": 1301709047,
    "name": "jittrippin",
    "full_name": "nxrmqlly/jittrippin",
    "default_branch": "master"
  },
  "sender": { "login": "nxrmqlly" }
}
EOF

PAYLOAD="/tmp/jtd-release.json"

SIGNATURE="$(
    openssl dgst -sha256 -hmac "$SECRET" "$PAYLOAD" |
    awk '{print "sha256=" $2}'
)"

curl -i \
    -X POST \
    "$URL" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: $SIGNATURE" \
    --data-binary "@$PAYLOAD"