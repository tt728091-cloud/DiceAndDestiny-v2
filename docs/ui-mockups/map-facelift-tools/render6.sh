#!/bin/zsh
mode=$1; shift
cd "${0:A:h}"
mkdir -p out6
for id in "$@"; do
  /Applications/Blender.app/Contents/MacOS/Blender -b --factory-startup -P concepts6.py -- $id out6/${mode}_$id.png $mode > out6/log_$id.txt 2>&1
  if grep -q "Saved:" out6/log_$id.txt; then echo "$id ok $(grep -E '^Time' out6/log_$id.txt)"; else echo "$id FAILED"; grep -E "Error|line [0-9]+" out6/log_$id.txt | tail -6; fi
done
