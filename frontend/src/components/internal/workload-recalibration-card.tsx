"use client";

import { useEffect, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Sliders,
  TrendingUp,
} from "lucide-react";
import { formatPercent, toNumber } from "@/lib/format";
import { getProducts, recalibrateWorkload } from "@/lib/api";
import { useToast } from "@/components/providers";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { CalibrationSuggestion, Product } from "@/lib/types";

interface WorkloadRecalibrationCardProps {
  calibration: CalibrationSuggestion;
  onSuccess?: () => void;
  userRole?: string;
  className?: string;
}

export function WorkloadRecalibrationCard({
  calibration,
  onSuccess,
  userRole,
  className = "",
}: WorkloadRecalibrationCardProps) {
  const toast = useToast();
  const avgActual = Math.round(toNumber(calibration.avg_actual_minutes) || 102);
  const estimated = Math.round(toNumber(calibration.estimated_minutes) || 90);

  const [products, setProducts] = useState<Product[]>([]);
  const [selectedProductId, setSelectedProductId] = useState<string>("");
  const [selectedProductName, setSelectedProductName] = useState<string>("Tart Ulang Tahun");

  const [adjustedMinutes, setAdjustedMinutes] = useState<string>(String(avgActual));
  const [submitting, setSubmitting] = useState(false);
  const [justApplied, setJustApplied] = useState<{ name: string; mins: number } | null>(null);

  useEffect(() => {
    let active = true;
    getProducts()
      .then((prods) => {
        if (!active) return;
        setProducts(prods);
        const tart = prods.find((p) => p.name.toLowerCase().includes("tart"));
        if (tart) {
          setSelectedProductId(tart.id);
          setSelectedProductName(tart.name);
        } else if (prods.length > 0) {
          setSelectedProductId(prods[0].id);
          setSelectedProductName(prods[0].name);
        }
      })
      .catch(() => {
        // Abaikan kegagalan jaringan sementara
      });
    return () => {
      active = false;
    };
  }, []);

  const isTartSelected = selectedProductName.toLowerCase().includes("tart");

  const handleProductChange = (id: string) => {
    setSelectedProductId(id);
    const prod = products.find((p) => p.id === id);
    if (prod) {
      setSelectedProductName(prod.name);
      if (prod.name.toLowerCase().includes("tart")) {
        setAdjustedMinutes(String(avgActual));
      }
    }
  };

  if (!calibration.should_recalibrate && !justApplied) {
    return null;
  }

  const handleApplyRecalibration = async () => {
    const minsNum = Number(adjustedMinutes);
    if (!minsNum || minsNum <= 0 || isNaN(minsNum)) {
      toast.error("Validasi Gagal", "Masukkan angka durasi menit yang valid (minimal 1 menit).");
      return;
    }

    setSubmitting(true);
    try {
      const res = await recalibrateWorkload({
        product_id: selectedProductId || undefined,
        new_minutes: minsNum,
      });
      toast.success(
        "Beban Kerja Berhasil Dikalibrasi",
        res.message || `Standar durasi kerja ${selectedProductName} berhasil diperbarui menjadi ${minsNum} menit.`
      );
      setJustApplied({ name: selectedProductName, mins: minsNum });
      onSuccess?.();
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "Gagal menerapkan rekalibrasi beban kerja.";
      toast.error("Gagal Kalibrasi", message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className={`rounded-2xl border border-amber-300 bg-linear-to-r from-amber-50/90 via-amber-50/50 to-white p-3.5 text-amber-950 shadow-xs sm:p-5 ${className}`}
    >
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        {/* Kolom Kiri: Deteksi Deviasi Riil & Pemilih Menu */}
        <div className="space-y-2.5">
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-300 bg-amber-100 px-2.5 py-0.5 text-[11px] font-bold text-amber-900 sm:text-xs">
              <AlertTriangle className="h-3.5 w-3.5 shrink-0 text-amber-700" />
              <span>Rekomendasi Rekalibrasi Beban Kerja (Skenario S6)</span>
            </span>
            <Badge variant="amber" className="text-[11px]">
              {userRole === "owner" ? "Wewenang Pemilik & Manajer" : "Wewenang Manajer Operasional"}
            </Badge>
          </div>

          <div className="flex flex-col items-start gap-1.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-2.5">
            <h3 className="text-sm font-bold text-stone-900 sm:text-base">
              Penyesuaian Waktu Produksi:
            </h3>
            {/* Dropdown Pemilihan Menu Produk */}
            <div className="relative inline-flex w-full max-w-full items-center sm:w-auto">
              <select
                value={selectedProductId}
                onChange={(e) => handleProductChange(e.target.value)}
                aria-label="Pilih Menu Produk untuk Kalibrasi Waktu Kerja"
                className="w-full max-w-full cursor-pointer truncate rounded-xl border border-amber-400 bg-white py-1.5 pl-3 pr-8 text-xs font-bold text-amber-950 shadow-2xs hover:border-amber-500 focus:border-amber-600 focus:outline-none focus:ring-2 focus:ring-amber-500/20 sm:w-auto sm:py-1"
              >
                {products.length > 0 ? (
                  products.map((p) => {
                    const isTart = p.name.toLowerCase().includes("tart");
                    return (
                      <option key={p.id} value={p.id}>
                        {p.name} {isTart ? "(⚠️ Terdeteksi Deviasi S6)" : ""}
                      </option>
                    );
                  })
                ) : (
                  <option value="">Tart Ulang Tahun (⚠️ Terdeteksi Deviasi S6)</option>
                )}
              </select>
            </div>
          </div>

          {isTartSelected ? (
            <p className="max-w-2xl text-xs leading-relaxed text-stone-700">
              Berdasarkan <strong>{calibration.order_count} pesanan selesai</strong> terakhir, durasi pengerjaan riil staf dapur rata-rata memakan waktu{" "}
              <strong className="text-amber-900">{calibration.avg_actual_minutes} menit</strong>, menyimpang{" "}
              <strong className="text-rose-700">+{formatPercent(calibration.deviation)}</strong> dari estimasi standar awal ({calibration.estimated_minutes} menit).
            </p>
          ) : (
            <p className="max-w-2xl text-xs leading-relaxed text-stone-700">
              Sesuaikan standar durasi kerja (menit pengerjaan koki) per unit <strong>{selectedProductName}</strong> untuk memperbarui kalkulasi kapasitas dan utilisasi dapur harian.
            </p>
          )}

          {isTartSelected && (
            <div className="grid grid-cols-1 gap-2 text-xs sm:flex sm:flex-wrap sm:items-center sm:gap-4">
              <div className="flex items-center gap-1.5 rounded-lg border border-amber-200/80 bg-white/90 px-3 py-1.5 shadow-2xs">
                <Clock className="h-3.5 w-3.5 shrink-0 text-stone-500" />
                <span className="text-stone-600">Standar Awal:</span>
                <strong className="font-semibold text-stone-900">{estimated} menit/unit</strong>
              </div>

              <div className="flex items-center gap-1.5 rounded-lg border border-amber-200/80 bg-white/90 px-3 py-1.5 shadow-2xs">
                <TrendingUp className="h-3.5 w-3.5 shrink-0 text-amber-600" />
                <span className="text-stone-600">Rata-rata Riil Staf:</span>
                <strong className="font-semibold text-amber-900">{avgActual} menit/unit</strong>
              </div>
            </div>
          )}
        </div>

        {/* Kolom Kanan: Kontrol Penyesuaian oleh Manajer */}
        <div className="flex w-full shrink-0 flex-col gap-2.5 rounded-xl border border-amber-200 bg-white p-3.5 shadow-xs sm:w-auto sm:min-w-80 sm:p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <label className="flex items-center gap-1 text-xs font-bold text-stone-800">
              <Sliders className="h-3.5 w-3.5 shrink-0 text-amber-600" />
              <span className="truncate">Durasi Kerja ({selectedProductName}):</span>
            </label>
            <span className="text-[11px] text-stone-500">Bisa diubah Manajer</span>
          </div>

          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Input
                type="number"
                min="1"
                max="1440"
                value={adjustedMinutes}
                onChange={(e) => setAdjustedMinutes(e.target.value)}
                className="w-full pl-3 pr-12 text-sm font-bold text-stone-900 focus:border-amber-500 focus:ring-amber-500/20"
                placeholder="102"
              />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs font-semibold text-stone-500">
                menit
              </span>
            </div>

            <Button
              size="sm"
              disabled={submitting}
              onClick={() => void handleApplyRecalibration()}
              className="gap-1.5 bg-amber-600 font-bold text-white shadow-xs hover:bg-amber-700"
            >
              <CheckCircle2 className={`h-3.5 w-3.5 ${submitting ? "animate-spin" : ""}`} />
              {submitting ? "Menyimpan..." : "Terapkan"}
            </Button>
          </div>

          {/* Tombol Preset Cepat */}
          <div className="flex flex-wrap items-center gap-1.5 pt-1">
            <span className="text-[10px] font-medium text-stone-500">Pilihan cepat:</span>
            {[
              { label: isTartSelected ? `${avgActual} m (Riil)` : "30 m", val: isTartSelected ? String(avgActual) : "30" },
              { label: isTartSelected ? "100 m" : "45 m", val: isTartSelected ? "100" : "45" },
              { label: isTartSelected ? "95 m" : "60 m", val: isTartSelected ? "95" : "60" },
              { label: isTartSelected ? "105 m" : "90 m", val: isTartSelected ? "105" : "90" },
            ].map((p) => (
              <button
                key={p.val}
                type="button"
                onClick={() => setAdjustedMinutes(p.val)}
                className={`rounded-md border px-2 py-0.5 text-[10px] font-semibold transition ${
                  adjustedMinutes === p.val
                    ? "border-amber-500 bg-amber-100 text-amber-900"
                    : "border-stone-200 bg-stone-50 text-stone-600 hover:bg-stone-100 hover:text-stone-900"
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>

          {justApplied && (
            <p className="mt-1 flex items-center gap-1 text-[11px] font-medium text-emerald-700">
              <CheckCircle2 className="h-3 w-3 shrink-0" />
              Standar durasi {justApplied.name} disetel ke {justApplied.mins} menit.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

