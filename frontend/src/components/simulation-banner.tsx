import { FlaskConical } from "lucide-react";

export function SimulationBanner() {
  return (
    <div
      role="region"
      aria-label="Informasi data simulasi"
      className="border-b border-amber-200/80 bg-amber-50 px-3 py-1.5 text-center text-xs font-medium text-amber-900 print:hidden"
    >
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-center gap-1.5">
        <FlaskConical className="h-3.5 w-3.5 shrink-0 text-amber-700" />
        <span className="font-semibold">Data simulasi</span>
        <span className="hidden text-amber-800/80 sm:inline">
          — Lingkungan demonstrasi sistem manajemen pesanan &amp; kapasitas UMKM
          kuliner (GTNIC 2026).
        </span>
      </div>
    </div>
  );
}

