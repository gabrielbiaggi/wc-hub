package application

import "testing"

func TestTOTPCodeMatchesRFC6238Vector(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if got := totpCode(secret, 1); got != "287082" {
		t.Fatalf("expected RFC vector 287082, got %s", got)
	}
}
