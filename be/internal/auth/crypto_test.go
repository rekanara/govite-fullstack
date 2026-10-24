package auth

import "testing"

// Hashes from the Nest API (node-argon2) must keep working after migrating.
func TestVerifyNodeArgon2Hash(t *testing.T) {
	node := "$argon2id$v=19$m=65536,t=3,p=4$sjA8DVcUMg3+41FwKvbd5g$cwktOnespIDRKLsk3tk2TCTAsWcpaZEPMriuuv5bGmk"
	if !VerifyPassword(node, "Password123!") || VerifyPassword(node, "wrong") {
		t.Fatal("node-argon2 hash not compatible")
	}
	h, err := HashPassword("Password123!")
	if err != nil || !VerifyPassword(h, "Password123!") {
		t.Fatal("roundtrip failed")
	}
}

func TestEncryptRoundtrip(t *testing.T) {
	k := NewKeys("x", "pepper-pepper-pepper")
	got, err := k.Decrypt(k.Encrypt("JBSWY3DPEHPK3PXP"))
	if err != nil || got != "JBSWY3DPEHPK3PXP" {
		t.Fatal(got, err)
	}
}
