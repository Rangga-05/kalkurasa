"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRight,
  BarChart3,
  Boxes,
  CalendarRange,
  CheckCircle2,
  Clock,
  CreditCard,
  Plus,
  RefreshCw,
  Scale,
  ShoppingCart,
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

export default function ManagerDashboardPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedDate, setSelectedDate] = useState<string | null>(null);

  const dashboardQuery = useQuery({
    queryKey: ["manager-dashboard"],
    queryFn: getManagerDashboard,
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
        <div>
          <h1 className="text-2xl font-bold text-stone-900">
            Dashboard Manajer Operasional
          </h1>
          <p className="text-sm text-stone-500">
            Memuat kalender kapasitas, KPI operasional, jadwal produksi, stok,
            dan pantauan DP...
          </p>
        </div>
        <TableSkeleton rows={6} />
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
      {/* Header Dashboard */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="amber">Manajer Operasional</Badge>
            <span className="text-xs font-medium text-stone-500">
              Tanggal Operasional: {formatDateLong(data.today)}
            </span>
          </div>
          <h1 className="mt-1.5 text-2xl font-bold tracking-tight text-stone-900">
            Dashboard Operasional Dapur &amp; Kapasitas
          </h1>
          <p className="mt-0.5 text-sm text-stone-600">
            Kendalikan beban kapasitas bulanan, ketepatan jadwal produksi,
            peringatan stok bahan, dan tenggat pembayaran DP.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Link href="/app/pesanan">
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              Catat Pesanan Manual
            </Button>
          </Link>
          <Link href="/app/belanja">
            <Button variant="outline" size="sm">
              <ShoppingCart className="h-3.5 w-3.5" />
              Daftar Belanja ({shopList?.items.length ?? 0})
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
      {data.kpi?.calibration && (
        <WorkloadRecalibrationCard
          calibration={data.kpi.calibration}
          onSuccess={() => void loadDashboard()}
          userRole="manager"
        />
      )}

      {/* ELEMEN 1: Ringkasan KPI (Ketepatan Waktu & Utilisasi) */}
      <section aria-label="Ringkasan KPI Operasional" className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
            <BarChart3 className="h-4 w-4 text-amber-700" />
            1. Ringkasan KPI Operasional (Ketepatan Waktu &amp; Utilisasi)
          </h2>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>Ketepatan Waktu Selesai</span>
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

          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>Utilisasi Rata-rata Dapur</span>
              <CalendarRange className="h-4 w-4 text-amber-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {formatPercent(kpi.avg_utilization)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.active_production_days} hari produksi · Puncak{" "}
              {formatPercent(kpi.peak_utilization)}
              {kpi.peak_utilization_date
                ? ` (${formatDateShort(kpi.peak_utilization_date)})`
                : ""}
            </p>
          </div>

          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>Tingkat Pembatalan / DP Lewat</span>
              <AlertTriangle className="h-4 w-4 text-rose-600" />
            </div>
            <p className="mt-2 text-2xl font-bold text-stone-900">
              {formatPercent(kpi.cancellation_rate)}
            </p>
            <p className="mt-1 text-xs text-stone-500">
              {kpi.cancelled_or_expired_orders} batal/kedaluwarsa dari{" "}
              {kpi.accepted_orders_total} pesanan diterima
            </p>
          </div>

          <div className="rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
            <div className="flex items-center justify-between text-xs font-semibold text-stone-500">
              <span>Status Penyangga Kas (G3)</span>
              <CheckCircle2
                className={`h-4 w-4 ${
                  cashStatus.passed ? "text-emerald-600" : "text-rose-600"
                }`}
              />
            </div>
            <div className="mt-2 flex items-center gap-2">
              <Badge variant={cashStatus.passed ? "emerald" : "rose"}>
                {cashStatus.passed ? "Lolos Buffer" : "Perlu Perhatian DP"}
              </Badge>
            </div>
            <p className="mt-1.5 text-xs text-stone-500">
              {cashStatus.days_below_buffer.length} hari rawan dalam 14 hari ke
              depan
            </p>
          </div>
        </div>
      </section>

      {/* ELEMEN 2: Kalender Kapasitas Bulanan */}
      <section aria-label="Kalender Kapasitas Bulanan" className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
              <CalendarRange className="h-4 w-4 text-amber-700" />
              2. Kalender Kapasitas Bulanan
            </h2>
            <p className="text-xs text-stone-500">
              Klik tanggal untuk meninjau status utilisasi harian atau buka
              halaman kapasitas penuh.
            </p>
          </div>
          <Link
            href={
              selectedDate
                ? `/app/kapasitas?date=${encodeURIComponent(selectedDate)}`
                : "/app/kapasitas"
            }
            className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
          >
            Buka Rincian Kapasitas Harian
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </div>

        <CapacityCalendar
          days={calendar}
          selectedDate={selectedDate}
          onSelectDate={(dt) => setSelectedDate(dt)}
        />

        {selectedCapDay ? (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-stone-200 bg-white px-4 py-3 text-xs">
            <div className="flex flex-wrap items-center gap-3">
              <span className="font-bold text-stone-900">
                Tanggal Terpilih: {formatDateLong(selectedCapDay.date)}
              </span>
              <span className="text-stone-600">
                Utilisasi:{" "}
                <strong>{formatPercent(selectedCapDay.utilization)}</strong>
              </span>
              {selectedCapDay.bottleneck ? (
                <Badge variant="amber">
                  Penghambat: {selectedCapDay.bottleneck}
                </Badge>
              ) : null}
            </div>
            <Link
              href={`/app/kapasitas?date=${encodeURIComponent(selectedCapDay.date)}`}
              className="font-semibold text-amber-800 hover:underline"
            >
              Lihat Beban Oven &amp; Tenaga Kerja →
            </Link>
          </div>
        ) : null}
      </section>

      {/* ELEMEN 3 & 4: Jadwal Produksi/Pengantaran & Peringatan Stok */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
        {/* ELEMEN 3: Jadwal Produksi & Pengantaran */}
        <Card className="lg:col-span-7">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle className="flex items-center gap-2">
                <UtensilsCrossed className="h-4 w-4 text-amber-700" />
                3. Jadwal Produksi &amp; Pengantaran ({prodSchedule.length})
              </CardTitle>
              <CardDescription>
                Pesanan terkonfirmasi (DP lunas), sedang diproduksi, atau siap
                diantar/diambil pada horizon 14 hari.
              </CardDescription>
            </div>
            <Link
              href="/app/karyawan"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Papan Tugas Dapur
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </CardHeader>
          <CardContent>
            {prodSchedule.length === 0 ? (
              <p className="py-8 text-center text-xs text-stone-500">
                Belum ada jadwal produksi aktif pada rentang hari ini.
              </p>
            ) : (
              <div className="space-y-2.5">
                {prodSchedule.slice(0, 6).map((ord) => (
                  <div
                    key={ord.id}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-stone-200 bg-stone-50/60 p-3 text-xs"
                  >
                    <div className="space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <Link
                          href={`/app/pesanan/${ord.id}`}
                          className="font-mono text-sm font-bold text-amber-800 hover:underline"
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
                      <p className="font-semibold text-stone-800">
                        {ord.customer_name} · Produksi:{" "}
                        {formatDateShort(ord.production_date)} · Acara:{" "}
                        {formatDateShort(ord.event_date)}
                      </p>
                      <p className="text-stone-600">
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

        {/* ELEMEN 4: Peringatan Stok & Ringkasan Belanja */}
        <Card className="lg:col-span-5">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Boxes className="h-4 w-4 text-amber-700" />
                4. Peringatan Stok Minimum ({lowStocks.length})
              </CardTitle>
              <CardDescription>
                Bahan baku yang berada di bawah ambang stok minimum dan daftar
                belanja aktif.
              </CardDescription>
            </div>
            <Link
              href="/app/belanja"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Catat Belanja
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex items-center justify-between rounded-xl border border-amber-200 bg-amber-50/70 px-3.5 py-2.5 text-xs text-amber-950">
              <span>
                Kebutuhan Belanja Pesanan Aktif:{" "}
                <strong>{shopList?.items.length ?? 0} bahan</strong>
              </span>
              <span className="font-bold">
                Est: {formatRupiah(shopList?.total_estimated)}
              </span>
            </div>

            {lowStocks.length === 0 ? (
              <div className="rounded-xl border border-emerald-200 bg-emerald-50/60 p-4 text-center text-xs text-emerald-800">
                Seluruh stok bahan baku berada di atas batas minimum.
              </div>
            ) : (
              <div className="space-y-2">
                {lowStocks.slice(0, 6).map((item) => (
                  <div
                    key={item.ingredient_id}
                    className="flex items-center justify-between gap-2 rounded-xl border border-rose-200 bg-rose-50/50 px-3 py-2 text-xs"
                  >
                    <div>
                      <p className="font-semibold text-stone-900">
                        {item.ingredient_name}
                      </p>
                      <p className="text-[11px] text-stone-600">
                        Stok saat ini:{" "}
                        <strong className="text-rose-700">
                          {formatNumber(item.stock)} {item.use_unit}
                        </strong>{" "}
                        (Min: {formatNumber(item.min_stock)} {item.use_unit})
                      </p>
                    </div>
                    <Badge variant="rose">
                      Kurang {formatNumber(item.deficit)} {item.use_unit}
                    </Badge>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* ELEMEN 5: Pesanan yang Belum di-DP & Antrian Keputusan */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <Card className="lg:col-span-6">
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
            <div>
              <CardTitle className="flex items-center gap-2">
                <CreditCard className="h-4 w-4 text-amber-700" />
                5. Pesanan Menunggu Pembayaran DP ({dpAlerts.length})
              </CardTitle>
              <CardDescription>
                Pesanan berstatus Disetujui yang mereservasi kapasitas sementara
                sampai tenggat DP.
              </CardDescription>
            </div>
            <Link
              href="/app/pesanan?status=accepted"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Semua Menunggu DP
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </CardHeader>
          <CardContent>
            {dpAlerts.length === 0 ? (
              <p className="py-8 text-center text-xs text-stone-500">
                Tidak ada pesanan yang sedang menunggu pembayaran DP.
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
                      className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-sky-200 bg-sky-50/50 p-3 text-xs"
                    >
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <Link
                            href={`/app/pesanan/${dp.order_id}`}
                            className="font-mono text-sm font-bold text-amber-800 hover:underline"
                          >
                            #{dp.code}
                          </Link>
                          <span className="font-semibold text-stone-800">
                            {dp.customer_name}
                          </span>
                          <Badge variant="sky">
                            Acara: {formatDateShort(dp.event_date)}
                          </Badge>
                        </div>
                        <p className="text-stone-700">
                          Wajib DP:{" "}
                          <strong>{formatRupiah(dp.dp_required)}</strong> ·
                          Sudah Masuk: {formatRupiah(dp.paid)} · Kurang:{" "}
                          <strong className="text-rose-700">
                            {formatRupiah(shortfall)}
                          </strong>
                        </p>
                        {dp.dp_deadline ? (
                          <p className="text-[11px] font-medium text-amber-900">
                            Tenggat DP: {formatDateTimeWIB(dp.dp_deadline)}
                          </p>
                        ) : null}
                      </div>
                      <Link href={`/app/pesanan/${dp.order_id}`}>
                        <Button size="sm">Catat Pembayaran</Button>
                      </Link>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Antrian Keputusan Manajer (Pesanan di Bawah Ambang) */}
        <div className="space-y-3 lg:col-span-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
                <Scale className="h-4 w-4 text-amber-700" />
                Antrian Keputusan Operasional ({queue.length})
              </h2>
              <p className="text-xs text-stone-500">
                Manajer dapat memutuskan pesanan di bawah ambang pesanan besar.
              </p>
            </div>
            <Link
              href="/app/keputusan"
              className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
            >
              Semua Antrian
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          </div>

          {queue.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-xs text-stone-500">
                Tidak ada pesanan yang menunggu keputusan saat ini.
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {queue.slice(0, 2).map((item) => (
                <DecisionCard
                  key={item.id}
                  item={item}
                  onDecided={handleDecided}
                />
              ))}
            </div>
          )}
        </div>
      </div>

      {/* ELEMEN 5: Simulasi Dampak Bisnis 30 Hari */}
      <section aria-label="Simulasi Dampak Bisnis 30 Hari">
        <BusinessImpactSimulation />
      </section>
    </div>
  );
}
