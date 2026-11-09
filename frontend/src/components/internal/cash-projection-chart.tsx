"use client";

import {
  Area,
  Bar,
  CartesianGrid,
  ComposedChart,
  ReferenceDot,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { formatDateShort, formatRupiah, toNumber } from "@/lib/format";
import type { CashProjectionDayDTO, DecimalValue } from "@/lib/types";

interface CashProjectionChartProps {
  days: CashProjectionDayDTO[];
  cashBuffer: DecimalValue;
  lowestDate: string;
  lowestAmount: DecimalValue;
}

interface ChartPoint {
  date: string;
  label: string;
  closing: number;
  inflow: number;
  outflow: number;
  fixedCost: number;
  totalOut: number;
  belowBuffer: boolean;
}

export function CashProjectionChart({
  days,
  cashBuffer,
  lowestDate,
  lowestAmount,
}: CashProjectionChartProps) {
  const bufferNum = toNumber(cashBuffer);
  const lowestNum = toNumber(lowestAmount);

  const data: ChartPoint[] = days.map((d) => {
    const outflow = toNumber(d.outflow);
    const fixedCost = toNumber(d.fixed_cost);
    return {
      date: d.date,
      label: formatDateShort(d.date),
      closing: toNumber(d.closing),
      inflow: toNumber(d.inflow),
      outflow,
      fixedCost,
      totalOut: outflow + fixedCost,
      belowBuffer: d.below_buffer,
    };
  });

  const lowestPoint = data.find((d) => d.date === lowestDate);

  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-xl border border-dashed border-stone-200 bg-stone-50 text-sm text-stone-500">
        Tidak ada data proyeksi kas untuk ditampilkan.
      </div>
    );
  }

  return (
    <div className="w-full min-w-0 space-y-3">
      {/* Legenda & Ringkasan Responsif (Mencegah Teks SVG Terpotong di Layar Mobile) */}
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-2xl border border-[#12100E]/15 bg-[#F7EFE2]/70 px-3 py-2 text-[11px] font-bold text-[#3D352E]">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#b45309]" />
            <span>Saldo Akhir</span>
          </span>
          <span className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-xs bg-[#10b981]/60" />
            <span>Kas Masuk</span>
          </span>
          <span className="inline-flex items-center gap-1.5 text-[#be123c]">
            <span className="w-3.5 shrink-0 border-t-2 border-dashed border-[#e11d48]" />
            <span>Buffer: {formatRupiah(bufferNum)}</span>
          </span>
        </div>

        {lowestPoint ? (
          <div className="inline-flex items-center gap-1.5 rounded-full border border-[#12100E]/20 bg-[#FFFDF9] px-2.5 py-0.5 text-[11px] font-extrabold text-[#12100E]">
            <span
              className={`h-2 w-2 shrink-0 rounded-full ${
                lowestPoint.belowBuffer ? "bg-[#e11d48]" : "bg-[#0284c7]"
              }`}
            />
            <span>
              Terendah ({lowestPoint.label}):{" "}
              <strong
                className={
                  lowestPoint.belowBuffer ? "text-[#be123c]" : "text-[#0369a1]"
                }
              >
                {formatRupiah(lowestNum)}
              </strong>
            </span>
          </div>
        ) : null}
      </div>

      {/* Area Grafik Kurva Responsif */}
      <div className="h-56 w-full min-w-0 sm:h-76">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart
            data={data}
            margin={{ top: 10, right: 8, left: -6, bottom: 2 }}
          >
            <defs>
              <linearGradient id="cashClosingGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#b45309" stopOpacity={0.28} />
                <stop offset="95%" stopColor="#b45309" stopOpacity={0.03} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
            <XAxis
              dataKey="label"
              tick={{ fontSize: 10, fill: "#57534e" }}
              tickLine={false}
              axisLine={{ stroke: "#d6d3d1" }}
              minTickGap={16}
              interval="preserveStartEnd"
            />
            <YAxis
              tickFormatter={(val: number) => {
                if (Math.abs(val) >= 1_000_000) {
                  const jt = val / 1_000_000;
                  return `${Number.isInteger(jt) ? jt : jt.toFixed(1)}jt`;
                }
                if (Math.abs(val) >= 1_000) {
                  return `${Math.round(val / 1_000)}rb`;
                }
                return `${val}`;
              }}
              tick={{ fontSize: 10, fill: "#57534e" }}
              tickLine={false}
              axisLine={{ stroke: "#d6d3d1" }}
              width={46}
            />
            <Tooltip
              content={({ active, payload }) => {
                if (!active || !payload || payload.length === 0) return null;
                const pt = payload[0]?.payload as ChartPoint | undefined;
                if (!pt) return null;
                return (
                  <div className="max-w-[230px] space-y-1.5 rounded-xl border border-stone-200 bg-white p-2.5 text-[11px] shadow-lg sm:max-w-none sm:p-3 sm:text-xs">
                    <div className="flex items-center justify-between gap-3 border-b border-stone-100 pb-1">
                      <span className="font-bold text-stone-900">
                        {pt.label}
                      </span>
                      {pt.belowBuffer ? (
                        <span className="rounded bg-rose-100 px-1.5 py-0.5 text-[10px] font-semibold text-rose-800">
                          Rawan
                        </span>
                      ) : (
                        <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-800">
                          Aman
                        </span>
                      )}
                    </div>
                    <div className="flex justify-between gap-3">
                      <span className="text-stone-500">Saldo Akhir:</span>
                      <span
                        className={
                          pt.belowBuffer
                            ? "font-bold text-rose-700"
                            : "font-bold text-stone-900"
                        }
                      >
                        {formatRupiah(pt.closing)}
                      </span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span className="text-stone-500">Kas Masuk:</span>
                      <span className="font-medium text-emerald-700">
                        +{formatRupiah(pt.inflow)}
                      </span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span className="text-stone-500">Belanja/Keluar:</span>
                      <span className="font-medium text-rose-700">
                        -{formatRupiah(pt.outflow)}
                      </span>
                    </div>
                    <div className="flex justify-between gap-3">
                      <span className="text-stone-500">Biaya Tetap:</span>
                      <span className="font-medium text-stone-700">
                        -{formatRupiah(pt.fixedCost)}
                      </span>
                    </div>
                  </div>
                );
              }}
            />
            <ReferenceLine
              y={bufferNum}
              stroke="#e11d48"
              strokeDasharray="5 5"
              strokeWidth={1.75}
            />
            <Bar
              dataKey="inflow"
              name="Kas Masuk"
              fill="#10b981"
              opacity={0.35}
              maxBarSize={12}
              radius={[3, 3, 0, 0]}
            />
            <Area
              type="monotone"
              dataKey="closing"
              name="Saldo Akhir"
              stroke="#b45309"
              strokeWidth={2.5}
              fill="url(#cashClosingGradient)"
              dot={{ r: 2.5, fill: "#b45309", strokeWidth: 1, stroke: "#fff" }}
              activeDot={{ r: 5, fill: "#92400e" }}
            />
            {lowestPoint ? (
              <ReferenceDot
                x={lowestPoint.label}
                y={lowestNum}
                r={5}
                fill={lowestPoint.belowBuffer ? "#e11d48" : "#0284c7"}
                stroke="#ffffff"
                strokeWidth={2}
              />
            ) : null}
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

