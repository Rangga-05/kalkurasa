# Panduan Pengujian Manual Antarmuka (UI Test Checklist) — Kalkurasa

Dokumen ini memuat panduan langkah demi langkah untuk pengujian manual antarmuka pengguna (UI/UX) Kalkurasa pada peramban web (*browser*), mencakup verifikasi lintas peran (*Role-Based Access Control*), siklus hidup pesanan, responsivitas tampilan, dan validasi 5 skenario demonstrasi (S1–S5).

> [!NOTE]
> Semua pengujian dilakukan pada lingkungan lokal atau demo dengan dataset simulasi yang telah diinisialisasi melalui perintah `go run ./cmd/seed` atau endpoint `POST /api/v1/admin/reset-demo`.

---

## 1. Lingkungan Pengujian dan Matriks Peramban

### 1.1 Persiapan Awal
1. Pastikan backend aktif: `http://localhost:8080/api/healthz` merespons `{"status":"ok"}`.
2. Pastikan frontend aktif: `http://localhost:3000`.
3. Kata sandi seluruh akun demo disetel seragam sesuai nilai variabel lingkungan `SEED_DEMO_PASSWORD` (dibagikan secara privat kepada penguji).

### 1.2 Matriks Peramban & Resolusi Layar
| Kategori | Target Peramban | Target Resolusi / Viewport | Catatan |
|---|---|---|---|
| **Desktop** | Google Chrome, Mozilla Firefox, Microsoft Edge | 1280 × 800 s.d. 1920 × 1080 | Tampilan bilah sisi (*sidebar*) penuh, tabel tanpa lipatan berlebih. |
| **Tablet** | Safari iPadOS / Chrome Device Emulation (iPad Air) | 768 × 1024 (Potret & Lanskap) | Navigasi responsif, tata letak grid 2 kolom. |
| **Ponsel** | Chrome Android / Safari iOS (iPhone 13/14, Pixel 7) | 375 × 667 s.d. 390 × 844 | Menu hamburger/drawer, kartu bertumpuk vertikal, scroll horizontal tabel. |

---

## 2. Matriks Pengujian Lintas Peran (RBAC & Otentikasi)

| ID | Pengujian | Akun Uji | Langkah | Hasil yang Diharapkan | Status |
|---|---|---|---|---|---|
| **AUTH-01** | Login Pemilik (*Owner*) | `owner@demo.test` | Buka `/login`, masukkan kredensial, klik "Masuk". | Berhasil masuk, dialihkan otomatis ke `/app/owner`. Cookie `session` terpasang (`HttpOnly`). | [ ] |
| **AUTH-02** | Login Manajer (*Manager*) | `manager@demo.test` | Buka `/login`, masukkan kredensial, klik "Masuk". | Berhasil masuk, dialihkan otomatis ke `/app/manajer`. | [ ] |
| **AUTH-03** | Login Karyawan (*Staff*) | `staff@demo.test` | Buka `/login`, masukkan kredensial, klik "Masuk". | Berhasil masuk, dialihkan otomatis ke `/app/karyawan`. | [ ] |
| **AUTH-04** | Login Admin | `admin@demo.test` | Buka `/login`, masukkan kredensial, klik "Masuk". | Berhasil masuk, dialihkan otomatis ke `/app/admin`. | [ ] |
| **AUTH-05** | Login Pelanggan | `customer@demo.test` | Buka `/login`, masukkan kredensial, klik "Masuk". | Berhasil masuk, dialihkan otomatis ke `/pesanan-saya`. | [ ] |
| **AUTH-06** | Blokir Akses Pelanggan ke Area Internal | `customer@demo.test` | Dalam kondisi login pelanggan, akses manual URL `/app/owner` atau `/app/pesanan`. | Dicegat oleh middleware: dialihkan kembali ke `/pesanan-saya` atau muncul halaman akses ditolak (403). | [ ] |
| **AUTH-07** | Perlindungan Karyawan (RBAC Data Finansial) | `staff@demo.test` | Buka `/app/karyawan` dan `/app/pesanan`. Periksa tabel dan detail. | Tidak ada angka harga satuan, total omset, margin kotor/bersih, maupun saldo kas yang tampil di layar. Menu Kas dan Keputusan disembunyikan. | [ ] |
| **AUTH-08** | Pendaftaran Pelanggan Baru | - | Buka `/daftar`, isi nama, email unik, telepon, alamat, dan kata sandi. | Akun terdaftar, otomatis login, dan diarahkan ke `/pesanan-saya`. | [ ] |
| **AUTH-09** | Logout Pengguna | Semua peran | Klik tombol "Keluar" di bilah navigasi. | Cookie sesi dihapus, dialihkan ke halaman login `/login`. Mencoba akses `/app/*` langsung dialihkan ke login. | [ ] |

---

## 3. Pengujian Alur Pelanggan (Pemesanan & Pelacakan)

| ID | Fitur | Langkah Pengujian | Hasil yang Diharapkan | Status |
|---|---|---|---|---|
| **CUST-01** | Formulir Pemesanan (`/pesan`) | Login sebagai `customer@demo.test`, buka `/pesan`. Pilih produk (misal: "Snack Box", 50 box), pilih tanggal acara (minimal H+2 dari hari ini). | Formulir menampilkan ringkasan perkiraan harga, input tanggal menghormati batas *lead time*, tombol kirim aktif. | [ ] |
| **CUST-02** | Validasi Lead Time | Di formulir `/pesan`, ubah tanggal acara menjadi H+1 atau hari ini. | Formulir menampilkan peringatan validasi: "Tanggal acara terlalu dekat (minimal H+2)" dan tombol kirim dinonaktifkan. | [ ] |
| **CUST-03** | Batas Slot Pesanan Belum DP | Login sebagai `pelanggan3@demo.test` (yang sudah memiliki 2 pesanan aktif berstatus menunggu DP). Buka `/pesan` dan kirim pesanan baru. | Sistem menolak pengajuan: muncul pesan kesalahan jelas bahwa batas 2 pesanan aktif belum DP telah tercapai. | [ ] |
| **CUST-04** | Pelacakan Publik (`/lacak/[kode]`) | Salin kode pesanan demo (contoh: `ORD-...`), buka peramban dalam mode *Incognito* (tanpa login), buka `/lacak/[kode]`. | Status pesanan, tanggal acara, dan riwayat linimasa tampil lengkap tanpa memerlukan otentikasi login. | [ ] |
| **CUST-05** | Tanggapi Tawaran Balik | Login sebagai `pelanggan2@demo.test` (memiliki pesanan dengan tawaran balik tanggal/harga). Buka `/pesanan-saya`. | Muncul kartu tawaran balik dengan opsi: **"Terima Tawaran Balik"** (memperbarui tanggal/harga dan lanjut ke DP) atau **"Batalkan Pesanan"**. | [ ] |

---

## 4. Pengujian Manajemen Pesanan & Pembayaran Internal

| ID | Fitur | Lokasi | Langkah Pengujian | Hasil yang Diharapkan | Status |
|---|---|---|---|---|---|
| **ORD-01** | Antrian Keputusan & DecisionCard | `/app/keputusan` (Owner/Manager) | Buka halaman antrian keputusan. Buka kartu pesanan yang memerlukan persetujuan. | Kartu `DecisionCard` menampilkan status 4 gerbang (G1 Kapasitas, G2 Margin, G3 Kas, G4 Lead time), rekomendasi mesin, serta tombol aksi (Terima, Tawar Balik, Tolak). | [ ] |
| **ORD-02** | Catat Override Alasan | `/app/keputusan` | Pilih pesanan dengan rekomendasi "Tolak" atau "Tawar Balik", lalu klik "Terima dengan Override". | Muncul modal konfirmasi yang **mewajibkan** pengisian teks "Alasan Override". Tanpa alasan, tombol simpan tidak dapat diklik. | [ ] |
| **ORD-03** | Tombol Cepat Pembayaran (Quick Fill) | `/app/pesanan/[id]` (Pesanan status `accepted`) | Buka detail pesanan yang belum lunas DP-nya. Lihat bagian "Catat Pembayaran Baru". | Terdapat 2 tombol cepat di atas input nominal: <br>1. **"Uang Muka/Nyicil"** (otomatis mengisi nominal DP minimal). <br>2. **"Bayar Sisa Tagihan (Lunas)"** (otomatis mengisi sisa tagihan penuh). | [ ] |
| **ORD-04** | Form Terkunci Saat Lunas | `/app/pesanan/[id]` (Pesanan lunas) | Catat pembayaran hingga total terbayar (`paid`) sama dengan total tagihan (`total`). | Form "Catat Pembayaran Baru" otomatis disembunyikan/dikunci. Tampil kotak informasi hijau: **"Pesanan ini sudah lunas"**. | [ ] |
| **ORD-05** | Progresi Status Pesanan | `/app/pesanan/[id]` | Ubah status pesanan secara bertahap: `confirmed` → `in_production` → `ready` → `delivering` → `completed`. | Setiap transisi berhasil, linimasa pesanan bertambah, dan tercatat di audit log. | [ ] |
| **ORD-06** | Pembatalan & Hitung Eksposur | `/app/pesanan/[id]` | Buka pesanan yang sudah `confirmed` lalu klik "Batalkan Pesanan". | Modal pembatalan menampilkan rincian eksposur pembatalan (biaya bahan terpesan dikurangi DP yang sudah masuk), status berubah menjadi `cancelled`. | [ ] |

---

## 5. Pengujian Kapasitas, Bahan, Belanja, dan Kas

| ID | Fitur | Lokasi | Langkah Pengujian | Hasil yang Diharapkan | Status |
|---|---|---|---|---|---|
| **OPS-01** | Kalender Kapasitas | `/app/kapasitas` | Buka kalender bulanan. Periksa warna indikator setiap tanggal. | Hari terisi pesanan menampilkan persentase utilisasi: Hijau (<70%), Kuning (70–89%), Merah (≥90%). Mengklik tanggal menampilkan rincian beban per sumber daya (oven & tenaga kerja). | [ ] |
| **OPS-02** | Daftar Belanja Otomatis | `/app/belanja` | Buka daftar belanja untuk rentang 7 hari ke depan. | Kebutuhan bahan dari pesanan `confirmed` diagregasi otomatis, memperhitungkan stok gudang, buffer 10%, dan pembulatan ke satuan beli terkecil (misal: kg, bungkus). | [ ] |
| **OPS-03** | Catat Pembelian Bahan | `/app/belanja` atau `/app/bahan` | Klik "Catat Pembelian" untuk salah satu bahan, masukkan jumlah satuan beli dan harga aktual. | Stok bahan bertambah di katalog bahan, transaksi tercatat di riwayat mutasi stok, dan kas keluar tercatat otomatis di arus kas. | [ ] |
| **OPS-04** | Koreksi Stok Manual | `/app/bahan` | Pilih bahan, klik "Sesuaikan Stok", masukkan angka perubahan (-2 kg karena rusak) dan isi alasan. | Alasan wajib diisi. Stok terpotong, tercatat di `stock_movements` dengan penanda manual. | [ ] |
| **OPS-05** | Proyeksi Kas 14 Hari | `/app/kas` | Buka grafik proyeksi kas. | Menampilkan grafik saldo harian 14 hari ke depan dengan garis batas aman (*buffer* kas). Hari dengan saldo di bawah buffer ditandai dengan sorotan rawan/merah. | [ ] |
| **OPS-06** | Entri Kas Manual | `/app/kas` | Tambah entri kas manual (misal: "Biaya Listrik & Air Dapur" - Rp 350.000). | Saldo kas terhitung ulang, grafik proyeksi terbarui seketika. | [ ] |

---

## 6. Pengujian Dashboard Per Peran & Audit Log

| ID | Dashboard | URL | Komponen Kunci yang Harus Tampil | Status |
|---|---|---|---|---|
| **DASH-01** | Pemilik (*Owner*) | `/app/owner` | 1. Kartu Antrian Keputusan (`DecisionCard`) <br>2. Ringkasan KPI (7 indikator utama) <br>3. Grafik Proyeksi Kas & Hari Rawan <br>4. Margin Realisasi Rata-rata <br>5. Pesanan Perlu Perhatian | [ ] |
| **DASH-02** | Manajer (*Manager*) | `/app/manajer` | 1. Kalender Kapasitas Bulanan Ringkas <br>2. KPI Ketepatan Waktu & Utilisasi <br>3. Jadwal Produksi & Antar Hari Ini <br>4. Peringatan Bahan di Bawah Minimum <br>5. Pesanan Menunggu DP | [ ] |
| **DASH-03** | Karyawan (*Staff*) | `/app/karyawan` | 1. Daftar Tugas Produksi Hari Ini (dengan toggle status & edit durasi aktual inline) <br>2. Daftar Pengantaran Hari Ini <br>3. Catatan Resep Ringkas <br>*(Bebas dari angka harga/omset/kas)* | [ ] |
| **DASH-04** | Admin Sistem | `/app/admin` | 1. Metrik Akun Pengguna Aktif per Peran <br>2. Statistik Master Data (Produk, Bahan, Resep) <br>3. Tabel 10 Baris Audit Log Terakhir <br>4. Panel Pemeliharaan: **Tombol Reset Data Demo** | [ ] |
| **DASH-05** | Halaman Audit Log Lengkap | `/app/audit` | Tabel log seluruh aktivitas (aktor, aksi, detail perubahan, timestamp). Filter berdasarkan "Action" dan "Role", serta tombol navigasi paginasi. Hanya bisa diakses Owner & Admin. | [ ] |

---

## 7. Verifikasi Skenario Demonstrasi Khusus (S1–S5)

Berikut adalah pengujian 5 skenario inti sesuai spesifikasi lomba GTNIC 2026:

| Skenario | Judul Skenario | Langkah Verifikasi UI | Indikator Keberhasilan | Status |
|---|---|---|---|---|
| **S1** | **Pesanan 100 Snack Box di Hari Penuh** | Cari pesanan S1 di `/app/keputusan` atau detail pesanan bersangkutan. | Gerbang G1 (Kapasitas) bertanda **GAGAL / MERAH** (utilisasi tenaga kerja melebihi 90%). Mesin menyiapkan **tawaran balik tanggal alternatif** (H+1) yang kapasitasnya masih mencukupi (<90%). | [ ] |
| **S2** | **Permintaan Harga Murah (Margin Rendah)** | Buka pesanan S2 dengan permintaan harga Rp 21.000 per box. | Gerbang G2 (Margin) bertanda **GAGAL / MERAH** (margin kotor hanya 9,5%, di bawah batas minimum 20%). Mesin menyarankan harga minimal **Rp 23.750**. | [ ] |
| **S3** | **DP Dinamis dari Titik Terendah Kas** | Periksa pesanan S3 pada halaman detail pesanan. | Tagihan DP minimal menampilkan **Rp 600.000** (berdasarkan titik terendah proyeksi kas), dan DP yang ditagihkan adalah **Rp 712.500** (30% dari total tagihan pesanan Rp 2.375.000). | [ ] |
| **S4** | **Daftar Belanja Bahan & Pembulatan** | Buka `/app/belanja` untuk tanggal produksi S4. | Kebutuhan tepung terhitung bersih 12,84 kg (memperhitungkan stok awal & buffer 10%), dan sistem merekomendasikan pembelian bulat sebanyak **13 kg**. | [ ] |
| **S5** | **Eksposur Pembatalan Pesanan** | Buka pesanan S5 yang dibatalkan setelah belanja bahan dilakukan. | Layar pembatalan menampilkan perhitungan eksposur pembatalan tepat senilai **Rp 587.500** (total belanja bahan terikat Rp 1.300.000 dikurangi DP diterima Rp 712.500). | [ ] |

---

## 8. Pengujian Reset Data Demo (Admin Feature)

| ID | Fitur | Langkah Pengujian | Hasil yang Diharapkan | Status |
|---|---|---|---|---|
| **RST-01** | Reset Data via UI Admin | Login sebagai `admin@demo.test`, buka `/app/admin`. Temukan tombol **"Reset Data Demo"**, klik dan konfirmasi dialog peringatan. | Tombol menampilkan status loading, muncul toast sukses "Data demo berhasil direset ke kondisi awal". Seluruh pesanan demo, mutasi kas, dan jadwal tugas kembali bersih ke kondisi awal seed 3 minggu. | [ ] |
| **RST-02** | Keamanan Endpoint Reset | Coba kirim `POST /api/v1/admin/reset-demo` menggunakan token login peran non-admin (misal: Karyawan atau Pelanggan). | Server merespons `403 Forbidden` dan data database tidak berubah. | [ ] |

---

## 9. Pengujian Responsivitas & Tampilan Antarmuka

| Komponen / Tampilan | Ponsel (375px - 414px) | Tablet (768px - 1024px) | Desktop (1280px+) |
|---|---|---|---|
| **Bilah Navigasi / Header** | Menu beralih ke ikon toggle/dropdown, teks ringkas. | Menyesuaikan lebar, menu utama tetap terbaca. | Bilah sisi (*sidebar*) permanen di kiri, breadcrumb aktif. |
| **Kartu Keputusan (`DecisionCard`)** | Rincian gerbang bertumpuk 1 kolom, tombol aksi mudah disentuh jempol. | Rincian 2 kolom, tombol aksi berdampingan. | Tampilan penuh dengan meteran visual kapasitas dan grafik mini. |
| **Tabel Pesanan & Audit Log** | Pembungkus (*wrapper*) scroll horizontal aktif, tidak memecah layout layar. | Kolom sekunder disederhanakan, paginasi rapi di bawah. | Menampilkan seluruh kolom lengkap beserta badge status. |
| **Kalender Kapasitas** | Kotak tanggal berukuran kompak, angka utilisasi tetap terbaca jelas. | Grid kalender 7 kolom proporsional. | Tampilan bulanan luas dengan tooltip detail per sumber daya. |
| **Grafik Proyeksi Kas** | Grafik SVG/Canvas menyesuaikan lebar 100% viewport, tooltip dapat disentuh. | Grafik proporsional dengan legenda di samping/bawah. | Grafik interaktif penuh dengan garis batas aman buffer kas. |
| **Modal / Dialog Konfirmasi** | Mengisi lebar layar dengan padding aman (*bottom sheet* atau modal terpusat). | Modal terpusat dengan lebar proporsional (maks. 500px). | Modal terpusat bersih dengan backdrop redup. |

---

## 10. Lembar Catatan Temuan Pengujian (Bug Log)

| No | Modul / Halaman | Deskripsi Temuan / Masalah | Tingkat Dampak (Low/Med/High) | Tindakan Perbaikan |
|---|---|---|---|---|
| 1 | | | | |
| 2 | | | | |
| 3 | | | | |

---
*Kalkurasa — Sistem Manajemen Keputusan Pesanan & Kapasitas UMKM Kuliner Pre-Order (GTNIC 2026).*

