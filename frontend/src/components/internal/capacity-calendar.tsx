"use client";

import { Flame } from "lucide-react";
import {
  formatPercent,
  parseDateKey,
  RESOURCE_LABELS,
  toNumber,
} from "@/lib/format";
import type { CapacityCalendarDay, DayCapacityStatus } from "@/lib/types";

const DAY_HEADERS = ["Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"];

const STATUS_STYLE: Record<
  DayCapacityStatus,
  { bg: string; badge: string; bar: string; label: string }
> = {
  green: {
    bg: "border-[#12100E] bg-[#ECFCCB] hover:bg-[#D9F99D] text-[#12100E]",
    badge: "bg-[#15803D] text-white",
    bar: "bg-[#15803D]",
    label: "Longgar",
  },
  yellow: {
    bg: "border-[#12100E] bg-[#FEF9C3] hover:bg-[#FEF08A] text-[#12100E]",
    badge: "bg-[#B45309] text-white",
    bar: "bg-[#D97706]",
    label: "Padat",
  },
  red: {
    bg: "border-[#12100E] bg-[#FFEDD5] hover:bg-[#FED7AA] text-[#12100E]",
    badge: "bg-[#C2410C] text-white",
    bar: "bg-[#C2410C]",
    label: "Kritis",
  },
  full: {
    bg: "border-[#12100E] bg-[#FFE4E6] hover:bg-[#FECDD3] text-[#12100E]",
    badge: "bg-[#BE123C] text-white",
    bar: "bg-[#BE123C]",
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
      <div className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-8 text-center text-sm font-semibold text-[#5C5046] shadow-[5px_5px_0px_#12100E]">
        Belum ada data kalender kapasitas untuk bulan ini.
      </div>
    );
  }

  const firstDt = parseDateKey(days[0].date);
  const jsDay = firstDt ? firstDt.getUTCDay() : 1;
  const leadingBlanks = (jsDay + 6) % 7;

  return (
    <div className="w-full min-w-0 overflow-hidden rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-3 shadow-[5px_5px_0px_#12100E] sm:p-5">
      {/* Legenda Warna */}
      <div className="mb-3 flex flex-col gap-2 border-b-2 border-[#12100E] pb-3 sm:mb-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <p className="text-[11px] font-extrabold tracking-wider text-[#12100E] uppercase sm:text-xs">
          Kalender Utilisasi Kapasitas Oven &amp; Kru Harian
        </p>
        <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] font-bold text-[#3D352E] sm:flex sm:flex-wrap sm:items-center sm:gap-3 sm:text-xs">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full border border-[#12100E] bg-[#D9F99D]" />
            Longgar (&lt;70%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full border border-[#12100E] bg-[#FDE047]" />
            Padat (70%-90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full border border-[#12100E] bg-[#C2410C]" />
            Kritis (&gt;90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full border border-[#12100E] bg-[#BE123C]" />
            Penuh (≥100%)
          </span>
        </div>
      </div>

      {/* Grid 7 Kolom Responsif (Muat Penuh di Mobile Tanpa Scroll Horizontal) */}
      <div className="w-full">
        {/* Header Hari */}
        <div className="grid grid-cols-7 gap-1 text-center font-extrabold text-[#12100E] uppercase sm:gap-2">
          {DAY_HEADERS.map((h) => (
            <div
              key={h}
              className="rounded-md bg-[#F7EFE2] py-1 text-[10px] sm:rounded-lg sm:py-1.5 sm:text-[11px]"
            >
              {h}
            </div>
          ))}
        </div>

        {/* Grid Tanggal */}
        <div className="mt-1.5 grid grid-cols-7 gap-1 sm:mt-2 sm:gap-2">
          {Array.from({ length: leadingBlanks }).map((_, idx) => (
            <div
              key={`blank-${idx}`}
              className="min-h-15 rounded-xl border border-dashed border-[#12100E]/25 bg-[#F7EFE2]/40 sm:min-h-24 sm:rounded-2xl sm:border-2"
            />
          ))}

          {days.map((d) => {
            const dayNum = Number.parseInt(d.date.slice(8, 10), 10);
            const st = STATUS_STYLE[d.status] ?? STATUS_STYLE.green;
            const isSelected = selectedDate === d.date;
            const utilPct = Math.min(
              100,
              Math.max(0, toNumber(d.utilization) * 100)
            );
            const roundedPct = Math.round(toNumber(d.utilization) * 100);

            return (
              <button
                key={d.date}
                type="button"
                onClick={() => onSelectDate(d.date)}
                aria-pressed={isSelected}
                className={`flex min-h-15 min-w-0 flex-col justify-between overflow-hidden rounded-xl border p-1 text-left transition-all sm:min-h-24 sm:rounded-2xl sm:border-2 sm:p-2 ${
                  st.bg
                } ${
                  isSelected
                    ? "-translate-y-0.5 shadow-[2px_2px_0px_#C2410C] ring-2 ring-[#C2410C] sm:shadow-[4px_4px_0px_#C2410C]"
                    : "shadow-[1px_1px_0px_#12100E] hover:-translate-y-0.5 sm:shadow-[2px_2px_0px_#12100E] sm:hover:shadow-[4px_4px_0px_#12100E]"
                }`}
              >
                <div className="flex w-full min-w-0 flex-col items-start gap-0.5 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-1">
                  <span className="font-mono-num text-[11px] leading-none font-extrabold sm:text-sm">
                    {dayNum}
                  </span>
                  <span
                    className={`font-mono-num shrink-0 rounded px-1 py-0.5 text-[9px] leading-none font-extrabold sm:rounded-full sm:px-1.5 sm:text-[10px] ${st.badge}`}
                  >
                    <span className="sm:hidden">{roundedPct}%</span>
                    <span className="hidden sm:inline">
                      {formatPercent(d.utilization)}
                    </span>
                  </span>
                </div>

                <div className="mt-1 w-full min-w-0 space-y-1 sm:mt-2">
                  <div className="h-1.5 w-full overflow-hidden rounded-full border border-[#12100E] bg-[#FFFDF9] sm:h-2">
                    <div
                      className={`h-full ${st.bar}`}
                      style={{ width: `${utilPct}%` }}
                    />
                  </div>
                  <p className="hidden truncate text-[11px] font-extrabold sm:block">
                    {st.label}
                  </p>
                  {d.bottleneck ? (
                    <p className="hidden min-w-0 items-center gap-1 truncate text-[10px] font-semibold opacity-85 sm:flex">
                      <Flame className="h-3 w-3 shrink-0" aria-hidden="true" />
                      <span className="truncate">
                        {RESOURCE_LABELS[d.bottleneck] ?? d.bottleneck}
                      </span>
                    </p>
                  ) : null}
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
