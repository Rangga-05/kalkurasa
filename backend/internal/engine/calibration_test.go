package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

func durations(vals ...string) []decimal.Decimal {
	out := make([]decimal.Decimal, len(vals))
	for i, v := range vals {
		out[i] = d(v)
	}
	return out
}

func TestTC07_Calibration(t *testing.T) {
	estimate := d("90")
	actual := durations("105", "100", "95", "110", "100")

	avg := Average(actual)
	dev := Deviation(avg, estimate)

	assertExact(t, "rata-rata", avg, d("102"))
	assertRounded(t, "deviasi", dev, d("0.1333"), 4)

	tests := []struct {
		name    string
		samples int
		want    bool
	}{
		{"5 sampel → saran muncul", len(actual), true},
		{"4 sampel → tidak muncul", len(actual[:4]), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldRecalibrate(dev, tc.samples)
			t.Logf("%-28s dihasilkan=%v diharapkan=%v", "rekalibrasi", got, tc.want)
			if got != tc.want {
				t.Errorf("dihasilkan %v, diharapkan %v", got, tc.want)
			}
		})
	}

	t.Run("4 sampel dari data nyata", func(t *testing.T) {
		four := actual[:4] // 105,100,95,110 → rata-rata 102,5
		dev4 := Deviation(Average(four), estimate)
		if ShouldRecalibrate(dev4, len(four)) {
			t.Errorf("4 sampel tidak boleh memicu rekalibrasi (deviasi %s)", dev4)
		}
	})
}

func TestShouldRecalibrateBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		dev     string
		samples int
		want    bool
	}{
		{"tepat 10% tidak memicu (harus di atas)", "0.10", 5, false},
		{"10,01% memicu", "0.1001", 5, true},
		{"deviasi negatif besar memicu", "-0.15", 5, true},
		{"deviasi negatif kecil tidak", "-0.05", 10, false},
		{"tepat 5 sampel cukup", "0.20", 5, true},
		{"deviasi besar, sampel kurang", "0.50", 4, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldRecalibrate(d(tc.dev), tc.samples)
			if got != tc.want {
				t.Errorf("dev=%s sampel=%d: dihasilkan %v, diharapkan %v", tc.dev, tc.samples, got, tc.want)
			}
		})
	}
}

func TestCalibrationGuards(t *testing.T) {
	assertExact(t, "rata-rata kosong", Average(nil), decimal.Zero)
	assertExact(t, "deviasi estimasi 0", Deviation(d("10"), decimal.Zero), decimal.Zero)
}

