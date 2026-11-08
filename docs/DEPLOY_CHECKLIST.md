# Panduan Penerapan Sistem (Deployment Checklist) — Kalkurasa

Dokumen ini memuat panduan lengkap penerapan (*deployment*) sistem Kalkurasa ke lingkungan produksi berbasis cloud, mencakup penyiapan basis data terkelola **Supabase**, penerapan antarmuka pengguna di **Vercel**, serta pilihan penerapan backend API di **Railway / VPS** maupun **Vercel Serverless**.

---

## 1. Ringkasan Topologi Arsitektur Produksi

```mermaid
flowchart TD
    Browser["Peramban Klien (Pengguna / Staf)"]
    
    subgraph Vercel_Platform["Platform Vercel"]
        FE["Frontend Next.js (App Router)<br/>Domain: https://kalkurasa.vercel.app"]
    end
    
    subgraph Backend_Hosting["Backend API (Pilihan A: Railway / VPS | Pilihan B: Vercel)"]
        API["Go Gin REST API Engine<br/>URL: https://api-kalkurasa.up.railway.app"]
    end
    
    subgraph Cloud_Database["Basis Data Terkelola (Supabase)"]
        S_Tx["Supabase Transaction Pooler (Port 6543)<br/>Pool Mode: Transaction"]
        S_DB[("PostgreSQL Database (Asia Tenggara / Singapore sin1)")]
    end
    
    Browser -->|"Akses Web & Panggilan API (/api/*)"| FE
    FE -->|"Proxy Rewrites /api/* (Same-Origin Cookie)"| API
    API -->|"Koneksi pgx (Maks. 3–5 koneksi)"| S_Tx
    S_Tx --> S_DB
```

> [!IMPORTANT]
> **Arsitektur Reverse Proxy / Rewrites:**
> Peramban web hanya berinteraksi langsung dengan domain frontend. Next.js secara transparan meneruskan rute `/api/:path*` ke alamat backend API (`GIN_API_URL`). Hal ini memastikan cookie sesi JWT `HttpOnly` diperlakukan sebagai **first-party / same-origin**, mencegah pemblokiran cookie pihak ketiga oleh kebijakan keamanan peramban modern.

---

## 2. Bagian I: Konfigurasi Basis Data (Supabase)

### 2.1 Pembuatan Proyek Supabase
1. Masuk ke [dashboard.supabase.com](https://dashboard.supabase.com).
2. Buat proyek baru (*New Project*):
   - **Name:** `kalkurasa-db`
   - **Database Password:** Buat kata sandi kuat dan simpan di pengelola sandi (*password manager*).
   - **Region:** Pilih **Southeast Asia (Singapore) - `ap-southeast-1`** untuk latensi terendah dengan server Vercel kawasan `sin1`.
   - **Pricing Plan:** Free tier sudah mencukupi untuk demonstrasi kompetisi.

### 2.2 Pengambilan String Koneksi
Buka menu **Project Settings** → **Database** → bagian **Connection String**:
1. **Transaction Pooler (URI)** — Digunakan oleh runtime Backend API:
   - Pilih mode **Transaction** (biasanya port `6543`).
   - Format: `postgresql://postgres.[project-ref]:[YOUR-PASSWORD]@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres`
   - *Simpan string ini sebagai `DATABASE_URL`.*
2. **Session Pooler / Direct Connection (URI)** — Digunakan untuk eksekusi migrasi tabel:
   - Pilih mode **Session** (biasanya port `5432`).
   - Format: `postgresql://postgres.[project-ref]:[YOUR-PASSWORD]@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres`
   - *Simpan string ini sebagai `DATABASE_URL_MIGRATE`.*

### 2.3 Menjalankan Migrasi & Data Seed Awal (Dari Mesin Lokal)
Eksekusi migrasi skema tabel dan inisialisasi dataset demo dari terminal lokal pengembang:

```bash
cd backend

# 1. Jalankan migrasi seluruh tabel (18 tabel + RLS + indeks)
DATABASE_URL_MIGRATE="postgresql://postgres.[ref]:[password]@aws-0-ap-southeast-1.pooler.supabase.com:5432/postgres" go run ./cmd/migrate

# 2. Jalankan seed data katalog, konfigurasi awal, dan 40 pesanan demo skenario S1-S5
DATABASE_URL="postgresql://postgres.[ref]:[password]@aws-0-ap-southeast-1.pooler.supabase.com:6543/postgres" \
SEED_DEMO_PASSWORD="[KATA_SANDI_DEMO_PILIHAN]" \
go run ./cmd/seed
```

> [!TIP]
> Perintah `seed` bersifat idempoten. Jika ingin membersihkan pesanan uji coba dan mengembalikan dataset ke kondisi awal, perintah `seed` dapat dijalankan ulang sewaktu-waktu tanpa menyebabkan duplikasi baris data master.

---

## 3. Bagian II: Penerapan Backend API

Tersedia dua opsi penerapan backend: **Opsi A (Vercel Serverless)** sebagai opsi utama, dan **Opsi B (Railway/VPS)** sebagai opsi cadangan jika ada masalah koneksi.

### Opsi A: Penerapan di Vercel (Utama)
1. Masuk ke [vercel.com](https://vercel.com) → *Add New Project*.
2. Hubungkan repository Kalkurasa.
3. Konfigurasi Proyek:
   - **Project Name:** `kalkurasa-api`
   - **Framework Preset:** *Other*
   - **Root Directory:** `backend`
4. Di bagian **Environment Variables**, masukkan variabel berikut:
   | Nama Variabel | Nilai Contoh | Keterangan |
   |---|---|---|
   | `DATABASE_URL` | *`postgresql://...:6543/postgres`* | String Supabase Transaction Pooler |
   | `DB_MAX_CONNS` | `3` | Batas koneksi pgx per instance |
   | `JWT_SECRET` | *`string-acak-minimal-32-karakter`* | Token penandatangan JWT cookie |
   | `APP_ENV` | `production` | Mengaktifkan mode produksi Gin |
   | `ALLOWED_ORIGINS` | `https://kalkurasa.vercel.app` | URL domain frontend Vercel |
   | `COOKIE_SECURE` | `true` | Wajib `true` agar cookie bertanda Secure (HTTPS) |
   | `TZ` | `Asia/Jakarta` | Zona waktu operasional UMKM (WIB) |
5. Berkas `backend/vercel.json` akan otomatis mengatur region `sin1` (Singapura) dan flag kompilasi Go.
6. Klik **Deploy**.
7. Uji kesehatan API: buka peramban ke `https://kalkurasa-api.vercel.app/api/healthz`. Respons yang benar: `{"status":"ok", "db":"ok"}`.

### Opsi B: Penerapan di Railway / VPS (Cadangan)
Jika memilih Railway:
1. Masuk ke [railway.app](https://railway.app) dan buat *New Project* → *Deploy from GitHub repo*.
2. Pilih repository Kalkurasa.
3. Di tab **Settings**:
   - **Root Directory:** Ubah menjadi `/backend`
   - **Build Command:** `go build -o api ./cmd/api`
   - **Start Command:** `./api`
4. Masukkan **Environment Variables** yang sama seperti Opsi A, lalu klik **Deploy**.

## 4. Bagian III: Penerapan Frontend (Vercel)

1. Masuk ke [vercel.com](https://vercel.com) → *Add New Project*.
2. Impor repositori Kalkurasa yang sama.
3. Konfigurasi Proyek:
   - **Project Name:** `kalkurasa`
   - **Framework Preset:** Next.js
   - **Root Directory:** Pilih dan simpan `frontend`
4. Di bagian **Environment Variables**, tambahkan:
   | Nama Variabel | Nilai Contoh | Keterangan |
   |---|---|---|
   | `GIN_API_URL` | `https://kalkurasa-api.up.railway.app` | Alamat URL backend produksi (tanpa trailing slash) |
   | `NEXT_PUBLIC_APP_NAME` | `Kalkurasa` | Nama aplikasi di antarmuka |
5. Klik **Deploy**.
6. Vercel akan menjalankan `npm run build` dan mempublikasikan frontend (misal: `https://kalkurasa.vercel.app`).

---

## 5. Matriks Pemeriksaan Pasca-Penerapan (Post-Deployment Checklist)

Setelah kedua layanan aktif, lakukan verifikasi operasional secara berurutan:

- [ ] **1. Pemeriksaan Proxy API Frontend:**
  Buka `https://<domain-frontend>/api/healthz` di peramban. Halaman harus mengembalikan status JSON `{"status":"ok"}`. Ini membuktikan fitur *Next.js Rewrites* berhasil meneruskan permintaan ke backend.

- [ ] **2. Pemeriksaan Halaman Publik:**
  Akses halaman utama `https://<domain-frontend>/`, halaman katalog produk, formulir `/pesan`, dan halaman pelacakan `/lacak/ORD-SAMPLE`. Seluruh styling Tailwind CSS 3 harus termuat sempurna tanpa error visual atau kegagalan aset.

- [ ] **3. Pengujian Otentikasi & Cookie:**
  - Buka `/login`, masukkan akun `owner@demo.test` beserta kata sandi yang disetel saat seed.
  - Periksa *Developer Tools* (`F12` → tab *Application* → *Cookies*): cookie `session` harus berstatus `HttpOnly`, `SameSite=Lax`, dan bertanda centang `Secure`.
  - Pastikan pengguna berhasil dialihkan ke dashboard `/app/owner`.

- [ ] **4. Uji Pengambilan Data & Dashboard:**
  - Periksa indikator KPI, grafik proyeksi kas, dan kalender kapasitas di antarmuka internal.
  - Buka `/app/keputusan` dan pastikan kartu pesanan demo terload dengan metrik gerbang G1–G4.

- [ ] **5. Pengujian Reset Data Demo:**
  - Masuk sebagai `admin@demo.test`.
  - Buka `/app/admin`, klik tombol **Reset Data Demo**.
  - Pastikan muncul notifikasi sukses dan data kembali sinkron.

- [ ] **6. Penanganan Galat CORS:**
  Buka tab *Console* pada *Developer Tools*. Pastikan tidak ada peringatan galat *Cross-Origin Request Blocked* (CORS) saat melakukan navigasi dan manipulasi data.

---

## 6. Prosedur Pemeliharaan & Troubleshooting

### Masalah A: Sambungan Basis Data Terputus (*Too many connections*)
- **Penyebab:** Pool koneksi PostgreSQL Supabase habis karena batas maksimum instance terlampaui.
- **Solusi:** Pastikan `DATABASE_URL` backend mengarah ke port **6543** (*Transaction Pooler / PgBouncer*), bukan port 5432 (*Direct Session*). Turunkan nilai `DB_MAX_CONNS` menjadi `3` atau `2` pada variabel lingkungan backend.

### Masalah B: Sesi Login Hilang Saat Pindah Halaman (*Session Drops*)
- **Penyebab:** Konfigurasi `COOKIE_SECURE` bernilai `false` di domain HTTPS atau domain API diakses langsung tanpa melalui Next.js proxy.
- **Solusi:** Pastikan `COOKIE_SECURE=true` pada variabel lingkungan backend dan peramban hanya memanggil rute internal `/api/*` dari domain frontend.

### Masalah C: Perubahan Skema Baru di Masa Mendatang
- **Prosedur:** Tulis berkas migrasi baru berformat SQL di `backend/migrations/` dengan penomoran urut (contoh: `019_nama_tabel.sql`). Jalankan `go run ./cmd/migrate` menggunakan `DATABASE_URL_MIGRATE`.

---
*Kalkurasa — Panduan Resmi Penerapan Sistem (GTNIC 2026).*
