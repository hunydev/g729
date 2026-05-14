#!/bin/sh
set -eu

bundle_dir="${1:-/tmp/g729-fcb-verifier-handoff}"
archive="${2:-/tmp/g729-fcb-verifier-handoff-2026-05-10.tar.gz}"

handoff_dir="testdata/oracle/handoff"
bundle_handoff_dir="$bundle_dir/testdata/oracle/handoff"

require_blank_expected() {
  path="$1"
  awk -F, '
    NR > 1 && $NF != "" {
      printf "%s line %d has a filled expected cell; remaining verifier bundle templates must stay blank\n", FILENAME, NR > "/dev/stderr"
      exit 1
    }
  ' "$path"
}

if [ "${G729_ALLOW_FILLED_VERIFIER_BUNDLE:-0}" != "1" ]; then
  # The focused FCB templates may already contain verifier-filled numeric
  # results. The broad closed-loop stage remains the only outgoing blank
  # template from the post-FCB verifier flow. The pitch-instability decision
  # template may be partially filled and can be resent to fill remaining blanks.
  require_blank_expected "$handoff_dir/encoder_closedloop_stage_expected_template.csv"
fi

rm -rf "$bundle_dir"
mkdir -p "$bundle_handoff_dir"

cp \
  "$handoff_dir/HANDOFF_MANIFEST.md" \
  "$handoff_dir/README.md" \
  "$handoff_dir/EXTERNAL_VERIFIER_REQUEST.md" \
  "$handoff_dir/create_verifier_bundle.sh" \
  "$handoff_dir/validate_verifier_output.sh" \
  "$handoff_dir/REMAINING_CONFORMANCE_VERIFIER_PROMPT.md" \
  "$handoff_dir/FCB_TREE_SEARCH_VERIFIER_PROMPT.md" \
  "$handoff_dir/FCB_TREE_SEARCH_USER_AUDIO_VERIFIER_PROMPT.md" \
  "$handoff_dir/ENCODER_CLOSEDLOOP_STAGE_VERIFIER_PROMPT.md" \
  "$handoff_dir/DECODER_PITCH_INSTABILITY_DECISION_PROMPT.md" \
  "$handoff_dir/fcb_tree_search_expected_template.csv" \
  "$handoff_dir/fcb_tree_search_got.csv" \
  "$handoff_dir/fcb_tree_search_user_audio_expected_template.csv" \
  "$handoff_dir/fcb_tree_search_user_audio_got.csv" \
  "$handoff_dir/encoder_closedloop_stage_expected_template.csv" \
  "$handoff_dir/encoder_closedloop_stage_got.csv" \
  "$handoff_dir/decoder_pitch_instability_decision_expected_template.csv" \
  "$handoff_dir/decoder_pitch_instability_decision_got.csv" \
  "$bundle_handoff_dir/"

parent_dir=$(dirname "$bundle_dir")
base_dir=$(basename "$bundle_dir")
mkdir -p "$(dirname "$archive")"

tar -C "$parent_dir" \
  --sort=name \
  --mtime="UTC 2026-05-10 00:00:00" \
  --owner=0 --group=0 --numeric-owner \
  --use-compress-program="gzip -n" \
  -cf "$archive" \
  "$base_dir"

sha256sum "$archive"
