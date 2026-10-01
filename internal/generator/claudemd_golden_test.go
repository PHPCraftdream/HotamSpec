package generator

import (
	"crypto/sha256"
	"encoding/hex"
)

// fullProfileCrystalGolden is the sha256 of the full-profile crystal rendered
// for the fixture graph by the generator BEFORE the lightweight consumer
// profile landed; the full profile must stay byte-identical.
const fullProfileCrystalGolden = "b54093902af07bde09b33fb8dd63a60d6f504533b790f456125c79221a4b07e5"

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
