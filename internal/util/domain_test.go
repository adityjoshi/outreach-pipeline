package util

import "testing"

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"stripe.com", "stripe.com"},
		{"https://www.Stripe.com/about", "stripe.com"},
		{"WWW.EXAMPLE.COM", "example.com"},
		{"  notion.so  ", "notion.so"},
	}

	for _, tc := range tests {
		got := NormalizeDomain(tc.in)
		if got != tc.want {
			t.Fatalf("NormalizeDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
