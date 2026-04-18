# Dice

Owns dice rules.

This is the first likely package for real gameplay work after the spike.

Responsibilities:

- dice pools
- numeric dice rolls such as 5D6 if needed
- symbolic dice values such as sword, shield, and focus
- deterministic rolling through injected RNG/fake rollers
- focused dice tests

This package should not know about Godot, C++, GDExtension, or JSON transport.
