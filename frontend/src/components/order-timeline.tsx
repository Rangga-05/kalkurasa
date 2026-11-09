import {
  Check,
  Clock,
  AlertTriangle,
  XCircle,
  Truck,
  ChefHat,
  CreditCard,
  FileCheck,
  PackageCheck,
} from "lucide-react";
import type { OrderStatus } from "@/lib/types";
import { cn } from "@/lib/utils";

interface StepDef {
  key: string;
  label: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
}

const STEPS: StepDef[] = [
  {
    key: "review",
    label: "Pengajuan & Evaluasi",
    description: "Pesanan diterima dan dievaluasi terhadap jadwal & kapasitas.",
    icon: FileCheck,
  },
  {
    key: "accepted",
    label: "Menunggu Pembayaran DP",
    description: "Slot kapasitas direservasi; menunggu pembayaran uang muka.",
    icon: CreditCard,
  },
  {
    key: "confirmed",
    label: "Terkonfirmasi",
    description: "DP telah diverifikasi, bahan baku dijadwalkan untuk dibeli.",
    icon: Check,
  },
  {
    key: "in_production",
    label: "Dalam Produksi",
    description: "Tim dapur sedang mengolah pesanan sesuai standar resep.",
    icon: ChefHat,
  },
  {
    key: "ready_deliver",
    label: "Siap / Pengantaran",
    description: "Pesanan telah selesai dikemas dan siap diambil/diantar.",
    icon: Truck,
  },
  {
    key: "completed",
    label: "Selesai",
    description: "Pesanan telah diterima pelanggan dan pembayaran lunas.",
    icon: PackageCheck,
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
    case "delivering":
      return 4;
    case "completed":
      return 5;
    default:
      return -1;
  }
}

export function OrderTimeline({
  status,
  isFullyPaid,
}: {
  status: OrderStatus;
  isFullyPaid?: boolean;
}) {
  const isTerminalNegative =
    status === "rejected" || status === "cancelled" || status === "expired";
  const currentIdx = getStepIndex(status);

  return (
    <div className="space-y-4">
      {isTerminalNegative ? (
        <div className="flex items-start gap-3 rounded-xl border border-rose-200 bg-rose-50 p-4 text-rose-950">
          <XCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
          <div className="text-sm">
            <p className="font-semibold">
              {status === "rejected"
                ? "Pesanan Tidak Dapat Dipenuhi (Ditolak)"
                : status === "cancelled"
                  ? "Pesanan Dibatalkan"
                  : "Pesanan Kedaluwarsa (Melewati Tenggat DP)"}
            </p>
            <p className="mt-0.5 text-xs text-rose-800">
              {status === "expired"
                ? "Pembayaran DP belum tercatat hingga melewati batas waktu sehingga reservasi kapasitas otomatis dilepas."
                : status === "cancelled"
                  ? "Pesanan ini telah dibatalkan dan tidak lagi dijadwalkan untuk produksi."
                  : "Kapasitas dapur pada tanggal yang diminta tidak mencukupi atau tawaran balik ditolak."}
            </p>
          </div>
        </div>
      ) : null}

      <ol className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {STEPS.map((step, index) => {
          const Icon = step.icon;
          const isCompleted =
            !isTerminalNegative &&
            (status === "completed" ? true : index < currentIdx);
          const isCurrent =
            !isTerminalNegative &&
            status !== "completed" &&
            index === currentIdx;

          return (
            <li
              key={step.key}
              className={cn(
                "flex items-start gap-3 rounded-xl border p-3.5 transition-colors",
                isCompleted &&
                  "border-emerald-200 bg-emerald-50/50 text-emerald-950",
                isCurrent &&
                  (status === "countered" || status === "pending_decision"
                    ? "border-amber-300 bg-amber-50/70 text-amber-950 ring-1 ring-amber-300"
                    : "border-sky-300 bg-sky-50/70 text-sky-950 ring-1 ring-sky-300"),
                !isCompleted &&
                  !isCurrent &&
                  "border-stone-200 bg-stone-50/60 text-stone-500"
              )}
            >
              <div
                className={cn(
                  "flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-xs font-semibold",
                  isCompleted &&
                    "border-emerald-600 bg-emerald-600 text-white",
                  isCurrent &&
                    (status === "countered" || status === "pending_decision"
                      ? "border-amber-600 bg-amber-600 text-white"
                      : "border-sky-600 bg-sky-600 text-white"),
                  !isCompleted &&
                    !isCurrent &&
                    "border-stone-300 bg-white text-stone-400"
                )}
              >
                {isCompleted ? (
                  <Check className="h-4 w-4" />
                ) : isCurrent &&
                  (status === "countered" || status === "pending_decision") ? (
                  <AlertTriangle className="h-4 w-4" />
                ) : isCurrent ? (
                  <Clock className="h-4 w-4" />
                ) : (
                  <Icon className="h-4 w-4" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-1.5">
                  <span className="text-[11px] font-semibold uppercase tracking-wider opacity-70">
                    Tahap {index + 1}
                  </span>
                  {isCurrent ? (
                    <span className="rounded-full bg-black/10 px-2 py-0.5 text-[10px] font-semibold">
                      Saat Ini
                    </span>
                  ) : null}
                </div>
                <p className="mt-0.5 text-sm font-semibold leading-snug">
                  {step.label}
                </p>
                <p className="mt-0.5 text-xs leading-relaxed opacity-80">
                  {isFullyPaid && step.key === "confirmed"
                    ? "Pembayaran telah diverifikasi lunas, bahan baku dijadwalkan untuk dibeli."
                    : step.description}
                </p>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

