package twofa

import "testing"

func TestHashBackupCodeNormalization(t *testing.T) {
	pepper := []byte("test-pepper")
	// The canonical normalized form: 8 uppercase alphanumerics, no dash.
	canonical := "ABCD1234"

	// Same plaintext codes with different display/case forms must hash equal.
	forms := []string{
		canonical,
		"abcd1234",   // lowercase
		"ABCD-1234",  // display form with dash
		"abcd-1234",  // lowercase + dash
		" ABCD1234 ", // surrounding spaces (space is stripped)
		"AbCd-1234",  // mixed case + dash
	}
	want := hashBackupCode(pepper, canonical)
	for _, f := range forms {
		got := hashBackupCode(pepper, f)
		if got != want {
			t.Errorf("hashBackupCode(%q) = %s, want %s (= canonical)", f, got, want)
		}
	}
}

func TestHashBackupCodePepperMatters(t *testing.T) {
	// Without the pepper a DB-leak hash is different (the pepper is the app key).
	code := "ABCD1234"
	with := hashBackupCode([]byte("pepper"), code)
	without := hashBackupCode(nil, code)
	if with == without {
		t.Error("hash with pepper equals hash without pepper; pepper must change the digest")
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, hashes, err := generateBackupCodes([]byte("pepper"), 8)
	if err != nil {
		t.Fatalf("generateBackupCodes: %v", err)
	}
	if len(codes) != 8 || len(hashes) != 8 {
		t.Fatalf("got %d codes, %d hashes, want 8/8", len(codes), len(hashes))
	}
	seen := map[string]bool{}
	for i, c := range codes {
		// Display form is XXXX-XXXX (9 chars, one dash).
		if len(c) != 9 || c[4] != '-' {
			t.Errorf("code %d = %q, want XXXX-XXXX form", i, c)
		}
		if seen[c] {
			t.Errorf("duplicate code generated: %s", c)
		}
		seen[c] = true
		// The hash must match hashBackupCode of the display form (verifies the
		// stored hash matches what VerifyCodeOrBackup would compute on input).
		want := hashBackupCode([]byte("pepper"), c)
		if hashes[i] != want {
			t.Errorf("hash %d mismatch: stored %s vs recompute %s", i, hashes[i], want)
		}
	}
}
