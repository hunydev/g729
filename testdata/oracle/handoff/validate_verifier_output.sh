#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: sh testdata/oracle/handoff/validate_verifier_output.sh <returned-expected-csv-dir>" >&2
  exit 2
fi

src_dir="$1"
handoff_dir="testdata/oracle/handoff"

if [ ! -d "$src_dir" ]; then
  echo "$src_dir is not a directory" >&2
  exit 2
fi

validate_expected() {
  name="$1"
  key_columns="$2"
  src="$src_dir/$name"
  dst="$handoff_dir/$name"

  [ -f "$src" ] || return 0
  found_any=1

  awk -F, -v keys="$key_columns" '
    NR == FNR {
      if (FNR == 1) {
        header = $0
        headerNF = NF
        next
      }
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      key = $1
      for (i = 2; i <= keys; i++) {
        key = key "," $i
      }
      rows++
      templateKey[rows] = key
      next
    }
    FNR == 1 {
      if ($0 != header) {
        printf "%s header changed; expected %s\n", FILENAME, header > "/dev/stderr"
        exit 1
      }
      if (NF != headerNF) {
        printf "%s header has %d columns, want %d\n", FILENAME, NF, headerNF > "/dev/stderr"
        exit 1
      }
      next
    }
    {
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      row = FNR - 1
      if (row > rows) {
        printf "%s has extra data row at line %d\n", FILENAME, FNR > "/dev/stderr"
        exit 1
      }
      key = $1
      for (i = 2; i <= keys; i++) {
        key = key "," $i
      }
      if (key != templateKey[row]) {
        printf "%s line %d key changed; got %s want %s\n", FILENAME, FNR, key, templateKey[row] > "/dev/stderr"
        exit 1
      }
      if ($NF !~ /^-?[0-9]+$/) {
        printf "%s line %d expected cell is not a numeric scalar\n", FILENAME, FNR > "/dev/stderr"
        exit 1
      }
      filled++
    }
    END {
      if (filled != rows) {
        printf "%s filled rows=%d, want %d complete numeric expected cells\n", FILENAME, filled, rows > "/dev/stderr"
        exit 1
      }
      printf "%s: validated %d numeric expected cells\n", FILENAME, filled
    }
  ' "$dst" "$src"

  validated_files="$validated_files $name"
}

validate_fcb_position_clarification() {
  name="decoder_itu_fcb_position_clarification_expected.csv"
  src="$src_dir/$name"
  template="$handoff_dir/decoder_itu_fcb_position_clarification_expected_template.csv"

  [ -f "$src" ] || return 0
  found_any=1

  awk -F, '
    NR == FNR {
      if (FNR == 1) {
        header = $0
        headerNF = NF
        next
      }
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      rows++
      templateC[rows] = $1
      next
    }
    FNR == 1 {
      if ($0 != header) {
        printf "%s header changed; expected %s\n", FILENAME, header > "/dev/stderr"
        exit 1
      }
      if (NF != headerNF) {
        printf "%s header has %d columns, want %d\n", FILENAME, NF, headerNF > "/dev/stderr"
        exit 1
      }
      next
    }
    {
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      row = FNR - 1
      if (row > rows) {
        printf "%s has extra data row at line %d\n", FILENAME, FNR > "/dev/stderr"
        exit 1
      }
      if ($1 != templateC[row]) {
        printf "%s line %d C key changed; got %s want %s\n", FILENAME, FNR, $1, templateC[row] > "/dev/stderr"
        exit 1
      }
      for (i = 2; i <= 10; i++) {
        if ($i !~ /^-?[0-9]+$/) {
          printf "%s line %d column %d is not a numeric scalar\n", FILENAME, FNR, i > "/dev/stderr"
          exit 1
        }
      }
      if ($11 != "formula_ok" && $11 != "formula_conflict" && $11 != "unknown") {
        printf "%s line %d note %s is not an allowed controlled note\n", FILENAME, FNR, $11 > "/dev/stderr"
        exit 1
      }
      filled += 10
    }
    END {
      if (filled != rows * 10) {
        printf "%s filled cells=%d, want %d numeric-or-controlled cells\n", FILENAME, filled, rows * 10 > "/dev/stderr"
        exit 1
      }
      printf "%s: validated %d numeric-or-controlled cells\n", FILENAME, filled
    }
  ' "$template" "$src"

  validated_files="$validated_files $name"
}

validate_decoder_tame_stage_wide() {
  name="decoder_tame_stage_wide_expected.csv"
  src="$src_dir/$name"
  dst="$handoff_dir/$name"

  [ -f "$src" ] || return 0
  found_any=1

  awk -F, '
    NR == FNR {
      if (FNR == 1) {
        header = $0
        headerNF = NF
        next
      }
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      rows++
      templateKey[rows] = $1 "," $2
      next
    }
    FNR == 1 {
      if ($0 != header) {
        printf "%s header changed; expected %s\n", FILENAME, header > "/dev/stderr"
        exit 1
      }
      if (NF != headerNF) {
        printf "%s header has %d columns, want %d\n", FILENAME, NF, headerNF > "/dev/stderr"
        exit 1
      }
      next
    }
    {
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      row = FNR - 1
      if (row > rows) {
        printf "%s has extra data row at line %d\n", FILENAME, FNR > "/dev/stderr"
        exit 1
      }
      key = $1 "," $2
      if (key != templateKey[row]) {
        printf "%s line %d key changed; got %s want %s\n", FILENAME, FNR, key, templateKey[row] > "/dev/stderr"
        exit 1
      }
      for (i = 3; i <= NF; i++) {
        value = $i
        sub(/\r$/, "", value)
        if (value !~ /^-?[0-9]+$/) {
          printf "%s line %d column %d is not a numeric scalar\n", FILENAME, FNR, i > "/dev/stderr"
          exit 1
        }
        filled++
      }
    }
    END {
      want = rows * (headerNF - 2)
      if (filled != want) {
        printf "%s filled cells=%d, want %d complete numeric cells\n", FILENAME, filled, want > "/dev/stderr"
        exit 1
      }
      printf "%s: validated %d numeric cells\n", FILENAME, filled
    }
  ' "$dst" "$src"

  validated_files="$validated_files $name"
}

validate_decoder_tame_stage_wide_onset() {
  name="decoder_tame_stage_wide_onset_expected_template.csv"
  src="$src_dir/$name"
  dst="$handoff_dir/$name"

  [ -f "$src" ] || return 0
  found_any=1

  awk -F, '
    NR == FNR {
      if (FNR == 1) {
        header = $0
        headerNF = NF
        next
      }
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      rows++
      templateKey[rows] = $1 "," $2
      next
    }
    FNR == 1 {
      if ($0 != header) {
        printf "%s header changed; expected %s\n", FILENAME, header > "/dev/stderr"
        exit 1
      }
      if (NF != headerNF) {
        printf "%s header has %d columns, want %d\n", FILENAME, NF, headerNF > "/dev/stderr"
        exit 1
      }
      next
    }
    {
      if (NF != headerNF) {
        printf "%s line %d has %d columns, want %d\n", FILENAME, FNR, NF, headerNF > "/dev/stderr"
        exit 1
      }
      row = FNR - 1
      if (row > rows) {
        printf "%s has extra data row at line %d\n", FILENAME, FNR > "/dev/stderr"
        exit 1
      }
      key = $1 "," $2
      if (key != templateKey[row]) {
        printf "%s line %d key changed; got %s want %s\n", FILENAME, FNR, key, templateKey[row] > "/dev/stderr"
        exit 1
      }
      for (i = 3; i <= NF; i++) {
        value = $i
        sub(/\r$/, "", value)
        if (value == "") {
          blanks++
          continue
        }
        if (value !~ /^-?[0-9]+$/) {
          printf "%s line %d column %d is neither blank nor numeric\n", FILENAME, FNR, i > "/dev/stderr"
          exit 1
        }
        filled++
      }
    }
    END {
      want = rows * (headerNF - 2)
      if (filled + blanks != want) {
        printf "%s accounted cells=%d, want %d\n", FILENAME, filled + blanks, want > "/dev/stderr"
        exit 1
      }
      printf "%s: validated %d numeric cells and %d blank cells\n", FILENAME, filled, blanks
    }
  ' "$dst" "$src"

  validated_files="$validated_files $name"
}

allowed_file() {
  case "$1" in
    fcb_tree_search_expected_template.csv|\
    fcb_tree_search_user_audio_expected_template.csv|\
    encoder_closedloop_stage_expected_template.csv|\
    decoder_itu_frame0_hp_input_inverse_expected_template.csv|\
    decoder_tame_stage_wide_onset_expected_template.csv|\
    decoder_tame_pre_acb_history_expected_template.csv|\
    decoder_tame_excitation_history_expected_template.csv|\
    decoder_support_tables_expected_template.csv|\
    decoder_itu_fcb_position_clarification_expected.csv|\
    decoder_tame_stage_wide_expected.csv)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

found_any=0
validated_files=""
entry_list=$(mktemp)
trap 'rm -f "$entry_list"' EXIT

find "$src_dir" -mindepth 1 -maxdepth 1 | sort > "$entry_list"
while IFS= read -r path; do
  if [ -L "$path" ]; then
    echo "$path is a symlink; verifier-returned expected CSV files must be regular files" >&2
    exit 1
  fi
  if [ ! -f "$path" ]; then
    echo "$path is not an allowed verifier-returned expected CSV file" >&2
    exit 1
  fi
  name=$(basename "$path")
  if ! allowed_file "$name"; then
    echo "$path is not an allowed verifier-returned expected CSV" >&2
    exit 1
  fi
done < "$entry_list"

validate_expected "fcb_tree_search_expected_template.csv" 4
validate_expected "fcb_tree_search_user_audio_expected_template.csv" 4
validate_expected "encoder_closedloop_stage_expected_template.csv" 6
validate_expected "decoder_itu_frame0_hp_input_inverse_expected_template.csv" 5
validate_expected "decoder_tame_pre_acb_history_expected_template.csv" 5
validate_expected "decoder_tame_excitation_history_expected_template.csv" 5
validate_expected "decoder_support_tables_expected_template.csv" 3
validate_fcb_position_clarification
validate_decoder_tame_stage_wide
validate_decoder_tame_stage_wide_onset

if [ "$found_any" -ne 1 ]; then
  echo "$src_dir contains no allowed verifier-returned expected CSV files" >&2
  exit 1
fi

if [ "${G729_APPLY_VERIFIER_OUTPUT:-0}" = "1" ]; then
  for name in $validated_files; do
    cp "$src_dir/$name" "$handoff_dir/$name"
    echo "$handoff_dir/$name: applied"
  done
else
  echo "validation only; set G729_APPLY_VERIFIER_OUTPUT=1 to copy validated files into $handoff_dir"
fi
