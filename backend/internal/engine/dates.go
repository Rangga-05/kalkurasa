package engine

import "time"

// DateKey mengubah tanggal menjadi kunci "YYYY-MM-DD" (tanggal kalender pada
// zona waktu milik t). Dipakai sebagai kunci ExistingLoads.
func DateKey(t time.Time) string { return t.Format("2006-01-02") }

// civil membuang jam dan zona: hasilnya 00:00 UTC pada tanggal kalender t.
func civil(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// addDays menambah n hari kalender (aman terhadap pergantian bulan).
func addDays(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+n, 0, 0, 0, 0, time.UTC)
}

// daysBetween = jumlah hari kalender dari from ke to (negatif bila to lebih awal).
func daysBetween(from, to time.Time) int {
	return int(civil(to).Sub(civil(from)) / (24 * time.Hour))
}

