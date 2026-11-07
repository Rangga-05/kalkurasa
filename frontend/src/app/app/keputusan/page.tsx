"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardCheck,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { DecisionCard } from "@/components/internal/decision-card";
import { getDecisionQueue, getErrorMessage } from "@/lib/api";
import type { DecisionQueueItem, OrderDetail } from "@/lib/types";

export default function DecisionQueuePage() {
  const { user } = useInternalSession();
  const [items, setItems] = useState<DecisionQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [banner, setBanner] = useState<{
    code: string;
    orderId: string;
    action: "accept" | "counter" | "reject";
  } | null>(null);

  const loadQueue = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const data = await getDecisionQueue();
      setItems(data);
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat antrian keputusan."));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadQueue();
  }, [loadQueue]);

  function handleDecided(
    updated: OrderDetail,
    action: "accept" | "counter" | "reject"
  ) {
    setBanner({ code: updated.code, orderId: updated.id, action });
    setItems((prev) => prev.filter((it) => it.id !== updated.id));
  }

  const actionLabel: Record<"accept" | "counter" | "reject", string> = {
    accept: "disetujui (menunggu pembayaran DP)",
    counter: "dikirimkan tawaran balik (counter-offer)",
    reject: "ditolak",
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
            <ClipboardCheck className="h-3.5 w-3.5" />
            Mesin Pendukung Keputusan (G1–G4)
          </span>
          <h1 className="mt-2 text-2xl font-bold text-stone-900">
            Antrian Keputusan Pesanan
          </h1>
          <p className="mt-1 text-sm text-stone-600">
            Tinjau hasil evaluasi kapasitas, margin, arus kas, dan lead time
            sebelum menyetujui, menawar balik, atau menolak pesanan.
          </p>
        </div>

        <button
          type="button"
          onClick={() => void loadQueue()}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-4 py-2 text-xs font-semibold text-stone-700 shadow-2xs transition hover:bg-stone-50 disabled:opacity-50"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          Muat Ulang ({items.length})
        </button>
      </div>

      {user?.role === "manager" && (
        <div className="rounded-2xl border border-stone-200 bg-white p-4 text-xs text-stone-600 shadow-2xs">
          Anda masuk sebagai <strong>Manajer Operasional</strong>. Anda dapat
          memutuskan pesanan reguler, sedangkan <strong>Pesanan Besar</strong>{" "}
          (di atas ambang nilai atau jumlah unit) memerlukan keputusan{" "}
          <strong>Pemilik Usaha (Owner)</strong>.
        </div>
      )}

      {banner && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
          <div className="flex items-center gap-2.5">
            <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-600" />
            <span>
              Pesanan <strong>#{banner.code}</strong> berhasil{" "}
              {actionLabel[banner.action]}.
            </span>
          </div>
          <Link
            href={`/app/pesanan/${banner.orderId}`}
            className="text-xs font-bold text-emerald-800 underline hover:text-emerald-950"
          >
            Lihat Detail Pesanan →
          </Link>
        </div>
      )}

      {errorMsg && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <AlertTriangle className="h-5 w-5 shrink-0 text-rose-600" />
          <span>{errorMsg}</span>
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center rounded-2xl border border-stone-200 bg-white py-16 text-sm text-stone-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin text-amber-600" />
          Memuat antrian keputusan dan menghitung pratinjau dampak terkini...
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-2xl border border-stone-200 bg-white p-12 text-center shadow-xs">
          <CheckCircle2 className="mx-auto h-10 w-10 text-emerald-500" />
          <h2 className="mt-3 text-base font-bold text-stone-900">
            Tidak Ada Pesanan yang Menunggu Keputusan
          </h2>
          <p className="mx-auto mt-1 max-w-md text-xs text-stone-500">
            Seluruh pesanan yang masuk sudah diputuskan atau otomatis diterima
            oleh sistem karena memenuhi keempat gerbang kelayakan.
          </p>
          <div className="mt-5">
            <Link
              href="/app/pesanan"
              className="inline-flex items-center gap-2 rounded-xl bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-amber-700"
            >
              Lihat Semua Daftar Pesanan
            </Link>
          </div>
        </div>
      ) : (
        <div className="space-y-5">
          {items.map((item) => (
            <DecisionCard
              key={item.id}
              item={item}
              onDecided={handleDecided}
            />
          ))}
        </div>
      )}
    </div>
  );
}

