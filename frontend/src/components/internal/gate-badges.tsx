"use client";

import { CheckCircle2, XCircle } from "lucide-react";
import {
  formatNumber,
  formatPercent,
  formatRupiah,
  GATE_LABELS,
} from "@/lib/format";
import type { DecimalValue, GateDTO } from "@/lib/types";

function formatGateMetric(gate: string, val: DecimalValue): string {
  switch (gate) {
    case "G1":
    case "G2":
      return formatPercent(val);
    case "G3":
      return formatRupiah(val);
    case "G4":
      return `${formatNumber(val)} hari`;
    default:
      return formatNumber(val);
  }
}

function gateComparator(gate: string): string {
  return gate === "G1" ? "≤" : "≥";
}

export function GateBadges({
  gates,
  compact = false,
}: {
  gates?: GateDTO[] | null;
  compact?: boolean;
}) {
  if (!gates || gates.length === 0) {
    return null;
  }

  return (
    <div
      className={
        compact
          ? "grid grid-cols-2 gap-2 sm:grid-cols-4"
          : "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4"
      }
    >
      {gates.map((g) => {
        const meta = GATE_LABELS[g.gate] ?? {
          name: g.gate,
          shortDesc: "Gerbang kelayakan",
        };
        const passed = g.passed;
        return (
          <div
            key={g.gate}
            className={`rounded-2xl border p-3.5 transition-all ${
              passed
                ? "border-emerald-300 bg-emerald-50/80 text-emerald-950 dark:border-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-100"
                : "border-rose-300 bg-rose-50/85 text-rose-950 dark:border-rose-800 dark:bg-rose-950/60 dark:text-rose-100"
            }`}
          >
            <div className="flex items-center justify-between gap-1.5">
              <span className="text-xs font-bold tracking-wide uppercase">
                {meta.name}
              </span>
              <span
                className={`inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-bold ${
                  passed
                    ? "bg-emerald-200/80 text-emerald-900 dark:bg-emerald-900 dark:text-emerald-200"
                    : "bg-rose-200/80 text-rose-900 dark:bg-rose-900 dark:text-rose-200"
                }`}
              >
                {passed ? (
                  <>
                    <CheckCircle2 className="h-3 w-3" aria-hidden="true" />
                    Lolos
                  </>
                ) : (
                  <>
                    <XCircle className="h-3 w-3" aria-hidden="true" />
                    Gagal
                  </>
                )}
              </span>
            </div>
            <p className="mt-1.5 font-mono text-base font-bold">
              {formatGateMetric(g.gate, g.value)}
            </p>
            <p className="mt-0.5 font-mono text-[11px] opacity-85">
              Batas {gateComparator(g.gate)}{" "}
              {formatGateMetric(g.gate, g.threshold)}
            </p>
          </div>
        );
      })}
    </div>
  );
}
