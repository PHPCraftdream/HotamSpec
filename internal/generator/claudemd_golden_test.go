package generator

import (
	"crypto/sha256"
	"encoding/hex"
)

// fullProfileCrystalGolden is the sha256 of the full-profile crystal rendered
// for the fixture graph by the generator BEFORE the lightweight consumer
// profile landed; the full profile must stay byte-identical.
const fullProfileCrystalGolden = "bf03fbf183519ab242362bb7159df14312a331e5158436bfee09a9350c7e793a"

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
