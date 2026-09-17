# Dice randomness audit — September 15, 2026

## Finding

No repeated-seed, rapid-click, or cursor-reset bug was found in the reported reroll. The exact recorded results match successive, distinct draws from the existing generator. The production algorithm was not changed, and repeated faces remain legal.

## Reported battle

Battle: `learned-1789498819-10139738-91444`, seed `1789498827225662`, round 3.
Evidence: the workspace's `client/user/debug/authority-transcript.jsonl` (runtime-only; not copied into source control).

| Transcript event | UTC time | Dice rolled (one-based) | Full pool | Matching generator cursors (zero-based) |
| --- | --- | --- | --- | --- |
| 26618 | Initial roll | 1–5 | 6, 3, 4, 2, 4 | 85–89 |
| 26627 | 19:03:02.315049 | 1, 3, 5 | 5, 3, 5, 2, 6 | 90–92 |
| 26636 | 19:03:04.608075 | 1, 3, 5 | 5, 3, 5, 2, 6 | 93–95 |

The reroll requests specify zero-based indices `[0,2,4]`; both keep requests specify `[1,3]`. The rerolls occurred about 2.293 seconds apart. The kept 3 and 2 were correctly preserved. The three new numeric results were 5, 5, 6 on both rerolls. This was not a duplicate log event or an unsubmitted reroll.

For independent fair D6s, matching all three previous numeric results in their original positions has probability `1/216`, about **0.463% per three-die reroll**. Symbols repeat more often because Venom has three Fang faces, two Gland faces, and one Coil face.

## Implementation traced

- `random/random.go`: unseeded normal battles use Go `crypto/rand.Int`, which samples uniformly from the requested range using operating-system randomness. [Official Go documentation](https://pkg.go.dev/crypto/rand#Int).
- Learned battles intentionally pass a seed through `learned/session.go` → `mlsim/environment.go` → `authority.go`. They use `sha256-counter-v1`, hashing the battle seed and advancing cursor for every random draw. This is deterministic pseudorandomness for replay and debugging, not fresh physical entropy per die.
- The client currently chooses the learned seed once at battle creation/rematch from microsecond wall time XOR elapsed microseconds. That time-derived seed is **not regenerated on individual rolls**. It is not intended as a secret/unpredictable seed for adversarial multiplayer.
- `engine/settled_engine.go`: each requested die calls `namedIntn` separately. The ordinary live path falls back to `BattleSource`; scripted named sources are test injections. Keeping dice does not draw or reset randomness.
- The client submits keep, applies its returned pending-input state, then submits the reroll with explicit indices. The random cursor resides in battle state and survives cloning/serialization. Explicit replay/snapshot restoration intentionally restores the sequence.
- Reproducible mode maps a 64-bit hash value to a face using modulo. This has a theoretical imbalance of at most one input value in `2^64` between buckets. It cannot plausibly explain visible repeats. Changing this versioned algorithm for that negligible difference would change existing seeded replays; it was left intact.

## Verification added

- `engine/reroll_randomness_test.go`: replay the reported keep/reroll commands with the recorded seed and cursor; verify identical results consume six different draws, held dice remain unchanged, cloning and JSON restoration preserve state, and an exhausted reroll consumes no randomness.
- `random/distribution_test.go`: fast versus delayed calls produce the same sequence for the same seed; normal-mode draws stay in range and advance; invalid bounds consume nothing.
- Deterministic distribution regression: **600,000 D6 draws across six seeds**, including the reported seed, with all 36 adjacent-face transitions checked for gross bias/correlation.

| Measurement | Observed | Expected for fair dice |
| --- | --- | --- |
| Face 1 | 100,140 | 100,000 |
| Face 2 | 100,227 | 100,000 |
| Face 3 | 100,087 | 100,000 |
| Face 4 | 100,386 | 100,000 |
| Face 5 | 99,316 | 100,000 |
| Face 6 | 99,844 | 100,000 |
| Adjacent equal faces | 100,009 / 599,994 | 99,999 |
| Consecutive identical ordered triples | 936 / 199,992 | About 925.9 |

These checks provide evidence against gross bias and accidental state reuse; they do not mathematically prove randomness. No anti-repeat filtering or weighting was introduced.

Validation: focused randomness/regression tests and the complete server `go test ./...` suite passed. No production code or Godot behavior changed in this audit.
