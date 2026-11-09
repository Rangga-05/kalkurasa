"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Boxes,
  CalendarRange,
  CheckCircle2,
  Clock,
  CreditCard,
  Plus,
  RefreshCw,
  Scale,
  ShoppingCart,
  SlidersHorizontal,
  Truck,
  UtensilsCrossed,
} from "lucide-react";
import { getErrorMessage, getManagerDashboard } from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  formatDateTimeWIB,
  formatNumber,
  formatPercent,
  formatRupiah,
  toNumber,
} from "@/lib/format";
import type { OrderDetail } from "@/lib/types";
import { CapacityCalendar } from "@/components/internal/capacity-calendar";
import { DecisionCard } from "@/components/internal/decision-card";
import { OrderStatusBadge } from "@/components/internal/order-timeline";
import { WorkloadRecalibrationCard } from "@/components/internal/workload-recalibration-card";
import { BusinessImpactSimulation } from "@/components/internal/business-impact-simulation";
import { ErrorState, TableSkeleton } from "@/components/ui/states";
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

type ManagerTab = "schedule" | "decisions_stock" | "calendar" | "simulation";

export default function ManagerDashboardPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ManagerTab>("schedule");

  const dashboardQuery = useQuery({
    queryKey: ["manager-dashboard"],
    queryFn: ({ signal }) => getManagerDashboard(signal),
  });

  const data = dashboardQuery.data ?? null;
  const loading = dashboardQuery.isLoading;
  const errorMsg = dashboardQuery.error
    ? getErrorMessage(
        dashboardQuery.error,
        "Gagal memuat data Dashboard Manajer Operasional."
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
    void queryClient.invalidateQueries({ queryKey: ["manager-dashboard"] });
    void queryClient.invalidateQueries({ queryKey: ["orders"] });
    void dashboardQuery.refetch();
  }

  if (loading && !data) {
    return (
      <div className="space-y-6">
        <h1 className="font-display text-2xl font-extrabold text-[#12100E]">
          Dashboard Manajer Operasional
        </h1>
        <TableSkeleton rows={5} />
      </div>
    );
  }

  if (errorMsg && !data) {
    return (
      <ErrorState
        title="Gagal Memuat Dashboard Manajer"
        message={errorMsg}
        onRetry={() => void loadDashboard()}
      />
    );
  }

  if (!data) return null;

  const {
    kpi,
    capacity_calendar: calendar,
    production_schedule: prodSchedule,
    low_stock_alerts: lowStocks,
    dp_due_alerts: dpAlerts,
    decision_queue: queue,
    shopping_list: shopList,
    cash_status: cashStatus,
  } = data;

  const selectedCapDay = calendar.find((d) => d.date === selectedDate);

  return (
    <div className="space-y-6">
      {/* Header Ringkas */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="emerald">Manajer Operasional</Badge>
            <span className="text-xs font-bold text-[#5C5046]">
              {formatDateLong(data.today)}
            </span>
          </div>
          <h1 className="font-display mt-1 text-2xl font-extrabold tracking-tight text-[#12100E] sm:text-3xl">
            Kendali Produksi &amp; Kapasitas Harian
          </h1>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Link href="/app/pesanan">
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              Catat Pesanan
            </Button>
          </Link>
          <Link href="/app/belanja">
            <Button variant="outline" size="sm">
              <ShoppingCart className="h-3.5 w-3.5" />
              Belanja ({shopList?.items.length ?? 0})
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

      {/* 4 KPI Operasional Ringkas */}
      <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[4px_4px_0px_#12100E]">
          <div className="flex items-center justify-between text-xs font-extrabold text-[#5C5046] uppercase">
            <span>Ketepatan Jadwal</span>
            <Clock className="h-4 w-4 text-[#15803D]" />
          </div>
          <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
            {formatPercent(kpi.on_time_rate)}
          </p>
          <p className="mt-1 text-xs font-semibold text-[#5C5046]">
            {kpi.on_time_orders}/{kpi.completed_orders} pesanan tepat waktu
          </p>
        </div>

        <div className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[4px_4px_0px_#12100E]">
          <div className="flex items-center justify-between text-xs font-extrabold text-[#5C5046] uppercase">
            <span>Utilisasi Rata-rata</span>
            <CalendarRange className="h-4 w-4 text-[#C2410C]" />
          </div>
          <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
            {formatPercent(kpi.avg_utilization)}
          </p>
          <p className="mt-1 text-xs font-semibold text-[#5C5046]">
            Puncak {formatPercent(kpi.peak_utilization)}
          </p>
        </div>

        <div className="rounded-3xl border-2 border-[#12100E] bg-[#FEF9C3] p-4 shadow-[4px_4px_0px_#12100E]">
          <div className="flex items-center justify-between text-xs font-extrabold text-[#12100E] uppercase">
            <span>Perlu Keputusan</span>
            <Scale className="h-4 w-4 text-[#C2410C]" />
          </div>
          <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
            {queue.length} Pesanan
          </p>
          <p className="mt-1 text-xs font-bold text-[#3D352E]">
            {dpAlerts.length} menunggu verifikasi DP
          </p>
        </div>

        <div
          className={`rounded-3xl border-2 border-[#12100E] p-4 shadow-[4px_4px_0px_#12100E] ${
            lowStocks.length > 0 ? "bg-[#FFE4E6]" : "bg-[#ECFCCB]"
          }`}
        >
          <div className="flex items-center justify-between text-xs font-extrabold text-[#12100E] uppercase">
            <span>Stok &amp; Buffer Kas</span>
            <CheckCircle2 className="h-4 w-4 text-[#15803D]" />
          </div>
          <p className="font-mono-num mt-2 text-2xl font-extrabold text-[#12100E]">
            {lowStocks.length === 0 ? "Stok Aman" : `${lowStocks.length} Bahan Tipis`}
          </p>
          <p className="mt-1 text-xs font-bold text-[#3D352E]">
            G3 Kas: {cashStatus.passed ? "Lolos Buffer" : "Perlu Perhatian"}
          </p>
        </div>
      </div>

      {/* Tab Navigasi Fokus Operasional */}
      <div
        role="tablist"
        aria-label="Fokus tampilan Dashboard Manajer"
        className="grid grid-cols-1 gap-2 rounded-3xl border-2 border-[#12100E] bg-[#F7EFE2] p-2 shadow-[4px_4px_0px_#12100E] sm:grid-cols-2 xl:flex xl:flex-wrap xl:items-center"
      >
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "schedule"}
          onClick={() => setActiveTab("schedule")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "schedule"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <UtensilsCrossed className="h-3.5 w-3.5 shrink-0" />
          <span>1. Jadwal Produksi &amp; DP ({prodSchedule.length})</span>
        </button>

        <button
          type="button"
          role="tab"
          aria-selected={activeTab === "decisions_stock"}
          onClick={() => setActiveTab("decisions_stock")}
          className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-full px-4 py-2 text-xs font-extrabold uppercase transition-all xl:justify-start ${
            activeTab === "decisions_stock"
              ? "border-2 border-[#12100E] bg-[#12100E] text-[#D9F99D]"
              : "text-[#12100E] hover:bg-[#FFFDF9]"
          }`}
        >
          <Scale className="h-3.5 w-3.5 shrink-0" />
          <span>2. Keputusan ({queue.length}) &amp; Stok ({lowStocks.length})</span>
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

      {/* TAB 1: JADWAL PRODUKSI & MENUNGGU DP */}
      {activeTab === "schedule" && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
          <Card className="min-w-0 lg:col-span-7">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <UtensilsCrossed className="h-4 w-4 text-[#C2410C]" />
                  Jadwal Produksi &amp; Pengantaran ({prodSchedule.length})
                </CardTitle>
                <CardDescription>
                  Pesanan terkonfirmasi (DP lunas) dan siap diproduksi/diantar.
                </CardDescription>
              </div>
              <Link
                href="/app/karyawan"
                className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
              >
                Papan Tugas Dapur
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </CardHeader>
            <CardContent>
              {prodSchedule.length === 0 ? (
                <p className="py-8 text-center text-xs text-[#5C5046]">
                  Belum ada jadwal produksi aktif pada rentang hari ini.
                </p>
              ) : (
                <div className="space-y-2.5">
                  {prodSchedule.slice(0, 6).map((ord) => (
                    <div
                      key={ord.id}
                      className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border-2 border-[#12100E] bg-[#FFF9F0] p-3 text-xs"
                    >
                      <div className="space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Link
                            href={`/app/pesanan/${ord.id}`}
                            className="font-mono-num text-sm font-extrabold text-[#C2410C] hover:underline"
                          >
                            #{ord.code}
                          </Link>
                          <OrderStatusBadge status={ord.status} />
                          <Badge variant="slate">
                            <Truck className="mr-1 h-3 w-3" />
                            {ord.delivery_type === "delivery"
                              ? "Diantar"
                              : "Ambil Sendiri"}
                          </Badge>
                        </div>
                        <p className="font-bold text-[#12100E]">
                          {ord.customer_name} · Produksi{" "}
                          {formatDateShort(ord.production_date)}
                        </p>
                        <p className="font-medium text-[#5C5046]">
                          {ord.items
                            .map((it) => `${it.product_name} × ${it.qty}`)
                            .join(", ")}
                        </p>
                      </div>
                      <Link href={`/app/pesanan/${ord.id}`}>
                        <Button variant="outline" size="sm">
                          Kelola
                        </Button>
                      </Link>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="min-w-0 lg:col-span-5">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <CreditCard className="h-4 w-4 text-[#C2410C]" />
                  Menunggu Pembayaran DP ({dpAlerts.length})
                </CardTitle>
                <CardDescription>
                  Reservasi kapasitas sementara sampai tenggat DP.
                </CardDescription>
              </div>
            </CardHeader>
            <CardContent>
              {dpAlerts.length === 0 ? (
                <p className="py-8 text-center text-xs text-[#5C5046]">
                  Tidak ada pesanan yang menunggu pembayaran DP.
                </p>
              ) : (
                <div className="space-y-2.5">
                  {dpAlerts.map((dp) => {
                    const shortfall = Math.max(
                      0,
                      toNumber(dp.dp_required) - toNumber(dp.paid)
                    );
                    return (
                      <div
                        key={dp.order_id}
                        className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border-2 border-[#12100E] bg-[#FEF9C3] p-3 text-xs"
                      >
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <Link
                              href={`/app/pesanan/${dp.order_id}`}
                              className="font-mono-num text-sm font-extrabold text-[#12100E] hover:underline"
                            >
                              #{dp.code}
                            </Link>
                            <span className="font-bold text-[#12100E]">
                              {dp.customer_name}
                            </span>
                          </div>
                          <p className="font-semibold text-[#3D352E]">
                            Kurang DP:{" "}
                            <strong className="text-[#BE123C]">
                              {formatRupiah(shortfall)}
                            </strong>
                          </p>
                          {dp.dp_deadline ? (
                            <p className="text-[11px] font-bold text-[#5C5046]">
                              Tenggat: {formatDateTimeWIB(dp.dp_deadline)}
                            </p>
                          ) : null}
                        </div>
                        <Link href={`/app/pesanan/${dp.order_id}`}>
                          <Button size="sm">Catat DP</Button>
                        </Link>
                      </div>
                    );
                  })}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* TAB 2: KEPUTUSAN OPERASIONAL & STOK MINIMUM */}
      {activeTab === "decisions_stock" && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
          <div className="min-w-0 space-y-3 lg:col-span-7">
            <div className="flex items-center justify-between">
              <h2 className="font-display text-lg font-extrabold text-[#12100E]">
                Antrian Keputusan Operasional ({queue.length})
              </h2>
              <Link
                href="/app/keputusan"
                className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
              >
                Semua Antrian
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </div>

            {queue.length === 0 ? (
              <Card>
                <CardContent className="py-8 text-center text-xs text-[#5C5046]">
                  Tidak ada pesanan yang menunggu keputusan saat ini.
                </CardContent>
              </Card>
            ) : (
              <div className="space-y-4">
                {queue.slice(0, 3).map((item) => (
                  <DecisionCard
                    key={item.id}
                    item={item}
                    onDecided={handleDecided}
                  />
                ))}
              </div>
            )}
          </div>

          <Card className="min-w-0 lg:col-span-5">
            <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Boxes className="h-4 w-4 text-[#C2410C]" />
                  Peringatan Stok Minimum ({lowStocks.length})
                </CardTitle>
                <CardDescription>
                  Kebutuhan belanja aktif: {shopList?.items.length ?? 0} bahan (
                  {formatRupiah(shopList?.total_estimated)})
                </CardDescription>
              </div>
              <Link
                href="/app/belanja"
                className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
              >
                Daftar Belanja
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </CardHeader>
            <CardContent className="space-y-2.5">
              {lowStocks.length === 0 ? (
                <div className="rounded-2xl border-2 border-[#12100E] bg-[#ECFCCB] p-4 text-center text-xs font-bold text-[#12100E]">
                  Seluruh stok bahan baku berada di atas batas minimum.
                </div>
              ) : (
                lowStocks.slice(0, 6).map((item) => (
                  <div
                    key={item.ingredient_id}
                    className="flex items-center justify-between gap-2 rounded-2xl border-2 border-[#12100E] bg-[#FFE4E6] px-3.5 py-2.5 text-xs"
                  >
                    <div>
                      <p className="font-extrabold text-[#12100E]">
                        {item.ingredient_name}
                      </p>
                      <p className="text-[11px] font-semibold text-[#5C5046]">
                        Stok: {formatNumber(item.stock)} {item.use_unit} (Min:{" "}
                        {formatNumber(item.min_stock)})
                      </p>
                    </div>
                    <Badge variant="rose">
                      -{formatNumber(item.deficit)} {item.use_unit}
                    </Badge>
                  </div>
                ))
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* TAB 3: KALENDER KAPASITAS BULANAN */}
      {activeTab === "calendar" && (
        <section aria-label="Kalender Kapasitas Bulanan" className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="font-display text-lg font-extrabold text-[#12100E]">
              Kalender Kapasitas Bulanan
            </h2>
            <Link
              href={
                selectedDate
                  ? `/app/kapasitas?date=${encodeURIComponent(selectedDate)}`
                  : "/app/kapasitas"
              }
              className="inline-flex items-center gap-1 text-xs font-extrabold text-[#C2410C] hover:underline"
            >
              Rincian Kapasitas Harian
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>

          <CapacityCalendar
            days={calendar}
            selectedDate={selectedDate}
            onSelectDate={(dt) => setSelectedDate(dt)}
          />

          {selectedCapDay ? (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] px-4 py-3 text-xs shadow-[4px_4px_0px_#12100E]">
              <div className="flex flex-wrap items-center gap-3">
                <span className="font-extrabold text-[#12100E]">
                  {formatDateLong(selectedCapDay.date)}
                </span>
                <span className="font-semibold text-[#5C5046]">
                  Utilisasi:{" "}
                  <strong>{formatPercent(selectedCapDay.utilization)}</strong>
                </span>
              </div>
              <Link
                href={`/app/kapasitas?date=${encodeURIComponent(selectedCapDay.date)}`}
                className="font-extrabold text-[#C2410C] hover:underline"
              >
                Lihat Beban Oven &amp; Tim →
              </Link>
            </div>
          ) : null}
        </section>
      )}

      {/* TAB 4: SIMULASI & KALIBRASI */}
      {activeTab === "simulation" && (
        <div className="space-y-6">
          {data.kpi?.calibration ? (
            <WorkloadRecalibrationCard
              calibration={data.kpi.calibration}
              onSuccess={() => void loadDashboard()}
              userRole="manager"
            />
          ) : null}
          <BusinessImpactSimulation />
        </div>
      )}
    </div>
  );
}
