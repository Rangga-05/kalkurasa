# Asumsi dan Keputusan — Kalkurasa

Setiap asumsi atau penyimpangan dari spesifikasi dicatat di sini dengan alasan singkat.

---

## Tahap 0 — Fondasi

### A0-01: Pola deploy Vercel Go — Single Server Entrypoint
**Keputusan:** Menggunakan pola *Go Framework Preset* dengan entrypoint `cmd/api/main.go`, bukan pola file-based `api/` directory.
**Alasan:** Dokumentasi Vercel terbaru (Nov 2026) merekomendasikan pola single server entrypoint untuk aplikasi Go dengan custom router (Gin). Vercel mendeteksi `go.mod` di Root Directory dan menjalankan server yang listen pada `PORT`. Pola `api/index.go` cocok untuk fungsi individual, bukan router lengkap.

### A0-02: Package `internal/server` sebagai pembuat router bersama
**Keputusan:** Gin router dibuat di `internal/server/server.go` dan dipanggil oleh `cmd/api/main.go`.
**Alasan:** Memisahkan pembuatan router dari entrypoint agar kode bisa dipakai ulang dan lebih mudah diuji.

### A0-03: Versi Go di `go.mod` = 1.26.0
**Keputusan:** `go.mod` menyatakan `go 1.26.0` sesuai versi Go yang terpasang di mesin pengembangan.
**Alasan:** Vercel membaca versi dari `go.mod` dan mengunduh toolchain yang sesuai. Memakai versi yang sama dengan lokal mencegah perbedaan perilaku.

### A0-04: `/api/healthz` tidak bergantung pada database
**Keputusan:** Endpoint selalu mengembalikan `200 OK` dengan `"status": "ok"`. Ping database dilakukan opsional dengan timeout 2 detik dan dilaporkan sebagai field `"db"` (`"ok"`, `"error"`, atau `"not_configured"`).
**Alasan:** Health check harus tetap berjalan meskipun database belum dikonfigurasi atau sedang down. Vercel dan load balancer membutuhkan health check yang selalu responsif.

### A0-05: `Today()` dipanggil malas, aman tanpa tabel settings
**Keputusan:** `Today()` menerima `*pgxpool.Pool` yang bisa nil. Jika pool nil, atau baris `demo_today` tidak ada, atau nilainya `null`/kosong, fungsi mengembalikan tanggal hari ini di zona Asia/Jakarta.
**Alasan:** Pada Tahap 0, tabel settings mungkin belum diisi (migrasi belum dijalankan ke database). `Today()` harus aman dipanggil kapan saja tanpa panic.

### A0-06: Import `time/tzdata` di paket `db`
**Keputusan:** Paket `db/today.go` mengimpor `time/tzdata` untuk menyematkan data zona waktu.
**Alasan:** Environment serverless Vercel mungkin tidak memiliki data timezone sistem (`zoneinfo`). Embed dari Go standard library memastikan `time.LoadLocation("Asia/Jakarta")` selalu berhasil.

### A0-07: Migrasi dicatat di tabel `schema_migrations`
**Keputusan:** `cmd/migrate` membuat tabel `schema_migrations` (version, applied_at) dan mencatat setiap file SQL yang sudah dijalankan. File yang sudah tercatat dilewati.
**Alasan:** Mencegah migrasi dijalankan dua kali dan memberikan jejak audit kapan migrasi diterapkan.

### A0-08: Semua route backend di bawah prefix `/api`
**Keputusan:** Health check ada di `/api/healthz`, bukan `/healthz`. Semua endpoint yang akan datang juga di bawah `/api/v1/...`.
**Alasan:** Konsisten dengan rewrite frontend (`/api/:path*` → backend) dan mencegah konflik path dengan halaman Next.js.

### A0-09: Frontend memakai `npm` (bukan pnpm)
**Keputusan:** Package manager frontend adalah npm.
**Alasan:** pnpm tidak terpasang di mesin pengembangan; npm tersedia bawaan Node.js.

### A0-10: Region Vercel `sin1` (Singapura)
**Keputusan:** `backend/vercel.json` menyetel `"regions": ["sin1"]`.
**Alasan:** Supabase project akan dibuat di Asia Tenggara (Singapura); backend function di region terdekat meminimalkan latensi koneksi database.

### A0-11: RLS aktif tanpa policy — backend memakai role `postgres`
**Keputusan:** Semua tabel mengaktifkan RLS tanpa policy. Backend diharapkan terhubung dengan role `postgres` yang melewati (bypass) RLS.
**Alasan:** Sesuai brief Bagian 2. Ini mencegah akses via Supabase Data API publik. Jika role koneksi Supabase bukan `postgres`, perlu ditambahkan policy atau dicatat di sini.

### A0-12: godotenv untuk memuat `.env` di pengembangan lokal
**Keputusan:** `cmd/api` dan `cmd/migrate` memanggil `godotenv.Load()` di awal `main()`. Panggilan ini tidak menghasilkan error jika file `.env` tidak ada, dan tidak menimpa variabel lingkungan yang sudah diatur di OS.
**Alasan:** Memudahkan pengembangan lokal tanpa harus mengekspor variabel satu per satu. Di Vercel (produksi), file `.env` tidak ada — variabel diatur lewat Project Settings.

### A0-13: Server development bind ke 127.0.0.1 saja
**Keputusan:** Jika `APP_ENV=development`, server Gin listen di `127.0.0.1:PORT` (loopback). Jika `production`, listen di `:PORT` (semua interface).
**Alasan:** Mencegah server development terekspos ke jaringan lokal secara tidak sengaja. Di Vercel, variabel `PORT` dan binding dikelola runtime.

### A0-14: Error database tidak memuat DSN atau password
**Keputusan:** `db.New()` mengembalikan pesan error generik tanpa menyertakan DSN asli. `/api/healthz` menyaring field `db_error` dari kata kunci sensitif (`password`, `postgres://`, `@`) dan memotong pesan panjang.
**Alasan:** Mencegah kebocoran kredensial database melalui log server atau respons API. Sesuai NFR-05 di spesifikasi.


---

## Tahap 1A — Engine bisnis (fungsi murni)

### A1-01: Cakupan Tahap 1A
**Keputusan:** Hanya fungsi yang diminta: Capacity, DayLoads, Utilization, DayUtilization, DayStatus, UnitCost, Margin, MinPrice, MaxDiscount, ShoppingNeed, CancellationExposure, Deviation, ShouldRecalibrate. ProjectCash, MinDP, DPCharged, EvaluateOrder, dan TC-04 ditunda ke Tahap 1B.
**Alasan:** Sesuai instruksi Tahap 1A; TC-04 bergantung pada ProjectCash/MinDP/DPCharged.

### A1-02: Helper `Average` ditambahkan
**Keputusan:** `engine.Average([]decimal.Decimal)` ditambahkan (tidak ada di daftar Bagian 7).
**Alasan:** TC-07 menuntut "rata-rata 102" dan rumus BR-12 butuh rata-rata aktual; menaruhnya di engine mencegah rumus bocor ke lapisan lain.

### A1-03: Pembagian memakai presisi eksplisit 18 digit
**Keputusan:** Semua pembagian memakai `DivRound(x, 18)`, bukan `decimal.Div`.
**Alasan:** `decimal.DivisionPrecision` adalah variabel global; presisi eksplisit membuat hasil deterministik. Sesuai BR-15, tidak ada pembulatan di dalam engine, pembulatan hanya untuk tampilan.

### A1-04: Batas status hari inklusif ke status berikutnya
**Keputusan:** util < 0,70 hijau; 0,70 ≤ util < 0,90 kuning; 0,90 ≤ util ≤ 1,00 merah; util > 1,00 penuh. Batas 1,00 tetap, hanya dua ambang lain yang dapat diatur.
**Alasan:** Mengikuti BR-04 ("70% sampai di bawah 90%", "90% sampai 100%", "di atas 100%").

### A1-05: Penghambat saat seri dipilih alfabetis
**Keputusan:** Bila dua sumber daya punya utilisasi sama, kode yang lebih dulu secara alfabet jadi penghambat; iterasi memakai kunci terurut.
**Alasan:** Iterasi map Go acak; hasil harus deterministik (NFR-08).

### A1-06: Penjaga pembagi nol mengembalikan 0
**Keputusan:** Utilization (kapasitas ≤ 0), Margin (harga ≤ 0), MinPrice (margin minimum ≥ 1), MaxDiscount (harga ≤ 0), Deviation (estimasi ≤ 0) mengembalikan 0 tanpa panic.
**Alasan:** Fungsi murni tanpa error pada signature brief; skema database sudah menjamin kapasitas dan harga valid, jadi ini hanya pengaman.

### A1-07: MaxDiscount dibatasi minimal 0
**Keputusan:** Bila harga minimal melebihi harga normal, diskon maksimum 0, bukan angka negatif.
**Alasan:** Diskon negatif tidak bermakna bagi pengguna; ini satu-satunya penyimpangan dari rumus mentah BR-06 `1 − min/harga`.

### A1-08: ShoppingNeed — kasus tepi pembulatan
**Keputusan:** `need = 0` memberi `buy = 0`. Rounding `pack` dengan `packSize ≤ 0`, atau nilai rounding tak dikenal, memakai pembulatan `loose`. `buy` dinyatakan dalam satuan pakai (kelipatan packSize untuk `pack`).
**Alasan:** Menghindari pembagian nol dan nilai tak terdefinisi; TC-05 menunjukkan `pack 25 → buy 25` dalam satuan pakai (kg).

### A1-09: Beban yang sudah ada dinyatakan sebagai LoadItem dengan Qty = 1
**Keputusan:** Beban existing yang hanya diketahui totalnya (mis. tenaga 1.020 menit pada TC-01) dimasukkan sebagai `LoadItem{Qty: 1, Load: total}`.
**Alasan:** BR-02 mendefinisikan beban sebagai Σ(qty × beban); satu tipe cukup untuk beban agregat dan beban per produk.

### A1-10: Tanpa state global di engine
**Keputusan:** Ambang BR-12 (0,10) berupa fungsi dan jumlah sampel minimum berupa konstanta, bukan variabel paket; engine tidak mengimpor time, database, maupun HTTP.
**Alasan:** Aturan Bagian 1 butir 2 dan instruksi Tahap 1A.

### A1-11: Perilaku `ShouldRecalibrate` pada tepat 10%
**Keputusan:** |deviasi| tepat 0,10 tidak memicu saran (harus di atas 0,10); tepat 5 sampel sudah cukup.
**Alasan:** BR-12 berbunyi "di atas 10%" dan "sampel minimal 5".
