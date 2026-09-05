#!/usr/bin/env bash
# Fails if a .vue file outside src/locales/ contains hardcoded French accented
# text and isn't listed in src/locales/.i18n-migration-allowlist.txt. The
# allowlist shrinks as each feature-folder batch is migrated to vue-i18n
# (see the i18n rollout plan) — this guardrail stops the un-migrated surface
# from growing in the meantime.
set -euo pipefail
cd "$(dirname "$0")/.."

# Force a UTF-8 locale: under a plain "C" locale, grep -P's Unicode
# character class below degenerates into byte-wise matching and produces
# false positives/negatives on any multi-byte UTF-8 sequence.
export LC_ALL=C.utf8

ALLOWLIST="src/locales/.i18n-migration-allowlist.txt"

mapfile -t offenders < <(
  grep -rlP '[éèàêôûîçÉÈÀÊÔÛÎÇœŒ]' src --include='*.vue' | grep -v '^src/locales/' | sort
)

mapfile -t allowed < <(grep -v '^\s*#' "$ALLOWLIST" | grep -v '^\s*$' | sort)

new_offenders=$(comm -23 <(printf '%s\n' "${offenders[@]}") <(printf '%s\n' "${allowed[@]}"))

if [[ -n "$new_offenders" ]]; then
  echo "Hardcoded French text found outside the i18n migration allowlist:"
  echo "$new_offenders" | sed 's/^/  /'
  echo
  echo "Extract these strings into src/locales/{fr,en}/*.json, or if they're"
  echo "genuinely not yet migrated, add them to $ALLOWLIST."
  exit 1
fi

echo "OK: no hardcoded French text outside the i18n migration allowlist."
