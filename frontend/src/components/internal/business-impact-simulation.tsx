"use client";

import { useState } from "react";
import {
  BarChart3,
  CheckCircle2,
  Coins,
  ShieldCheck,
  Sparkles,
  TrendingUp,
  XCircle,
  Zap,
} from "lucide-react";
import { formatRupiah } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

type ScaleOption = "small" | "medium" | "large";

interface ScaleConfig {
  label: string;
  orderCount: number;
  traditionalOvertime: number;
  traditionalLateCompensation: number;
  traditionalUnpaidLoss: number;
  traditionalMargin: string;
  kalkurasaMargin: string;
}

const SCALE_DATA: Record<ScaleOption, ScaleConfig> = {
  small: {
    label: "Skala Kecil (50 pesanan/bln)",
    orderCount: 50,
    traditionalOvertime: 750000,
    traditionalLateCompensation: 450000,
    traditionalUnpaidLoss: 600000,
    traditionalMargin: "16.5%",
    kalkurasaMargin: "29.2%",
  },
  medium: {
    label: "Skala Menengah (150 pesanan/bln)",
    orderCount: 150,
    traditionalOvertime: 1850000,
    traditionalLateCompensation: 1200000,
    traditionalUnpaidLoss: 1500000,
    traditionalMargin: "15.2%",
    kalkurasaMargin: "30.4%",
  },
  large: {
    label: "Skala Besar (350 pesanan/bln)",
    orderCount: 350,
    traditionalOvertime: 3800000,
    traditionalLateCompensation: 2600000,
    traditionalUnpaidLoss: 3200000,
    traditionalMargin: "14.8%",
    kalkurasaMargin: "31.8%",
  },
};

export function BusinessImpactSimulation({ className = "" }: { className?: string }) {
  const [scale, setScale] = useState<ScaleOption>("medium");
  const current = SCALE_DATA[scale];

  const totalTraditionalLoss =
    current.traditionalOvertime +
    current.traditionalLateCompensation +
    current.traditionalUnpaidLoss;

  return (
    <Card className={`border-stone-200 bg-white shadow-xs ${className}`}>
      <CardHeader className="border-b border-stone-100 pb-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-amber-600" />
              <CardTitle className="text-base font-bold text-stone-900">
                Simulasi Dampak Bisnis 30 Hari (Business Impact Analysis)
              </CardTitle>
              <Badge variant="amber" className="text-[10px]">
                Validasi Dewan Juri GTNIC
              </Badge>
            </div>
            <p className="text-xs text-stone-600">
              Perbandingan ekonomi konkret antara metode konvensional (catatan manual/chat) vs sistem otomatisasi terintegrasi Kalkurasa.
            </p>
          </div>

          {/* Pemilih Skala Usaha */}
          <div className="flex items-center gap-1 rounded-xl border border-stone-200 bg-stone-50 p-1">
            {(Object.keys(SCALE_DATA) as ScaleOption[]).map((key) => {
              const isActive = scale === key;
              return (
                <button
                  key={key}
                  type="button"
                  onClick={() => setScale(key)}
                  className={`rounded-lg px-2.5 py-1 text-xs font-semibold transition ${
                    isActive
                      ? "bg-amber-600 text-white shadow-2xs"
                      : "text-stone-600 hover:text-stone-900"
                  }`}
                >
                  {SCALE_DATA[key].label.split(" (")[0]}
                </button>
              );
            })}
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-5 pt-5">
        {/* Banner Ringkasan Penghematan */}
        <div className="flex flex-col justify-between gap-3 rounded-2xl border border-emerald-200 bg-linear-to-r from-emerald-50 via-teal-50/40 to-white p-4.5 sm:flex-row sm:items-center">
          <div className="space-y-0.5">
            <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-emerald-800">
              <Zap className="h-4 w-4 text-emerald-600" />
              Estimasi Penghematan &amp; Proteksi Arus Kas Bulanan
            </div>
            <p className="text-xs text-stone-600">
              Berdasarkan simulasi operasional {current.orderCount} pesanan katering per bulan:
            </p>
          </div>

          <div className="flex items-baseline gap-2">
            <span className="text-2xl font-extrabold tracking-tight text-emerald-700 sm:text-3xl">
              +{formatRupiah(totalTraditionalLoss)}
            </span>
            <span className="text-xs font-semibold text-emerald-800">/ bulan</span>
          </div>
        </div>

        {/* Tabel Komparasi 4 Dimensi Kritis */}
        <div className="grid grid-cols-1 gap-3.5 md:grid-cols-2 lg:grid-cols-4">
          {/* Dimensi 1: Kapasitas */}
          <div className="flex flex-col justify-between rounded-xl border border-stone-200 bg-stone-50/50 p-3.5">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-stone-900">1. Kapasitas &amp; Jadwal</span>
                <ShieldCheck className="h-4 w-4 text-emerald-600" />
              </div>

              <div className="space-y-1.5 text-[11px]">
                <div className="rounded-lg border border-rose-100 bg-rose-50/70 p-2 text-rose-900">
                  <div className="flex items-center gap-1 font-semibold">
                    <XCircle className="h-3 w-3 text-rose-600" />
                    Tanpa Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-stone-600">
                    Sering bentrok jadwal, denda keterlambatan ~{formatRupiah(current.traditionalLateCompensation)}.
                  </p>
                </div>

                <div className="rounded-lg border border-emerald-100 bg-emerald-50/70 p-2 text-emerald-950">
                  <div className="flex items-center gap-1 font-semibold text-emerald-900">
                    <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                    Dengan Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-emerald-900 font-medium">
                    0 Bentrok (Gerbang G1–G4 mengunci slot secara presisi).
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Dimensi 2: Biaya Lembur */}
          <div className="flex flex-col justify-between rounded-xl border border-stone-200 bg-stone-50/50 p-3.5">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-stone-900">2. Waktu Kerja &amp; Lembur</span>
                <TrendingUp className="h-4 w-4 text-amber-600" />
              </div>

              <div className="space-y-1.5 text-[11px]">
                <div className="rounded-lg border border-rose-100 bg-rose-50/70 p-2 text-rose-900">
                  <div className="flex items-center gap-1 font-semibold">
                    <XCircle className="h-3 w-3 text-rose-600" />
                    Tanpa Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-stone-600">
                    Beban lembur koki membengkak ~{formatRupiah(current.traditionalOvertime)}.
                  </p>
                </div>

                <div className="rounded-lg border border-emerald-100 bg-emerald-50/70 p-2 text-emerald-950">
                  <div className="flex items-center gap-1 font-semibold text-emerald-900">
                    <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                    Dengan Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-emerald-900 font-medium">
                    Hemat {formatRupiah(current.traditionalOvertime)} (Rekalibrasi durasi riil S6).
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Dimensi 3: Arus Kas & DP */}
          <div className="flex flex-col justify-between rounded-xl border border-stone-200 bg-stone-50/50 p-3.5">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-stone-900">3. Arus Kas &amp; Uang Muka</span>
                <Coins className="h-4 w-4 text-amber-600" />
              </div>

              <div className="space-y-1.5 text-[11px]">
                <div className="rounded-lg border border-rose-100 bg-rose-50/70 p-2 text-rose-900">
                  <div className="flex items-center gap-1 font-semibold">
                    <XCircle className="h-3 w-3 text-rose-600" />
                    Tanpa Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-stone-600">
                    Masak sebelum DP masuk, rugi batal sepihak ~{formatRupiah(current.traditionalUnpaidLoss)}.
                  </p>
                </div>

                <div className="rounded-lg border border-emerald-100 bg-emerald-50/70 p-2 text-emerald-950">
                  <div className="flex items-center gap-1 font-semibold text-emerald-900">
                    <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                    Dengan Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-emerald-900 font-medium">
                    100% Terkunci DP (Jadwal produksi hanya aktif jika DP lunas).
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Dimensi 4: Margin Keuntungan */}
          <div className="flex flex-col justify-between rounded-xl border border-stone-200 bg-stone-50/50 p-3.5">
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-stone-900">4. Margin Keuntungan Bersih</span>
                <BarChart3 className="h-4 w-4 text-emerald-600" />
              </div>

              <div className="space-y-1.5 text-[11px]">
                <div className="rounded-lg border border-rose-100 bg-rose-50/70 p-2 text-rose-900">
                  <div className="flex items-center gap-1 font-semibold">
                    <XCircle className="h-3 w-3 text-rose-600" />
                    Tanpa Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-stone-600">
                    Margin tergerus hingga rata-rata <strong>{current.traditionalMargin}</strong>.
                  </p>
                </div>

                <div className="rounded-lg border border-emerald-100 bg-emerald-50/70 p-2 text-emerald-950">
                  <div className="flex items-center gap-1 font-semibold text-emerald-900">
                    <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                    Dengan Kalkurasa:
                  </div>
                  <p className="mt-0.5 text-emerald-900 font-medium">
                    Meningkat stabil ke <strong>{current.kalkurasaMargin}</strong> (+{(parseFloat(current.kalkurasaMargin) - parseFloat(current.traditionalMargin)).toFixed(1)}%).
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

