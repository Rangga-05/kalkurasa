"use client";

import { useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowRight,
  CalendarClock,
  CheckCircle2,
  ExternalLink,
  Loader2,
  RefreshCw,
  ShieldAlert,
  Sparkles,
  XCircle,
} from "lucide-react";
import { decideOrder, getErrorMessage } from "@/lib/api";
import {
  formatDateLong,
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
  onDecided: (updated: OrderDetail, action: "accept" | "counter" | "reject") => void;
}) {
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
    emerald: "border-emerald-200 bg-emerald-50 text-emerald-800",
    amber: "border-amber-200 bg-amber-50 text-amber-800",
    rose: "border-rose-200 bg-rose-50 text-rose-800",
    slate: "border-stone-200 bg-stone-100 text-stone-700",
  };

  return (
    <article className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-xs">
      {/* Header Kartu */}
      <div className="flex flex-wrap items-start justify-between gap-4 border-b border-stone-100 bg-stone-50/70 px-5 py-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm font-bold text-stone-900">
              #{item.code}
            </span>
            <span
              className={`inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-xs font-semibold ${
                toneBadgeClass[recMeta.tone] ?? toneBadgeClass.slate
              }`}
            >
              <Sparkles className="h-3 w-3" />
              {recMeta.label}
            </span>
            {item.is_big_order && (
              <span className="inline-flex items-center gap-1 rounded-full border border-purple-200 bg-purple-50 px-2.5 py-0.5 text-xs font-semibold text-purple-800">
                Pesanan Besar
              </span>
            )}
            <span className="rounded-full bg-stone-200/70 px-2 py-0.5 text-[11px] font-medium text-stone-700">
              {item.source === "manual" ? "Manual" : "Publik"}
            </span>
          </div>
          <h3 className="mt-1.5 text-base font-bold text-stone-900">
            {item.customer_name}{" "}
            <span className="text-sm font-normal text-stone-500">
              ({item.customer_contact})
            </span>
          </h3>
          <p className="mt-0.5 text-xs text-stone-600">
            Acara: <strong>{formatDateLong(item.event_date)}</strong> • Produksi:{" "}
            <strong>{formatDateShort(item.production_date)}</strong> •{" "}
            {item.delivery_type === "delivery" ? "Diantar" : "Ambil Sendiri"}
          </p>
        </div>

        <div className="text-right">
          <p className="text-xs font-medium text-stone-500">
            Total ({item.total_units} unit)
          </p>
          <p className="text-lg font-extrabold text-stone-900">
            {formatRupiah(item.total)}
          </p>
          <p className="text-xs text-stone-500">
            Tagihan DP: <strong>{formatRupiah(item.dp_required)}</strong>
          </p>
          <Link
            href={`/app/pesanan/${item.id}`}
            className="mt-1.5 inline-flex items-center gap-1 text-xs font-semibold text-amber-700 hover:text-amber-800"
          >
            Buka Detail Pesanan
            <ExternalLink className="h-3 w-3" />
          </Link>
        </div>
      </div>

      <div className="space-y-5 p-5">
        {/* Alasan Rekomendasi */}
        <div className="rounded-xl border border-stone-200/80 bg-stone-50/70 p-3.5 text-sm text-stone-800">
          <p className="text-xs font-bold tracking-wider text-stone-500 uppercase">
            Ringkasan Evaluasi Mesin Keputusan
          </p>
          <p className="mt-1 font-medium">
            {item.recommendation_reason || item.evaluation?.reason}
          </p>
        </div>

        {/* 4 Badge Gerbang G1..G4 */}
        <div>
          <p className="mb-2 text-xs font-bold tracking-wider text-stone-500 uppercase">
            Evaluasi 4 Gerbang Kelayakan (G1–G4)
          </p>
          <GateBadges gates={item.evaluation?.gates} />
        </div>

        {/* Pratinjau Dampak Sebelum vs Sesudah */}
        {impact && (
          <div>
            <p className="mb-2 text-xs font-bold tracking-wider text-stone-500 uppercase">
              Pratinjau Dampak Sebelum vs Sesudah Pesanan
            </p>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div className="rounded-xl border border-stone-200 bg-stone-50/50 p-3">
                <p className="text-xs font-medium text-stone-500">
                  Utilisasi Kapasitas ({formatDateShort(item.production_date)})
                </p>
                <div className="mt-1 flex items-center gap-1.5 text-sm font-bold text-stone-900">
                  <span>{formatPercent(impact.utilization_before)}</span>
                  <ArrowRight className="h-3.5 w-3.5 text-stone-400" />
                  <span className="text-amber-700">
                    {formatPercent(impact.utilization_after)}
                  </span>
                </div>
                {impact.bottleneck && (
                  <p className="mt-1 text-[11px] text-stone-500">
                    Bottleneck:{" "}
                    <strong>
                      {RESOURCE_LABELS[impact.bottleneck] ?? impact.bottleneck}
                    </strong>
                  </p>
                )}
              </div>

              <div className="rounded-xl border border-stone-200 bg-stone-50/50 p-3">
                <p className="text-xs font-medium text-stone-500">
                  Estimasi Margin Kotor
                </p>
                <p className="mt-1 text-sm font-bold text-stone-900">
                  {formatPercent(impact.margin)}
                </p>
                <p className="mt-1 text-[11px] text-stone-500">
                  Dihitung dari harga jual vs HPP resep + tenaga + kemasan
                </p>
              </div>

              <div className="rounded-xl border border-stone-200 bg-stone-50/50 p-3">
                <p className="text-xs font-medium text-stone-500">
                  Titik Terendah Kas Proyeksi
                </p>
                <div className="mt-1 flex flex-wrap items-center gap-1.5 text-sm font-bold text-stone-900">
                  <span>{formatRupiah(impact.cash_lowest_before)}</span>
                  <ArrowRight className="h-3.5 w-3.5 text-stone-400" />
                  <span className="text-amber-700">
                    {formatRupiah(impact.cash_lowest_after)}
                  </span>
                </div>
                <p className="mt-1 text-[11px] text-stone-500">
                  Proyeksi arus kas harian s.d. tanggal acara
                </p>
              </div>
            </div>
          </div>
        )}

        {/* Daftar Item Pesanan */}
        <div>
          <p className="mb-2 text-xs font-bold tracking-wider text-stone-500 uppercase">
            Rincian Item Pesanan
          </p>
          <div className="overflow-x-auto rounded-xl border border-stone-200">
            <table className="min-w-full divide-y divide-stone-200 text-xs">
              <thead className="bg-stone-50 text-left font-semibold text-stone-600">
                <tr>
                  <th className="px-3 py-2">Produk</th>
                  <th className="px-3 py-2 text-right">Qty</th>
                  <th className="px-3 py-2 text-right">Harga Jual</th>
                  <th className="px-3 py-2 text-right">HPP / Unit</th>
                  <th className="px-3 py-2 text-right">Subtotal</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100 bg-white text-stone-800">
                {item.items.map((it) => (
                  <tr key={it.id}>
                    <td className="px-3 py-2 font-medium">{it.product_name}</td>
                    <td className="px-3 py-2 text-right">{it.qty}</td>
                    <td className="px-3 py-2 text-right">
                      {formatRupiah(it.unit_price)}
                    </td>
                    <td className="px-3 py-2 text-right text-stone-500">
                      {formatRupiah(it.unit_cost)}
                    </td>
                    <td className="px-3 py-2 text-right font-semibold">
                      {formatRupiah(it.subtotal)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Usulan Tawaran Balik */}
        {(item.counter_offer || showCounterForm) && (
          <div className="rounded-xl border border-amber-200 bg-amber-50/60 p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2 text-xs font-bold text-amber-900 uppercase">
                <CalendarClock className="h-4 w-4 text-amber-700" />
                Parameter Tawaran Balik (Counter Offer)
              </div>
              <button
                type="button"
                onClick={() => setShowCounterForm((v) => !v)}
                className="text-xs font-semibold text-amber-800 underline hover:text-amber-950"
              >
                {showCounterForm ? "Sembunyikan Editor" : "Ubah Tanggal / DP"}
              </button>
            </div>

            {item.counter_offer && !showCounterForm && (
              <div className="mt-2 grid grid-cols-1 gap-2 text-xs text-amber-900 sm:grid-cols-3">
                <div>
                  <span className="text-amber-700">Tanggal Usulan:</span>{" "}
                  <strong>{formatDateLong(item.counter_offer.date)}</strong>
                </div>
                <div>
                  <span className="text-amber-700">Tagihan DP Usulan:</span>{" "}
                  <strong>{formatRupiah(item.counter_offer.dp)}</strong>
                </div>
                {item.counter_offer.utilization_after !== undefined && (
                  <div>
                    <span className="text-amber-700">Utilisasi Tanggal Baru:</span>{" "}
                    <strong>
                      {formatPercent(item.counter_offer.utilization_after)}
                    </strong>
                  </div>
                )}
              </div>
            )}

            {showCounterForm && (
              <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-semibold text-amber-900">
                    Tanggal Acara Tawaran Balik (YYYY-MM-DD)
                  </label>
                  <input
                    type="date"
                    value={counterDate}
                    onChange={(e) => setCounterDate(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-amber-300 bg-white px-3 py-1.5 text-sm text-stone-900"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-amber-900">
                    Nominal DP Tawaran Balik (Rp)
                  </label>
                  <input
                    type="number"
                    min={0}
                    step={1000}
                    value={counterDp}
                    onChange={(e) => setCounterDp(e.target.value)}
                    className="mt-1 w-full rounded-lg border border-amber-300 bg-white px-3 py-1.5 text-sm text-stone-900"
                  />
                </div>
              </div>
            )}
          </div>
        )}

        {/* Input Override Reason bila Rekomendasi bukan Accept */}
        {item.can_decide && needsOverrideToAccept && (
          <div className="rounded-xl border border-amber-200 bg-amber-50/40 p-3.5">
            <label className="block text-xs font-bold text-amber-900">
              Alasan Override (Wajib diisi bila tetap memilih &ldquo;Terima&rdquo; di luar rekomendasi sistem)
            </label>
            <input
              type="text"
              value={overrideReason}
              onChange={(e) => setOverrideReason(e.target.value)}
              placeholder="Contoh: Bahan sudah disiapkan terpisah & jadwal lembur disetujui pemilik"
              className="mt-1.5 w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 placeholder:text-stone-400 focus:border-amber-600 focus:outline-none"
            />
          </div>
        )}

        {/* Peringatan Wewenang Pesanan Besar */}
        {!item.can_decide && (
          <div className="flex items-center gap-2.5 rounded-xl border border-purple-200 bg-purple-50/70 p-3.5 text-xs font-medium text-purple-900">
            <ShieldAlert className="h-4 w-4 shrink-0 text-purple-700" />
            <span>
              Pesanan ini melewati ambang <strong>Pesanan Besar</strong>. Sesuai
              aturan wewenang, hanya <strong>Pemilik Usaha (Owner)</strong> yang
              dapat mengambil keputusan pada pesanan ini.
            </span>
          </div>
        )}

        {errorMsg && (
          <div className="flex items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
            <AlertTriangle className="h-4 w-4 shrink-0 text-rose-600" />
            <span>{errorMsg}</span>
          </div>
        )}

        {/* Tombol Aksi Keputusan */}
        <div className="flex flex-wrap items-center justify-end gap-2.5 border-t border-stone-100 pt-4">
          {!item.counter_offer && !showCounterForm && item.can_decide && (
            <button
              type="button"
              onClick={() => setShowCounterForm(true)}
              className="rounded-xl border border-stone-200 px-3 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50"
            >
              Atur Tawaran Balik Manual
            </button>
          )}

          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => handleDecision("reject")}
            className="inline-flex items-center gap-1.5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-2 text-xs font-bold text-rose-800 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submittingAction === "reject" ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <XCircle className="h-3.5 w-3.5" />
            )}
            Tolak
          </button>

          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => {
              if (!item.counter_offer && !showCounterForm) {
                setShowCounterForm(true);
                return;
              }
              handleDecision("counter");
            }}
            className="inline-flex items-center gap-1.5 rounded-xl border border-amber-300 bg-amber-100/80 px-4 py-2 text-xs font-bold text-amber-950 transition hover:bg-amber-200/80 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submittingAction === "counter" ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
            Tawar Balik
          </button>

          <button
            type="button"
            disabled={!item.can_decide || submittingAction !== null}
            onClick={() => handleDecision("accept")}
            className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white shadow-xs transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submittingAction === "accept" ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <CheckCircle2 className="h-3.5 w-3.5" />
            )}
            {needsOverrideToAccept ? "Terima (Override)" : "Terima Pesanan"}
          </button>
        </div>
      </div>
    </article>
  );
}

