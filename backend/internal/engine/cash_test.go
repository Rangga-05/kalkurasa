package engine

import "testing"

func TestTC06_CancellationExposure(t *testing.T) {
	assertExact(t, "eksposur", CancellationExposure(d("1300000"), d("712500")), d("587500"))
}

func TestCancellationExposureFloor(t *testing.T) {
	tests := []struct {
		name     string
		shop, dp string
		want     string
	}{
		{"DP melebihi belanja → 0", "500000", "712500", "0"},
		{"DP sama dengan belanja → 0", "712500", "712500", "0"},
		{"belum ada DP", "1300000", "0", "1300000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertExact(t, "eksposur", CancellationExposure(d(tc.shop), d(tc.dp)), d(tc.want))
		})
	}
}
