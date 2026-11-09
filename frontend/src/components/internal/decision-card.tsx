"use client";

import { useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Loader2,
  RefreshCw,
  ShieldAlert,
  XCircle,
} from "lucide-react";
import { decideOrder, getErrorMessage } from "@/lib/api";
import {
  formatDateShort,
  formatPercent,
  formatRupiah,
  RECOMMENDATION_META,
  RESOURCE_LABELS,
} from "@/lib/format";
import type { DecisionQueueItem, OrderDetail } from "@/lib/types";
import { GateBadges } from "./gate-badges";

export function DecisionCard({
  item,
  onDecided,
}: {
  item: DecisionQueueItem;
  onDecided: (
    updated: OrderDetail,
    action: "accept" | "counter" | "reject"
  ) => void;
}) {
  const [showDetails, setShowDetails] = useState(false);
  const [overrideReason, setOverrideReason] = useState("");
  const [showCounterForm, setShowCounterForm] = useState(false);
  const [counterDate, setCounterDate] = useState(
    item.counter_offer?.date ?? item.event_date
  );
  const [counterDp, setCounterDp] = useState(
    String(item.counter_offer?.dp ?? item.dp_required)
  );
  const [submittingAction, setSubmittingAction] = useState<
    "accept" | "counter" | "reject" | null
  >(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const recMeta = RECOMMENDATION_META[item.recommendation] ?? {
    label: `Rekomendasi: ${item.recommendation}`,
    tone: "slate" as const,
  };
  const needsOverrideToAccept = item.recommendation !== "accept";
  const impact = item.evaluation?.impact;

  async function handleDecision(action: "accept" | "counter" | "reject") {
    setErrorMsg(null);
    if (!item.can_decide) {
      setErrorMsg(
        "Pesanan besar hanya dapat diputuskan oleh Pemilik Usaha (Owner)."
      );
      return;
    }

    if (action === "accept" && needsOverrideToAccept && !overrideReason.trim()) {
      setErrorMsg(
        "Alasan override wajib diisi karena rekomendasi sistem bukan Terima."
      );
      return;
    }

    if (action === "counter" && !counterDate.trim()) {
      setErrorMsg("Tanggal tawaran balik wajib diisi (format YYYY-MM-DD).");
      return;
    }

    setSubmittingAction(action);
    try {
      const updated = await decideOrder(item.id, {
        action,
        override_reason: overrideReason.trim() || undefined,
        counter:
          action === "counter"
            ? {
                date: counterDate.trim(),
                min_price_per_item: item.counter_offer?.min_price_per_item,
                dp: counterDp.trim() || item.dp_required,
              }
            : undefined,
      });
      onDecided(updated, action);
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal menyimpan keputusan pesanan."));
    } finally {
      setSubmittingAction(null);
    }
  }

  const toneBadgeClass: Record<string, string> = {
    emerald: "border-[#12100E] bg-[#D9F99D] text-[#12100E]",
    amber: "border-[#12100E] bg-[#FDE047] text-[#12100E]",
    rose: "border-[#12100E] bg-[#FFE4E6] text-[#9F1239]",
    slate: "border-[#12100E] bg-[#F7EFE2] text-[#12100E]",
  };

  return (
    <article className="overflow-hidden rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] shadow-[5px_5px_0px_#12100E]">
      {/* Ringkasan Utama Pesanan (Padat & Mudah Dipindai) */}
      <div className="flex flex-col gap-3 border-b-2 border-[#12100E] bg-[#F7EFE2] px-3.5 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:px-5 sm:py-4">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-1.5 sm:gap-2">
            <span className="font-mono-num text-sm font-extrabold text-[#12100E]">
              #{item.code}
            </span>
            <span
              className={`inline-flex items-center gap-1 rounded-full border-2 px-2.5 py-0.5 text-[11px] font-extrabold sm:px-3 sm:text-xs ${
                toneBadgeClass[recMeta.tone] ?? toneBadgeClass.slate
              }`}
            >
              {recMeta.label}
            </span>
            {item.is_big_order ? (
              <span className="rounded-full border-2 border-[#12100E] bg-[#F3E8FF] px-2.5 py-0.5 text-[11px] font-extrabold text-[#12100E] sm:text-xs">
                Pesanan Besar
              </span>
            ) : null}
          </div>

          <h3 className="font-display mt-1.5 text-base font-extrabold text-[#12100E] sm:text-lg">
            {item.customer_name}{" "}
            <span className="font-sans text-xs font-semibold text-[#5C5046]">
              · Acara {formatDateShort(item.event_date)} ({item.total_units} unit)
            </span>
          </h3>
        </div>

        <div className="flex w-full items-center justify-between gap-3 border-t border-[#12100E]/15 pt-2.5 sm:w-auto sm:justify-end sm:gap-4 sm:border-t-0 sm:pt-0">
          <div className="text-left sm:text-right">
            <p className="font-mono-num text-base font-extrabold text-[#12100E] sm:text-lg">
              {formatRupiah(item.total)}
            </p>
            <p className="font-mono-num text-xs font-bold text-[#5C5046]">
              DP: {formatRupiah(item.dp_required)}
            </p>
          </div>
          <Link
            href={`/app/pesanan/${item.id}`}
            title="Buka halaman detail pesanan"
            className="twist-btn inline-flex h-9 shrink-0 items-center gap-1 rounded-full bg-[#FFFDF9] px-3 text-xs font-extrabold text-[#12100E] uppercase"
          >
            <span>Detail</span>
            <ExternalLink className="h-3 w-3" />
          </Link>
        </div>
      </div>

      <div className="space-y-3.5 p-3.5 sm:space-y-4 sm:p-5">
        {/* Ringkasan 1 Kalimat + 4 Gerbang */}
        <p className="text-xs leading-relaxed font-semibold text-[#3D352E] sm:text-sm">
          {item.recommendation_reason || item.evaluation?.reason}
        </p>

        <GateBadges gates={item.evaluation?.gates} compact />

        {/* Tombol Buka/Tutup Rincian Dampak & Item (Progressive Disclosure) */}
        <div>
          <button
            type="button"
            onClick={() => setShowDetails((v) => !v)}
            aria-expanded={showDetails}
            className="flex w-full items-center justify-between gap-1.5 rounded-xl border border-[#12100E] bg-[#F7EFE2] px-3.5 py-2 text-left text-xs font-extrabold text-[#12100E] transition-colors hover:bg-[#FDE047] sm:inline-flex sm:w-auto sm:justify-start sm:rounded-full sm:py-1.5"
          >
            {showDetails ? (
              <>
                <span>Sembunyikan Rincian Item &amp; Dampak</span>
                <ChevronUp className="h-3.5 w-3.5 shrink-0" />
              </>
            ) : (
              <>
                <span>
                  Lihat Rincian Item ({item.items.length}) &amp; Dampak Dapur
                </span>
                <ChevronDown className="h-3.5 w-3.5 shrink-0" />
              </>
            )}
          </button>

          {showDetails ? (
            <div className="mt-3 space-y-3.5 rounded-2xl border-2 border-[#12100E] bg-[#FFF9F0] p-3 sm:space-y-4 sm:p-4">
              {impact ? (
                <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-3 sm:gap-3">
                  <div className="rounded-xl border border-[#12100E] bg-[#FFFDF9] p-3">
                    <p className="text-[11px] font-bold text-[#5C5046] uppercase">
                      Utilisasi ({formatDateShort(item.production_date)})
                    </p>
                    <div className="font-mono-num mt-1 flex items-center gap-1.5 text-sm font-extrabold text-[#12100E]">
                      <span>{formatPercent(impact.utilization_before)}</span>
                      <ArrowRight className="h-3.5 w-3.5 text-[#5C5046]" />
                      <span className="text-[#C2410C]">
                        {formatPercent(impact.utilization_after)}
                      </span>
                    </div>
                    {impact.bottleneck ? (
                      <p className="mt-0.5 text-[11px] font-semibold text-[#5C5046]">
                        Penghambat:{" "}
                        {RESOURCE_LABELS[impact.bottleneck] ?? impact.bottleneck}
                      </p>
                    ) : null}
                  </div>

                  <div className="rounded-xl border border-[#12100E] bg-[#FFFDF9] p-3">
                    <p className="text-[11px] font-bold text-[#5C5046] uppercase">
                      Estimasi Margin
                    </p>
                    <p className="font-mono-num mt-1 text-sm font-extrabold text-[#12100E]">
                      {formatPercent(impact.margin)}
                    </p>
                  </div>

                  <div className="rounded-xl border border-[#12100E] bg-[#FFFDF9] p-3">
                    <p className="text-[11px] font-bold text-[#5C5046] uppercase">
                       Kas Terendah
                    </p>
                    <div className="font-mono-num mt-1 flex flex-wrap items-center gap-1.5 text-xs font-extrabold text-[#12100E] sm:text-sm">
                      <span>{formatRupiah(impact.cash_lowest_before)}</span>
                      <ArrowRight className="h-3.5 w-3.5 text-[#5C5046]" />
                      <span className="text-[#C2410C]">
                        {formatRupiah(impact.cash_lowest_after)}
                      </span>
                    </div>
                  </div>
                </div>
              ) : null}

              {/* Tampilan Mobile: Daftar Item Ringkas */}
              <div className="space-y-2 sm:hidden">
                {item.items.map((it) => (
                  <div
                    key={it.id}
                    className="flex items-center justify-between gap-2 rounded-xl border border-[#12100E] bg-[#FFFDF9] px-3 py-2 text-xs"
                  >
                    <div className="min-w-0">
                      <p className="truncate font-bold text-[#12100E]">
                        {it.product_name}
                      </p>
                      <p className="font-mono-num text-[11px] font-semibold text-[#5C5046]">
                        {it.qty} × {formatRupiah(it.unit_price)}
                      </p>
                    </div>
                    <span className="font-mono-num shrink-0 font-extrabold text-[#12100E]">
                      {formatRupiah(it.subtotal)}
                    </span>
                  </div>
                ))}
              </div>

              {/* Tampilan Tablet/Desktop: Tabel Item */}
              <div className="hidden overflow-x-auto rounded-xl border border-[#12100E] bg-[#FFFDF9] sm:block">
                <table className="min-w-full divide-y divide-[#12100E]/20 text-xs">
                  <thead className="bg-[#F7EFE2] text-left font-extrabold text-[#12100E]">
                    <tr>
                      <th className="px-3 py-2">Produk</th>
                      <th className="px-3 py-2 text-right">Qty</th>
                      <th className="px-3 py-2 text-right">Harga</th>
                      <th className="px-3 py-2 text-right">Subtotal</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[#12100E]/10 font-semibold text-[#12100E]">
                    {item.items.map((it) => (
                      <tr key={it.id}>
                        <td className="px-3 py-2">{it.product_name}</td>
                        <td className="font-mono-num px-3 py-2 text-right">
                          {it.qty}
                        </td>
                        <td className="font-mono-num px-3 py-2 text-right">
                          {formatRupiah(it.unit_price)}
                        </td>
                        <td className="font-mono-num px-3 py-2 text-right font-extrabold">
                          {formatRupiah(it.subtotal)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          ) : null}
        </div>

        {/* Usulan Tawaran Balik */}
        {(item.counter_offer || showCounterForm) && (
          <div className="rounded-2xl border-2 border-[#12100E] bg-[#FEF9C3] p-3.5">
            <div className="flex flex-col items-start justify-between gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <div className="flex items-start gap-2 text-xs font-extrabold text-[#12100E] uppercase sm:items-center">
                <CalendarClock className="mt-0.5 h-4 w-4 shrink-0 text-[#C2410C] sm:mt-0" />
                <span>Tawaran Balik: {formatDateShort(counterDate)} · DP {formatRupiah(counterDp)}</span>
              </div>
              <button
                type="button"
                onClick={() => setShowCounterForm((v) => !v)}
                className="text-xs font-extrabold text-[#C2410C] underline"
              >
                {showCounterForm ? "Tutup Editor" : "Ubah Tanggal / DP"}
              </button>
            </div>

            {showCounterForm ? (
              <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label
                    htmlFor={`counter-date-${item.id}`}
                    className="block text-xs font-bold text-[#12100E]"
                  >
                    Tanggal Acara Baru
                  </label>
                  <input
                    id={`counter-date-${item.id}`}
                    type="date"
                    value={counterDate}
                    onChange={(e) => setCounterDate(e.target.value)}
                    className="mt-1 w-full rounded-xl border-2 border-[#12100E] bg-[#FFFDF9] px-3 py-1.5 text-sm font-semibold text-[#12100E]"
                  />
                </div>
                <div>
                  <label
                    htmlFor={`counter-dp-${item.id}`}
                    className="block text-xs font-bold text-[#12100E]"
                  >
                    Nominal DP Baru (Rp)
                  </label>
                  <input
                    id={`counter-dp-${item.id}`}
                    type="number"
                    min={0}
                    step={1000}
                    value={counterDp}
                    onChange={(e) => setCounterDp(e.target.value)}
                    className="mt-1 w-full rounded-xl border-2 border-[#12100E] bg-[#FFFDF9] px-3 py-1.5 text-sm font-semibold text-[#12100E]"
                  />
                </div>
              </div>
            ) : null}
          </div>
        )}

        {/* Input Override Reason bila Rekomendasi bukan Accept */}
        {item.can_decide && needsOverrideToAccept ? (
          <div>
            <label
              htmlFor={`override-reason-${item.id}`}
              className="block text-xs font-extrabold text-[#12100E]"
            >
              Alasan Override (Diisi jika tetap memilih Terima Pesanan):
            </label>
            <input
              id={`override-reason-${item.id}`}
              type="text"
              value={overrideReason}
              onChange={(e) => setOverrideReason(e.target.value)}
              placeholder="Contoh: Lembur tim dapur & stok bahan cadangan sudah disetujui"
              className="mt-1 w-full rounded-xl border-2 border-[#12100E] bg-[#FFFDF9] px-3 py-2 text-xs font-semibold text-[#12100E]"
            />
          </div>
        ) : null}

        {!item.can_decide ? (
          <div className="flex items-center gap-2 rounded-xl border-2 border-[#12100E] bg-[#F3E8FF] p-3 text-xs font-bold text-[#12100E]">
            <ShieldAlert className="h-4 w-4 shrink-0 text-[#6B21A8]" />
            <span>
              Pesanan Besar: hanya <strong>Pemilik Usaha (Owner)</strong> yang
              dapat memutuskan pesanan ini.
            </span>
          </div>
        ) : null}

        {errorMsg ? (
          <div className="flex items-center gap-2 rounded-xl border-2 border-[#12100E] bg-[#FFE4E6] p-3 text-xs font-bold text-[#9F1239]">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        ) : null}

        {/* Tombol Aksi Keputusan */}
        <div className="grid grid-cols-2 gap-2 border-t-2 border-dashed border-[#12100E]/20 pt-3 sm:flex sm:flex-wrap sm:items-center sm:justify-end">
          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => handleDecision("reject")}
            className="twist-btn inline-flex items-center justify-center gap-1.5 rounded-full bg-[#FFE4E6] px-3.5 py-2 text-xs font-extrabold text-[#9F1239] uppercase disabled:opacity-50 sm:px-4"
          >
            {submittingAction === "reject" ? (
              <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
            ) : (
              <XCircle className="h-3.5 w-3.5 shrink-0" />
            )}
            <span>Tolak</span>
          </button>

          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => {
              if (!item.counter_offer && !showCounterForm) {
                setShowCounterForm(true);
                return;
              }
              void handleDecision("counter");
            }}
            className="twist-btn inline-flex items-center justify-center gap-1.5 rounded-full bg-[#FDE047] px-3.5 py-2 text-xs font-extrabold text-[#12100E] uppercase disabled:opacity-50 sm:px-4"
          >
            {submittingAction === "counter" ? (
              <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5 shrink-0" />
            )}
            <span>Tawar Balik</span>
          </button>

          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => void handleDecision("accept")}
            className="twist-btn col-span-2 inline-flex items-center justify-center gap-1.5 rounded-full bg-[#D9F99D] px-5 py-2.5 text-xs font-extrabold text-[#12100E] uppercase disabled:opacity-50 sm:col-span-1 sm:py-2"
          >
            {submittingAction === "accept" ? (
              <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" />
            ) : (
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-[#15803D]" />
            )}
            <span>
              {needsOverrideToAccept ? "Terima (Override)" : "Terima Pesanan"}
            </span>
          </button>
        </div>
      </div>
    </article>
  );
}
