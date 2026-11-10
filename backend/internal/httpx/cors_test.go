package httpx

import "testing"

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed []string
		want    bool
	}{
		{
			name:    "wildcard asterisk allows all",
			origin:  "https://kalkurasa-rouge.vercel.app",
			allowed: []string{"*"},
			want:    true,
		},
		{
			name:    "wildcard subdomain match",
			origin:  "https://kalkurasa-rouge.vercel.app",
			allowed: []string{"https://*.vercel.app"},
			want:    true,
		},
		{
			name:    "wildcard subdomain mismatch",
			origin:  "https://evil.com",
			allowed: []string{"https://*.vercel.app"},
			want:    false,
		},
		{
			name:    "exact match with trailing slash normalization",
			origin:  "http://localhost:3000/",
			allowed: []string{"http://localhost:3000"},
			want:    true,
		},
		{
			name:    "exact match allowed list has trailing slash",
			origin:  "http://localhost:3000",
			allowed: []string{"http://localhost:3000/"},
			want:    true,
		},
		{
			name:    "empty origin",
			origin:  "",
			allowed: []string{"*"},
			want:    false,
		},
		{
			name:    "comma separated list simulated via multiple elements",
			origin:  "http://127.0.0.1:3000",
			allowed: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OriginAllowed(tt.origin, tt.allowed)
			if got != tt.want {
				t.Errorf("OriginAllowed(%q, %v) = %v; want %v", tt.origin, tt.allowed, got, tt.want)
			}
		})
	}
}

