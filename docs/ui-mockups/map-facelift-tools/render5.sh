#!/bin/zsh
mode=$1; shift
cd "${0:A:h}"
mkdir -p out5
for id in "$@"; do
  /Applications/Blender.app/Contents/MacOS/Blender -b --factory-startup -P concepts5.py -- $id out5/${mode}_$id.png $mode > out5/log_$id.txt 2>&1
  if grep -q "Saved:" out5/log_$id.txt; then echo "$id ok $(grep -E '^Time' out5/log_$id.txt)"; else echo "$id FAILED"; grep -E "Error|line [0-9]+" out5/log_$id.txt | tail -6; fi
done
