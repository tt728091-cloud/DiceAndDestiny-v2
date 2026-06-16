package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Fingerprint(spec Spec) (string, error) {
	canonical := spec
	canonical.BattleID = ""
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
