"use client";

import { Flame } from "lucide-react";
import { formatPercent, parseDateKey, RESOURCE_LABELS } from "@/lib/format";
import type { CapacityCalendarDay, DayCapacityStatus } from "@/lib/types";

const DAY_HEADERS = ["Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"];

const STATUS_STYLE: Record<
  DayCapacityStatus,
  { bg: string; badge: string; label: string }
> = {
  green: {
    bg: "border-emerald-200 bg-emerald-50/70 hover:bg-emerald-100/70 text-emerald-950",
    badge: "bg-emerald-600 text-white",
    label: "Longgar",
  },
  yellow: {
    bg: "border-amber-200 bg-amber-50/80 hover:bg-amber-100/80 text-amber-950",
    badge: "bg-amber-600 text-white",
    label: "Padat",
  },
  red: {
    bg: "border-orange-300 bg-orange-50 hover:bg-orange-100 text-orange-950",
    badge: "bg-orange-600 text-white",
    label: "Kritis",
  },
  full: {
    bg: "border-rose-300 bg-rose-50 hover:bg-rose-100 text-rose-950",
    badge: "bg-rose-600 text-white",
    label: "Penuh",
  },
};

export function CapacityCalendar({
  days,
  selectedDate,
  onSelectDate,
}: {
  days: CapacityCalendarDay[];
  selectedDate: string | null;
  onSelectDate: (date: string) => void;
}) {
  if (days.length === 0) {
    return (
      <div className="rounded-2xl border border-stone-200 bg-white p-8 text-center text-sm text-stone-500">
        Belum ada data kalender kapasitas untuk bulan ini.
      </div>
    );
  }

  // Hitung offset hari pertama (Senin = 0 ... Minggu = 6)
  const firstDt = parseDateKey(days[0].date);
  const jsDay = firstDt ? firstDt.getUTCDay() : 1; // 0=Minggu..6=Sabtu
  const leadingBlanks = (jsDay + 6) % 7;

  return (
    <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
      {/* Legenda Warna */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3 border-b border-stone-100 pb-3">
        <p className="text-xs font-bold tracking-wider text-stone-500 uppercase">
          Kalender Utilisasi Kapasitas Harian
        </p>
        <div className="flex flex-wrap items-center gap-3 text-xs">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
            Hijau (&lt;70%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-amber-500" />
            Kuning (70–90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-orange-500" />
            Merah (&gt;90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-rose-600" />
            Penuh (≥100%)
          </span>
        </div>
      </div>

      {/* Header Hari */}
      <div className="grid grid-cols-7 gap-2 text-center text-xs font-bold text-stone-500">
        {DAY_HEADERS.map((h) => (
          <div key={h} className="py-1">
            {h}
          </div>
        ))}
      </div>

      {/* Grid Tanggal */}
      <div className="mt-2 grid grid-cols-7 gap-2">
        {Array.from({ length: leadingBlanks }).map((_, idx) => (
          <div
            key={`blank-${idx}`}
            className="min-h-24 rounded-xl border border-dashed border-stone-100 bg-stone-50/30"
          />
        ))}

        {days.map((d) => {
          const dayNum = Number.parseInt(d.date.slice(8, 10), 10);
          const st = STATUS_STYLE[d.status] ?? STATUS_STYLE.green;
          const isSelected = selectedDate === d.date;

          return (
            <button
              key={d.date}
              type="button"
              onClick={() => onSelectDate(d.date)}
              className={`flex min-h-24 flex-col justify-between rounded-xl border p-2.5 text-left transition ${
                st.bg
              } ${
                isSelected
                  ? "ring-2 ring-stone-900 ring-offset-1"
                  : ""
              }`}
            >
              <div className="flex w-full items-center justify-between gap-1">
                <span className="text-sm font-extrabold">{dayNum}</span>
                <span
                  className={`rounded-full px-1.5 py-0.5 text-[10px] font-bold ${st.badge}`}
                >
                  {formatPercent(d.utilization)}
                </span>
              </div>

              <div className="mt-2 space-y-0.5">
                <p className="text-[11px] font-semibold">{st.label}</p>
                {d.bottleneck && (
                  <p className="flex items-center gap-1 truncate text-[10px] opacity-80">
                    <Flame className="h-3 w-3 shrink-0" />
                    <span className="truncate">
                      {RESOURCE_LABELS[d.bottleneck] ?? d.bottleneck}
                    </span>
                  </p>
                )}
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

