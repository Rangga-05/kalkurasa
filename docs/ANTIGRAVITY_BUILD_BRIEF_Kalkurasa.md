# BUILD BRIEF: Kalkurasa (untuk Antigravity)

Dokumen ini adalah instruksi build untuk AI coding agent. Bangun website **Kalkurasa** end-to-end dengan **Go + Gin** (backend), **Next.js** (frontend), dan **Supabase Postgres** (database).

**Sumber kebenaran:**
- Perilaku produk, aturan bisnis, peran, status, dan kasus uji: `docs/Spesifikasi_Kalkurasa_GTNIC2026.md` (taruh di repo). Bila ada konflik soal perilaku produk, spesifikasi itu menang.
- Teknologi, struktur, skema, dan kontrak API: dokumen ini yang menang.

Konteks lomba: GTNIC 2026 cabang Web Development, tema “Accelerating Digital Transformation for Business and SMEs”. Deadline pengumpulan **10 November 2026 pukul 23.59 WIB**. Waktu sangat terbatas, jadi ikuti urutan tahap di Bagian 14 dan jangan menambah fitur di luar cakupan.

---

## 0. Prompt pembuka (tempel ke Antigravity)

> Kamu adalah engineer utama proyek ini. Baca seluruh isi `docs/Spesifikasi_Kalkurasa_GTNIC2026.md` dan `docs/ANTIGRAVITY_BUILD_BRIEF_Kalkurasa.md` sampai selesai sebelum menulis kode apa pun.
>
> Tugas pertamamu hanya **Tahap 0** pada Bagian 14 brief: inisialisasi monorepo (`frontend/`, `backend/`, `docs/`), koneksi Supabase, migrasi `001_init.sql`, helper `Today()`, error JSON seragam, endpoint `/healthz`, dan halaman Next.js kosong yang memanggil `/healthz` lewat rewrites. Pastikan kerangka ini bisa dijalankan lokal dan siap di-deploy ke dua project Vercel (Root Directory `frontend` dan `backend`).
>
> Sebelum membuat file, tulis rencana singkat berisi struktur folder, daftar file yang akan dibuat, dan asumsi yang kamu ambil. Setelah itu kerjakan. Di akhir tahap, laporkan apa yang selesai, cara menjalankan dan mengujinya, dan asumsi baru yang dicatat di `docs/ASSUMPTIONS.md`.
>
> Aturan tetap: semua data tampil dari database (tanpa data palsu di UI); rumus bisnis hanya di `backend/internal/engine`; backend harus stateless (cocok untuk serverless Vercel); jangan membuat fitur di luar cakupan; jangan menaruh kredensial atau file `.env` di repo; jangan lanjut ke tahap berikutnya sebelum aku memintanya.

Prompt untuk tahap berikutnya cukup singkat, misalnya: “Lanjut ke Tahap 1 pada Bagian 14 brief. Ikuti aturan yang sama dan laporkan hasilnya di akhir.”

---

## 1. Aturan main untuk agent

1. **Semua data tampil dari database.** Tidak boleh ada data dummy hard-coded di frontend. Data contoh masuk lewat seed (Bagian 11).
2. **Rumus bisnis hanya di satu tempat:** paket `internal/engine` di backend, berupa fungsi murni (tanpa akses database) dengan unit test. Frontend hanya menampilkan hasil dari API. Jangan menduplikasi rumus di frontend.
3. **Jangan mengubah rumus, ambang, atau aturan** di spesifikasi tanpa mencatatnya di `docs/ASSUMPTIONS.md`.
4. **Tanpa AI/LLM, payment gateway, integrasi WhatsApp, payroll, multi-cabang** (Won't di spesifikasi).
5. **Keamanan:** tidak ada kredensial atau `.env` di repo; hanya `.env.example` tanpa nilai rahasia. Semua query pakai parameter (tidak ada string concatenation SQL).
6. **Bahasa antarmuka Indonesia**, mata uang Rupiah, tanggal `dd MMM yyyy`, zona Asia/Jakarta.
7. **Label simulasi:** tampilkan banner “Data simulasi” di area internal dan halaman publik (footer).
8. **Backend harus stateless** karena berjalan di Vercel: tidak ada state in-memory yang penting (cache, rate limiter, sesi), tidak ada goroutine atau job latar belakang, tidak menulis ke filesystem lokal, dan tidak bergantung pada urutan request. Semua state ada di database.
9. Jika ada hal ambigu, pilih opsi paling sederhana yang memenuhi spesifikasi, lalu catat di `docs/ASSUMPTIONS.md`. Jangan berhenti untuk bertanya kecuali benar-benar terblokir.

---

## 2. Stack

| Bagian | Pilihan |
|---|---|
| Backend | Go (versi stabil terbaru yang terpasang), Gin, `pgx/v5` + `pgxpool`, `shopspring/decimal` untuk uang, `golang-jwt/jwt/v5`, `bcrypt` (`golang.org/x/crypto`), `go-playground/validator` (bawaan Gin binding) |
| Database | Supabase Postgres, skema lewat file SQL di `backend/migrations` |
| Frontend | Next.js (App Router) + TypeScript, Tailwind CSS, shadcn/ui, TanStack Query, react-hook-form + zod, Recharts (grafik kas), date-fns |
| Test | `go test` (unit + integrasi), Playwright opsional (hanya bila sisa waktu) |
| Deploy | Semua di Vercel: **dua project dari satu repo** (frontend Next.js dengan Root Directory `frontend`; backend Go dengan Root Directory `backend`); database di Supabase |

**Keputusan arsitektur (jangan diubah):**
- **Frontend tidak memakai Supabase client SDK untuk data.** Semua data lewat API Gin. Supabase hanya dipakai sebagai Postgres.
- **Auth buatan sendiri di Gin** (tabel `users`, bcrypt, JWT), bukan Supabase Auth, supaya sederhana.
- **Backend berjalan di Vercel (serverless), jadi koneksi runtime memakai Transaction pooler Supabase (port 6543).** Transaction pooler tidak mendukung prepared statement, sehingga `pgx` **wajib** diset ke `QueryExecModeSimpleProtocol`, dengan `pgxpool` kecil (`MaxConns` 3 sampai 5, dari env `DB_MAX_CONNS`). Jangan memakai advisory lock atau fitur level sesi apa pun. `SELECT ... FOR UPDATE` di dalam transaksi aman dipakai.
- **Migrasi dan seed dijalankan dari laptop**, memakai `DATABASE_URL_MIGRATE` berisi Session pooler (port 5432) atau direct connection, bukan Transaction pooler.
- **Region:** pilih region Supabase Asia Tenggara (Singapura) dan atur region function Vercel berdekatan (misalnya Singapura, bila tersedia di paketmu) lewat `vercel.json`.
- **Aktifkan RLS pada semua tabel tanpa policy** supaya tabel tidak bisa diakses lewat Data API publik Supabase. Pastikan role koneksi backend tidak terblokir RLS (role `postgres` melewati RLS); jika terblokir, catat dan sesuaikan.
- **Concurrency pakai row lock**, bukan advisory lock (lihat Bagian 8).
- Frontend memanggil `/api/*` pada domain yang sama; Next.js `rewrites` meneruskan ke Gin (`GIN_API_URL`). Cookie sesi `httpOnly`, `Secure`, `SameSite=Lax`.

---

## 3. Struktur repo

```
/
├─ docs/
│  ├─ Spesifikasi_Kalkurasa_GTNIC2026.md
│  ├─ ANTIGRAVITY_BUILD_BRIEF_Kalkurasa.md
│  └─ ASSUMPTIONS.md
├─ backend/
│  ├─ cmd/api/main.go            # server
│  ├─ cmd/migrate/main.go        # menjalankan migrations/*.sql berurutan
│  ├─ cmd/seed/main.go           # seed data demo
│  ├─ internal/
│  │  ├─ config/                 # env
│  │  ├─ db/                     # pgxpool, helper transaksi
│  │  ├─ httpx/                  # middleware: auth, rbac, ratelimit, error, logger
│  │  ├─ engine/                 # RUMUS BISNIS (fungsi murni) + *_test.go
│  │  ├─ auth/ users/ settings/ catalog/ orders/ capacity/ costing/
│  │  ├─ stock/ cash/ tasks/ kpi/ admin/
│  ├─ migrations/001_init.sql
│  ├─ go.mod                     # harus di root folder backend (Root Directory project Vercel)
│  ├─ vercel.json
│  └─ .env.example
├─ frontend/
│  ├─ src/app/(public)/...       # /, /pesan, /lacak/[kode]
│  ├─ src/app/(internal)/app/... # dashboard dan modul per peran
│  ├─ src/components/ src/lib/ (api client, format, zod schema)
│  └─ .env.example
├─ .gitignore                    # .env*, node_modules, .next, .vercel, binary Go
└─ README.md
```

Folder `docs/` berada **di dalam root repo** yang akan di-push ke GitHub (sejajar dengan `frontend/` dan `backend/`).

---

## 4. Environment variables

`backend/.env.example` (tanpa nilai rahasia):
```
PORT=8080                # Vercel mengisi PORT sendiri; server harus listen di sini
DATABASE_URL=            # Supabase Transaction pooler URI (port 6543), dipakai saat runtime
DATABASE_URL_MIGRATE=    # Session pooler (5432) atau direct, hanya dipakai lokal untuk migrate dan seed
DB_MAX_CONNS=3
JWT_SECRET=              # string acak panjang
APP_ENV=development
ALLOWED_ORIGINS=http://localhost:3000
COOKIE_SECURE=false      # true di produksi
TZ=Asia/Jakarta
```
Di Vercel, isi variabel ini lewat Project Settings, bukan lewat file. `DATABASE_URL_MIGRATE` tidak perlu diisi di Vercel.
`frontend/.env.example`:
```
GIN_API_URL=http://localhost:8080
NEXT_PUBLIC_APP_NAME=Kalkurasa
```

---

## 5. Database (`backend/migrations/001_init.sql`)

Uang memakai `numeric(16,4)`; di Go pakai `decimal.Decimal`. Aktifkan `pgcrypto` untuk `gen_random_uuid()`.

```sql
create extension if not exists pgcrypto;

create table users (
  id uuid primary key default gen_random_uuid(),
  email text unique not null,
  name text not null,
  role text not null check (role in ('owner','manager','staff','admin')),
  password_hash text not null,
  is_active boolean not null default true,
  created_at timestamptz not null default now()
);

create table resources (
  id uuid primary key default gen_random_uuid(),
  code text unique not null,              -- 'oven' | 'labor'
  name text not null,
  unit text not null,                     -- 'oven_hour' | 'minute'
  count int not null check (count > 0),
  amount_per_unit_day numeric(12,4) not null check (amount_per_unit_day > 0),
  efficiency numeric(5,4) not null check (efficiency > 0 and efficiency <= 1)
);

create table products (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  category text not null default 'umum',
  price numeric(16,4) not null check (price >= 0),
  packaging_cost numeric(16,4) not null default 0,
  overhead_cost numeric(16,4) not null default 0,
  is_active boolean not null default true
);

create table product_loads (
  product_id uuid references products(id) on delete cascade,
  resource_id uuid references resources(id) on delete cascade,
  load numeric(12,4) not null check (load >= 0),
  primary key (product_id, resource_id)
);

create table ingredients (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  use_unit text not null,                 -- kg, g, butir, liter
  buy_unit text not null,                 -- kg, sak, tray
  buy_unit_size numeric(12,4) not null check (buy_unit_size > 0), -- isi satuan beli dalam use_unit
  buy_price numeric(16,4) not null check (buy_price >= 0),        -- per satuan beli
  stock numeric(16,4) not null default 0,                          -- dalam use_unit
  min_stock numeric(16,4) not null default 0,
  rounding text not null default 'loose' check (rounding in ('loose','pack')),
  is_active boolean not null default true
);

create table recipe_items (
  product_id uuid references products(id) on delete cascade,
  ingredient_id uuid references ingredients(id),
  qty_per_unit numeric(16,6) not null check (qty_per_unit > 0),
  primary key (product_id, ingredient_id)
);

create table settings (
  key text primary key,
  value jsonb not null
);

create table orders (
  id uuid primary key default gen_random_uuid(),
  code text unique not null,
  source text not null default 'public' check (source in ('public','manual')),
  customer_name text not null,
  customer_contact text not null,
  delivery_type text not null check (delivery_type in ('delivery','pickup')),
  address text,
  notes text,
  event_date date not null,
  production_date date not null,
  status text not null check (status in ('submitted','pending_decision','countered','accepted','confirmed',
    'in_production','ready','delivering','completed','rejected','cancelled','expired')),
  total numeric(16,4) not null,
  dp_required numeric(16,4) not null default 0,
  dp_deadline timestamptz,
  paid numeric(16,4) not null default 0,
  recommendation text,                    -- accept | accept_with_conditions | reject
  recommendation_reason text,
  counter_offer jsonb,
  decided_by uuid references users(id),
  decided_at timestamptz,
  override_reason text,
  snapshot jsonb,                         -- harga, biaya, parameter, hasil gerbang saat keputusan
  created_by uuid references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index on orders (production_date, status);
create index on orders (status);

create table order_items (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  product_id uuid not null references products(id),
  qty int not null check (qty > 0),
  unit_price numeric(16,4) not null,
  unit_cost numeric(16,4) not null
);

create table order_evaluations (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  gate text not null check (gate in ('G1','G2','G3','G4')),
  value numeric(18,6),
  threshold numeric(18,6),
  passed boolean not null,
  detail jsonb,
  created_at timestamptz not null default now()
);

create table payments (
  id uuid primary key default gen_random_uuid(),
  order_id uuid not null references orders(id) on delete cascade,
  amount numeric(16,4) not null check (amount > 0),
  method text not null default 'transfer',
  paid_at timestamptz not null default now(),
  note text,
  recorded_by uuid references users(id)
);

create table tasks (
  id uuid primary key default gen_random_uuid(),
  order_id uuid references orders(id) on delete cascade,
  type text not null check (type in ('produce','shop','deliver')),
  title text not null,
  due_date date not null,
  assignee_id uuid references users(id),
  status text not null default 'todo' check (status in ('todo','doing','done')),
  actual_minutes int,
  created_at timestamptz not null default now()
);

create table purchases (
  id uuid primary key default gen_random_uuid(),
  ingredient_id uuid not null references ingredients(id),
  qty_buy_units numeric(16,4) not null check (qty_buy_units > 0),
  unit_price numeric(16,4) not null,
  bought_at timestamptz not null default now(),
  bought_by uuid references users(id)
);

create table stock_movements (
  id uuid primary key default gen_random_uuid(),
  ingredient_id uuid not null references ingredients(id),
  delta numeric(16,4) not null,
  reason text not null,
  ref_type text, ref_id uuid,
  created_at timestamptz not null default now()
);

create table cash_entries (
  id uuid primary key default gen_random_uuid(),
  entry_date date not null,
  amount numeric(16,4) not null,          -- positif masuk, negatif keluar
  note text,
  created_by uuid references users(id)
);

create table audit_logs (
  id uuid primary key default gen_random_uuid(),
  actor_id uuid references users(id),
  action text not null,                   -- decision, override, payment, status, setting, master
  entity text not null, entity_id uuid,
  detail jsonb,
  created_at timestamptz not null default now()
);

-- kunci baris per hari untuk anti-overbooking
create table day_locks (day date primary key);

-- pembatasan laju berbasis database (backend stateless di Vercel)
create table rate_limits (
  key text not null,                      -- mis. 'order:<ip>'
  window_start timestamptz not null,      -- awal menit (date_trunc('minute', now()))
  count int not null default 1,
  primary key (key, window_start)
);

-- RLS aktif tanpa policy pada semua tabel
do $$
declare t text;
begin
  for t in select tablename from pg_tables where schemaname = 'public' loop
    execute format('alter table public.%I enable row level security', t);
  end loop;
end $$;
```

**Settings (key → value JSON), default sesuai spesifikasi 8.2:**

| key | value awal |
|---|---|
| `capacity_limit` | 0.90 |
| `status_green_below` | 0.70 |
| `status_yellow_below` | 0.90 |
| `margin_min` | 0.20 |
| `labor_rate_per_minute` | 400 |
| `waste_pct` | 0.05 |
| `shopping_buffer_pct` | 0.10 |
| `cash_buffer` | 400000 (satu hari biaya tetap) |
| `fixed_cost_daily` | 400000 |
| `opening_cash` | 1800000 |
| `lead_time_days` | 2 |
| `dp_pct` | 0.30 |
| `dp_deadline_hours` | 24 |
| `big_order_total` | 2000000 |
| `big_order_units` | 50 |
| `shopping_lead_days` | 2 |
| `demo_today` | null (jika diisi `YYYY-MM-DD`, dipakai sebagai “hari ini”) |

Semua kode memakai helper `Today()` yang menghormati `demo_today`, bukan `time.Now()` langsung.

---

## 6. Auth, peran, dan keamanan

- `POST /api/v1/auth/login`: verifikasi bcrypt, keluarkan JWT (HS256, masa berlaku 12 jam) dengan klaim `sub`, `role`; set cookie `session` (`httpOnly`, `Secure` di produksi, `SameSite=Lax`). Terima juga header `Authorization: Bearer` untuk pengujian.
- Middleware `RequireRole(...)` menegakkan hak akses di server sesuai matriks Bagian 4 spesifikasi. Karyawan tidak boleh menerima field harga jual, margin, atau kas (jangan kirim field itu di respons untuk peran `staff`).
- Endpoint publik: batasi laju (misalnya 30 permintaan per menit per IP untuk `POST /public/orders`). **Jangan in-memory** (instance serverless tidak berbagi memori). Pakai tabel `rate_limits`: `INSERT ... ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limits.count + 1 RETURNING count`, tolak bila melebihi batas. Ambil IP dari header `X-Forwarded-For` (elemen pertama). Bersihkan baris lama secara lazy saat menulis.
- Kode pesanan: 10 karakter acak dari `crypto/rand` (base32 tanpa karakter ambigu). `GET /public/orders/:code` hanya mengembalikan field yang aman (status, item, total, tagihan DP, tenggat, tawaran balik).
- CORS: hanya origin di `ALLOWED_ORIGINS`.
- Validasi input di server pada semua endpoint (binding tag). Error JSON seragam: `{ "error": { "code": "...", "message": "pesan berbahasa Indonesia" } }`.
- Audit log wajib untuk: keputusan, override, pembayaran, perubahan status, perubahan parameter, dan perubahan master data.

---

## 7. Engine bisnis (`backend/internal/engine`)

Semua fungsi di sini murni (input → output) dan **wajib** punya unit test dari vektor uji di bawah. Gunakan `decimal.Decimal` untuk uang.

**Fungsi yang harus ada:**
```go
Capacity(r Resource) decimal.Decimal                         // count * amount_per_unit_day * efficiency
DayLoads(items []LoadItem) map[string]decimal.Decimal         // per resource code: sum(qty * load)
Utilization(load, capacity decimal.Decimal) decimal.Decimal
DayUtilization(loads, caps map[string]decimal.Decimal) (util decimal.Decimal, bottleneck string)
DayStatus(util, greenBelow, yellowBelow decimal.Decimal) string   // green|yellow|red|full (red sampai 1.00, full di atas 1.00)

UnitCost(c CostInput) decimal.Decimal    // bahan*(1+waste) + kemasan + menit*tarif + overhead
Margin(price, cost decimal.Decimal) decimal.Decimal
MinPrice(cost, marginMin decimal.Decimal) decimal.Decimal         // cost / (1 - marginMin)
MaxDiscount(price, minPrice decimal.Decimal) decimal.Decimal      // 1 - minPrice/price

ShoppingNeed(req, stock, bufferPct, packSize decimal.Decimal, rounding string) (need, buy decimal.Decimal)
  // need = max(0, req*(1+buffer) - stock); 'loose' dibulatkan ke atas ke bilangan bulat satuan pakai; 'pack' ke kelipatan packSize

ProjectCash(opening decimal.Decimal, start time.Time, days int, events []CashEvent, fixedDaily decimal.Decimal) CashProjection
  // per hari: saldo = saldo_kemarin + sum(events hari itu) - fixedDaily; kembalikan saldo harian, titik terendah, tanggal terendah
MinDP(buffer, lowestWithoutDP decimal.Decimal) decimal.Decimal    // max(0, buffer - lowestWithoutDP)
DPCharged(dpPct, total, minDP decimal.Decimal) decimal.Decimal    // max(dpPct*total, minDP)
CancellationExposure(shoppingCost, dpReceived decimal.Decimal) decimal.Decimal // max(0, ...)
Deviation(avgActual, estimate decimal.Decimal) decimal.Decimal    // (avg - est)/est
ShouldRecalibrate(dev decimal.Decimal, samples int) bool           // |dev| > 0.10 && samples >= 5

EvaluateOrder(in EvalInput) EvalResult
  // menjalankan G1..G4 dan menghasilkan rekomendasi + counter offer (Bagian 8)
```

**Aturan penjadwalan kas (untuk `CashEvent`):**
- DP masuk pada tanggal tenggat DP (atau saat dicatat).
- Pelunasan masuk pada **hari setelah tanggal acara** (konservatif; titik terendah pada hari acara dihitung sebelum pelunasan).
- Belanja bahan keluar pada `tanggal acara − shopping_lead_days`, senilai `qty × purchase_cost_per_unit` (bahan dengan waste + kemasan).
- Biaya tetap harian berlaku setiap hari, termasuk hari mulai.

**Vektor uji wajib (Go test, nilai persis):**

| Tes | Input | Hasil yang diharapkan |
|---|---|---|
| TC-01 | Resource: tenaga 4 × 420 × 0,85; oven 2 × 8 × 0,85. Beban existing Sabtu: tenaga 1.020, oven 5,0. Tambah 100 snack box (12 menit, 0,05 oven-jam) | Kapasitas tenaga 1.428; oven 13,6. Beban 2.220 menit (util 1,5546 → 155,5%), oven 10,0 (0,7353 → 73,5%); status `full`; penghambat `labor` |
| TC-02 | Minggu beban tenaga existing 60, tambah 100 box | 1.260 menit; util 0,8824; status `yellow` |
| TC-03 | harga 21.000, biaya 19.000, margin_min 0,20 | margin 0,0952; harga minimal 23.750; diskon maksimum terhadap 25.000 = 0,05; G2 gagal |
| TC-04 | Saldo awal H-2 1.800.000; belanja 1.300.000 di H-2; DP pesanan lain +500.000 di H-1; biaya tetap 400.000/hari; buffer 400.000; total pesanan 2.375.000; dp_pct 0,30; pelunasan masuk H+1 | Titik terendah tanpa DP = −200.000 (di H0); MinDP = 600.000; DPCharged = 712.500; dengan DP 712.500 diterima di H-2: titik terendah 512.500; G3 lolos |
| TC-05 | Kebutuhan tepung: 6 tart × 0,4 + 100 box × 0,12 = 14,4 kg; buffer 0,10; stok 3 | need 12,84; `loose` → buy 13; `pack` 25 → buy 25 |
| TC-06 | Belanja 1.300.000, DP diterima 712.500 | eksposur 587.500 |
| TC-07 | Durasi aktual 105,100,95,110,100; estimasi 90 | rata-rata 102; deviasi 0,1333; rekalibrasi true; dengan 4 sampel false |

---

## 8. Alur kritis (transaksi dan concurrency)

### 8.1 Buat pesanan (`POST /api/v1/public/orders` dan `POST /api/v1/orders`)
Dalam **satu transaksi database**:
1. Validasi input; ambil harga dan biaya produk terkini; hitung `total`.
2. Kunci hari: `INSERT INTO day_locks(day) VALUES ($1) ON CONFLICT DO NOTHING`, lalu `SELECT day FROM day_locks WHERE day = $1 FOR UPDATE` (urutkan tanggal bila lebih dari satu, untuk menghindari deadlock). Ini yang mencegah overbooking.
3. Muat beban existing hari itu dari pesanan berstatus `accepted`, `confirmed`, `in_production` pada `production_date` tersebut; muat kapasitas sumber daya; muat peristiwa kas dari pesanan aktif.
4. Jalankan `EvaluateOrder` → simpan 4 baris `order_evaluations`, `snapshot`, `recommendation`, `recommendation_reason`, dan `counter_offer` (bila ada).
5. Tentukan status:
   - Semua gerbang lolos **dan** `total < big_order_total` **dan** unit `< big_order_units` → status `accepted` (kapasitas langsung direservasi), `dp_required` dan `dp_deadline` terisi.
   - Selain itu → `pending_decision`. Kapasitas **belum** direservasi.
6. Simpan item pesanan, audit log, commit.

### 8.2 Keputusan (`POST /api/v1/orders/:id/decision`)
Body: `{ "action": "accept" | "counter" | "reject", "override_reason": "...", "counter": {...} }`.
- Dalam transaksi: kunci hari, **evaluasi ulang** gerbang dengan data terbaru.
- Jika `accept` padahal rekomendasi bukan Terima → wajib `override_reason` (hanya `owner`, atau `manager` untuk pesanan di bawah ambang pesanan besar) dan catat di audit log (siapa, kapan, rekomendasi awal).
- `accept` → status `accepted`, reservasi kapasitas, hitung DP. `counter` → status `countered` dengan `counter_offer`. `reject` → status `rejected`.
- Pesanan besar hanya boleh diputuskan `owner`; `manager` hanya boleh mengusulkan (catat di audit log dan biarkan tetap `pending_decision`).

### 8.3 Tawaran balik
`EvaluateOrder` menghasilkan `counter_offer` berisi: tanggal terdekat (dalam 14 hari ke depan, memenuhi lead time) dengan utilisasi setelah menerima ≤ `capacity_limit`; harga minimal per item; DP minimal. Pelanggan menerima lewat `POST /public/orders/:code/counter-response` → status `accepted` (evaluasi ulang dalam transaksi dengan kunci hari baru) atau `rejected`.

### 8.4 Pembayaran dan konfirmasi
- `POST /orders/:id/payments` menambah baris `payments` dan `orders.paid`.
- Saat `paid >= dp_required` dan status `accepted` → status `confirmed`, buat tugas: `produce` (due `production_date`), `shop` (due `event_date − shopping_lead_days`), `deliver` (due `event_date`).
- Saat `paid >= total` pada status `delivering` atau `ready` → boleh `completed`.
- Cron/pengecekan malas (lazy check saat endpoint dipanggil, tidak perlu scheduler): pesanan `accepted` yang melewati `dp_deadline` dan belum DP → `expired`, kapasitas dilepas.

### 8.5 Transisi status (validasi di server)
`confirmed → in_production → ready → delivering (atau pickup siap) → completed`. Karyawan boleh `in_production → ready` dan `ready → delivering`; Manajer/Pemilik boleh semua transisi yang valid. Tugas `produce` selesai → stok bahan berkurang sesuai resep (tulis `stock_movements`).

### 8.6 Pembatalan (`POST /orders/:id/cancel`)
Dalam transaksi: status `cancelled`, kapasitas dilepas (otomatis karena status tidak lagi terhitung), tugas dibatalkan, bahan yang sudah dibeli tetap menjadi stok, hitung `CancellationExposure`, tulis audit log. Keputusan DP hangus atau dikembalikan dicatat lewat catatan, bukan otomatis.

---

## 9. Kontrak API (prefix `/api/v1`)

Semua respons JSON. Pagination sederhana `?page=&limit=` untuk daftar.

**Publik**
| Method | Path | Fungsi |
|---|---|---|
| GET | `/public/products` | Katalog produk aktif |
| POST | `/public/availability` | Body `{items:[{product_id,qty}], from, to}` → daftar hari `{date, status, utilization_after, available}` (G1 dan G4 saja) |
| POST | `/public/orders` | Buat pesanan (Bagian 8.1); respons berisi `code`, `status`, `recommendation` ringkas |
| GET | `/public/orders/:code` | Lacak pesanan (field aman) |
| POST | `/public/orders/:code/counter-response` | `{accept: bool}` |

**Auth**
`POST /auth/login`, `POST /auth/logout`, `GET /auth/me`.

**Internal (butuh login)**
| Area | Endpoint | Peran |
|---|---|---|
| Pesanan | `GET /orders`, `GET /orders/:id`, `POST /orders` (manual), `POST /orders/:id/decision`, `POST /orders/:id/payments`, `POST /orders/:id/cancel`, `POST /orders/:id/transition` | manager, owner (staff lihat terbatas) |
| Antrian keputusan | `GET /decision-queue` (memuat pratinjau dampak: util, margin, titik terendah kas sebelum dan sesudah) | manager, owner |
| Kapasitas | `GET /capacity/calendar?month=YYYY-MM`, `GET /capacity/day/:date` | manager, owner |
| Biaya | `GET /costing/products` (biaya, margin, harga minimal, diskon maksimum) | manager, owner |
| Belanja dan stok | `GET /shopping-list?from=&to=`, `POST /purchases`, `POST /stock/adjust`, `GET /ingredients` | manager, staff (belanja) |
| Kas | `GET /cash/projection?days=14`, `POST /cash/entries` | owner; manager hanya status lolos atau tidak |
| Tugas | `GET /tasks?mine=true`, `PATCH /tasks/:id` (`status`, `actual_minutes`) | staff, manager |
| KPI | `GET /kpi` | manager, owner |
| Master | CRUD `/products`, `/ingredients`, `/recipes`, `/resources`, `/product-loads` | manager, admin |
| Parameter | `GET/PUT /settings` | owner |
| Pengguna | CRUD `/users` | admin |
| Audit | `GET /audit-logs` | admin, owner |
| Demo (Tahap lanjutan) | `PUT /admin/demo-date`, `POST /admin/reset-demo`, `POST /admin/simulate` | admin |
| Dashboard | `GET /dashboard/owner`, `/dashboard/manager`, `/dashboard/staff` | per peran |
| Health | `GET /healthz` | publik |

Respons `EvaluateOrder` (dipakai di antrian keputusan dan halaman lacak) berbentuk:
```json
{
  "recommendation": "accept_with_conditions",
  "reason": "Beban tenaga 155,5% pada 14 Nov; tersedia 15 Nov (88,2%).",
  "gates": [
    {"gate":"G1","passed":false,"value":1.5546,"threshold":0.90},
    {"gate":"G2","passed":true,"value":0.24,"threshold":0.20},
    {"gate":"G3","passed":true,"value":512500,"threshold":400000},
    {"gate":"G4","passed":true,"value":6,"threshold":2}
  ],
  "impact": {"utilization_before":0.714,"utilization_after":1.5546,"margin":0.24,
             "cash_lowest_before":812500,"cash_lowest_after":512500},
  "counter_offer": {"date":"2026-11-15","min_price_per_item":{"<product_id>":23750},"dp":712500}
}
```

---

## 10. Frontend

### Halaman publik
- `/` landing singkat + katalog produk.
- `/pesan`: form pesanan (produk, jumlah, tanggal, nama, kontak, antar atau ambil, alamat, catatan). Pemilih tanggal memanggil `/public/availability` setiap kali isi pesanan berubah (debounce) dan menandai hari: tersedia, hampir penuh, penuh, di bawah lead time (hari tidak tersedia tidak bisa dipilih). Tampilkan estimasi total. Validasi zod dengan pesan Indonesia.
- `/lacak/[kode]`: status, timeline status, tagihan DP dan tenggat, instruksi bayar manual, dan tombol terima atau tolak bila ada tawaran balik.

### Area internal (`/login`, lalu `/app/...`)
Sidebar berbeda per peran. Redirect setelah login ke dashboard peran.

| Peran | Dashboard (maksimal 5 elemen) |
|---|---|
| Pemilik `/app/owner` | Antrian keputusan dengan pratinjau dampak; grafik proyeksi kas 14 hari dengan hari rawan; margin dan KPI ringkas; kalender kapasitas ringkas |
| Manajer `/app/manajer` | Kalender kapasitas; antrian keputusan di bawah ambang; jadwal produksi dan daftar belanja; peringatan stok dan tenggat DP |
| Karyawan `/app/karyawan` | Tugas hari ini; ubah status tugas; input durasi aktual; daftar antar |
| Admin `/app/admin` | Pengguna dan peran; master data; audit log; kontrol data demo |

Modul: `/app/pesanan` (daftar dan detail), `/app/keputusan`, `/app/kapasitas`, `/app/belanja`, `/app/kas`, `/app/produk`, `/app/bahan`, `/app/pengaturan`, `/app/pengguna`, `/app/audit`.

### Komponen kunci
- `CapacityCalendar` (bulanan, warna sesuai status, klik hari untuk rincian per sumber daya dan penghambat).
- `DecisionCard` (rekomendasi, alasan, 4 lencana gerbang, pratinjau sebelum dan sesudah, tombol Terima, Tawar balik, Tolak; override meminta alasan).
- `CashProjectionChart` (Recharts, garis buffer, titik terendah ditandai).
- `OrderTimeline`, `GateBadges`, `EmptyState`, `ErrorState`, `TableSkeleton`.

### Aturan UI
- Responsif untuk 360, 768, 1280 px tanpa scroll horizontal pada konten utama.
- Setiap daftar dan aksi tulis punya status loading, kosong, error, dan sukses (toast).
- Format Rupiah dengan `Intl.NumberFormat('id-ID')`; persen satu desimal.
- Akses halaman dijaga di middleware Next.js dan di server (API tetap sumber otorisasi).
- Gaya bersih dan profesional; jangan menghabiskan waktu pada animasi.

---

## 11. Seed dan skenario demo (`cmd/seed`)

Seed idempoten (aman dijalankan ulang). Isi:
- **Pengguna** (kata sandi demo ditulis di README, bukan rahasia): `owner@demo.test`, `manager@demo.test`, `staff@demo.test`, `admin@demo.test`.
- **Sumber daya:** oven (2 × 8 jam, efisiensi 0,85), tenaga (4 × 420 menit, efisiensi 0,85).
- **Produk (±6):** tart ulang tahun (harga 280.000; oven 0,5 jam; tenaga 90 menit), snack box (harga 25.000; oven 0,05; tenaga 12; bahan sekitar 10.500 sebelum waste, kemasan 2.000, overhead 1.200), cupcake box, nasi box, kue kering, dan satu produk lain.
- **Bahan (±15)** dengan satuan beli, isi, harga, stok, dan aturan pembulatan; resep tiap produk (tepung per tart 0,4 kg dan per snack box 0,12 kg).
- **±40 pesanan** dengan beragam status yang tersebar dalam 3 minggu relatif terhadap `Today()`, agar kalender kapasitas dan proyeksi kas tampak hidup.
- **Pengaturan** sesuai tabel Bagian 5.

**Skenario demo yang harus bisa dijalankan dari seed:**
- **S1:** pesanan 100 snack box pada hari yang sudah terisi → G1 gagal, tawaran balik ke hari berikutnya.
- **S2:** permintaan harga Rp 21.000 per box → G2 gagal, harga minimal Rp 23.750.
- **S3:** DP minimal 600.000 dari model kas, DP default 712.500 ditagih.
- **S4:** daftar belanja minggu itu (tepung 12,84 kg → beli 13).
- **S5:** pembatalan → kapasitas kembali, bahan menjadi stok, eksposur 587.500.
- **S6:** saran kalibrasi tart dari durasi aktual 105, 100, 95, 110, 100 (hanya jika fitur kalibrasi dikerjakan).

---

## 12. Pengujian dan definisi selesai

**Wajib (go test):**
- Unit test engine: TC-01 sampai TC-07 persis seperti Bagian 7.
- Integrasi TC-08: dua goroutine memesan slot terakhir yang sama secara bersamaan; hanya satu berstatus `accepted`, yang lain `pending_decision` atau mendapat tawaran balik. Jalankan terhadap database uji.
- TC-09: karyawan mengakses `/cash/projection` → 403; pelanggan menebak kode pesanan lain → 404.
- TC-10, TC-11: lead time dan kedaluwarsa DP.
- TC-12: override tanpa alasan ditolak (422); dengan alasan tercatat di `audit_logs`.

**Definisi selesai tingkat proyek:**
1. Alur AU-1 di spesifikasi berjalan penuh dari form publik sampai status `completed`, memakai data dari database.
2. Empat peran internal bisa login dan hanya melihat yang berhak.
3. Semua test di atas hijau.
4. Semua aksi tulis memiliki validasi, feedback sukses dan error.
5. README lengkap (Bagian 13) dan seed bisa dijalankan dari nol.
6. Tidak ada kredensial di repo (cek `git log` dan file `.env`).

---

## 13. README dan deploy

README wajib memuat: deskripsi masalah dan solusi, fitur, tech stack beserta alasan singkat, diagram arsitektur sederhana, diagram relasi tabel (Mermaid ER), cara menjalankan lokal, cara menjalankan migrasi dan seed, daftar env (tanpa nilai rahasia), link demo, akun demo, tangkapan layar, dan batasan yang diketahui.

**Deploy (dua project Vercel dari satu repo GitHub):**
1. Buat proyek Supabase (region Asia Tenggara). Ambil **Transaction pooler** URI → `DATABASE_URL`, dan **Session pooler** URI → `DATABASE_URL_MIGRATE` (lokal saja).
2. Dari laptop: `cd backend && go run ./cmd/migrate && go run ./cmd/seed` (memakai `DATABASE_URL_MIGRATE`).
3. **Project backend:** import repo di Vercel, set **Root Directory = `backend`**. Vercel mendeteksi `go.mod` di root folder itu dan entrypoint `cmd/api/main.go`; server harus listen di env `PORT` dan Gin dijalankan dalam mode release. Jika preset Go tidak terdeteksi otomatis, set `framework` ke `go` di `backend/vercel.json`. Set env: `DATABASE_URL`, `DB_MAX_CONNS`, `JWT_SECRET`, `ALLOWED_ORIGINS`, `COOKIE_SECURE=true`, `APP_ENV=production`.
4. **Project frontend:** import repo yang sama, **Root Directory = `frontend`**. Set `GIN_API_URL` ke URL project backend. `next.config` meneruskan `/api/:path*` ke `GIN_API_URL`.
5. Verifikasi `GET /healthz` hijau lewat domain frontend (bukti rewrites bekerja), login demo berfungsi (cookie tersimpan), dan halaman pesan dan lacak terbuka publik tanpa login.
6. Cek dokumentasi Vercel terbaru bila ada hal yang berbeda dari brief ini (runtime Go, batas durasi function, dan paket yang kamu pakai). Catat penyimpangan di `docs/ASSUMPTIONS.md`.

**Risiko Vercel yang perlu diwaspadai:** cold start membuat request pertama lebih lambat; koneksi database harus kecil dan dibuka per instance; tidak ada job terjadwal (kedaluwarsa DP sudah memakai pengecekan malas di Bagian 8.4).

**Ketentuan guidebook GTNIC:** website live dan repository wajib publik hingga kompetisi berakhir; browser yang diuji mencakup Chrome, Firefox, Safari, dan Edge; proposal wajib memuat diagram database dan daftar teknologi beserta alasannya. Pastikan proyek Supabase dan Vercel tetap aktif selama kompetisi, termasuk Grand Final 14 November (live pitching via Zoom).

---

## 14. Urutan pengerjaan (kerjakan berurutan)

**Tahap 0, fondasi.** Inisialisasi repo, struktur, `.env.example`, koneksi Supabase (Transaction pooler dengan `QueryExecModeSimpleProtocol`), migrasi `001_init.sql`, helper `Today()`, error JSON seragam, `/healthz`, dan halaman Next.js kosong yang memanggil `/healthz` lewat rewrites. **Deploy kerangka ini ke dua project Vercel sekarang juga** (Bagian 13) supaya masalah hosting ketahuan sejak awal, bukan di hari terakhir. Selesai bila server menyala lokal, migrasi berjalan, dan `/healthz` bisa dipanggil lewat domain frontend di Vercel.

**Tahap 1, engine dan test.** Implementasikan seluruh fungsi `internal/engine` dan test TC-01 sampai TC-07. Selesai bila semua hijau. **Jangan lanjut sebelum ini hijau.**

**Tahap 2, auth dan master data.** Login, RBAC, CRUD pengguna, produk, bahan, resep, sumber daya, beban produksi, parameter. Seed dasar. Selesai bila data master bisa dikelola lewat API.

**Tahap 3, alur pesanan inti.** Endpoint publik (produk, availability, buat pesanan, lacak), kunci hari dan reservasi kapasitas, `EvaluateOrder`, keputusan, tawaran balik, pembayaran manual, status `confirmed`, pembuatan tugas. Test TC-08, TC-10 sampai TC-12. Selesai bila AU-1 berjalan lewat API.

**Tahap 4, frontend alur inti.** `/pesan`, `/lacak/[kode]`, `/login`, antrian keputusan dengan `DecisionCard`, kalender kapasitas, detail pesanan, tugas karyawan. Selesai bila AU-1 berjalan penuh lewat UI.

**Tahap 5, bahan, belanja, kas.** Daftar belanja, pencatatan pembelian, stok berkurang saat produksi selesai, proyeksi kas dengan grafik, pembatalan penuh (TC-06). Selesai bila TC-04 sampai TC-06 terlihat benar di UI.

**Tahap 6, dashboard per peran dan KPI.** Empat dashboard sesuai Bagian 10, KPI dasar. Selesai bila tiap peran mendarat di dashboard yang benar.

**Tahap 7, finalisasi.** Seed lengkap dan skenario S1 sampai S5, README, deploy, uji lintas browser dan ukuran layar, cek keamanan (Bagian 6). Selesai sesuai Bagian 12.

**Tahap 8, hanya bila waktu tersisa (Should):** audit log UI, kalibrasi (S6), simulasi 30 hari dan kontrol demo, notifikasi dalam aplikasi, daftar antar terurut. Kerjakan dalam urutan itu; berhenti kapan pun waktu habis.

---

## 15. Larangan dan catatan akhir

- Jangan menambah modul di luar spesifikasi (HR, akuntansi lengkap, multi-cabang, marketplace).
- Jangan memakai library UI tambahan di luar daftar tanpa alasan kuat.
- Jangan menulis data simulasi sebagai data nyata; semua klaim dampak harus berlabel simulasi.
- Jangan commit file `.env`, token, atau kunci apa pun.
- Setiap asumsi yang kamu buat dicatat di `docs/ASSUMPTIONS.md` dengan satu kalimat alasan.
- Prioritaskan alur AU-1 yang mulus dan benar daripada banyak fitur setengah jadi.
