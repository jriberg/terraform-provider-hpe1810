#!/usr/bin/env bash
# Tag, push, build, sign and upload a provider release to the self-hosted
# boring-registry (registry.phirna.uk, see ohana's nomad/boring-registry.md).
#
#   scripts/release.sh 1.0.1
#   UPLOAD_ONLY=true scripts/release.sh 1.0.0   # tag + dist/ already exist,
#                                               # just (re-)upload + verify
#
# Needs: git, gpg (signing key in the keyring), goreleaser, vault (logged in,
# VAULT_ADDR/VAULT_TOKEN set), aws CLI, docker, dig, curl.
#
# Everything below can be overridden via env. S3 credentials are read from
# Vault and passed per-command only -- never exported -- so they can't leak
# into the shell and clash with tofu's own S3 backend credentials (that
# exact mix-up cost an afternoon on 2026-10-13).
set -euo pipefail

VERSION="${1:-}"
GPG_FINGERPRINT="${GPG_FINGERPRINT:-4F7EB1978A092E7CD82B8ADA444673E82E48AC05}"
NAMESPACE="${NAMESPACE:-phirna}"
REGISTRY="${REGISTRY:-registry.phirna.uk}"
BUCKET="${BUCKET:-boring-registry}"
S3_HOST="${S3_HOST:-garage-s3.service.consul}"
S3_KEY_ID="${S3_KEY_ID:-GK74a4126735bd5a44ff0754de}"
VAULT_PATH="${VAULT_PATH:-secret/nomad/boring-registry}"
BR_IMAGE="${BR_IMAGE:-ghcr.io/boring-registry/boring-registry:v0.19.0}"
PUBLISH_GITHUB="${PUBLISH_GITHUB:-false}" # true = also create a GitHub release (needs GITHUB_TOKEN)
UPLOAD_ONLY="${UPLOAD_ONLY:-false}"

NAME="$(basename "$(git rev-parse --show-toplevel)" | sed 's/^terraform-provider-//')"

die() { echo "error: $*" >&2; exit 1; }
step() { echo; echo "==> $*"; }

# --- preflight -------------------------------------------------------------
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
  || die "usage: $0 <MAJOR.MINOR.PATCH> (semver, no leading v) -- got '${VERSION}'"
TAG="v${VERSION}"

for bin in git gpg goreleaser vault aws docker dig curl; do
  command -v "$bin" >/dev/null || die "$bin not found in PATH"
done

if [[ "$UPLOAD_ONLY" != "true" ]]; then
  [[ "$(git rev-parse --abbrev-ref HEAD)" == "main" ]] || die "not on main"
  [[ -z "$(git status --porcelain)" ]] || die "working tree not clean"
  git fetch -q --tags origin
  [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] \
    || die "main is not in sync with origin/main (push or pull first)"
  if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
    die "tag ${TAG} already exists (use UPLOAD_ONLY=true to just upload dist/)"
  fi
fi

vault token lookup >/dev/null 2>&1 || die "vault not logged in (VAULT_ADDR/VAULT_TOKEN)"
S3_SECRET="$(vault kv get -field=s3_secret_key "$VAULT_PATH")"
API_TOKEN="$(vault kv get -field=auth_token "$VAULT_PATH")"

# Resolve on the host: the docker container doesn't reliably resolve .consul.
S3_IP="$(dig +short "$S3_HOST" | head -1)"
[[ -n "$S3_IP" ]] || die "could not resolve $S3_HOST"
S3_ENDPOINT="http://${S3_IP}:3900"

SUMS="dist/terraform-provider-${NAME}_${VERSION}_SHA256SUMS"

if [[ "$UPLOAD_ONLY" != "true" ]]; then
# --- unlock signing key ----------------------------------------------------
# goreleaser runs gpg with --batch, which can't prompt for a passphrase.
# Sign something interactively first so gpg-agent caches it.
step "Unlocking GPG key ${GPG_FINGERPRINT: -16}"
export GPG_TTY="$(tty)"
echo test | gpg --local-user "$GPG_FINGERPRINT" --clearsign >/dev/null

# --- tag + push ------------------------------------------------------------
step "Tagging ${TAG} and pushing"
git tag "$TAG"
git push -q origin "$TAG"

# --- build + sign ----------------------------------------------------------
step "Building with goreleaser"
GR_ARGS=(release --clean)
[[ "$PUBLISH_GITHUB" == "true" ]] || GR_ARGS+=(--skip=publish)
GPG_FINGERPRINT="$GPG_FINGERPRINT" goreleaser "${GR_ARGS[@]}"

fi

[[ -f "$SUMS" && -f "${SUMS}.sig" ]] || die "missing $SUMS or its .sig"

# .goreleaser.yml lists the registry manifest in SHA256SUMS under this name,
# but goreleaser only writes the renamed copy to dist/ when it publishes to
# GitHub. boring-registry's upload validates every SHA256SUMS entry, so with
# --skip=publish it fails with "failed to validate checksum for file
# ..._manifest.json" unless the file is there. Same bytes, so the signed
# checksum still matches.
cp terraform-registry-manifest.json "dist/terraform-provider-${NAME}_${VERSION}_manifest.json"
(cd dist && shasum -a 256 -c --quiet "$(basename "$SUMS")") || die "checksum mismatch in dist/"

# --- signing-keys.json (idempotent, cheap to re-upload) --------------------
step "Uploading signing-keys.json for namespace ${NAMESPACE}"
KEY_ID="${GPG_FINGERPRINT: -16}"
TMP_KEYS="$(mktemp)"
trap 'rm -f "$TMP_KEYS"' EXIT
gpg --armor --export "$KEY_ID" | python3 -c \
  "import json,sys;print(json.dumps({'gpg_public_keys':[{'key_id':'$KEY_ID','ascii_armor':sys.stdin.read()}]}))" \
  >"$TMP_KEYS"
AWS_ACCESS_KEY_ID="$S3_KEY_ID" AWS_SECRET_ACCESS_KEY="$S3_SECRET" AWS_REGION=garage \
  aws --endpoint-url "$S3_ENDPOINT" s3 cp --only-show-errors \
  "$TMP_KEYS" "s3://${BUCKET}/providers/${NAMESPACE}/signing-keys.json"

# --- upload to boring-registry ---------------------------------------------
step "Uploading ${NAME} ${VERSION} to ${REGISTRY}"
docker run --rm -v "$PWD/dist:/dist" \
  -e AWS_ACCESS_KEY_ID="$S3_KEY_ID" -e AWS_SECRET_ACCESS_KEY="$S3_SECRET" \
  "$BR_IMAGE" upload provider \
  --storage-s3-bucket "$BUCKET" \
  --storage-s3-endpoint "$S3_ENDPOINT" \
  --storage-s3-region garage --storage-s3-pathstyle \
  --namespace "$NAMESPACE" \
  --filename-sha256sums "/dist/$(basename "$SUMS")"

# --- verify ----------------------------------------------------------------
step "Verifying"
curl -fsS -H "Authorization: Bearer ${API_TOKEN}" \
  "https://${REGISTRY}/v1/providers/${NAMESPACE}/${NAME}/versions" \
  | V="$VERSION" python3 -c "import json,sys,os;v=[x['version'] for x in json.load(sys.stdin)['versions']];print('versions:',', '.join(v));sys.exit(0 if os.environ['V'] in v else 1)" \
  || die "${VERSION} not listed by the registry"

echo
echo "Done: ${REGISTRY}/${NAMESPACE}/${NAME} ${VERSION}"
