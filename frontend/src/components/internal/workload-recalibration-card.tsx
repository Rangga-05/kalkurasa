"use client";

import { useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  Sliders,
  TrendingUp,
} from "lucide-react";
import { formatPercent, toNumber } from "@/lib/format";
import { recalibrateWorkload } from "@/lib/api";
import { useToast } from "@/components/providers";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { CalibrationSuggestion } from "@/lib/types";

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

  const [adjustedMinutes, setAdjustedMinutes] = useState<string>(String(avgActual));
  const [submitting, setSubmitting] = useState(false);
  const [justApplied, setJustApplied] = useState<number | null>(null);

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
        new_minutes: minsNum,
      });
      toast.success(
        "Beban Kerja Berhasil Dikalibrasi",
        res.message || `Standar durasi kerja berhasil diperbarui menjadi ${minsNum} menit.`
      );
      setJustApplied(minsNum);
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
      className={`rounded-2xl border border-amber-300 bg-linear-to-r from-amber-50/90 via-amber-50/50 to-white p-5 text-amber-950 shadow-xs ${className}`}
    >
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        {/* Kolom Kiri: Deteksi Deviasi Riil */}
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-300 bg-amber-100 px-2.5 py-0.5 text-xs font-bold text-amber-900">
              <AlertTriangle className="h-3.5 w-3.5 text-amber-700" />
              Rekomendasi Rekalibrasi Beban Kerja (Skenario S6)
            </span>
            <Badge variant="amber" className="text-[11px]">
              {userRole === "owner" ? "Wewenang Pemilik & Manajer" : "Wewenang Manajer Operasional"}
            </Badge>
          </div>

          <h3 className="text-base font-bold text-stone-900">
            Penyesuaian Waktu Produksi: Tart Ulang Tahun
          </h3>

          <p className="max-w-2xl text-xs leading-relaxed text-stone-700">
            Berdasarkan <strong>{calibration.order_count} pesanan selesai</strong> terakhir, durasi pengerjaan riil staf dapur rata-rata memakan waktu{" "}
            <strong className="text-amber-900">{calibration.avg_actual_minutes} menit</strong>, menyimpang{" "}
            <strong className="text-rose-700">+{formatPercent(calibration.deviation)}</strong> dari estimasi standar awal ({calibration.estimated_minutes} menit).
          </p>

          <div className="flex flex-wrap items-center gap-4 text-xs">
            <div className="flex items-center gap-1.5 rounded-lg border border-amber-200/80 bg-white/90 px-3 py-1.5 shadow-2xs">
              <Clock className="h-3.5 w-3.5 text-stone-500" />
              <span className="text-stone-600">Standar Awal:</span>
              <strong className="font-semibold text-stone-900">{estimated} menit/unit</strong>
            </div>

            <div className="flex items-center gap-1.5 rounded-lg border border-amber-200/80 bg-white/90 px-3 py-1.5 shadow-2xs">
              <TrendingUp className="h-3.5 w-3.5 text-amber-600" />
              <span className="text-stone-600">Rata-rata Riil Staf:</span>
              <strong className="font-semibold text-amber-900">{avgActual} menit/unit</strong>
            </div>
          </div>
        </div>

        {/* Kolom Kanan: Kontrol Penyesuaian oleh Manajer */}
        <div className="flex shrink-0 flex-col gap-2.5 rounded-xl border border-amber-200 bg-white p-4 shadow-xs sm:min-w-80">
          <div className="flex items-center justify-between gap-2">
            <label className="flex items-center gap-1 text-xs font-bold text-stone-800">
              <Sliders className="h-3.5 w-3.5 text-amber-600" />
              Durasi Kerja Baru:
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
              { label: `${avgActual} m (Riil)`, val: String(avgActual) },
              { label: "100 m", val: "100" },
              { label: "95 m", val: "95" },
              { label: "105 m", val: "105" },
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
              Standar durasi berhasil disetel ke {justApplied} menit.
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

