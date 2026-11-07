"use client";

import {
  AlertTriangle,
  CheckCircle2,
  Clock,
  PackageCheck,
  Sparkles,
  Truck,
  UtensilsCrossed,
  XCircle,
} from "lucide-react";
import { getOrderStatusMeta } from "@/lib/format";
import type { OrderStatus } from "@/lib/types";

const STEPS: {
  key: OrderStatus;
  label: string;
  shortDesc: string;
  icon: typeof Clock;
}[] = [
  {
    key: "pending_decision",
    label: "Evaluasi",
    shortDesc: "Cek gerbang G1–G4",
    icon: Clock,
  },
  {
    key: "accepted",
    label: "Disetujui",
    shortDesc: "Menunggu DP",
    icon: Sparkles,
  },
  {
    key: "confirmed",
    label: "Terkonfirmasi",
    shortDesc: "DP lunas & terjadwal",
    icon: CheckCircle2,
  },
  {
    key: "in_production",
    label: "Dalam Produksi",
    shortDesc: "Dapur memproses",
    icon: UtensilsCrossed,
  },
  {
    key: "ready",
    label: "Siap",
    shortDesc: "Selesai dikemas",
    icon: PackageCheck,
  },
  {
    key: "delivering",
    label: "Penyerahan",
    shortDesc: "Diantar / diambil",
    icon: Truck,
  },
  {
    key: "completed",
    label: "Selesai",
    shortDesc: "Pesanan tuntas",
    icon: CheckCircle2,
  },
];

function getStepIndex(status: OrderStatus): number {
  switch (status) {
    case "submitted":
    case "pending_decision":
    case "countered":
      return 0;
    case "accepted":
      return 1;
    case "confirmed":
      return 2;
    case "in_production":
      return 3;
    case "ready":
      return 4;
    case "delivering":
      return 5;
    case "completed":
      return 6;
    default:
      return -1;
  }
}

export function OrderStatusBadge({ status }: { status: OrderStatus | string }) {
  const meta = getOrderStatusMeta(status);
  const toneClasses: Record<string, string> = {
    emerald: "border-emerald-200 bg-emerald-50 text-emerald-800",
    amber: "border-amber-200 bg-amber-50 text-amber-800",
    sky: "border-sky-200 bg-sky-50 text-sky-800",
    indigo: "border-indigo-200 bg-indigo-50 text-indigo-800",
    rose: "border-rose-200 bg-rose-50 text-rose-800",
    slate: "border-slate-200 bg-slate-100 text-slate-700",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold ${
        toneClasses[meta.tone] ?? toneClasses.slate
      }`}
    >
      {meta.label}
    </span>
  );
}

export function OrderTimeline({ status }: { status: OrderStatus }) {
  const meta = getOrderStatusMeta(status);
  const isTerminalNegative =
    status === "rejected" || status === "cancelled" || status === "expired";
  const currentIdx = getStepIndex(status);

  return (
    <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-stone-100 pb-4">
        <div>
          <p className="text-xs font-semibold tracking-wider text-stone-400 uppercase">
            Alur Siklus Pesanan
          </p>
          <p className="mt-1 text-sm text-stone-600">{meta.description}</p>
        </div>
        <OrderStatusBadge status={status} />
      </div>

      {isTerminalNegative ? (
        <div className="mt-4 flex items-center gap-3 rounded-xl border border-rose-200 bg-rose-50/70 p-4 text-rose-900">
          {status === "expired" ? (
            <AlertTriangle className="h-5 w-5 shrink-0 text-rose-600" />
          ) : (
            <XCircle className="h-5 w-5 shrink-0 text-rose-600" />
          )}
          <div className="text-sm">
            <p className="font-semibold">{meta.label}</p>
            <p className="text-xs text-rose-700">{meta.description}</p>
          </div>
        </div>
      ) : (
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {STEPS.map((step, idx) => {
            const Icon = step.icon;
            const isDone = idx < currentIdx || status === "completed";
            const isCurrent = idx === currentIdx && status !== "completed";

            return (
              <div
                key={step.key}
                className={`rounded-xl border p-3 transition ${
                  isCurrent
                    ? "border-amber-400 bg-amber-50/80 text-amber-950 ring-2 ring-amber-200/70"
                    : isDone
                      ? "border-emerald-200 bg-emerald-50/50 text-emerald-900"
                      : "border-stone-200 bg-stone-50/60 text-stone-400"
                }`}
              >
                <div className="flex items-center justify-between">
                  <span
                    className={`flex h-7 w-7 items-center justify-center rounded-lg ${
                      isCurrent
                        ? "bg-amber-600 text-white"
                        : isDone
                          ? "bg-emerald-600 text-white"
                          : "bg-stone-200 text-stone-500"
                    }`}
                  >
                    <Icon className="h-3.5 w-3.5" />
                  </span>
                  <span className="text-[11px] font-bold">#{idx + 1}</span>
                </div>
                <p className="mt-2 text-xs font-bold">
                  {idx === 0 && status === "countered"
                    ? "Tawaran Balik"
                    : step.label}
                </p>
                <p className="mt-0.5 text-[11px] opacity-80">{step.shortDesc}</p>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

