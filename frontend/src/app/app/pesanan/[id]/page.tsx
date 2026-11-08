"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Ban,
  CheckCircle2,
  ClipboardList,
  CreditCard,
  History,
  Loader2,
  Play,
  Printer,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { GateBadges } from "@/components/internal/gate-badges";
import { OrderInvoicePrint } from "@/components/order-invoice-print";
import {
  OrderStatusBadge,
  OrderTimeline,
} from "@/components/internal/order-timeline";
import {
  cancelOrder,
  decideOrder,
  getErrorMessage,
  getOrderById,
  recordOrderPayment,
  transitionOrder,
} from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  formatDateTimeWIB,
  formatPercent,
  formatRupiah,
  RECOMMENDATION_META,
  RESOURCE_LABELS,
  TASK_STATUS_LABELS,
  TASK_TYPE_LABELS,
  toNumber,
} from "@/lib/format";
import type { CancelOrderResult, OrderDetail } from "@/lib/types";

export default function OrderDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { user } = useInternalSession();
  const role = user?.role ?? "staff";
  const isStaff = role === "staff";
  const canManage = role === "owner" || role === "manager";

  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // State form pembayaran
  const [payAmount, setPayAmount] = useState("");
  const [payMethod, setPayMethod] = useState("transfer");
  const [payNote, setPayNote] = useState("");
  const [submittingPay, setSubmittingPay] = useState(false);

  // State form transisi status
  const [actualMinutes, setActualMinutes] = useState("");
  const [submittingTransition, setSubmittingTransition] = useState(false);

  // State form keputusan langsung (bila pending_decision)
  const [overrideReason, setOverrideReason] = useState("");
  const [counterDate, setCounterDate] = useState("");
  const [submittingDecision, setSubmittingDecision] = useState<string | null>(
    null
  );

  // State dialog batalkan pesanan
  const [showCancelDialog, setShowCancelDialog] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [cancelDpDisp, setCancelDpDisp] = useState<
    "forfeited" | "refunded" | "pending"
  >("forfeited");
  const [cancelNote, setCancelNote] = useState("");
  const [submittingCancel, setSubmittingCancel] = useState(false);
  const [cancelResult, setCancelResult] = useState<CancelOrderResult | null>(
    null
  );

  const loadDetail = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const data = await getOrderById(id);
      setOrder(data);
      setCounterDate(data.counter_offer?.date ?? data.event_date);
      if (!isStaff) {
        const totalNum = toNumber(data.total);
        const paidNum = toNumber(data.paid);
        const dpNum = toNumber(data.dp_required);
        if (paidNum < dpNum) {
          setPayAmount(String(Math.max(0, Math.round(dpNum - paidNum))));
        } else if (paidNum < totalNum) {
          setPayAmount(String(Math.max(0, Math.round(totalNum - paidNum))));
        } else {
          setPayAmount("");
        }
      }
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat detail pesanan."));
    } finally {
      setLoading(false);
    }
  }, [id, isStaff]);

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  async function handleRecordPayment(e: React.FormEvent) {
    e.preventDefault();
    if (!order) return;
    setActionError(null);
    setActionSuccess(null);

    const amt = toNumber(payAmount);
    if (amt <= 0) {
      setActionError("Nominal pembayaran harus lebih dari Rp 0.");
      return;
    }

    setSubmittingPay(true);
    try {
      const res = await recordOrderPayment(order.id, {
        amount: String(amt),
        method: payMethod,
        note: payNote.trim() || undefined,
      });
      setPayNote("");
      setActionSuccess(
        `Pembayaran sebesar ${formatRupiah(res.payment.amount)} berhasil dicatat. Status pesanan kini: ${res.status}.`
      );
      await loadDetail();
    } catch (err) {
      setActionError(getErrorMessage(err, "Gagal mencatat pembayaran."));
    } finally {
      setSubmittingPay(false);
    }
  }

  async function handleTransition(
    toStatus: "in_production" | "ready" | "delivering" | "completed"
  ) {
    if (!order) return;
    setActionError(null);
    setActionSuccess(null);

    let mins: number | undefined;
    if (toStatus === "ready" && actualMinutes.trim()) {
      const parsed = Number.parseInt(actualMinutes.trim(), 10);
      if (!Number.isFinite(parsed) || parsed <= 0) {
        setActionError("Durasi produksi aktual harus lebih dari 0 menit.");
        return;
      }
      mins = parsed;
    }

    setSubmittingTransition(true);
    try {
      const updated = await transitionOrder(order.id, {
        to_status: toStatus,
        actual_minutes: mins,
      });
      setOrder(updated);
      setActionSuccess(`Status pesanan berhasil diubah ke "${toStatus}".`);
    } catch (err) {
      setActionError(getErrorMessage(err, "Gagal mengubah status pesanan."));
    } finally {
      setSubmittingTransition(false);
    }
  }

  async function handleQuickDecision(action: "accept" | "counter" | "reject") {
    if (!order) return;
    setActionError(null);
    setActionSuccess(null);

    const needsOverride =
      action === "accept" && order.recommendation !== "accept";
    if (needsOverride && !overrideReason.trim()) {
      setActionError(
        "Alasan override wajib diisi karena rekomendasi sistem bukan Terima."
      );
      return;
    }

    setSubmittingDecision(action);
    try {
      const updated = await decideOrder(order.id, {
        action,
        override_reason: overrideReason.trim() || undefined,
        counter:
          action === "counter"
            ? {
                date: counterDate || order.event_date,
                min_price_per_item: order.counter_offer?.min_price_per_item,
                dp: order.counter_offer?.dp ?? order.dp_required,
              }
            : undefined,
      });
      setOrder(updated);
      setActionSuccess(`Keputusan "${action}" berhasil disimpan.`);
      await loadDetail();
    } catch (err) {
      setActionError(
        getErrorMessage(err, "Gagal menyimpan keputusan pesanan.")
      );
    } finally {
      setSubmittingDecision(null);
    }
  }

  async function handleCancelOrder(e: React.FormEvent) {
    e.preventDefault();
    if (!order) return;
    setActionError(null);
    setActionSuccess(null);

    setSubmittingCancel(true);
    try {
      const res = await cancelOrder(order.id, {
        reason: cancelReason.trim() || undefined,
        dp_disposition: cancelDpDisp,
        note: cancelNote.trim() || undefined,
      });
      setCancelResult(res);
      setShowCancelDialog(false);
      setActionSuccess(
        `Pesanan #${res.code} telah dibatalkan. Eksposur pembatalan: ${formatRupiah(res.cancellation_exposure)}.`
      );
      await loadDetail();
    } catch (err) {
      setActionError(getErrorMessage(err, "Gagal membatalkan pesanan."));
    } finally {
      setSubmittingCancel(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center rounded-2xl border border-stone-200 bg-white py-20 text-sm text-stone-500">
        <Loader2 className="mr-2 h-5 w-5 animate-spin text-amber-600" />
        Memuat detail pesanan...
      </div>
    );
  }

  if (errorMsg || !order) {
    return (
      <div className="space-y-4">
        <Link
          href="/app/pesanan"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-stone-600 hover:text-stone-900"
        >
          <ArrowLeft className="h-4 w-4" />
          Kembali ke Daftar Pesanan
        </Link>
        <div className="rounded-2xl border border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          {errorMsg || "Pesanan tidak ditemukan."}
        </div>
      </div>
    );
  }

  const totalNum = toNumber(order.total);
  const paidNum = toNumber(order.paid);
  const dpNum = toNumber(order.dp_required);
  const remainingNum = Math.max(0, totalNum - paidNum);
  const isFullyPaid = remainingNum <= 0 || (paidNum >= totalNum && totalNum > 0);
  const dpRemainingNum = Math.max(0, Math.round(dpNum - paidNum));
  const isDpUnpaid = paidNum < dpNum && dpRemainingNum > 0;

  const canRecordPayment =
    canManage &&
    ["accepted", "confirmed", "in_production", "ready", "delivering"].includes(
      order.status
    );

  const canCancel =
    canManage &&
    !["completed", "cancelled", "rejected", "expired"].includes(order.status);

  const impact = order.evaluation?.impact;

  return (
    <>
      <div className="space-y-6 print:hidden">
        {/* Navigasi Atas & Header */}
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <Link
              href="/app/pesanan"
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-stone-500 hover:text-stone-900"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              Kembali ke Daftar Pesanan
            </Link>
            <div className="mt-2 flex flex-wrap items-center gap-2.5">
              <h1 className="font-mono text-2xl font-extrabold text-stone-900">
                #{order.code}
              </h1>
              <OrderStatusBadge status={order.status} />
              <span className="rounded-full bg-stone-200/70 px-2.5 py-0.5 text-xs font-medium text-stone-700">
                {order.source === "manual" ? "Pesanan Manual" : "Pesanan Publik"}
              </span>
              {!isStaff && order.is_big_order && (
                <span className="rounded-full border border-purple-200 bg-purple-50 px-2.5 py-0.5 text-xs font-semibold text-purple-800">
                  Pesanan Besar
                </span>
              )}
            </div>
            <p className="mt-1 text-xs text-stone-500">
              Dibuat pada {formatDateTimeWIB(order.created_at)} • Terakhir
              diperbarui {formatDateTimeWIB(order.updated_at)}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {isStaff ? (
              <button
                type="button"
                onClick={() => window.print()}
                className="inline-flex items-center gap-1.5 rounded-xl border border-stone-200 bg-white px-3.5 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50 cursor-pointer shadow-2xs"
              >
                <Printer className="h-3.5 w-3.5 text-stone-600" />
                Cetak Lembar Kerja Dapur
              </button>
            ) : isFullyPaid ? (
              <button
                type="button"
                onClick={() => window.print()}
                className="inline-flex items-center gap-1.5 rounded-xl border border-emerald-300 bg-emerald-50 px-3.5 py-2 text-xs font-bold text-emerald-900 hover:bg-emerald-100 cursor-pointer shadow-2xs"
              >
                <Printer className="h-3.5 w-3.5 text-emerald-700" />
                Cetak Nota Lunas
              </button>
            ) : (
              <button
                type="button"
                disabled
                title={`Nota lunas baru dapat dicetak setelah pesanan lunas. Sisa tagihan: ${formatRupiah(remainingNum)}`}
                className="inline-flex items-center gap-1.5 rounded-xl border border-stone-200 bg-stone-100 px-3.5 py-2 text-xs font-semibold text-stone-400 cursor-not-allowed shadow-2xs"
              >
                <Printer className="h-3.5 w-3.5 text-stone-400" />
                Cetak Nota (Menunggu Pelunasan)
              </button>
            )}

            <button
              type="button"
              onClick={() => void loadDetail()}
              className="inline-flex items-center gap-1.5 rounded-xl border border-stone-200 bg-white px-3.5 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              Segarkan
            </button>

          {canCancel && (
            <button
              type="button"
              onClick={() => setShowCancelDialog(true)}
              className="inline-flex items-center gap-1.5 rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-2 text-xs font-bold text-rose-800 hover:bg-rose-100"
            >
              <Ban className="h-3.5 w-3.5" />
              Batalkan Pesanan
            </button>
          )}
        </div>
      </div>

      {actionSuccess && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
          <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-600" />
          <span>{actionSuccess}</span>
        </div>
      )}

      {actionError && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <AlertTriangle className="h-5 w-5 shrink-0 text-rose-600" />
          <span>{actionError}</span>
        </div>
      )}

      {/* Hasil Eksposur Pembatalan bila baru saja dibatalkan */}
      {cancelResult && (
        <div className="rounded-2xl border border-rose-200 bg-rose-50/70 p-5 text-sm text-rose-950">
          <h2 className="font-bold">Ringkasan Eksposur Pembatalan (BR-11)</h2>
          <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="rounded-xl bg-white/80 p-3">
              <p className="text-xs text-stone-500">Estimasi Belanja Bahan</p>
              <p className="text-base font-bold text-stone-900">
                {formatRupiah(cancelResult.shopping_cost)}
              </p>
            </div>
            <div className="rounded-xl bg-white/80 p-3">
              <p className="text-xs text-stone-500">DP / Pembayaran Diterima</p>
              <p className="text-base font-bold text-emerald-700">
                {formatRupiah(cancelResult.dp_received)}
              </p>
            </div>
            <div className="rounded-xl bg-white/80 p-3">
              <p className="text-xs text-stone-500">
                Eksposur Kerugian Pembatalan
              </p>
              <p className="text-base font-extrabold text-rose-700">
                {formatRupiah(cancelResult.cancellation_exposure)}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Timeline Siklus Pesanan */}
      <OrderTimeline status={order.status} />

      {/* Panel Transisi Status Operasional (Bagian 8.5) */}
      {(order.status === "confirmed" ||
        order.status === "in_production" ||
        order.status === "ready" ||
        order.status === "delivering") && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50/50 p-5 shadow-2xs">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div>
              <h2 className="text-sm font-bold text-stone-900">
                Tindakan Transisi Status Operasional
              </h2>
              <p className="text-xs text-stone-600">
                Sesuai aturan Bagian 8.5, ubah tahapan pesanan saat pekerjaan
                dapur atau pengantaran selesai.
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              {/* confirmed -> in_production (manager, owner) */}
              {order.status === "confirmed" && canManage && (
                <div className="flex flex-wrap items-center gap-2.5">
                  {order.tasks?.some((t) => t.type === "shop" && t.status !== "done") && (
                    <span className="inline-flex items-center gap-1.5 rounded-xl border border-amber-300 bg-amber-100/80 px-3 py-1.5 text-xs font-medium text-amber-900">
                      <AlertTriangle className="h-3.5 w-3.5 text-amber-700 shrink-0" />
                      Tugas Belanja belum selesai. Pastikan bahan sudah tersedia di dapur.
                    </span>
                  )}
                  <button
                    type="button"
                    disabled={submittingTransition}
                    onClick={() => handleTransition("in_production")}
                    className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-4 py-2 text-xs font-bold text-white shadow-xs hover:bg-indigo-700 disabled:opacity-50"
                  >
                    {submittingTransition ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Play className="h-3.5 w-3.5" />
                    )}
                    Mulai Produksi (In Production)
                  </button>
                </div>
              )}

              {order.status === "confirmed" && isStaff && (
                <span className="text-xs font-medium text-stone-500">
                  Menunggu Manajer/Owner memulai status Dalam Produksi.
                </span>
              )}

              {/* in_production -> ready (staff, manager, owner) */}
              {order.status === "in_production" && (
                <div className="flex flex-col gap-1.5 sm:flex-row sm:items-center">
                  <div className="flex flex-col">
                    <input
                      type="number"
                      min={1}
                      placeholder="Menit aktual (opsional)"
                      value={actualMinutes}
                      onChange={(e) => setActualMinutes(e.target.value)}
                      title="Waktu riil memasak di dapur (menit). Digunakan sistem untuk kalibrasi HPP tenaga kerja."
                      className="w-44 rounded-xl border border-stone-300 bg-white px-3 py-2 text-xs text-stone-900 placeholder:text-stone-400"
                    />
                    <span className="mt-0.5 text-[10px] text-stone-500">
                      ⏱ Waktu riil masak (evaluasi resep)
                    </span>
                  </div>
                  <button
                    type="button"
                    disabled={submittingTransition}
                    onClick={() => handleTransition("ready")}
                    className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white shadow-xs hover:bg-emerald-700 disabled:opacity-50"
                  >
                    {submittingTransition ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <CheckCircle2 className="h-3.5 w-3.5" />
                    )}
                    Tandai Produksi Selesai (Ready)
                  </button>
                </div>
              )}

              {/* ready -> delivering (staff, manager, owner) */}
              {order.status === "ready" && (
                <button
                  type="button"
                  disabled={submittingTransition}
                  onClick={() => handleTransition("delivering")}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-4 py-2 text-xs font-bold text-white shadow-xs hover:bg-sky-700 disabled:opacity-50"
                >
                  {submittingTransition ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Play className="h-3.5 w-3.5" />
                  )}
                  Mulai Pengantaran / Penyerahan (Delivering)
                </button>
              )}

              {/* ready/delivering -> completed (manager, owner; wajib lunas) */}
              {(order.status === "ready" || order.status === "delivering") &&
                canManage && (
                  <button
                    type="button"
                    disabled={submittingTransition || !isFullyPaid}
                    onClick={() => handleTransition("completed")}
                    title={
                      !isFullyPaid
                        ? "Pembayaran harus lunas terlebih dahulu sebelum pesanan diselesaikan"
                        : undefined
                    }
                    className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-700 px-4 py-2 text-xs font-bold text-white shadow-xs hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {submittingTransition ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <CheckCircle2 className="h-3.5 w-3.5" />
                    )}
                    Selesaikan Pesanan (Completed)
                  </button>
                )}
            </div>
          </div>
          {(order.status === "ready" || order.status === "delivering") &&
            canManage &&
            !isFullyPaid && (
              <p className="mt-2 text-xs font-medium text-amber-800">
                Catatan: Pesanan belum dapat ditandai <strong>Selesai</strong>{" "}
                karena masih ada sisa pembayaran sebesar{" "}
                <strong>{formatRupiah(remainingNum)}</strong>.
              </p>
            )}
        </div>
      )}

      {/* Panel Keputusan bila masih pending_decision (hanya untuk Owner/Manager) */}
      {!isStaff && order.status === "pending_decision" && canManage && (
        <div className="rounded-2xl border border-amber-300 bg-amber-50/70 p-5 shadow-xs">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-base font-bold text-stone-900">
              Pesanan Menunggu Keputusan
            </h2>
            {order.recommendation && (
              <span className="rounded-full border border-amber-300 bg-white px-3 py-0.5 text-xs font-bold text-amber-900">
                {RECOMMENDATION_META[order.recommendation]?.label ??
                  order.recommendation}
              </span>
            )}
          </div>
          <p className="mt-1 text-xs text-stone-700">
            {order.recommendation_reason}
          </p>

          {order.can_decide ? (
            <div className="mt-4 space-y-3">
              {order.recommendation !== "accept" && (
                <div>
                  <label className="block text-xs font-bold text-amber-950">
                    Alasan Override (Wajib diisi bila memilih Terima)
                  </label>
                  <input
                    type="text"
                    value={overrideReason}
                    onChange={(e) => setOverrideReason(e.target.value)}
                    placeholder="Contoh: Disetujui lembur & bahan tambahan sudah siap"
                    className="mt-1 w-full rounded-xl border border-amber-300 bg-white px-3 py-2 text-xs text-stone-900"
                  />
                </div>
              )}

              <div className="flex flex-wrap items-center gap-3">
                <div className="flex items-center gap-2">
                  <label className="text-xs font-semibold text-stone-700">
                    Tanggal Tawaran Balik:
                  </label>
                  <input
                    type="date"
                    value={counterDate}
                    onChange={(e) => setCounterDate(e.target.value)}
                    className="rounded-lg border border-stone-300 bg-white px-2.5 py-1.5 text-xs text-stone-900"
                  />
                </div>

                <div className="ml-auto flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    disabled={submittingDecision !== null}
                    onClick={() => handleQuickDecision("reject")}
                    className="rounded-xl border border-rose-200 bg-rose-50 px-3.5 py-2 text-xs font-bold text-rose-800 hover:bg-rose-100"
                  >
                    Tolak
                  </button>
                  <button
                    type="button"
                    disabled={submittingDecision !== null}
                    onClick={() => handleQuickDecision("counter")}
                    className="rounded-xl border border-amber-300 bg-amber-100 px-3.5 py-2 text-xs font-bold text-amber-950 hover:bg-amber-200"
                  >
                    Tawar Balik
                  </button>
                  <button
                    type="button"
                    disabled={submittingDecision !== null}
                    onClick={() => handleQuickDecision("accept")}
                    className="rounded-xl bg-emerald-600 px-4 py-2 text-xs font-bold text-white hover:bg-emerald-700"
                  >
                    Terima Pesanan
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <p className="mt-3 text-xs font-semibold text-purple-900">
              Pesanan ini tergolong Pesanan Besar sehingga hanya dapat
              diputuskan oleh Pemilik Usaha (Owner).
            </p>
          )}
        </div>
      )}

      {/* Grid Informasi Pelanggan & Ringkasan Item */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Informasi Pelanggan & Pengiriman */}
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
          <h2 className="text-sm font-bold text-stone-900">
            Informasi Pelanggan & Jadwal
          </h2>
          <dl className="mt-4 space-y-3 text-xs">
            <div>
              <dt className="text-stone-400">Nama Pelanggan</dt>
              <dd className="mt-0.5 font-semibold text-stone-900">
                {order.customer_name}
              </dd>
            </div>
            <div>
              <dt className="text-stone-400">Kontak</dt>
              <dd className="mt-0.5 font-medium text-stone-800">
                {order.customer_contact}
              </dd>
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <dt className="text-stone-400">Tanggal Acara</dt>
                <dd className="mt-0.5 font-semibold text-stone-900">
                  {formatDateLong(order.event_date)}
                </dd>
              </div>
              <div>
                <dt className="text-stone-400">Tanggal Produksi</dt>
                <dd className="mt-0.5 font-semibold text-stone-900">
                  {formatDateShort(order.production_date)}
                </dd>
              </div>
            </div>
            <div>
              <dt className="text-stone-400">Metode Penyerahan</dt>
              <dd className="mt-0.5 font-semibold text-stone-900">
                {order.delivery_type === "delivery"
                  ? "Diantar Kurir (Delivery)"
                  : "Diambil Sendiri (Pickup)"}
              </dd>
            </div>
            {order.address && (
              <div>
                <dt className="text-stone-400">Alamat Pengantaran</dt>
                <dd className="mt-0.5 text-stone-800">{order.address}</dd>
              </div>
            )}
            {order.notes && (
              <div>
                <dt className="text-stone-400">Catatan</dt>
                <dd className="mt-0.5 whitespace-pre-line rounded-lg bg-stone-50 p-2.5 text-stone-700">
                  {order.notes}
                </dd>
              </div>
            )}
            {order.override_reason && !isStaff && (
              <div>
                <dt className="text-amber-700 font-semibold">
                  Alasan Override Keputusan
                </dt>
                <dd className="mt-0.5 rounded-lg border border-amber-200 bg-amber-50 p-2.5 text-amber-950">
                  {order.override_reason}
                </dd>
              </div>
            )}
          </dl>
        </div>

        {/* Tabel Item Pesanan */}
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs lg:col-span-2">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-bold text-stone-900">
              Rincian Item Pesanan ({order.total_units} unit)
            </h2>
            {!isStaff && (
              <span className="text-base font-extrabold text-stone-900">
                Total: {formatRupiah(order.total)}
              </span>
            )}
          </div>

          <div className="mt-4 overflow-x-auto rounded-xl border border-stone-200">
            <table className="min-w-full divide-y divide-stone-200 text-xs">
              <thead className="bg-stone-50 text-left font-bold text-stone-600">
                <tr>
                  <th className="px-4 py-2.5">Nama Produk</th>
                  <th className="px-4 py-2.5 text-right">Kuantitas</th>
                  {!isStaff && (
                    <>
                      <th className="px-4 py-2.5 text-right">Harga Satuan</th>
                      <th className="px-4 py-2.5 text-right">HPP / Unit</th>
                      <th className="px-4 py-2.5 text-right">Subtotal</th>
                    </>
                  )}
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100 bg-white text-stone-800">
                {order.items.map((it) => (
                  <tr key={it.id}>
                    <td className="px-4 py-2.5 font-semibold text-stone-900">
                      {it.product_name}
                    </td>
                    <td className="px-4 py-2.5 text-right font-bold">
                      {it.qty} unit
                    </td>
                    {!isStaff && (
                      <>
                        <td className="px-4 py-2.5 text-right">
                          {formatRupiah(it.unit_price)}
                        </td>
                        <td className="px-4 py-2.5 text-right text-stone-500">
                          {formatRupiah(it.unit_cost)}
                        </td>
                        <td className="px-4 py-2.5 text-right font-bold text-stone-900">
                          {formatRupiah(it.subtotal)}
                        </td>
                      </>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {!isStaff && (
            <div className="mt-3 flex flex-col items-end gap-1 text-xs text-stone-600">
              {((order.tax_amount && toNumber(order.tax_amount) > 0) ||
                (order.platform_fee && toNumber(order.platform_fee) > 0)) ? (
                <>
                  <div className="flex gap-4">
                    <span>Subtotal Produk:</span>
                    <span className="font-semibold text-stone-900">
                      {formatRupiah(order.subtotal ?? 0)}
                    </span>
                  </div>
                  {order.tax_amount && toNumber(order.tax_amount) > 0 ? (
                    <div className="flex gap-4">
                      <span>
                        PPN ({Math.round(toNumber(order.tax_pct ?? 0) * 100)}%):
                      </span>
                      <span className="font-semibold text-stone-900">
                        {formatRupiah(order.tax_amount)}
                      </span>
                    </div>
                  ) : null}
                  {order.platform_fee && toNumber(order.platform_fee) > 0 ? (
                    <div className="flex gap-4">
                      <span>Biaya Layanan Aplikasi:</span>
                      <span className="font-semibold text-stone-900">
                        {formatRupiah(order.platform_fee)}
                      </span>
                    </div>
                  ) : null}
                </>
              ) : null}
              <div className="flex gap-4 border-t border-stone-200 pt-1 text-sm font-bold text-stone-900">
                <span>Total Tagihan:</span>
                <span>{formatRupiah(order.total)}</span>
              </div>
            </div>
          )}

          {/* Evaluasi 4 Gerbang & Dampak (Hanya untuk Non-Staff) */}
          {!isStaff && order.evaluation && (
            <div className="mt-6 space-y-4 border-t border-stone-100 pt-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="flex items-center gap-1.5 text-xs font-bold tracking-wider text-stone-500 uppercase">
                  <ShieldCheck className="h-4 w-4 text-amber-600" />
                  Hasil Evaluasi Gerbang G1–G4 & Dampak
                </h3>
                {order.recommendation && (
                  <span className="text-xs font-semibold text-stone-600">
                    {RECOMMENDATION_META[order.recommendation]?.label ??
                      order.recommendation}
                  </span>
                )}
              </div>

              <GateBadges gates={order.evaluation.gates} />

              {impact && (
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3">
                    <p className="text-[11px] font-medium text-stone-500">
                      Utilisasi Kapasitas
                    </p>
                    <div className="mt-1 flex items-center gap-1.5 text-sm font-bold text-stone-900">
                      <span>{formatPercent(impact.utilization_before)}</span>
                      <ArrowRight className="h-3.5 w-3.5 text-stone-400" />
                      <span className="text-amber-700">
                        {formatPercent(impact.utilization_after)}
                      </span>
                    </div>
                    {impact.bottleneck && (
                      <p className="mt-0.5 text-[11px] text-stone-500">
                        Bottleneck:{" "}
                        {RESOURCE_LABELS[impact.bottleneck] ?? impact.bottleneck}
                      </p>
                    )}
                  </div>

                  <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3">
                    <p className="text-[11px] font-medium text-stone-500">
                      Margin Kotor Pesanan
                    </p>
                    <p className="mt-1 text-sm font-bold text-stone-900">
                      {formatPercent(impact.margin)}
                    </p>
                  </div>

                  <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3">
                    <p className="text-[11px] font-medium text-stone-500">
                      Titik Terendah Kas
                    </p>
                    <div className="mt-1 flex flex-wrap items-center gap-1 text-sm font-bold text-stone-900">
                      <span>{formatRupiah(impact.cash_lowest_before)}</span>
                      <ArrowRight className="h-3.5 w-3.5 text-stone-400" />
                      <span className="text-amber-700">
                        {formatRupiah(impact.cash_lowest_after)}
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Bagian Pembayaran (Disembunyikan sepenuhnya untuk Staff) */}
      {!isStaff && (
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
          <div className="flex flex-wrap items-center justify-between gap-4 border-b border-stone-100 pb-4">
            <div className="flex items-center gap-2">
              <CreditCard className="h-4 w-4 text-amber-600" />
              <h2 className="text-sm font-bold text-stone-900">
                Pembayaran & Tagihan DP
              </h2>
            </div>
            {order.dp_deadline && order.status === "accepted" && (
              <span className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-900">
                Tenggat DP: {formatDateTimeWIB(order.dp_deadline)}
              </span>
            )}
          </div>

          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-4">
            <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3.5">
              <p className="text-xs text-stone-500">Total Tagihan</p>
              <p className="mt-1 text-base font-extrabold text-stone-900">
                {formatRupiah(order.total)}
              </p>
            </div>
            <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3.5">
              <p className="text-xs text-stone-500">Tagihan DP Minimum</p>
              <p className="mt-1 text-base font-bold text-amber-800">
                {formatRupiah(order.dp_required)}
              </p>
            </div>
            <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3.5">
              <p className="text-xs text-stone-500">Total Sudah Dibayar</p>
              <p className="mt-1 text-base font-bold text-emerald-700">
                {formatRupiah(order.paid)}
              </p>
            </div>
            <div className="rounded-xl border border-stone-200 bg-stone-50/60 p-3.5">
              <p className="text-xs text-stone-500">Sisa Pelunasan</p>
              <p className="mt-1 text-base font-bold text-stone-900">
                {formatRupiah(remainingNum)}
              </p>
            </div>
          </div>

          <div className="mt-5 grid grid-cols-1 gap-6 lg:grid-cols-2">
            {/* Riwayat Pembayaran */}
            <div>
              <h3 className="text-xs font-bold tracking-wider text-stone-500 uppercase">
                Riwayat Pembayaran ({order.payments?.length ?? 0})
              </h3>
              {!order.payments || order.payments.length === 0 ? (
                <p className="mt-2 rounded-xl border border-dashed border-stone-200 p-4 text-xs text-stone-500">
                  Belum ada pembayaran yang tercatat untuk pesanan ini.
                </p>
              ) : (
                <div className="mt-2 space-y-2">
                  {order.payments.map((pm) => (
                    <div
                      key={pm.id}
                      className="flex items-center justify-between rounded-xl border border-stone-200 bg-stone-50/50 px-3.5 py-2.5 text-xs"
                    >
                      <div>
                        <p className="font-bold text-stone-900">
                          {formatRupiah(pm.amount)}{" "}
                          <span className="font-normal text-stone-500">
                            ({pm.method})
                          </span>
                        </p>
                        {pm.note && (
                          <p className="text-[11px] text-stone-600">{pm.note}</p>
                        )}
                      </div>
                      <span className="text-[11px] text-stone-500">
                        {formatDateTimeWIB(pm.paid_at)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Form Catat Pembayaran Baru / Indikator Lunas */}
            {isFullyPaid ? (
              <div className="flex flex-col justify-center rounded-xl border border-emerald-200 bg-emerald-50/70 p-4 text-emerald-900">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600" />
                  <p className="text-xs font-bold">Pesanan ini sudah lunas</p>
                </div>
                <p className="mt-1 text-xs text-emerald-700">
                  Seluruh tagihan pesanan sebesar {formatRupiah(order.total)} telah
                  dibayar penuh (Sisa Pelunasan Rp 0).
                </p>
              </div>
            ) : (
              canRecordPayment && (
                <form
                  onSubmit={handleRecordPayment}
                  className="rounded-xl border border-stone-200 bg-stone-50/60 p-4"
                >
                  <h3 className="text-xs font-bold tracking-wider text-stone-700 uppercase">
                    Catat Pembayaran Baru
                  </h3>

                  {/* Quick Fill Buttons */}
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() =>
                        setPayAmount(String(Math.round(remainingNum)))
                      }
                      className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-2.5 py-1.5 text-[11px] font-semibold text-emerald-800 hover:bg-emerald-100"
                    >
                      Bayar Sisa Tagihan (Lunas) ({formatRupiah(remainingNum)})
                    </button>
                    {isDpUnpaid && (
                      <button
                        type="button"
                        onClick={() => setPayAmount(String(dpRemainingNum))}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-[11px] font-semibold text-amber-800 hover:bg-amber-100"
                      >
                        Uang Muka/Nyicil ({formatRupiah(dpRemainingNum)})
                      </button>
                    )}
                  </div>

                  <div className="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <div>
                      <label className="block text-xs font-semibold text-stone-700">
                        Nominal Pembayaran (Rp) *
                      </label>
                      <input
                        type="number"
                        min={1}
                        required
                        value={payAmount}
                        onChange={(e) => setPayAmount(e.target.value)}
                        className="mt-1 w-full rounded-lg border border-stone-300 bg-white px-3 py-1.5 text-xs text-stone-900"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-semibold text-stone-700">
                        Metode Pembayaran
                      </label>
                      <select
                        value={payMethod}
                        onChange={(e) => setPayMethod(e.target.value)}
                        className="mt-1 w-full rounded-lg border border-stone-300 bg-white px-3 py-1.5 text-xs text-stone-900"
                      >
                        <option value="transfer">Transfer Bank</option>
                        <option value="qris">QRIS</option>
                        <option value="cash">Tunai (Cash)</option>
                      </select>
                    </div>
                  </div>
                  <div className="mt-2.5">
                    <label className="block text-xs font-semibold text-stone-700">
                      Catatan Pembayaran (Opsional)
                    </label>
                    <input
                      type="text"
                      value={payNote}
                      onChange={(e) => setPayNote(e.target.value)}
                      placeholder="Mis. DP 30% via BCA / Pelunasan"
                      className="mt-1 w-full rounded-lg border border-stone-300 bg-white px-3 py-1.5 text-xs text-stone-900"
                    />
                  </div>
                  <div className="mt-3 flex justify-end">
                    <button
                      type="submit"
                      disabled={submittingPay}
                      className="inline-flex items-center gap-1.5 rounded-xl bg-amber-600 px-4 py-2 text-xs font-bold text-white hover:bg-amber-700 disabled:opacity-50"
                    >
                      {submittingPay && (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      )}
                      Simpan Pembayaran
                    </button>
                  </div>
                </form>
              )
            )}
          </div>
        </div>
      )}

      {/* Tugas Operasional & Riwayat Audit Ringkas */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Daftar Tugas Operasional */}
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
          <div className="flex items-center justify-between">
            <h2 className="flex items-center gap-2 text-sm font-bold text-stone-900">
              <ClipboardList className="h-4 w-4 text-amber-600" />
              Tugas Operasional Pesanan ({order.tasks?.length ?? 0})
            </h2>
            <Link
              href="/app/karyawan"
              className="text-xs font-semibold text-amber-700 hover:text-amber-800"
            >
              Buka Halaman Tugas →
            </Link>
          </div>

          {!order.tasks || order.tasks.length === 0 ? (
            <p className="mt-4 rounded-xl border border-dashed border-stone-200 p-4 text-xs text-stone-500">
              Tugas operasional (belanja, produksi, penyerahan) akan otomatis
              dibuat setelah pembayaran DP terpenuhi (status Terkonfirmasi).
            </p>
          ) : (
            <div className="mt-4 space-y-2.5">
              {order.tasks.map((tk) => {
                const stMeta = TASK_STATUS_LABELS[tk.status] ?? {
                  label: tk.status,
                  tone: "slate",
                };
                return (
                  <div
                    key={tk.id}
                    className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-stone-200 bg-stone-50/50 p-3 text-xs"
                  >
                    <div>
                      <span className="font-bold text-stone-900">
                        {TASK_TYPE_LABELS[tk.type] ?? tk.type}
                      </span>
                      <p className="text-stone-600">{tk.title}</p>
                      <p className="mt-0.5 text-[11px] text-stone-500">
                        Tenggat: {formatDateShort(tk.due_date)}
                        {tk.actual_minutes
                          ? ` • Durasi aktual: ${tk.actual_minutes} menit`
                          : ""}
                      </p>
                    </div>
                    <span
                      className={`rounded-full px-2.5 py-0.5 text-[11px] font-bold ${
                        stMeta.tone === "emerald"
                          ? "bg-emerald-100 text-emerald-800"
                          : stMeta.tone === "amber"
                            ? "bg-amber-100 text-amber-800"
                            : "bg-stone-200 text-stone-700"
                      }`}
                    >
                      {stMeta.label}
                    </span>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Riwayat Audit Ringkas */}
        <div className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs">
          <h2 className="flex items-center gap-2 text-sm font-bold text-stone-900">
            <History className="h-4 w-4 text-amber-600" />
            Jejak Audit Pesanan ({order.audits?.length ?? 0})
          </h2>

          {!order.audits || order.audits.length === 0 ? (
            <p className="mt-4 rounded-xl border border-dashed border-stone-200 p-4 text-xs text-stone-500">
              Belum ada catatan audit untuk pesanan ini.
            </p>
          ) : (
            <div className="mt-4 max-h-72 space-y-2.5 overflow-y-auto pr-1">
              {order.audits.map((au) => {
                const op =
                  typeof au.detail?.op === "string" ? au.detail.op : au.action;
                const fromSt =
                  typeof au.detail?.from_status === "string"
                    ? au.detail.from_status
                    : "";
                const toSt =
                  typeof au.detail?.to_status === "string"
                    ? au.detail.to_status
                    : "";
                return (
                  <div
                    key={au.id}
                    className="rounded-xl border border-stone-200 bg-stone-50/50 p-3 text-xs"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-bold text-stone-900 uppercase">
                        {au.action} • {op}
                      </span>
                      <span className="text-[11px] text-stone-500">
                        {formatDateTimeWIB(au.created_at)}
                      </span>
                    </div>
                    <p className="mt-1 text-stone-600">
                      Pelaku:{" "}
                      <strong>{au.actor_name || "Sistem / Pelanggan"}</strong>
                      {fromSt || toSt
                        ? ` • Status: ${fromSt || "-"} → ${toSt || "-"}`
                        : ""}
                    </p>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* Dialog Konfirmasi Batalkan Pesanan */}
      {showCancelDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-950/50 p-4">
          <div className="w-full max-w-md rounded-2xl border border-stone-200 bg-white p-6 shadow-xl">
            <h2 className="text-base font-bold text-stone-900">
              Konfirmasi Pembatalan Pesanan #{order.code}
            </h2>
            <p className="mt-1 text-xs text-stone-600">
              Membatalkan pesanan akan melepas reservasi kapasitas dapur,
              membatalkan tugas aktif yang belum selesai, dan menghitung
              eksposur pembatalan (biaya belanja dikurangi DP diterima).
            </p>

            <form onSubmit={handleCancelOrder} className="mt-4 space-y-3">
              <div>
                <label className="block text-xs font-semibold text-stone-700">
                  Alasan Pembatalan
                </label>
                <input
                  type="text"
                  required
                  value={cancelReason}
                  onChange={(e) => setCancelReason(e.target.value)}
                  placeholder="Contoh: Acara pelanggan dibatalkan"
                  className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-xs text-stone-900"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-stone-700">
                  Keputusan atas DP yang Sudah Diterima
                </label>
                <select
                  value={cancelDpDisp}
                  onChange={(e) =>
                    setCancelDpDisp(
                      e.target.value as "forfeited" | "refunded" | "pending"
                    )
                  }
                  className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-xs text-stone-900"
                >
                  <option value="forfeited">DP Hangus (Forfeited)</option>
                  <option value="refunded">DP Dikembalikan (Refunded)</option>
                  <option value="pending">Belum Diputuskan (Pending)</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-semibold text-stone-700">
                  Catatan Tambahan (Opsional)
                </label>
                <input
                  type="text"
                  value={cancelNote}
                  onChange={(e) => setCancelNote(e.target.value)}
                  placeholder="Catatan kesepakatan dengan pelanggan"
                  className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-xs text-stone-900"
                />
              </div>

              <div className="flex justify-end gap-2 pt-3">
                <button
                  type="button"
                  onClick={() => setShowCancelDialog(false)}
                  className="rounded-xl border border-stone-200 px-4 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50"
                >
                  Kembali
                </button>
                <button
                  type="submit"
                  disabled={submittingCancel}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-rose-600 px-4 py-2 text-xs font-bold text-white hover:bg-rose-700 disabled:opacity-50"
                >
                  {submittingCancel && (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  )}
                  Ya, Batalkan Pesanan
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>

    {/* Nota Invoice Cetak Resmi (@media print) */}
    <OrderInvoicePrint
      order={order}
      hideFinancials={isStaff}
    />
  </>
  );
}
