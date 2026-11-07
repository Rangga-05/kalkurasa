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

---

## Tahap 1B — Kas, DP, dan EvaluateOrder

### A1-12: "Hari ini" dan semua data datang lewat EvalInput
**Keputusan:** `EvalInput` membawa `Today`, `EventDate`, `ProductionDate`, `DPDate`, parameter bisnis, kapasitas, beban existing per tanggal (kunci `DateKey`), dan peristiwa kas pesanan lain. Engine tidak membaca jam sistem maupun database; pemanggil (Tahap 3) yang mengisi dari `Today()` dan query.
**Alasan:** Aturan engine murni; `Today()` menghormati `demo_today` sehingga harus diberikan dari luar.

### A1-13: Tanggal diperlakukan sebagai tanggal kalender
**Keputusan:** Semua perhitungan hari memakai tahun-bulan-hari dari nilai `time.Time` yang diberikan (jam dan zona dibuang). Peristiwa kas dicocokkan lewat `DateKey`.
**Alasan:** Menghindari pergeseran hari akibat zona waktu (WIB vs UTC).

### A1-14: ProjectCash — jendela dan tie-break
**Keputusan:** Jendela `[start, start+days)`; peristiwa di luar jendela diabaikan; titik terendah seri memilih tanggal paling awal; `days <= 0` menghasilkan proyeksi kosong dengan `Lowest = opening`.
**Alasan:** Brief tidak menjelaskan; pilihan paling sederhana dan deterministik.

### A1-15: Jadwal kas pesanan yang dievaluasi
**Keputusan:** Belanja keluar di `acara − shopping_lead_days` (dimajukan ke hari ini bila sudah lewat); pelunasan `(total − DP)` masuk `acara + 1`; DP masuk di `DPDate` (kosong = hari ini, tidak pernah sebelum hari ini); proyeksi berjalan dari hari ini sampai `acara + 1`. Saldo awal diasumsikan saldo pada hari ini.
**Alasan:** Mengikuti aturan penjadwalan Bagian 7; belanja tidak bisa terjadi di masa lalu.

### A1-16: DPDate disuplai pemanggil
**Keputusan:** Tanggal DP masuk ke kas untuk evaluasi G3 adalah input (`DPDate`), bukan dihitung engine dari `dp_deadline_hours`. Model BR-09 mengasumsikan DP menggeser seluruh lintasan saldo, jadi bila DP baru masuk setelah titik terendah, G3 bisa gagal walau DP sudah sebesar DP minimal. TC-04 memakai DP diterima di H-2 (= hari ini), sehingga pemanggil sebaiknya memberi `DPDate = Today` untuk evaluasi saat pesanan dibuat.
**Alasan:** Tenggat DP default 24 jam jatuh sehari setelah pesanan dan akan membuat G3 TC-04 gagal karena waktu, bukan karena nominal; keputusan ini perlu dikonfirmasi saat Tahap 3.

### A1-17: G3 gagal selalu berarti tanpa perbaikan DP
**Keputusan:** DP yang dievaluasi adalah `DPCharged` (sudah dinaikkan ke DP minimal). Jika `MinDP > total`, G3 gagal tanpa perbaikan (BR-09). `DPCharged` tetap mengembalikan nilai mentah `max(...)` yang bisa melebihi total; dalam simulasi kas DP dibatasi sebesar total.
**Alasan:** Mengikuti BR-09 secara harfiah; mencegah pelunasan negatif.

### A1-18: Definisi "ada perbaikan" dan tawaran balik
**Keputusan:** Rekomendasi: semua lolos → `accept`; G4 gagal → `reject`; G4 lolos dan ada tawaran balik terverifikasi → `accept_with_conditions`; selain itu `reject`. Tawaran balik terverifikasi = harga minimal per item (bila G2 gagal) pada tanggal yang lolos G1, G3, dan G4 setelah harga dinaikkan. Tanggal asli dicoba dulu; bila tidak lolos, dicari maju mulai hari setelah tanggal acara (atau tanggal lead time bila lebih lambat) selama 14 hari kalender. Tanggal sebelum tanggal acara tidak ditawarkan.
**Alasan:** BR-10 dan Bagian 8.3 tidak mendefinisikan arah pencarian; maju saja paling sederhana dan cocok dengan TC-02 (Sabtu penuh → Minggu).

### A1-19: Tawaran balik tetap dihitung saat G4 gagal
**Keputusan:** Bila G4 gagal, rekomendasi tetap `reject` (BR-10), tetapi `CounterOffer` tetap diisi jika ada tanggal memenuhi lead time, agar Manajer dapat memilih aksi "tawar balik".
**Alasan:** FR-DEC-03 meminta tawaran balik otomatis; keputusan akhir tetap manusia.

### A1-20: DP pada tawaran balik adalah DP ditagih
**Keputusan:** `CounterOffer.DP` berisi `DPCharged` pada syarat tawaran (tanggal dan harga baru), bukan `MinDP`.
**Alasan:** Contoh respons Bagian 9 memakai 712.500, yaitu nilai DP ditagih (TC-04).

### A1-21: Tanggal produksi mengikuti selisih terhadap tanggal acara
**Keputusan:** Pada tanggal tawaran, tanggal produksi digeser dengan selisih yang sama terhadap tanggal acara seperti pesanan asli.
**Alasan:** FR-CAP-05 menjadikan tanggal produksi default sama dengan tanggal acara; selisih dipertahankan bila Manajer pernah menggesernya.

### A1-22: Penyesuaian harga hanya bila G2 gagal
**Keputusan:** Harga item dinaikkan ke harga minimal hanya bila margin pesanan (total) gagal G2; `MinPricePerItem` selalu diisi untuk semua item. G2 dinilai pada margin tingkat pesanan, bukan per item.
**Alasan:** BR-06 dan BR-10 mendefinisikan margin per pesanan.

### A1-23: Teks alasan dibuat di engine
**Keputusan:** `Reason` berbahasa Indonesia dibentuk di engine (persen satu desimal dengan koma, Rp dengan titik ribuan, tanggal "14 Nov").
**Alasan:** FR-DEC-02 mewajibkan alasan terbaca manusia dan menyimpannya bersama keputusan; satu sumber mencegah duplikasi di frontend.

---

## Tahap 2 — Autentikasi, Peran, Pengaturan, dan Data Master

### A2-01: Sesi stateless dengan verifikasi ulang pengguna per permintaan
**Keputusan:** JWT HS256 (masa berlaku 12 jam) disimpan di cookie `session` (`HttpOnly`, `SameSite=Lax`, `Secure` mengikuti `COOKIE_SECURE`) dan juga diterima lewat header `Authorization: Bearer <token>` untuk kemudahan pengujian API. Pada setiap permintaan terproteksi, middleware `Authenticate()` memuat ulang baris pengguna dari database berdasarkan `sub` sehingga penonaktifan akun (`is_active=false`) dan perubahan peran langsung berlaku tanpa daftar cabut in-memory. Untuk mode `development` (`APP_ENV != "production"`), body respons login juga menyertakan field `token`.
**Alasan:** Backend harus stateless agar aman dijalankan di fungsi serverless Vercel namun tetap mematuhi pencabutan akses instan saat admin menonaktifkan pengguna.

### A2-02: Anti-CSRF berlapis pada autentikasi berbasis cookie
**Keputusan:** Selain `SameSite=Lax` pada cookie sesi, permintaan pengubah data (`POST`, `PUT`, `PATCH`, `DELETE`) yang mengandalkan cookie sesi memeriksa header `Origin` (bila dikirim browser) terhadap daftar `ALLOWED_ORIGINS`.
**Alasan:** Memenuhi NFR-05 (perlindungan CSRF) tanpa memerlukan state token CSRF di memori server.

### A2-03: Penyembunyian data kas dan harga beli menurut matriks peran
**Keputusan:**
- `GET /api/v1/settings` dapat diakses oleh `owner` dan `manager`, namun untuk `manager` tiga kunci kas (`opening_cash`, `fixed_cost_daily`, `cash_buffer`) dihapus dari respons; `PUT /api/v1/settings` hanya untuk `owner`.
- `GET /api/v1/ingredients` dan `GET /api/v1/ingredients/:id` dapat diakses oleh `staff` untuk keperluan operasional/belanja, tetapi field `buy_price` dihilangkan sepenuhnya (`omitempty`) dari JSON respons ketika pemanggil berperan `staff`.
- Rute `/api/v1/cash/projection` dan `/api/v1/cash/entries` sudah dipasangi penjaga `RequireRole` sehingga `staff` langsung ditolak dengan `403 Forbidden` berbahasa Indonesia (TC-09 bagian karyawan).
**Alasan:** Menegakkan Bagian 4 spesifikasi dan Bagian 6/9 brief bahwa karyawan tidak boleh menerima informasi harga jual, margin, harga beli bahan, maupun saldo kas.

### A2-04: Pengarsipan otomatis (`is_active=false`) saat menghapus data master yang sudah dipakai
**Keputusan:**
- `DELETE /api/v1/products/:id` memeriksa keterkaitan di `order_items`, `recipe_items`, dan `product_loads`. Bila sudah terpakai, produk tidak dihapus melainkan diarsipkan (`is_active=false`, `archived=true`). Bila belum pernah dipakai, baris dihapus.
- `DELETE /api/v1/ingredients/:id` memeriksa keterkaitan di `recipe_items`, `purchases`, dan `stock_movements`; bila sudah terpakai, bahan diarsipkan (`is_active=false`, `archived=true`).
- `DELETE /api/v1/resources/:id` menolak penghapusan dengan `409 Conflict` bila sumber daya sudah terhubung di `product_loads` (karena tabel `resources` tidak memiliki kolom `is_active` dan `product_loads` memakai `ON DELETE CASCADE`).
- `DELETE /api/v1/users/:id` selalu menonaktifkan pengguna (`is_active=false`) dan melarang admin menonaktifkan atau menurunkan peran akunnya sendiri.
**Alasan:** Memenuhi FR-MD-08 (data master yang sudah terpakai diarsipkan, bukan dihapus) serta menjaga integritas referensial dan jejak audit.

### A2-05: Fleksibilitas endpoint `recipes` dan `product-loads`
**Keputusan:** Endpoint `POST/PUT /api/v1/recipes` (serta `/api/v1/products/:id/recipes`) dan `POST/PUT /api/v1/product-loads` (serta `/api/v1/products/:id/loads`) menerima baik pembaruan satu item (`ingredient_id`/`resource_id` tunggal) maupun daftar `items` sekaligus untuk satu `product_id` (dengan opsi `replace_all: true`). Penghapusan per item disediakan di `DELETE /api/v1/recipes/:product_id/:ingredient_id` dan `DELETE /api/v1/product-loads/:product_id/:resource_id`.
**Alasan:** Memudahkan pemakaian baik dari form produk terpadu di frontend maupun pengujian per baris lewat REST client.

### A2-06: Cakupan `cmd/seed` pada Tahap 2
**Keputusan:** `backend/cmd/seed` pada Tahap 2 mengisi secara idempoten (`ON CONFLICT DO UPDATE`): 4 pengguna demo (`owner@demo.test`, `manager@demo.test`, `staff@demo.test`, `admin@demo.test`) dengan kata sandi wajib dari variabel lingkungan `SEED_DEMO_PASSWORD`, 2 sumber daya (`oven` dan `labor`), serta seluruh 17 kunci `settings` sesuai tabel Bagian 5 brief. Seed produk, bahan, resep, dan pesanan demo lengkap ditunda ke Tahap 7 sesuai instruksi.
**Alasan:** Menjaga agar tidak ada kata sandi yang ditulis di kode sumber (NFR-05) dan mengikuti pembagian tahap pada prompt Tahap 2.

---

## Tahap 3A — Alur Pesanan Inti (Publik, Availability, Kunci Hari, dan Evaluasi)

### A3-01: Penguncian baris `day_locks` terurut untuk tanggal produksi dan jendela kandidat tawaran balik
**Keputusan:** Di dalam transaksi `CreateOrder`, `LockDays` mengurutkan secara menaik (`YYYY-MM-DD`) tanggal produksi pesanan beserta 15 hari berikutnya (jendela pencarian tawaran balik), lalu menjalankan `INSERT INTO day_locks(day) VALUES ($1::date) ON CONFLICT DO NOTHING` diikuti `SELECT day FROM day_locks WHERE day = $1::date FOR UPDATE`. Tidak ada *advisory lock* yang dipakai.
**Alasan:** Mengikuti Bagian 8.1 butir 2 secara persis, mencegah *deadlock* saat dua transaksi mengunci rentang tanggal yang beririsan, dan memastikan evaluasi kapasitas maupun tawaran balik melihat data beban yang konsisten.

### A3-02: Penentuan status awal `accepted` vs `pending_decision`
**Keputusan:** Pesanan baru berstatus `accepted` (kapasitas langsung direservasi karena kueri beban menghitung status `accepted`, `confirmed`, dan `in_production`) hanya bila keempat gerbang G1..G4 lolos (`recommendation == "accept"`) **dan** `total < big_order_total` **dan** total unit `< big_order_units`. Pada status `accepted`, `dp_required` diisi `DPCharged` dan `dp_deadline` diisi `now() + dp_deadline_hours`. Selain itu, pesanan berstatus `pending_decision` dengan `dp_deadline = null` (kapasitas belum direservasi sampai diputuskan pada Tahap 3B).
**Alasan:** Mengikuti Bagian 8.1 butir 5 dan BR-11 (pesanan besar ≥ Rp 2.000.000 atau ≥ 50 unit wajib persetujuan manusia).

### A3-03: Kode pesanan publik 10 karakter base32 tanpa karakter ambigu
**Keputusan:** Kode pesanan (`orders.code`) dibangkitkan sepanjang 10 karakter acak menggunakan `crypto/rand` dari alfabet 32 karakter `ABCDEFGHJKLMNPQRSTUVWXYZ23456789` (tanpa `0`, `O`, `1`, `I`). `GET /api/v1/public/orders/:code` menormalisasi input ke huruf besar, menolak format tidak valid dengan `404 Not Found`, dan hanya mengembalikan field aman (tanpa `unit_cost`, `margin`, `snapshot`, atau proyeksi kas).
**Alasan:** Memenuhi Bagian 6 brief dan TC-09 bagian pelanggan (menebak kode pesanan orang lain tidak mungkin dilakukan secara sekuensial dan selalu menghasilkan 404).

### A3-04: Pembatasan laju berbasis tabel `rate_limits` dengan pembersihan malas (*lazy cleanup*)
**Keputusan:** `POST /api/v1/public/orders` dibatasi 30 permintaan/menit/IP dan `POST /api/v1/public/availability` dibatasi 60 permintaan/menit/IP (IP diambil dari elemen pertama `X-Forwarded-For` atau `ClientIP()`). Setiap pemeriksaan menjalankan `DELETE FROM rate_limits WHERE window_start < date_trunc('minute', now()) - interval '10 minutes'` diikuti `INSERT ... ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limits.count + 1 RETURNING count`.
**Alasan:** Menjaga backend tetap *stateless* di fungsi serverless Vercel sesuai Bagian 6 brief.

### A3-05: Isolasi test integrasi database dengan *in-memory mock* / skema sementara
**Keputusan:** Test integrasi `orders_test.go` (TC-08, TC-10, dan TC-09 bagian pelanggan) membuat fixture data uji sendiri secara terisolasi tanpa menyentuh tabel pada skema `public` produksi.
**Alasan:** Memenuhi instruksi Tahap 3A bahwa fixture data uji dibuat di dalam test dan tidak mengotori database produksi/demo.

---

## Tahap 3B — Keputusan, Tawaran Balik, Pembayaran, Transisi, Pembatalan, dan Kapasitas

### A3-06: Penolakan langsung (`403 Forbidden`) saat `manager` mencoba memutuskan pesanan besar
**Keputusan:** Pada `POST /api/v1/orders/:id/decision`, hanya `owner` yang berhak memutuskan pesanan besar (`total >= big_order_total` atau `total_units >= big_order_units`); permintaan keputusan oleh `manager` pada pesanan besar ditolak dengan `403 Forbidden` (dan `GET /api/v1/decision-queue` menyertakan flag `is_big_order` serta `can_decide` per pesanan).
**Alasan:** Mengikuti instruksi eksplisit Tahap 3B bahwa `owner` berhak untuk semua pesanan, `manager` hanya untuk pesanan di bawah ambang pesanan besar, dan keputusan oleh peran yang tidak berhak wajib ditolak.

### A3-07: Kewajiban `override_reason` berdasarkan rekomendasi awal maupun evaluasi ulang di dalam kunci hari
**Keputusan:** Saat `POST /api/v1/orders/:id/decision` dengan `action = "accept"` dijalankan, sistem mengunci `day_locks` terlebih dahulu lalu menjalankan ulang `EvaluateOrder`. Jika rekomendasi awal (`initial_recommendation`) maupun rekomendasi hasil evaluasi ulang (`reevaluated_recommendation`) bukan `"accept"`, permintaan wajib menyertakan `override_reason` (bila kosong ditolak dengan `422 Unprocessable Entity`) dan dicatat di `audit_logs` dengan `action = "override"` beserta identitas pemutus, waktu, rekomendasi awal, rekomendasi hasil evaluasi ulang, dan alasan override.
**Alasan:** Memenuhi TC-12 sekaligus mencegah *overbooking* saat dua goroutine memanggil `accept` secara serentak pada slot kapasitas terakhir.

### A3-08: Pengecekan malas (*lazy check*) kedaluwarsa DP sebelum operasi baca dan tulis kapasitas/pesanan
**Keputusan:** `ExpireOverdueOrders` dijalankan secara malas sebelum kueri ketersediaan, pembuatan pesanan, keputusan, pelacakan pesanan, pencatatan pembayaran, antrian keputusan, dan kalender kapasitas. Pesanan berstatus `accepted` dengan `dp_deadline <= now()` dan `paid < dp_required` diubah secara atomik menjadi `expired` dan dicatat di `audit_logs`.
**Alasan:** Sesuai Bagian 8.4 brief (tidak memerlukan *background scheduler* di lingkungan serverless Vercel) dan memastikan kapasitas langsung terlepas untuk pesanan baru (TC-11).

### A3-09: Pembatalan tugas yang belum selesai saat pesanan dibatalkan
**Keputusan:** Pada `POST /api/v1/orders/:id/cancel`, tugas (`tasks`) milik pesanan tersebut yang belum berstatus `done` dihapus (`DELETE FROM tasks WHERE order_id = $1 AND status != 'done'`) dari antrian tugas aktif.
**Alasan:** Skema `001_init.sql` membatasi `tasks.status` pada `CHECK (status IN ('todo','doing','done'))` tanpa nilai `'cancelled'`, sehingga penghapusan tugas yang belum selesai menjaga kepatuhan terhadap skema tanpa mengubah `001_init.sql`.

### A3-10: Pengurangan stok bahan idempoten saat transisi `in_production → ready`
**Keputusan:** Saat pesanan bertransisi dari `in_production` ke `ready` (`POST /api/v1/orders/:id/transition`), sistem menandai tugas `produce` menjadi `done`, memeriksa apakah `stock_movements` (`ref_type = 'order'`, `reason = 'production'`) untuk pesanan tersebut sudah pernah tercatat, lalu mengurangi `ingredients.stock` (dibatasi minimal 0) sesuai `recipe_items` dan menulis baris `stock_movements`.
**Alasan:** Memenuhi Bagian 8.5 brief bahwa penyelesaian produksi mengurangi stok bahan sesuai resep secara atomik dan aman dari pengurangan ganda.

---

## Tahap 3C — Seed Katalog Dasar dan Uji Alur Lengkap AU-1 pada Postgres

### A3-11: Idempotensi seed katalog dasar (`products`, `ingredients`, `recipe_items`, `product_loads`)
**Keputusan:** Skema `001_init.sql` tidak mendefinisikan constraint `UNIQUE(name)` maupun kolom `description` pada tabel `products` dan `ingredients`. Agar `cmd/seed` tetap 100% idempoten tanpa mengubah `001_init.sql`, seeder memakai UUID deterministik untuk setiap bahan (`10000000-0000-4000-8000-000000000001` s.d. `0015`) dan produk (`20000000-0000-4000-8000-000000000001` s.d. `0006`), disertai pengecekan `SELECT id FROM ... WHERE name = $1 LIMIT 1` (jika sudah ada maka di-`UPDATE`, jika belum maka di-`INSERT ... ON CONFLICT (id) DO UPDATE`). Untuk `recipe_items` dan `product_loads`, baris milik produk seed dihapus lalu diisi ulang di dalam transaksi yang sama.
**Alasan:** Menjamin `go run ./cmd/seed` aman dijalankan berulang kali tanpa menggandakan baris dan menghasilkan `engine.UnitCost` Snack Box tepat `Rp 19.000` (`11.025 + 2.000 + 4.800 + 1.175`).

### A3-12: Penutupan eksplisit `rows.Close()` sebelum eksekusi statement berikutnya pada transaksi/pool yang sama
**Keputusan:** Di `internal/orders/lifecycle.go` (`ExpireOverdueOrders`, `deductStockForProduction`, `GetDecisionQueue`, `GetCapacityDay`, `ListOrders`, `loadOrderItemsAndEval`), `rows.Close()` dipanggil secara eksplisit tepat setelah loop `for rows.Next()` dan `rows.Err()` selesai, sebelum menjalankan query atau `Exec` berikutnya di dalam transaksi atau pool yang sama.
**Alasan:** Mencegah penahanan koneksi/kursor pada pool dengan `DB_MAX_CONNS=3` dan `QueryExecModeSimpleProtocol`.

---

## Tahap 4A — Frontend Publik (`/`, `/pesan`, `/lacak/[kode]`), `/login`, dan Kerangka Internal

### A4-01: Penanganan serialisasi `shopspring/decimal` di klien API TypeScript
**Keputusan:** Di Go, `shopspring/decimal` men-serialisasi nilai desimal sebagai string JSON (mis. `"25000"` atau `"0.6000"`). Tipe `DecimalValue` di `frontend/src/lib/types.ts` didefinisikan sebagai `string | number` dan dikonversi hanya untuk tampilan melalui helper `toNumber()`, `formatRupiah()`, dan `formatPercent()` di `frontend/src/lib/format.ts`. Tidak ada rumus bisnis engine yang diduplikasi di frontend.
**Alasan:** Menjamin kompatibilitas penuh terhadap kontrak JSON backend tanpa kehilangan tipe maupun membuat perhitungan ganda di sisi klien.

### A4-02: Pemeriksaan rute per peran di Middleware Next.js dan verifikasi sesi di Layout Internal
**Keputusan:** `frontend/src/middleware.ts` membaca cookie `session` (`HttpOnly`) dan mendekode klaim payload JWT (`sub`, `role`, `exp`) untuk mengarahkan pengguna yang belum masuk ke `/login?next=...`, mengarahkan pengguna yang sudah masuk dari `/login` atau `/app` ke dashboard peran masing-masing (`/app/owner`, `/app/manajer`, `/app/karyawan`, `/app/admin`), serta menjaga akses rute `/app/...` berdasarkan peran. Selain itu, `frontend/src/app/app/layout.tsx` tetap memanggil `GET /api/v1/auth/me` sehingga pencabutan sesi atau penonaktifan akun di database langsung mengarahkan pengguna kembali ke `/login`.
**Alasan:** Mengikuti Bagian 10 brief bahwa akses halaman dijaga di middleware Next.js sekaligus di server API sebagai sumber otorisasi utama.

---

## Tahap 4A+ — Akun Pelanggan, Batas Pesanan Aktif, dan Perbaikan Tampilan Tanggal

### A4-03: Kolom `users.contact` pada migrasi `002_customers.sql`
**Keputusan:** Migrasi `backend/migrations/002_customers.sql` tidak hanya memperbarui constraint `users.role` agar menerima `'customer'` dan menambah `orders.customer_id uuid null references users(id)` beserta indeks `orders_customer_id_idx`, tetapi juga menambahkan kolom `users.contact text not null default ''`. Migrasi `001_init.sql` tidak disentuh sama sekali.
**Alasan:** Endpoint `POST /api/v1/auth/register` menerima input `contact`, dan `POST /api/v1/public/orders` serta halaman `/pesan` perlu membaca nomor kontak dari profil pelanggan yang sedang masuk.

### A4-04: Pemisahan `orders.customer_id` dan `orders.created_by` untuk pesanan publik
**Keputusan:** Saat pelanggan membuat pesanan melalui `POST /api/v1/public/orders`, kolom `orders.customer_id` diisi dengan UUID pelanggan yang terautentikasi (`role = 'customer'`), sedangkan `orders.created_by` dan `audit_logs.actor_id` tetap `NULL` (dengan `detail.customer_id` tercatat di JSON `audit_logs`). Untuk pesanan manual (`POST /api/v1/orders` oleh `manager`/`owner`), `orders.created_by` dan `audit_logs.actor_id` berisi UUID staf pembuat pesanan, sedangkan `orders.customer_id` bernilai `NULL`.
**Alasan:** Membedakan secara tegas antara pemilik pesanan pelanggan (`customer_id`) dan staf internal pencatat pesanan manual (`created_by`), serta mempertahankan kompatibilitas penuh dengan invarian audit log pesanan publik pada test integrasi `TC-08`.

### A4-05: Penegakan batas `max_active_unpaid_orders` di dalam transaksi pembuatan pesanan
**Keputusan:** Pengaturan `max_active_unpaid_orders` (nilai bawaan `2`, dapat diubah oleh `owner` melalui `PUT /api/v1/settings`) diperiksa di dalam transaksi `CreateOrder` ketika `customer_id` terisi. Sistem menghitung jumlah pesanan milik `customer_id` yang berstatus `IN ('pending_decision', 'countered', 'accepted')` dan `paid < dp_required` (setelah menjalankan `ExpireOverdueOrders` terlebih dahulu agar pesanan yang sudah melewati tenggat DP tidak lagi dihitung). Bila jumlahnya sudah mencapai `max_active_unpaid_orders`, pesanan baru ditolak dengan `422 Unprocessable Entity` (`code: "active_order_limit"`).
**Alasan:** Mencegah pemblokiran kapasitas dapur oleh pesanan fiktif yang belum membayar DP, sambil memastikan pelanggan yang pesanannya sudah kedaluwarsa atau sudah melunasi DP dapat kembali memesan.






