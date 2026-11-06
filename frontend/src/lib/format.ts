import type { DecimalValue, OrderStatus, UserRole } from "./types";

const rupiahFormatter = new Intl.NumberFormat("id-ID", {
  style: "currency",
  currency: "IDR",
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
});

const numberFormatter = new Intl.NumberFormat("id-ID", {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
});

const percentFormatter = new Intl.NumberFormat("id-ID", {
  minimumFractionDigits: 1,
  maximumFractionDigits: 1,
});

const dateLongFormatter = new Intl.DateTimeFormat("id-ID", {
  weekday: "long",
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "Asia/Jakarta",
});

const dateShortFormatter = new Intl.DateTimeFormat("id-ID", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "Asia/Jakarta",
});

const dateTimeFormatter = new Intl.DateTimeFormat("id-ID", {
  day: "numeric",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "Asia/Jakarta",
});

/**
 * Mengonversi nilai shopspring/decimal (string atau number) menjadi number JS
 * hanya untuk keperluan tampilan/format di UI (bukan rumus bisnis).
 */
export function toNumber(value: DecimalValue | null | undefined): number {
  if (value === null || value === undefined || value === "") return 0;
  if (typeof value === "number") return Number.isFinite(value) ? value : 0;
  const parsed = Number.parseFloat(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

/**
 * Format mata uang Rupiah menggunakan Intl.NumberFormat('id-ID') sesuai Bagian 10 brief.
 */
export function formatRupiah(value: DecimalValue | null | undefined): string {
  return rupiahFormatter.format(Math.round(toNumber(value)));
}

export function formatNumber(value: DecimalValue | null | undefined): string {
  return numberFormatter.format(toNumber(value));
}

/**
 * Format rasio (mis. 0.714 -> "71,4%") dengan satu angka desimal.
 */
export function formatPercent(ratio: DecimalValue | null | undefined): string {
  return `${percentFormatter.format(toNumber(ratio) * 100)}%`;
}

/**
 * Mengubah string YYYY-MM-DD menjadi objek Date pada pukul 12:00 WIB agar tidak bergeser hari.
 */
export function parseDateKey(dateStr: string): Date | null {
  if (!dateStr) return null;
  const clean = dateStr.slice(0, 10);
  const parts = clean.split("-");
  if (parts.length !== 3) return null;
  const dt = new Date(`${clean}T12:00:00+07:00`);
  return Number.isNaN(dt.getTime()) ? null : dt;
}

export function formatDateLong(dateStr: string | null | undefined): string {
  if (!dateStr) return "-";
  const dt = parseDateKey(dateStr) ?? new Date(dateStr);
  if (Number.isNaN(dt.getTime())) return dateStr;
  return dateLongFormatter.format(dt);
}

export function formatDateShort(dateStr: string | null | undefined): string {
  if (!dateStr) return "-";
  const dt = parseDateKey(dateStr) ?? new Date(dateStr);
  if (Number.isNaN(dt.getTime())) return dateStr;
  return dateShortFormatter.format(dt);
}

export function formatDateTimeWIB(isoStr: string | null | undefined): string {
  if (!isoStr) return "-";
  const dt = new Date(isoStr);
  if (Number.isNaN(dt.getTime())) return isoStr;
  return `${dateTimeFormatter.format(dt)} WIB`;
}

/**
 * Menghasilkan string YYYY-MM-DD di zona waktu Asia/Jakarta.
 */
export function toDateKeyWIB(date: Date): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: "Asia/Jakarta",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);
  const y = parts.find((p) => p.type === "year")?.value ?? "2026";
  const m = parts.find((p) => p.type === "month")?.value ?? "01";
  const d = parts.find((p) => p.type === "day")?.value ?? "01";
  return `${y}-${m}-${d}`;
}

export function addDaysDateKey(dateKey: string, days: number): string {
  const dt = parseDateKey(dateKey) ?? new Date();
  dt.setUTCDate(dt.getUTCDate() + days);
  return toDateKeyWIB(dt);
}

export const CATEGORY_LABELS: Record<string, string> = {
  snack_box: "Paket Snack Box",
  kue_tart: "Kue Tart & Perayaan",
  cupcake: "Cupcake & Hampers",
  nasi_box: "Katering Nasi Box",
  kue_kering: "Kue Kering & Toples",
  brownies: "Brownies & Cake Potong",
};

export function formatCategoryName(category: string): string {
  if (!category) return "Lainnya";
  return (
    CATEGORY_LABELS[category] ??
    category
      .replace(/_/g, " ")
      .replace(/\b\w/g, (l) => l.toUpperCase())
  );
}

export interface OrderStatusMeta {
  label: string;
  description: string;
  tone: "emerald" | "amber" | "sky" | "indigo" | "rose" | "slate";
}

export const ORDER_STATUS_META: Record<OrderStatus, OrderStatusMeta> = {
  submitted: {
    label: "Terkirim",
    description: "Pesanan baru diterima oleh sistem.",
    tone: "slate",
  },
  pending_decision: {
    label: "Menunggu Konfirmasi Usaha",
    description:
      "Pesanan sedang ditinjau oleh Manajer/Pemilik karena memerlukan penyesuaian jadwal atau persetujuan kapasitas.",
    tone: "amber",
  },
  countered: {
    label: "Ada Tawaran Balik",
    description:
      "Kami mengajukan penyesuaian tanggal/ketentuan pesanan. Silakan tinjau dan berikan tanggapan Anda.",
    tone: "amber",
  },
  accepted: {
    label: "Disetujui — Menunggu DP",
    description:
      "Slot kapasitas telah direservasi sementara. Segera selesaikan pembayaran DP sebelum tenggat berakhir.",
    tone: "sky",
  },
  confirmed: {
    label: "Terkonfirmasi (DP Lunas)",
    description:
      "Pembayaran DP telah diverifikasi. Pesanan Anda masuk ke jadwal belanja bahan dan produksi.",
    tone: "emerald",
  },
  in_production: {
    label: "Dalam Produksi",
    description: "Tim dapur sedang memproduksi pesanan Anda sesuai jadwal.",
    tone: "indigo",
  },
  ready: {
    label: "Siap Diambil / Diantar",
    description: "Produksi telah selesai dan pesanan sudah dikemas rapi.",
    tone: "emerald",
  },
  delivering: {
    label: "Sedang Diantar / Siap Diambil",
    description: "Pesanan sedang dalam perjalanan atau siap diambil di lokasi.",
    tone: "sky",
  },
  completed: {
    label: "Selesai",
    description: "Pesanan telah diterima dan seluruh pembayaran sudah lunas.",
    tone: "emerald",
  },
  rejected: {
    label: "Ditolak",
    description: "Maaf, pesanan tidak dapat dipenuhi pada jadwal yang diminta.",
    tone: "rose",
  },
  cancelled: {
    label: "Dibatalkan",
    description: "Pesanan ini telah dibatalkan.",
    tone: "rose",
  },
  expired: {
    label: "Kedaluwarsa (Melewati Tenggat DP)",
    description:
      "Batas waktu pembayaran DP telah terlewati sehingga reservasi slot otomatis dilepas.",
    tone: "slate",
  },
};

export function getOrderStatusMeta(status: string): OrderStatusMeta {
  return (
    ORDER_STATUS_META[status as OrderStatus] ?? {
      label: status,
      description: "Status pesanan.",
      tone: "slate",
    }
  );
}

export const ROLE_LABELS: Record<UserRole, string> = {
  owner: "Pemilik Usaha (Owner)",
  manager: "Manajer Operasional",
  staff: "Karyawan Dapur & Kurir",
  admin: "Administrator Sistem",
};

export const ROLE_DASHBOARD_PATH: Record<UserRole, string> = {
  owner: "/app/owner",
  manager: "/app/manajer",
  staff: "/app/karyawan",
  admin: "/app/admin",
};

export function getDashboardPathForRole(role?: string | null): string {
  if (!role) return "/login";
  return ROLE_DASHBOARD_PATH[role as UserRole] ?? "/app/owner";
}

