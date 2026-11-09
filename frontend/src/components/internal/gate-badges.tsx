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
          : "grid grid-cols-2 gap-2.5 lg:grid-cols-4"
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
            className={`min-w-0 rounded-2xl border-2 border-[#12100E] p-2.5 transition-all sm:p-3 ${
              passed
                ? "bg-[#ECFCCB] text-[#12100E]"
                : "bg-[#FFE4E6] text-[#9F1239]"
            }`}
          >
            <div className="flex items-center justify-between gap-1">
              <span className="truncate text-[10px] font-extrabold tracking-wide uppercase sm:text-[11px]">
                {meta.name}
              </span>
              {passed ? (
                <CheckCircle2
                  className="h-3.5 w-3.5 shrink-0 text-[#15803D] sm:h-4 sm:w-4"
                  aria-hidden="true"
                />
              ) : (
                <XCircle
                  className="h-3.5 w-3.5 shrink-0 text-[#BE123C] sm:h-4 sm:w-4"
                  aria-hidden="true"
                />
              )}
            </div>
            <p className="font-mono-num mt-1 truncate text-xs font-extrabold sm:text-sm">
              {formatGateMetric(g.gate, g.value)}
            </p>
            <p className="font-mono-num mt-0.5 text-[10px] leading-tight font-bold opacity-80">
              Syarat {gateComparator(g.gate)}{" "}
              {formatGateMetric(g.gate, g.threshold)}
            </p>
          </div>
        );
      })}
    </div>
  );
}
