package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

// snackBox: 12 menit tenaga dan 0,05 oven-jam per unit.
func snackBoxLoads(qty string) []LoadItem {
	return []LoadItem{
		{ResourceCode: "labor", Qty: d(qty), Load: d("12")},
		{ResourceCode: "oven", Qty: d(qty), Load: d("0.05")},
	}
}

func TestCapacity(t *testing.T) {
	assertExact(t, "kapasitas tenaga", Capacity(testLabor), d("1428"))
	assertExact(t, "kapasitas oven", Capacity(testOven), d("13.6"))
}

func TestTC01_SaturdayFull(t *testing.T) {
	existing := []LoadItem{
		{ResourceCode: "labor", Qty: d("1"), Load: d("1020")},
		{ResourceCode: "oven", Qty: d("1"), Load: d("5.0")},
	}
	loads := DayLoads(append(existing, snackBoxLoads("100")...))
	caps := testCaps()

	tests := []struct {
		name string
		got  func() decimal.Decimal
		want string
		dp   int32 // 0 = persis; >0 = dibulatkan
	}{
		{"kapasitas tenaga", func() decimal.Decimal { return caps["labor"] }, "1428", 0},
		{"kapasitas oven", func() decimal.Decimal { return caps["oven"] }, "13.6", 0},
		{"beban tenaga", func() decimal.Decimal { return loads["labor"] }, "2220", 0},
		{"beban oven", func() decimal.Decimal { return loads["oven"] }, "10", 0},
		{"util tenaga", func() decimal.Decimal { return Utilization(loads["labor"], caps["labor"]) }, "1.5546", 4},
		{"util oven", func() decimal.Decimal { return Utilization(loads["oven"], caps["oven"]) }, "0.7353", 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dp == 0 {
				assertExact(t, tc.name, tc.got(), d(tc.want))
			} else {
				assertRounded(t, tc.name, tc.got(), d(tc.want), tc.dp)
			}
		})
	}

	t.Run("status dan penghambat", func(t *testing.T) {
		util, bottleneck := DayUtilization(loads, caps)
		assertRounded(t, "util hari", util, d("1.5546"), 4)
		assertString(t, "penghambat", bottleneck, "labor")
		assertString(t, "status hari", DayStatus(util, d("0.70"), d("0.90")), StatusFull)
	})
}

func TestTC02_SundayYellow(t *testing.T) {
	existing := []LoadItem{{ResourceCode: "labor", Qty: d("1"), Load: d("60")}}
	loads := DayLoads(append(existing, snackBoxLoads("100")...))
	caps := testCaps()

	assertExact(t, "beban tenaga", loads["labor"], d("1260"))

	util, bottleneck := DayUtilization(loads, caps)
	assertRounded(t, "util hari", util, d("0.8824"), 4)
	assertString(t, "penghambat", bottleneck, "labor")
	assertString(t, "status hari", DayStatus(util, d("0.70"), d("0.90")), StatusYellow)
}

func TestDayStatusBoundaries(t *testing.T) {
	green, yellow := d("0.70"), d("0.90")
	tests := []struct {
		util string
		want string
	}{
		{"0", StatusGreen},
		{"0.6999", StatusGreen},
		{"0.70", StatusYellow}, // batas hijau/kuning inklusif ke kuning
		{"0.8999", StatusYellow},
		{"0.90", StatusRed}, // batas kuning/merah inklusif ke merah
		{"0.9999", StatusRed},
		{"1.00", StatusRed}, // merah sampai 1,00
		{"1.0001", StatusFull},
		{"1.5546", StatusFull},
	}
	for _, tc := range tests {
		t.Run(tc.util, func(t *testing.T) {
			assertString(t, "util="+tc.util, DayStatus(d(tc.util), green, yellow), tc.want)
		})
	}
}

func TestDayStatusCustomThresholds(t *testing.T) {
	// Ambang dapat diatur (BR-04), batas penuh tetap 1,00.
	assertString(t, "0.50 dg hijau<0.5", DayStatus(d("0.50"), d("0.5"), d("0.8")), StatusYellow)
	assertString(t, "0.80 dg kuning<0.8", DayStatus(d("0.80"), d("0.5"), d("0.8")), StatusRed)
}

func TestDayUtilizationEdgeCases(t *testing.T) {
	caps := testCaps()

	t.Run("tanpa beban", func(t *testing.T) {
		util, _ := DayUtilization(map[string]decimal.Decimal{}, caps)
		assertExact(t, "util", util, decimal.Zero)
	})
	t.Run("oven menjadi penghambat", func(t *testing.T) {
		loads := map[string]decimal.Decimal{"labor": d("100"), "oven": d("13.6")}
		util, bottleneck := DayUtilization(loads, caps)
		assertExact(t, "util", util, d("1"))
		assertString(t, "penghambat", bottleneck, "oven")
	})
	t.Run("seri dipilih alfabetis", func(t *testing.T) {
		c := map[string]decimal.Decimal{"a": d("10"), "b": d("10")}
		_, bottleneck := DayUtilization(map[string]decimal.Decimal{"a": d("5"), "b": d("5")}, c)
		assertString(t, "penghambat", bottleneck, "a")
	})
	t.Run("kapasitas nol tidak panik", func(t *testing.T) {
		assertExact(t, "util", Utilization(d("10"), decimal.Zero), decimal.Zero)
	})
}

