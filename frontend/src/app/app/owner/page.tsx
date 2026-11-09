"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRight,
  CalendarRange,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Clock,
  ExternalLink,
  RefreshCw,
  Scale,
  SlidersHorizontal,
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

type OwnerTab = "decisions" | "cash_margin" | "calendar" | "simulation";

export default function OwnerDashboardPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<OwnerTab>("decisions");
  const [showAllKpis, setShowAllKpis] = useState<boolean>(false);

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
        <h1 className="font-display text-2xl font-extrabold text-[#12100E]">
          Dashboard Pemilik Usaha
        </h1>
        <TableSkeleton rows={5} />
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
      {/* Header Ringkas */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="amber">Pemilik Usaha (Owner)</Badge>
            <span className="text-xs font-bold text-[#5C5046]">
              {formatDateLong(data.today)}
            </span>
          </div>
          <h1 className="font-display mt-1 text-2xl font-extrabold tracking-tight text-[#12100E] sm:text-3xl">
            Ringkasan Eksekutif &amp; Keputusan
          </h1>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Link href="/app/pengaturan">
            <Button variant="outline" size="sm">
              Parameter Usaha
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

      {/* 4 KPI Utama (Ringkas & Bebas Overload) */}
      <section aria-label="Indikator Kinerja Utama" className="space-y-3">
        <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[4px_4px_0px_#12100E]">
            <div className="flex items-center justify-between text-xs font-extrabold text-[#5C5046] uppercase">
              <span>Ketepatan Waktu</span>
              <Clock className="h-4 w-4 text-[#15803D]" />
            </div>
            <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
              {formatPercent(kpi.on_time_rate)}
            </p>
            <p className="mt-1 text-xs font-semibold text-[#5C5046]">
              {kpi.on_time_orders}/{kpi.completed_orders} selesai tepat jadwal
            </p>
          </div>

          <div className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[4px_4px_0px_#12100E]">
            <div className="flex items-center justify-between text-xs font-extrabold text-[#5C5046] uppercase">
              <span>Utilisasi Dapur</span>
              <CalendarRange className="h-4 w-4 text-[#C2410C]" />
            </div>
            <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
              {formatPercent(kpi.avg_utilization)}
            </p>
            <p className="mt-1 text-xs font-semibold text-[#5C5046]">
              Puncak {formatPercent(kpi.peak_utilization)} ({kpi.active_production_days} hari aktif)
            </p>
          </div>

          <div className="rounded-3xl border-2 border-[#12100E] bg-[#ECFCCB] p-4 shadow-[4px_4px_0px_#12100E]">
            <div className="flex items-center justify-between text-xs font-extrabold text-[#12100E] uppercase">
              <span>Margin Realisasi</span>
              <CheckCircle2 className="h-4 w-4 text-[#15803D]" />
            </div>
            <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
              {formatPercent(kpi.realized_margin)}
            </p>
            <p className="mt-1 text-xs font-bold text-[#3D352E]">
              Omzet {formatRupiah(kpi.total_revenue)}
            </p>
          </div>

          <div
            className={`rounded-3xl border-2 border-[#12100E] p-4 shadow-[4px_4px_0px_#12100E] ${
              kpi.vulnerable_cash_days > 0 ? "bg-[#FFE4E6]" : "bg-[#FEF9C3]"
            }`}
          >
            <div className="flex items-center justify-between text-xs font-extrabold text-[#12100E] uppercase">
              <span>Status Kas 14 Hari</span>
              <Wallet className="h-4 w-4 text-[#12100E]" />
            </div>
            <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
              {kpi.vulnerable_cash_days === 0
                ? "Aman"
                : `${kpi.vulnerable_cash_days} Hari Rawan`}
            </p>
            <p className="mt-1 text-xs font-bold text-[#3D352E]">
              Terendah: {formatRupiah(kpi.cash_lowest)}
            </p>
          </div>
        </div>

        {/* Laci Opsional untuk 3 KPI Sekunder */}
        <div>
          <button
            type="button"
            onClick={() => setShowAllKpis((v) => !v)}
            aria-expanded={showAllKpis}
            className="inline-flex items-center gap-1.5 rounded-full border border-[#12100E] bg-[#F7EFE2] px-3.5 py-1 text-xs font-extrabold text-[#12100E] transition-colors hover:bg-[#FDE047]"
          >
            {showAllKpis ? (
              <>
                <ChevronUp className="h-3.5 w-3.5" />
                <span>Sembunyikan 3 Metrik Tambahan</span>
              </>
            ) : (
              <>
                <ChevronDown className="h-3.5 w-3.5" />
                <span>Lihat Selisih Bahan, Pembatalan, &amp; Statistik Tawar Balik</span>
              </>
            )}
          </button>

          {showAllKpis ? (
            <div className="mt-3 grid grid-cols-1 gap-3.5 sm:grid-cols-3">
              <div className="rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] p-3.5">
                <p className="text-xs font-extrabold text-[#5C5046] uppercase">
                  Selisih Biaya Bahan
                </p>
                <p className="font-mono-num mt-1 text-xl font-extrabold text-[#12100E]">
                  {toNumber(kpi.ingredient_cost_variance) > 0 ? "+" : ""}
                  {formatPercent(kpi.ingredient_cost_variance)}
                </p>
                <p className="mt-0.5 text-xs font-medium text-[#5C5046]">
                  Aktual {formatRupiah(kpi.actual_ingredient_cost)} vs Est{" "}
                  {formatRupiah(kpi.estimated_ingredient_cost)}
                </p>
              </div>

              <div className="rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] p-3.5">
                <p className="text-xs font-extrabold text-[#5C5046] uppercase">
                  Tingkat Pembatalan
                </p>
                <p className="font-mono-num mt-1 text-xl font-extrabold text-[#12100E]">
                  {formatPercent(kpi.cancellation_rate)}
                </p>
                <p className="mt-0.5 text-xs font-medium text-[#5C5046]">
                  {kpi.cancelled_orders} batal + {kpi.expired_orders} DP lewat
                </p>
              </div>

              <div className="rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] p-3.5">
                <p className="text-xs font-extrabold text-[#5C5046] uppercase">
                  Ditawar Balik / Ditolak
                </p>
                <p className="font-mono-num mt-1 text-xl font-extrabold text-[#12100E]">
                  {kpi.countered_or_rejected.total_countered_or_rejected} Pesanan
                </p>
                <p className="mt-0.5 text-xs font-medium text-[#5C5046]">
                  {kpi.countered_or_rejected.total_countered} tawar balik ·{" "}
                  {kpi.countered_or_rejected.total_rejected} ditolak
                </p>
              </div>
            </div>
          ) : null}
        </div>
      </section>

      {/* Navigasi Tab Fokus (Mencegah Cognitive Overload) */}
      <div
        role="tablist"
        aria-label="Fokus tampilan Dashboard Owner"
        className="grid grid-cols-1 gap-2 rounded-3xl border-2 border-[#12100E] bg-[#F7EFE2] p-2 shadow-[4px_4px_0px_#12100E] sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-center"
      >
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "decisions"}
          onClick={() => setActiveTab("decisions")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "decisions"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <Scale className="h-3.5 w-3.5 shrink-0" />
          <span>1. Keputusan Masuk ({queue.length})</span>
        </button>

        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "cash_margin"}
          onClick={() => setActiveTab("cash_margin")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "cash_margin"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <Wallet className="h-3.5 w-3.5 shrink-0" />
          <span>2. Arus Kas &amp; Margin</span>
        </button>

        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "calendar"}
          onClick={() => setActiveTab("calendar")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "calendar"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <CalendarRange className="h-3.5 w-3.5 shrink-0" />
          <span>3. Kalender Kapasitas</span>
        </button>

        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "simulation"}
          onClick={() => setActiveTab("simulation")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "simulation"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <SlidersHorizontal className="h-3.5 w-3.5 shrink-0" />
          <span>4. Simulasi &amp; Kalibrasi</span>
        </button>
      </div>

      {/* TAB 1: ANTRIAN KEPUTUSAN */}
      {activeTab === "decisions" && (
        <section aria-label="Antrian Keputusan Pesanan" className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h2 className="font-display text-lg font-extrabold text-[#12100E]">
                Antrian Keputusan Pesanan ({queue.length})
              </h2>
              <p className="text-xs font-medium text-[#5C5046]">
                Tinjau ringkasan 4-Gerbang di bawah atau klik &ldquo;Lihat Rincian Item&rdquo; pada kartu pesanan.
              </p>
            </div>
            <Link
              href="/app/keputusan"
              className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
            >
              Halaman Keputusan Penuh
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>

          {queue.length === 0 ? (
            <EmptyState
              title="Tidak Ada Pesanan Menunggu Keputusan"
              description="Seluruh pesanan masuk telah selesai dievaluasi."
            />
          ) : (
            <div className="space-y-4">
              {queue.slice(0, 4).map((item) => (
                <DecisionCard
                  key={item.id}
                  item={item}
                  onDecided={handleDecided}
                />
              ))}
            </div>
          )}
        </section>
      )}

      {/* TAB 2: ARUS KAS 14 HARI & MARGIN REALISASI */}
      {activeTab === "cash_margin" && (
        <div className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Wallet className="h-4 w-4 text-[#C2410C]" />
                  Proyeksi Arus Kas 14 Hari
                </CardTitle>
                <CardDescription>
                  Saldo awal {formatRupiah(cash.opening_cash)} · Buffer minimum{" "}
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
                  className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
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
                <div className="flex flex-wrap items-center gap-2 rounded-2xl border-2 border-[#12100E] bg-[#FFE4E6] px-4 py-2.5 text-xs font-bold text-[#9F1239]">
                  <AlertTriangle className="h-4 w-4 shrink-0" />
                  <span>Hari Kas Rawan:</span>
                  {cash.days_below_buffer.map((dt) => (
                    <Badge key={dt} variant="rose">
                      {formatDateShort(dt)}
                    </Badge>
                  ))}
                </div>
              ) : null}
            </CardContent>
          </Card>

          <Card className="min-w-0 overflow-hidden">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <TrendingUp className="h-4 w-4 text-[#C2410C]" />
                  Margin Realisasi per Pesanan
                </CardTitle>
                <CardDescription>
                  Perbandingan margin estimasi awal terhadap biaya bahan &amp; jam
                  kerja aktual.
                </CardDescription>
              </div>
              <Link
                href="/app/pesanan"
                className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
              >
                Semua Pesanan
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </CardHeader>
            <CardContent>
              {kpi.orders_margin.length === 0 ? (
                <p className="py-8 text-center text-xs text-[#5C5046]">
                  Belum ada pesanan untuk perhitungan margin realisasi.
                </p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead>
                      <tr className="border-b-2 border-[#12100E] font-extrabold text-[#12100E]">
                        <th className="py-2.5 pr-3">Pesanan</th>
                        <th className="px-3 py-2.5">Status</th>
                        <th className="px-3 py-2.5 text-right">Total</th>
                        <th className="py-2.5 pl-3 text-right">
                          Margin Est → Realisasi
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[#12100E]/15">
                      {kpi.orders_margin.slice(0, 6).map((row) => {
                        const realM = toNumber(row.realized_margin);
                        return (
                          <tr key={row.order_id} className="hover:bg-[#F7EFE2]">
                            <td className="py-2.5 pr-3">
                              <Link
                                href={`/app/pesanan/${row.order_id}`}
                                className="font-mono-num font-extrabold text-[#C2410C] hover:underline"
                              >
                                #{row.code}
                              </Link>
                              <div className="text-[11px] font-semibold text-[#5C5046]">
                                {row.customer_name} ·{" "}
                                {formatDateShort(row.event_date)}
                              </div>
                            </td>
                            <td className="px-3 py-2.5">
                              <OrderStatusBadge status={row.status} />
                            </td>
                            <td className="font-mono-num px-3 py-2.5 text-right font-extrabold text-[#12100E]">
                              {formatRupiah(row.total)}
                            </td>
                            <td className="font-mono-num py-2.5 pl-3 text-right">
                              <span className="text-[#5C5046]">
                                {formatPercent(row.estimated_margin)} →{" "}
                              </span>
                              <span
                                className={`font-extrabold ${
                                  realM < 0.2
                                    ? "text-[#BE123C]"
                                    : "text-[#15803D]"
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
        </div>
      )}

      {/* TAB 3: KALENDER KAPASITAS BULAN INI */}
      {activeTab === "calendar" && (
        <div className="min-w-0 space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="font-display text-lg font-extrabold text-[#12100E]">
              Kalender Kapasitas Bulan Ini
            </h2>
            <Link
              href="/app/kapasitas"
              className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
            >
              Buka Halaman Kapasitas Penuh
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>

          <CapacityCalendar
            days={data.capacity_calendar}
            selectedDate={selectedDate}
            onSelectDate={(dt) => setSelectedDate(dt)}
          />

          {selectedCapDay ? (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] px-4 py-3 text-xs shadow-[4px_4px_0px_#12100E]">
              <div>
                <span className="font-extrabold text-[#12100E]">
                  {formatDateLong(selectedCapDay.date)}
                </span>
                <span className="ml-2 font-semibold text-[#5C5046]">
                  Utilisasi Puncak:{" "}
                  <strong>{formatPercent(selectedCapDay.utilization)}</strong>
                </span>
              </div>
              <Link
                href={`/app/kapasitas?date=${encodeURIComponent(selectedCapDay.date)}`}
                className="font-extrabold text-[#C2410C] hover:underline"
              >
                Buka Rincian Beban →
              </Link>
            </div>
          ) : null}
        </div>
      )}

      {/* TAB 4: SIMULASI 30 HARI & KALIBRASI BEBAN KERJA */}
      {activeTab === "simulation" && (
        <div className="space-y-6">
          {kpi.calibration ? (
            <WorkloadRecalibrationCard
              calibration={kpi.calibration}
              onSuccess={() => void loadDashboard()}
              userRole="owner"
            />
          ) : null}
          <BusinessImpactSimulation />
        </div>
      )}
    </div>
  );
}
