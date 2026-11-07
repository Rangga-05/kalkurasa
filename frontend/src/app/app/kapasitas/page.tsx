"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Flame,
  Gauge,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { CapacityCalendar } from "@/components/internal/capacity-calendar";
import { OrderStatusBadge } from "@/components/internal/order-timeline";
import {
  getCapacityCalendar,
  getCapacityDay,
  getErrorMessage,
} from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  formatNumber,
  formatPercent,
  formatRupiah,
  RESOURCE_LABELS,
  toDateKeyWIB,
  toNumber,
} from "@/lib/format";
import type { CapacityCalendarDay, CapacityDayDetail } from "@/lib/types";

function shiftMonth(monthKey: string, delta: number): string {
  const [yStr, mStr] = monthKey.split("-");
  const y = Number.parseInt(yStr, 10) || 2026;
  const m = Number.parseInt(mStr, 10) || 1;
  const dt = new Date(Date.UTC(y, m - 1 + delta, 1));
  const ny = dt.getUTCFullYear();
  const nm = String(dt.getUTCMonth() + 1).padStart(2, "0");
  return `${ny}-${nm}`;
}

export default function CapacityPage() {
  const [month, setMonth] = useState(() =>
    toDateKeyWIB(new Date()).slice(0, 7)
  );
  const [days, setDays] = useState<CapacityCalendarDay[]>([]);
  const [loadingCal, setLoadingCal] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [dayDetail, setDayDetail] = useState<CapacityDayDetail | null>(null);
  const [loadingDay, setLoadingDay] = useState(false);

  const loadCalendar = useCallback(async (targetMonth: string) => {
    setLoadingCal(true);
    setErrorMsg(null);
    try {
      const data = await getCapacityCalendar(targetMonth);
      setDays(data);
      if (data.length > 0) {
        const todayKey = toDateKeyWIB(new Date());
        const matchToday = data.find((d) => d.date === todayKey);
        const busiest = [...data].sort(
          (a, b) => toNumber(b.utilization) - toNumber(a.utilization)
        )[0];
        const initialDate =
          matchToday?.date ??
          (toNumber(busiest?.utilization) > 0 ? busiest.date : data[0].date);
        setSelectedDate(initialDate);
      }
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat kalender kapasitas."));
    } finally {
      setLoadingCal(false);
    }
  }, []);

  useEffect(() => {
    void loadCalendar(month);
  }, [month, loadCalendar]);

  const loadDayDetail = useCallback(async (dateKey: string) => {
    setLoadingDay(true);
    try {
      const detail = await getCapacityDay(dateKey);
      setDayDetail(detail);
    } catch (err) {
      setErrorMsg(
        getErrorMessage(err, `Gagal memuat rincian kapasitas ${dateKey}.`)
      );
    } finally {
      setLoadingDay(false);
    }
  }, []);

  useEffect(() => {
    if (selectedDate) {
      void loadDayDetail(selectedDate);
    }
  }, [selectedDate, loadDayDetail]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
            <CalendarDays className="h-3.5 w-3.5" />
            Perencanaan Kapasitas Produksi (G1)
          </span>
          <h1 className="mt-2 text-2xl font-bold text-stone-900">
            Kalender Kapasitas & Bottleneck
          </h1>
          <p className="mt-1 text-sm text-stone-600">
            Klik salah satu tanggal untuk melihat rincian beban per sumber daya
            (oven & tenaga kerja) serta daftar pesanan yang mereservasi slot.
          </p>
        </div>

        {/* Pemilih Bulan */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setMonth((m) => shiftMonth(m, -1))}
            className="rounded-xl border border-stone-200 bg-white p-2 text-stone-700 hover:bg-stone-50"
            title="Bulan Sebelumnya"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <input
            type="month"
            value={month}
            onChange={(e) => {
              if (e.target.value) setMonth(e.target.value);
            }}
            className="rounded-xl border border-stone-200 bg-white px-3 py-1.5 text-xs font-bold text-stone-900"
          />
          <button
            type="button"
            onClick={() => setMonth((m) => shiftMonth(m, 1))}
            className="rounded-xl border border-stone-200 bg-white p-2 text-stone-700 hover:bg-stone-50"
            title="Bulan Berikutnya"
          >
            <ChevronRight className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={() => void loadCalendar(month)}
            disabled={loadingCal}
            className="inline-flex items-center gap-1.5 rounded-xl border border-stone-200 bg-white px-3 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${loadingCal ? "animate-spin" : ""}`}
            />
            Segarkan
          </button>
        </div>
      </div>

      {errorMsg && (
        <div className="flex items-center gap-2 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <AlertTriangle className="h-4 w-4 shrink-0 text-rose-600" />
          <span>{errorMsg}</span>
        </div>
      )}

      {loadingCal ? (
        <div className="flex items-center justify-center rounded-2xl border border-stone-200 bg-white py-16 text-sm text-stone-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin text-amber-600" />
          Memuat kalender kapasitas bulanan...
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
          {/* Kalender Bulanan */}
          <div className="xl:col-span-2">
            <CapacityCalendar
              days={days}
              selectedDate={selectedDate}
              onSelectDate={(d) => setSelectedDate(d)}
            />
          </div>

          {/* Panel Detail Hari yang Dipilih */}
          <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
            <div className="flex items-center justify-between border-b border-stone-100 pb-3">
              <div>
                <p className="text-xs font-bold tracking-wider text-stone-400 uppercase">
                  Rincian Kapasitas Harian
                </p>
                <h2 className="mt-0.5 text-base font-bold text-stone-900">
                  {selectedDate
                    ? formatDateLong(selectedDate)
                    : "Pilih Tanggal"}
                </h2>
              </div>
              <Gauge className="h-5 w-5 text-amber-600" />
            </div>

            {loadingDay ? (
              <div className="flex items-center justify-center py-12 text-xs text-stone-500">
                <Loader2 className="mr-2 h-4 w-4 animate-spin text-amber-600" />
                Memuat rincian beban sumber daya...
              </div>
            ) : !dayDetail ? (
              <p className="py-8 text-center text-xs text-stone-500">
                Klik salah satu tanggal pada kalender untuk melihat rincian
                beban produksi.
              </p>
            ) : (
              <div className="mt-4 space-y-5">
                {/* Ringkasan Utilisasi & Bottleneck */}
                <div className="rounded-xl border border-stone-200 bg-stone-50/70 p-3.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-stone-600">
                      Utilisasi Puncak Hari Ini
                    </span>
                    <span className="text-base font-extrabold text-stone-900">
                      {formatPercent(dayDetail.utilization)}
                    </span>
                  </div>
                  {dayDetail.bottleneck && (
                    <div className="mt-2 flex items-center gap-1.5 text-xs text-amber-800">
                      <Flame className="h-3.5 w-3.5 shrink-0 text-amber-600" />
                      <span>
                        Sumber daya pembatas (Bottleneck):{" "}
                        <strong>
                          {RESOURCE_LABELS[dayDetail.bottleneck] ??
                            dayDetail.bottleneck}
                        </strong>
                      </span>
                    </div>
                  )}
                </div>

                {/* Beban per Sumber Daya */}
                <div>
                  <h3 className="text-xs font-bold tracking-wider text-stone-500 uppercase">
                    Beban per Sumber Daya ({dayDetail.resources.length})
                  </h3>
                  <div className="mt-2.5 space-y-3">
                    {dayDetail.resources.map((res) => {
                      const utilPct = Math.min(
                        100,
                        Math.round(toNumber(res.utilization) * 100)
                      );
                      const isBottleneck =
                        dayDetail.bottleneck === res.code &&
                        toNumber(res.utilization) > 0;
                      return (
                        <div
                          key={res.code}
                          className="rounded-xl border border-stone-200 p-3 text-xs"
                        >
                          <div className="flex items-center justify-between">
                            <span className="font-bold text-stone-900">
                              {res.name}{" "}
                              <span className="font-normal text-stone-400">
                                ({res.code})
                              </span>
                            </span>
                            <span className="font-bold text-stone-900">
                              {formatPercent(res.utilization)}
                            </span>
                          </div>
                          <div className="mt-1.5 h-2 w-full overflow-hidden rounded-full bg-stone-100">
                            <div
                              className={`h-full rounded-full transition-all ${
                                toNumber(res.utilization) > 0.9
                                  ? "bg-rose-600"
                                  : toNumber(res.utilization) >= 0.7
                                    ? "bg-amber-500"
                                    : "bg-emerald-500"
                              }`}
                              style={{ width: `${utilPct}%` }}
                            />
                          </div>
                          <div className="mt-1.5 flex items-center justify-between text-[11px] text-stone-500">
                            <span>
                              Terpakai: <strong>{formatNumber(res.load)}</strong>{" "}
                              / {formatNumber(res.capacity)} {res.unit}
                            </span>
                            {isBottleneck && (
                              <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-bold text-amber-800">
                                Bottleneck
                              </span>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Daftar Pesanan yang Mereservasi Kapasitas */}
                <div>
                  <h3 className="text-xs font-bold tracking-wider text-stone-500 uppercase">
                    Pesanan pada Tanggal Produksi Ini (
                    {dayDetail.orders.length})
                  </h3>
                  {dayDetail.orders.length === 0 ? (
                    <p className="mt-2 rounded-xl border border-dashed border-stone-200 p-4 text-center text-xs text-stone-500">
                      Belum ada pesanan yang mereservasi kapasitas pada tanggal
                      ini.
                    </p>
                  ) : (
                    <div className="mt-2.5 space-y-2">
                      {dayDetail.orders.map((ord) => (
                        <div
                          key={ord.id}
                          className="flex items-center justify-between rounded-xl border border-stone-200 bg-stone-50/60 p-3 text-xs"
                        >
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="font-mono font-bold text-stone-900">
                                #{ord.code}
                              </span>
                              <OrderStatusBadge status={ord.status} />
                            </div>
                            <p className="mt-1 font-semibold text-stone-800">
                              {ord.customer_name}
                            </p>
                            <p className="text-[11px] text-stone-500">
                              Acara: {formatDateShort(ord.event_date)} •{" "}
                              {formatRupiah(ord.total)}
                            </p>
                          </div>
                          <Link
                            href={`/app/pesanan/${ord.id}`}
                            className="inline-flex items-center gap-1 rounded-lg border border-stone-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-amber-700 hover:bg-amber-50"
                          >
                            Buka
                            <ExternalLink className="h-3 w-3" />
                          </Link>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

