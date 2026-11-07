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
          : "grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-4"
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
            className={`rounded-xl border p-3 transition ${
              passed
                ? "border-emerald-200 bg-emerald-50/70 text-emerald-950"
                : "border-rose-200 bg-rose-50/80 text-rose-950"
            }`}
          >
            <div className="flex items-center justify-between gap-1.5">
              <span className="text-xs font-bold tracking-wide uppercase">
                {meta.name}
              </span>
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold ${
                  passed
                    ? "bg-emerald-100 text-emerald-800"
                    : "bg-rose-100 text-rose-800"
                }`}
              >
                {passed ? (
                  <>
                    <CheckCircle2 className="h-3 w-3" />
                    Lolos
                  </>
                ) : (
                  <>
                    <XCircle className="h-3 w-3" />
                    Gagal
                  </>
                )}
              </span>
            </div>
            <p className="mt-1.5 text-sm font-bold">
              {formatGateMetric(g.gate, g.value)}
            </p>
            <p className="mt-0.5 text-[11px] opacity-80">
              Batas {gateComparator(g.gate)}{" "}
              {formatGateMetric(g.gate, g.threshold)}
            </p>
          </div>
        );
      })}
    </div>
  );
}

