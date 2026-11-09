"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRight,
  BarChart3,
  CalendarRange,
  CheckCircle2,
  Clock,
  ExternalLink,
  RefreshCw,
  Scale,
  ShieldAlert,
  TrendingUp,
  Wallet,
} from "lucide-react";
import { getErrorMessage, getOwnerDashboard } from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  formatPercent,
  formatRupiah,
  toNumber,
} from "@/lib/format";
import type { OrderDetail } from "@/lib/types";
import { DecisionCard } from "@/components/internal/decision-card";
import { CashProjectionChart } from "@/components/internal/cash-projection-chart";
import { CapacityCalendar } from "@/components/internal/capacity-calendar";
import { OrderStatusBadge } from "@/components/internal/order-timeline";
import { WorkloadRecalibrationCard } from "@/components/internal/workload-recalibration-card";
import { BusinessImpactSimulation } from "@/components/internal/business-impact-simulation";
import { EmptyState, ErrorState, TableSkeleton } from "@/components/ui/states";
import { useToast } from "@/components/providers";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function OwnerDashboardPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedDate, setSelectedDate] = useState<string | null>(null);

  const dashboardQuery = useQuery({
    queryKey: ["owner-dashboard"],
    queryFn: ({ signal }) => getOwnerDashboard(signal),
  });

  const data = dashboardQuery.data ?? null;
  const loading = dashboardQuery.isLoading;
  const errorMsg = dashboardQuery.error
    ? getErrorMessage(
        dashboardQuery.error,
        "Gagal memuat data Dashboard Pemilik Usaha."
      )
    : null;

  const loadDashboard = useCallback(async () => {
    await dashboardQuery.refetch();
  }, [dashboardQuery]);

  function handleDecided(
    updated: OrderDetail,
    action: "accept" | "counter" | "reject"
  ) {
    const labelMap = {
      accept: "diterima",
      counter: "diberi tawaran balik",
      reject: "ditolak",
    };
    toast.success(
      "Keputusan Tersimpan",
      `Pesanan #${updated.code} berhasil ${labelMap[action]}.`
    );
    void queryClient.invalidateQueries({ queryKey: ["owner-dashboard"] });
    void queryClient.invalidateQueries({ queryKey: ["orders"] });
    void dashboardQuery.refetch();
  }

  if (loading && !data) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-stone-900">
              Dashboard Pemilik Usaha (Owner)
            </h1>
            <p className="text-sm text-stone-500">
              Memuat ringkasan KPI, antrian keputusan, proyeksi kas, dan margin
              realisasi...
            </p>
          </div>
        </div>
        <TableSkeleton rows={6} />
      </div>
    );
  }

  if (errorMsg && !data) {
    return (
      <ErrorState
        title="Gagal Memuat Dashboard Pemilik"
        message={errorMsg}
        onRetry={() => void loadDashboard()}
      />
    );
  }

  if (!data) return null;

  const { kpi, cash_projection: cash, decision_queue: queue } = data;
  const selectedCapDay = data.capacity_calendar.find(
    (d) => d.date === selectedDate
  );

  return (
    <div className="space-y-6">
      {/* Header Dashboard */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="amber">Pemilik Usaha (Owner)</Badge>
            <span className="text-xs font-medium text-stone-500">
              Tanggal Operasional: {formatDateLong(data.today)}
            </span>
          </div>
          <h1 className="mt-1.5 text-2xl font-bold tracking-tight text-stone-900">
            Dashboard Eksekutif &amp; Kendali Keputusan
          </h1>
          <p className="mt-0.5 text-sm text-stone-600">
            Pantau antrian keputusan (termasuk pesanan besar &amp; override),
            7 KPI bisnis, ketahanan arus kas 14 hari, dan margin realisasi.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Link href="/app/pengaturan">
            <Button variant="outline" size="sm">
              Parameter Usaha
            </Button>
          </Link>
          <Link href="/app/audit">
            <Button variant="outline" size="sm">
              Log Audit
            </Button>
          </Link>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void loadDashboard()}
            disabled={loading}
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`}
            />
            Segarkan
          </Button>
        </div>
      </div>

      {/* Peringatan & Penyesuaian Rekalibrasi Beban Kerja (Skenario S6) */}
      {kpi.calibration && (
        <WorkloadRecalibrationCard
          calibration={kpi.calibration}
          onSuccess={() => void loadDashboard()}
          userRole="owner"
        />
      )}

      {/* ELEMEN 1: Ringkasan 7 KPI Spesifikasi */}
      <section aria-label="Ringkasan Indikator Kinerja Utama" className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
            <BarChart3 className="h-4 w-4 text-amber-700" />
            1. Ringkasan Indikator Kinerja Utama (7 KPI)
          </h2>
          <span className="text-xs text-stone-500">
            Dihitung otomatis dari transaksi pesanan, produksi &amp; kas
          </span>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {/* KPI 1: Ketepatan Waktu */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>1. Ketepatan Waktu</span>
              <Clock className="h-4 w-4 text-emerald-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {formatPercent(kpi.on_time_rate)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.on_time_orders} dari {kpi.completed_orders} pesanan selesai
              tepat jadwal
            </p>
          </div>

          {/* KPI 2: Utilisasi Rata-rata */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>2. Utilisasi Rata-rata</span>
              <CalendarRange className="h-4 w-4 text-amber-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {formatPercent(kpi.avg_utilization)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.active_production_days} hari aktif · Puncak{" "}
              {formatPercent(kpi.peak_utilization)}
              {kpi.peak_utilization_date
                ? ` (${formatDateShort(kpi.peak_utilization_date)})`
                : ""}
            </p>
          </div>

          {/* KPI 3: Selisih Biaya Bahan */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>3. Selisih Biaya Bahan</span>
              <TrendingUp className="h-4 w-4 text-sky-600" />
            </div>
            <p
              className={`mt-2 text-2xl font-bold ${
                toNumber(kpi.ingredient_cost_variance) > 0.05
                  ? "text-rose-700"
                  : "text-stone-900"
              }`}
            >
              {toNumber(kpi.ingredient_cost_variance) > 0 ? "+" : ""}
              {formatPercent(kpi.ingredient_cost_variance)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              Aktual {formatRupiah(kpi.actual_ingredient_cost)} vs Est{" "}
              {formatRupiah(kpi.estimated_ingredient_cost)}
            </p>
          </div>

          {/* KPI 4: Tingkat Pembatalan */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>4. Tingkat Pembatalan</span>
              <ShieldAlert className="h-4 w-4 text-rose-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {formatPercent(kpi.cancellation_rate)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.cancelled_orders} batal + {kpi.expired_orders} kedaluwarsa DP
              dari {kpi.accepted_orders_total} diterima
            </p>
          </div>

          {/* KPI 5: Ditawar Balik / Ditolak */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>5. Ditawar Balik / Ditolak</span>
              <Scale className="h-4 w-4 text-amber-700" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {kpi.countered_or_rejected.total_countered_or_rejected} Pesanan
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.countered_or_rejected.total_countered} tawar balik (
              {kpi.countered_or_rejected.counter_accepted} disetujui) ·{" "}
              {kpi.countered_or_rejected.total_rejected} ditolak
            </p>
          </div>

          {/* KPI 6: Margin Realisasi Agregat */}
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>6. Margin Realisasi</span>
              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-emerald-800">
              {formatPercent(kpi.realized_margin)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              Margin Estimasi: {formatPercent(kpi.estimated_margin)} · Omzet{" "}
              {formatRupiah(kpi.total_revenue)}
            </p>
          </div>

          {/* KPI 7: Hari Kas Rawan */}
          <div
            className={`rounded-2xl border p-4 shadow-2xs sm:col-span-2 ${
              kpi.vulnerable_cash_days > 0
                ? "border-rose-200 bg-rose-50/70"
                : "border-emerald-200 bg-emerald-50/50"
            }`}
          >
            <div className="flex items-center justify-between text-xs font-semibold text-stone-600">
              <span>7. Hari Kas Rawan (14 Hari ke Depan)</span>
              <Wallet
                className={`h-4 w-4 ${
                  kpi.vulnerable_cash_days > 0
                    ? "text-rose-600"
                    : "text-emerald-600"
                }`}
              />
            </div>
            <div className="mt-2 flex flex-wrap items-baseline justify-between gap-2">
              <p
                className={`text-2xl font-bold ${
                  kpi.vulnerable_cash_days > 0
                    ? "text-rose-800"
                    : "text-emerald-900"
                }`}
              >
                {kpi.vulnerable_cash_days} Hari di Bawah Buffer
              </p>
              <span className="text-xs font-semibold text-stone-700">
                Titik Terendah: {formatRupiah(kpi.cash_lowest)} (
                {formatDateShort(kpi.cash_lowest_date)}) · Buffer:{" "}
                {formatRupiah(kpi.cash_buffer)}
              </span>
            </div>
            {kpi.vulnerable_cash_dates.length > 0 ? (
              <p className="mt-1 text-xs font-medium text-rose-700">
                Tanggal rawan:{" "}
                {kpi.vulnerable_cash_dates
                  .map((d) => formatDateShort(d))
                  .join(", ")}
              </p>
            ) : (
              <p className="mt-1 text-xs text-emerald-800">
                Seluruh hari pada horizon 14 hari berada di atas penyangga kas
                minimum.
              </p>
            )}
          </div>
        </div>
      </section>

      {/* ELEMEN 2: Antrian Keputusan dengan DecisionCard */}
      <section aria-label="Antrian Keputusan Pesanan" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
              <Scale className="h-4 w-4 text-amber-700" />
              2. Antrian Keputusan &amp; Pratinjau Dampak ({queue.length})
            </h2>
            <p className="text-xs text-stone-500">
              Pesanan berstatus menunggu konfirmasi (termasuk pesanan besar yang
              wajib diputuskan Pemilik).
            </p>
          </div>
          <Link
            href="/app/keputusan"
            className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
          >
            Buka Halaman Keputusan Penuh
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </div>

        {queue.length === 0 ? (
          <EmptyState
            title="Tidak Ada Pesanan yang Menunggu Keputusan"
            description="Seluruh pesanan masuk telah dievaluasi atau diputuskan."
          />
        ) : (
          <div className="space-y-4">
            {queue.slice(0, 3).map((item) => (
              <DecisionCard
                key={item.id}
                item={item}
                onDecided={handleDecided}
              />
            ))}
            {queue.length > 3 ? (
              <div className="text-center">
                <Link href="/app/keputusan">
                  <Button variant="outline" size="sm">
                    Lihat {queue.length - 3} Pesanan Lainnya di Antrian
                    Keputusan
                  </Button>
                </Link>
              </div>
            ) : null}
          </div>
        )}
      </section>

      {/* ELEMEN 3: Proyeksi Kas 14 Hari dengan Hari Rawan */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Wallet className="h-4 w-4 text-amber-700" />
              3. Proyeksi Arus Kas 14 Hari &amp; Deteksi Hari Rawan
            </CardTitle>
            <CardDescription>
              Saldo awal {formatRupiah(cash.opening_cash)} · Biaya tetap harian{" "}
              {formatRupiah(cash.fixed_cost_daily)} · Penyangga kas{" "}
              {formatRupiah(cash.cash_buffer)}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant={cash.passed ? "emerald" : "rose"}>
              {cash.passed
                ? "G3 Kas Aman"
                : `${cash.days_below_buffer.length} Hari Rawan`}
            </Badge>
            <Link
              href="/app/kas"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Rincian Kas
              <ExternalLink className="h-3.5 w-3.5" />
            </Link>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <CashProjectionChart
            days={cash.days ?? []}
            cashBuffer={cash.cash_buffer}
            lowestDate={cash.lowest_date}
            lowestAmount={cash.lowest}
          />
          {cash.days_below_buffer.length > 0 ? (
            <div className="flex flex-wrap items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-2.5 text-xs text-rose-900">
              <AlertTriangle className="h-4 w-4 shrink-0 text-rose-600" />
              <span className="font-semibold">
                Peringatan Hari Kas Rawan (Saldo &lt;{" "}
                {formatRupiah(cash.cash_buffer)}):
              </span>
              {cash.days_below_buffer.map((dt) => (
                <Badge key={dt} variant="rose">
                  {formatDateShort(dt)}
                </Badge>
              ))}
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* ELEMEN 4 & 5: Margin Realisasi per Pesanan & Kalender Kapasitas Bulan Ini */}
      <div className="grid grid-cols-1 gap-6">
        {/* ELEMEN 4: Margin Realisasi per Pesanan */}
        <Card className="min-w-0 overflow-hidden">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle className="flex items-center gap-2">
                <TrendingUp className="h-4 w-4 text-amber-700" />
                4. Margin Realisasi per Pesanan
              </CardTitle>
              <CardDescription>
                Perbandingan biaya &amp; margin estimasi awal terhadap biaya
                bahan terkini dan jam kerja aktual.
              </CardDescription>
            </div>
            <Link
              href="/app/pesanan"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Semua Pesanan
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </CardHeader>
          <CardContent>
            {kpi.orders_margin.length === 0 ? (
              <p className="py-8 text-center text-xs text-stone-500">
                Belum ada pesanan aktif/selesai untuk perhitungan margin
                realisasi.
              </p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead>
                    <tr className="border-b border-stone-200 text-stone-500">
                      <th className="py-2.5 pr-3 font-semibold">Pesanan</th>
                      <th className="px-3 py-2.5 font-semibold">Status</th>
                      <th className="px-3 py-2.5 text-right font-semibold">
                        Total
                      </th>
                      <th className="px-3 py-2.5 text-right font-semibold">
                        Biaya Est / Aktual
                      </th>
                      <th className="py-2.5 pl-3 text-right font-semibold">
                        Margin Est → Realisasi
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-100">
                    {kpi.orders_margin.slice(0, 8).map((row) => {
                      const realM = toNumber(row.realized_margin);
                      const estM = toNumber(row.estimated_margin);
                      return (
                        <tr key={row.order_id} className="hover:bg-stone-50/80">
                          <td className="py-2.5 pr-3">
                            <Link
                              href={`/app/pesanan/${row.order_id}`}
                              className="font-mono font-bold text-amber-800 hover:underline"
                            >
                              #{row.code}
                            </Link>
                            <div className="text-[11px] text-stone-600">
                              {row.customer_name} ·{" "}
                              {formatDateShort(row.event_date)}
                            </div>
                          </td>
                          <td className="px-3 py-2.5">
                            <OrderStatusBadge status={row.status} />
                          </td>
                          <td className="px-3 py-2.5 text-right font-semibold text-stone-900">
                            {formatRupiah(row.total)}
                          </td>
                          <td className="px-3 py-2.5 text-right text-stone-600">
                            <div>Est: {formatRupiah(row.estimated_cost)}</div>
                            <div className="font-semibold text-stone-900">
                              Akt: {formatRupiah(row.actual_cost)}
                            </div>
                          </td>
                          <td className="py-2.5 pl-3 text-right">
                            <span className="text-stone-500">
                              {formatPercent(row.estimated_margin)} →{" "}
                            </span>
                            <span
                              className={`font-bold ${
                                realM < 0.2
                                  ? "text-rose-700"
                                  : realM < estM
                                    ? "text-amber-700"
                                    : "text-emerald-700"
                              }`}
                            >
                              {formatPercent(row.realized_margin)}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>

        {/* ELEMEN 5: Kalender Kapasitas Ringkas */}
        <div className="min-w-0 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
              <CalendarRange className="h-4 w-4 text-amber-700" />
              5. Kalender Kapasitas Bulan Ini
            </h2>
            <Link
              href="/app/kapasitas"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Detail Harian
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>

          <CapacityCalendar
            days={data.capacity_calendar}
            selectedDate={selectedDate}
            onSelectDate={(dt) => setSelectedDate(dt)}
          />

          {selectedCapDay ? (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-stone-200 bg-white px-4 py-3 text-xs">
              <div>
                <span className="font-bold text-stone-900">
                  {formatDateLong(selectedCapDay.date)}
                </span>
                <span className="ml-2 text-stone-600">
                  Utilisasi Puncak:{" "}
                  <strong>{formatPercent(selectedCapDay.utilization)}</strong>
                </span>
              </div>
              <Link
                href={`/app/kapasitas?date=${encodeURIComponent(selectedCapDay.date)}`}
                className="font-semibold text-amber-800 hover:underline"
              >
                Buka Rincian Beban →
              </Link>
            </div>
          ) : null}
        </div>
      </div>

      {/* ELEMEN 6: Simulasi Dampak Bisnis 30 Hari */}
      <section aria-label="Simulasi Dampak Bisnis 30 Hari">
        <BusinessImpactSimulation />
      </section>
    </div>
  );
}
