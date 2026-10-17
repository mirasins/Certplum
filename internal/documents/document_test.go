package documents

import "testing"

func TestRUT(t *testing.T) {
	for _, v := range []string{"12.345.678-5", "123456785"} {
		if !ValidRUT(v) {
			t.Fatalf("valid RUT rejected: %s", v)
		}
	}
	for _, v := range []string{"12.345.678-1", "", "1234", "ABC12345"} {
		if ValidRUT(v) {
			t.Fatalf("invalid RUT accepted: %s", v)
		}
	}
}
