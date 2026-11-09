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
    <div className="h-64 w-full min-w-0 sm:h-80">
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart
          data={data}
          margin={{ top: 20, right: 16, left: 4, bottom: 8 }}
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
            tick={{ fontSize: 11, fill: "#57534e" }}
            tickLine={false}
            axisLine={{ stroke: "#d6d3d1" }}
          />
          <YAxis
            tickFormatter={(val: number) => {
              if (Math.abs(val) >= 1_000_000) {
                return `Rp${(val / 1_000_000).toFixed(1)}jt`;
              }
              if (Math.abs(val) >= 1_000) {
                return `Rp${Math.round(val / 1_000)}rb`;
              }
              return `Rp${val}`;
            }}
            tick={{ fontSize: 11, fill: "#57534e" }}
            tickLine={false}
            axisLine={{ stroke: "#d6d3d1" }}
            width={76}
          />
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload || payload.length === 0) return null;
              const pt = payload[0]?.payload as ChartPoint | undefined;
              if (!pt) return null;
              return (
                <div className="rounded-xl border border-stone-200 bg-white p-3 shadow-lg text-xs space-y-1.5">
                  <div className="flex items-center justify-between gap-4 border-b border-stone-100 pb-1">
                    <span className="font-bold text-stone-900">
                      {pt.label} ({pt.date})
                    </span>
                    {pt.belowBuffer ? (
                      <span className="rounded bg-rose-100 px-1.5 py-0.5 text-[10px] font-semibold text-rose-800">
                        Di Bawah Penyangga
                      </span>
                    ) : (
                      <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-[10px] font-semibold text-emerald-800">
                        Aman
                      </span>
                    )}
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-stone-500">Saldo Akhir Hari:</span>
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
                  <div className="flex justify-between gap-4">
                    <span className="text-stone-500">Kas Masuk (DP/Pelunasan):</span>
                    <span className="font-medium text-emerald-700">
                      +{formatRupiah(pt.inflow)}
                    </span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-stone-500">Belanja & Kas Keluar:</span>
                    <span className="font-medium text-rose-700">
                      -{formatRupiah(pt.outflow)}
                    </span>
                  </div>
                  <div className="flex justify-between gap-4">
                    <span className="text-stone-500">Biaya Tetap Harian:</span>
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
            label={{
              value: `Penyangga Kas (${formatRupiah(bufferNum)})`,
              position: "insideTopRight",
              fill: "#be123c",
              fontSize: 11,
              fontWeight: 600,
            }}
          />
          <Bar
            dataKey="inflow"
            name="Kas Masuk"
            fill="#10b981"
            opacity={0.35}
            barSize={12}
            radius={[3, 3, 0, 0]}
          />
          <Area
            type="monotone"
            dataKey="closing"
            name="Saldo Akhir"
            stroke="#b45309"
            strokeWidth={2.5}
            fill="url(#cashClosingGradient)"
            dot={{ r: 3, fill: "#b45309", strokeWidth: 1, stroke: "#fff" }}
            activeDot={{ r: 5, fill: "#92400e" }}
          />
          {lowestPoint ? (
            <ReferenceDot
              x={lowestPoint.label}
              y={lowestNum}
              r={6}
              fill={lowestPoint.belowBuffer ? "#e11d48" : "#0284c7"}
              stroke="#ffffff"
              strokeWidth={2}
              label={{
                value: `Terendah: ${formatRupiah(lowestNum)}`,
                position: "top",
                fill: lowestPoint.belowBuffer ? "#be123c" : "#0369a1",
                fontSize: 11,
                fontWeight: 700,
              }}
            />
          ) : null}
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}

