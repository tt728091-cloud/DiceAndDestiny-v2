#!/bin/zsh
mode=$1; shift
cd "${0:A:h}"
mkdir -p out8
for id in "$@"; do
  /Applications/Blender.app/Contents/MacOS/Blender -b --factory-startup -P concepts8.py -- $id out8/${mode}_$id.png $mode > out8/log_$id.txt 2>&1
  if grep -q "Saved:" out8/log_$id.txt; then echo "$id ok $(grep -E '^Time' out8/log_$id.txt)"; else echo "$id FAILED"; grep -E "Error|line [0-9]+" out8/log_$id.txt | tail -6; fi
done
