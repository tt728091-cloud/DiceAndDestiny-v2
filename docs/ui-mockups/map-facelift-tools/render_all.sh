#!/bin/zsh
# usage: render_all.sh <mode: preview|final> ids...
mode=$1; shift
cd "${0:A:h}"
for id in "$@"; do
  /Applications/Blender.app/Contents/MacOS/Blender -b --factory-startup -P concepts.py -- $id out/${mode}_$id.png $mode > logs_$id.txt 2>&1
  if grep -q "Saved:" logs_$id.txt; then echo "$id ok $(grep -E '^Time' logs_$id.txt)"; else echo "$id FAILED"; grep -E "Error|line [0-9]+" logs_$id.txt | tail -6; fi
done
