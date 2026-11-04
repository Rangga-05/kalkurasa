# Kalkurasa

> Kalkulasi + Rasa — sistem manajemen pesanan untuk UMKM kuliner berbasis pre-order.

**Status:** Tahap 0 — Fondasi (monorepo, koneksi DB, migrasi, healthz).

Dokumentasi lengkap akan ditambahkan seiring pengerjaan.

## Struktur

```
frontend/   → Next.js (App Router, TypeScript, Tailwind CSS)
backend/    → Go (Gin, pgx)
docs/       → Spesifikasi dan dokumentasi
```

## Quick Start

Lihat `backend/.env.example` dan `frontend/.env.example` untuk variabel lingkungan yang diperlukan.

```bash
# Backend
cd backend
cp .env.example .env   # isi nilai yang sesuai
go run ./cmd/migrate   # jalankan migrasi (pakai DATABASE_URL_MIGRATE)
go run ./cmd/api       # server di :8080

# Frontend
cd frontend
npm install
cp .env.example .env.local  # isi GIN_API_URL
npm run dev            # dev server di :3000
```

---

*Data simulasi — proyek GTNIC 2026.*

