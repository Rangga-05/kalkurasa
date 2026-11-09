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
    <div className="w-full min-w-0 overflow-hidden rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[5px_5px_0px_#12100E] sm:p-5">
      {/* Legenda Warna */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-2 border-b-2 border-[#12100E] pb-3">
        <p className="text-xs font-extrabold tracking-wider text-[#12100E] uppercase">
          Kalender Utilisasi Kapasitas Oven &amp; Kru Harian
        </p>
        <div className="flex flex-wrap items-center gap-3 text-xs font-bold text-[#3D352E]">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-[#12100E] bg-[#D9F99D]" />
            Longgar (&lt;70%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-[#12100E] bg-[#FDE047]" />
            Padat (70%-90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-[#12100E] bg-[#C2410C]" />
            Kritis (&gt;90%)
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full border border-[#12100E] bg-[#BE123C]" />
            Penuh (≥100%)
          </span>
        </div>
      </div>

      {/* Scrollable wrapper dengan lebar minimum yang proporsional agar sel tanggal tidak pernah terhimpit */}
      <div className="w-full overflow-x-auto pb-1">
        <div className="min-w-[540px]">
          {/* Header Hari */}
          <div className="grid grid-cols-7 gap-2 text-center text-xs font-extrabold text-[#12100E] uppercase">
            {DAY_HEADERS.map((h) => (
              <div
                key={h}
                className="rounded-lg bg-[#F7EFE2] py-1.5 text-[11px]"
              >
                {h}
              </div>
            ))}
          </div>

          {/* Grid Tanggal */}
          <div className="mt-2 grid grid-cols-7 gap-2">
            {Array.from({ length: leadingBlanks }).map((_, idx) => (
              <div
                key={`blank-${idx}`}
                className="min-h-24 rounded-2xl border-2 border-dashed border-[#12100E]/25 bg-[#F7EFE2]/40"
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

              return (
                <button
                  key={d.date}
                  type="button"
                  onClick={() => onSelectDate(d.date)}
                  aria-pressed={isSelected}
                  className={`flex min-h-24 min-w-0 flex-col justify-between overflow-hidden rounded-2xl border-2 p-2 text-left transition-all ${
                    st.bg
                  } ${
                    isSelected
                      ? "-translate-y-0.5 shadow-[4px_4px_0px_#C2410C] ring-2 ring-[#C2410C]"
                      : "shadow-[2px_2px_0px_#12100E] hover:-translate-y-0.5 hover:shadow-[4px_4px_0px_#12100E]"
                  }`}
                >
                  <div className="flex w-full min-w-0 flex-wrap items-center justify-between gap-1">
                    <span className="font-mono-num text-sm leading-none font-extrabold">
                      {dayNum}
                    </span>
                    <span
                      className={`font-mono-num shrink-0 rounded-full px-1.5 py-0.5 text-[10px] leading-none font-extrabold ${st.badge}`}
                    >
                      {formatPercent(d.utilization)}
                    </span>
                  </div>

                  <div className="mt-2 w-full min-w-0 space-y-1">
                    <div className="h-2 w-full overflow-hidden rounded-full border border-[#12100E] bg-[#FFFDF9]">
                      <div
                        className={`h-full ${st.bar}`}
                        style={{ width: `${utilPct}%` }}
                      />
                    </div>
                    <p className="truncate text-[11px] font-extrabold">
                      {st.label}
                    </p>
                    {d.bottleneck ? (
                      <p className="flex min-w-0 items-center gap-1 truncate text-[10px] font-semibold opacity-85">
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
    </div>
  );
}
