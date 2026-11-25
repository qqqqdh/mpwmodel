package storage

import (
	"path/filepath"
	"testing"

	"pass/core"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.dat")
	salt, _ := core.GenerateSalt()
	key := core.DeriveKey([]byte("correct horse battery"), salt)
	in := &core.Vault{Accounts: []core.Account{{Service: "g", Username: "u", Password: []byte("p")}}}

	if err := SaveVault(path, in, key, salt); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadSalt(path); string(got) != string(salt) {
		t.Fatal("salt mismatch")
	}
	out, err := LoadVault(path, key)
	if err != nil || len(out.Accounts) != 1 || string(out.Accounts[0].Password) != "p" {
		t.Fatalf("round trip failed: %v %+v", err, out)
	}
	if _, err := LoadVault(path, core.DeriveKey([]byte("wrong"), salt)); err == nil {
		t.Fatal("wrong key must fail")
	}
}

func TestSaveKeepsBackupAndVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.dat")
	salt, _ := core.GenerateSalt()
	key := core.DeriveKey([]byte("correct horse battery"), salt)

	for _, svc := range []string{"first", "second"} {
		v := &core.Vault{Accounts: []core.Account{{Service: svc}}}
		if err := SaveVault(path, v, key, salt); err != nil {
			t.Fatal(err)
		}
	}
	cur, err := LoadVault(path, key)
	if err != nil || cur.Accounts[0].Service != "second" {
		t.Fatalf("vault: %v %+v", err, cur)
	}
	bak, err := LoadVault(path+".bak", key)
	if err != nil || bak.Accounts[0].Service != "first" {
		t.Fatalf("backup should hold previous version: %v %+v", err, bak)
	}
}
