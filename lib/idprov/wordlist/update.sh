#!/bin/bash

set -euxo pipefail

TMP=$(mktemp -d)
MAX=4
REPO="https://github.com/dolph/dictionary/"
FILE="popular.txt"

git clone "$REPO" "$TMP"

RE="^"

for i in $(seq 1 "$MAX"); do
  RE="$RE[a-z]?"
done

RE="$RE\$"

cat "$TMP/$FILE" | tr "[[:upper:]]" "[[:lower:]]" | sort | uniq | grep -E "$RE" > words.txt
