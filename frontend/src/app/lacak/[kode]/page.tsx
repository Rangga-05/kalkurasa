"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowLeft,
  Calendar,
  CheckCircle2,
  Copy,
  CreditCard,
  Info,
  MapPin,
  Package,
  Printer,
  RefreshCw,
  ShoppingBag,
  Truck,
  XCircle,
} from "lucide-react";
import {
  trackPublicOrder,
  respondCounterOffer,
  getErrorMessage,
} from "@/lib/api";
import {
  formatDateLong,
  formatDateTimeWIB,
  formatRupiah,
  getOrderStatusMeta,
  toNumber,
} from "@/lib/format";
import { PublicNavbar } from "@/components/public-navbar";
import { OrderTimeline } from "@/components/order-timeline";
import { OrderInvoicePrint } from "@/components/order-invoice-print";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/states";

export default function TrackOrderDetailPage({
  params,
}: {
  params: Promise<{ kode: string }>;
}) {
  const { kode } = use(params);
  const normalizedCode = decodeURIComponent(kode).trim().toUpperCase();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [copied, setCopied] = useState(false);

  const orderQuery = useQuery({
    queryKey: ["public-order", normalizedCode],
    queryFn: ({ signal }) => trackPublicOrder(normalizedCode, signal),
    enabled: Boolean(normalizedCode),
  });

  const counterMutation = useMutation({
    mutationFn: (accept: boolean) =>
      respondCounterOffer(normalizedCode, { accept }),
    onSuccess: (updated, accept) => {
      queryClient.setQueryData(["public-order", normalizedCode], updated);
      if (accept) {
        toast.success(
          "Tawaran Balik Disetujui",
          `Pesanan #${updated.code} kini berstatus ${getOrderStatusMeta(updated.status).label}.`
        );
      } else {
        toast.info(
          "Tawaran Balik Ditolak",
          `Pesanan #${updated.code} telah dibatalkan/ditolak sesuai pilihan Anda.`
        );
      }
    },
    onError: (err) => {
      toast.error("Gagal Mengirim Tanggapan", getErrorMessage(err));
    },
  });

  const handleCopyCode = async () => {
    try {
      await navigator.clipboard.writeText(normalizedCode);
      setCopied(true);
      toast.info("Kode Disalin", `Kode pesanan ${normalizedCode} disalin.`);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Abaikan jika clipboard tidak diizinkan
    }
  };

  const order = orderQuery.data;
  const statusMeta = order ? getOrderStatusMeta(order.status) : null;

  const totalNum = toNumber(order?.total);
  const dpRequiredNum = toNumber(order?.dp_required);
  const paidNum = toNumber(order?.paid);
  const remainingDP = Math.max(0, dpRequiredNum - paidNum);
  const remainingTotal = Math.max(0, totalNum - paidNum);
  const isFullyPaid = totalNum > 0 && remainingTotal <= 0;

  const productNameById = new Map<string, string>();
  if (order) {
    for (const item of order.items) {
      productNameById.set(item.product_id, item.product_name);
    }
  }

  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />

      <main className="mx-auto w-full max-w-4xl flex-1 space-y-6 px-4 py-6 sm:px-6 sm:py-8 print:p-0 print:m-0 print:max-w-none print:w-full">
        <div className="flex flex-wrap items-center justify-between gap-2 print:hidden">
          <Link
            href="/lacak"
            className="inline-flex items-center gap-1.5 text-xs font-medium text-stone-600 hover:text-stone-900 sm:text-sm"
          >
            <ArrowLeft className="h-4 w-4" />
            Cari Kode Pesanan Lain
          </Link>

          {orderQuery.data ? (
            <div className="flex items-center gap-2">
              {isFullyPaid ? (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => window.print()}
                  className="gap-1.5 cursor-pointer border-emerald-300 bg-emerald-50 text-emerald-900 hover:bg-emerald-100 font-semibold"
                >
                  <Printer className="h-3.5 w-3.5 text-emerald-700" />
                  Cetak Nota Lunas
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled
                  title={`Nota lunas resmi baru dapat dicetak setelah sisa pembayaran (${formatRupiah(remainingTotal)}) dilunasi.`}
                  className="gap-1.5 opacity-60 cursor-not-allowed text-stone-500"
                >
                  <Printer className="h-3.5 w-3.5 text-stone-400" />
                  Cetak Nota (Tersedia Setelah Lunas)
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => orderQuery.refetch()}
                disabled={orderQuery.isFetching}
              >
                <RefreshCw
                  className={`h-3.5 w-3.5 ${orderQuery.isFetching ? "animate-spin" : ""}`}
                />
                Muat Ulang Status
              </Button>
            </div>
          ) : null}
        </div>

        {/* Loading Skeleton */}
        {orderQuery.isLoading ? (
          <div className="space-y-4">
            <Card className="p-6">
              <div className="space-y-3">
                <Skeleton className="h-5 w-32" />
                <Skeleton className="h-8 w-64" />
                <Skeleton className="h-4 w-full max-w-md" />
              </div>
            </Card>
            <Card className="p-6">
              <Skeleton className="h-40 w-full" />
            </Card>
          </div>
        ) : null}

        {/* Error / 404 */}
        {orderQuery.isError ? (
          <ErrorState
            title="Pesanan Tidak Ditemukan"
            message={getErrorMessage(
              orderQuery.error,
              "Kode pesanan tidak ditemukan. Pastikan 10 karakter kode sudah benar."
            )}
            onRetry={() => orderQuery.refetch()}
          />
        ) : null}

        {/* Data Pesanan */}
        {order && statusMeta ? (
          <>
            <div className="space-y-6 print:hidden">
              {/* Header Ringkasan */}
            <Card className="border-stone-200/90">
              <CardHeader className="pb-4">
                <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-xs font-semibold uppercase tracking-wider text-stone-500">
                        Kode Pesanan
                      </span>
                      <button
                        type="button"
                        onClick={handleCopyCode}
                        className="inline-flex items-center gap-1.5 rounded-md bg-stone-100 px-2.5 py-1 font-mono text-sm font-bold tracking-wider text-stone-900 hover:bg-stone-200 cursor-pointer"
                        title="Salin kode pesanan"
                      >
                        {order.code}
                        <Copy className="h-3.5 w-3.5 text-stone-500" />
                      </button>
                      {copied ? (
                        <span className="text-xs text-emerald-700">
                          Tersalin!
                        </span>
                      ) : null}
                    </div>
                    <CardTitle className="pt-1 text-xl sm:text-2xl">
                      Pesanan a.n. {order.customer_name}
                    </CardTitle>
                    <CardDescription>
                      Dibuat pada {formatDateTimeWIB(order.created_at)} ·
                      Diperbarui {formatDateTimeWIB(order.updated_at)}
                    </CardDescription>
                  </div>

                  <Badge
                    variant={statusMeta.tone}
                    className="self-start px-3 py-1 text-xs sm:text-sm"
                  >
                    {statusMeta.label}
                  </Badge>
                </div>
              </CardHeader>

              <CardContent className="space-y-4">
                <div className="rounded-xl border border-stone-200 bg-stone-50/80 p-3.5 text-xs text-stone-700 sm:text-sm">
                  {statusMeta.description}
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                  <div className="flex items-start gap-2.5 rounded-xl border border-stone-200 p-3">
                    <Calendar className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                    <div>
                      <p className="text-xs text-stone-500">Tanggal Acara</p>
                      <p className="text-sm font-semibold text-stone-900">
                        {formatDateLong(order.event_date)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-start gap-2.5 rounded-xl border border-stone-200 p-3">
                    {order.delivery_type === "delivery" ? (
                      <Truck className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                    ) : (
                      <MapPin className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                    )}
                    <div>
                      <p className="text-xs text-stone-500">
                        Metode Pengambilan
                      </p>
                      <p className="text-sm font-semibold text-stone-900">
                        {order.delivery_type === "delivery"
                          ? "Diantar ke Alamat"
                          : "Ambil Sendiri di Dapur"}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-start gap-2.5 rounded-xl border border-stone-200 p-3">
                    <CreditCard className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                    <div>
                      <p className="text-xs text-stone-500">Total Pesanan</p>
                      <p className="text-sm font-bold text-amber-900">
                        {formatRupiah(order.total)}
                      </p>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Kartu Tawaran Balik (Bila status === 'countered' dan counter_offer tersedia) */}
            {order.status === "countered" && order.counter_offer ? (
              <Card className="border-amber-300 bg-amber-50/70 shadow-sm">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-2 text-amber-900">
                    <AlertTriangle className="h-5 w-5 shrink-0 text-amber-700" />
                    <CardTitle className="text-base sm:text-lg">
                      Tawaran Balik dari Kalkurasa — Menunggu Keputusan Anda
                    </CardTitle>
                  </div>
                  <CardDescription className="text-amber-900/80">
                    Karena kapasitas produksi pada jadwal awal sudah penuh atau
                    memerlukan penyesuaian, kami menawarkan jadwal/ketentuan
                    berikut agar pesanan Anda tetap dapat diproduksi dengan
                    kualitas terbaik:
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 gap-3 rounded-xl border border-amber-200 bg-white p-4 sm:grid-cols-2">
                    <div>
                      <p className="text-xs font-medium text-stone-500">
                        Tanggal Acara yang Ditawarkan
                      </p>
                      <p className="mt-0.5 text-base font-bold text-stone-900">
                        {formatDateLong(order.counter_offer.date)}
                      </p>
                      {order.counter_offer.date !== order.event_date ? (
                        <p className="mt-0.5 text-xs text-amber-800">
                          (Semula: {formatDateLong(order.event_date)})
                        </p>
                      ) : null}
                    </div>
                    <div>
                      <p className="text-xs font-medium text-stone-500">
                        Tagihan Uang Muka (DP) pada Syarat Baru
                      </p>
                      <p className="mt-0.5 text-base font-bold text-amber-800">
                        {formatRupiah(order.counter_offer.dp)}
                      </p>
                    </div>
                  </div>

                  {/* Rincian harga minimal per item bila ada penyesuaian */}
                  {order.counter_offer.min_price_per_item &&
                  Object.keys(order.counter_offer.min_price_per_item).length >
                    0 ? (
                    <div className="rounded-xl border border-amber-200 bg-white p-4">
                      <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-stone-500">
                        Rincian Harga Satuan Minimum Tawaran
                      </p>
                      <ul className="space-y-1.5 text-xs sm:text-sm">
                        {Object.entries(
                          order.counter_offer.min_price_per_item
                        ).map(([pid, minPrice]) => {
                          const pName =
                            productNameById.get(pid) ?? `Produk (${pid.slice(0, 8)})`;
                          return (
                            <li
                              key={pid}
                              className="flex items-center justify-between gap-2"
                            >
                              <span className="font-medium text-stone-800">
                                {pName}
                              </span>
                              <span className="font-semibold text-stone-900">
                                {formatRupiah(minPrice)} / unit
                              </span>
                            </li>
                          );
                        })}
                      </ul>
                    </div>
                  ) : null}

                  <div className="flex flex-col gap-2.5 pt-1 sm:flex-row">
                    <Button
                      type="button"
                      variant="success"
                      disabled={counterMutation.isPending}
                      onClick={() => counterMutation.mutate(true)}
                      className="flex-1 gap-2"
                    >
                      <CheckCircle2 className="h-4 w-4" />
                      {counterMutation.isPending
                        ? "Memproses..."
                        : "Terima Tawaran Balik"}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={counterMutation.isPending}
                      onClick={() => counterMutation.mutate(false)}
                      className="flex-1 gap-2"
                    >
                      <XCircle className="h-4 w-4" />
                      {counterMutation.isPending
                        ? "Memproses..."
                        : "Tolak Tawaran Balik"}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ) : null}

            {/* Timeline Progres */}
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base sm:text-lg">
                  Tahapan Progres Pesanan
                </CardTitle>
                <CardDescription>
                  Status diperbarui langsung dari sistem operasional dapur.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <OrderTimeline status={order.status} />
              </CardContent>
            </Card>

            {/* Tagihan DP, Tenggat & Instruksi Pembayaran Manual */}
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
              <Card className="lg:col-span-7">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-amber-700" />
                    <CardTitle className="text-base sm:text-lg">
                      Rincian Item Pesanan
                    </CardTitle>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="divide-y divide-stone-200 rounded-xl border border-stone-200">
                    {order.items.map((item) => (
                      <div
                        key={item.product_id}
                        className="flex items-center justify-between gap-3 p-3.5 text-sm"
                      >
                        <div className="min-w-0">
                          <p className="font-semibold text-stone-900">
                            {item.product_name}
                          </p>
                          <p className="text-xs text-stone-500">
                            {item.qty} unit × {formatRupiah(item.unit_price)}
                          </p>
                        </div>
                        <p className="shrink-0 font-semibold text-stone-900">
                          {formatRupiah(item.subtotal)}
                        </p>
                      </div>
                    ))}
                  </div>

                  <div className="space-y-2 rounded-xl bg-stone-50 p-4 text-sm ring-1 ring-stone-200/80">
                    {((order.tax_amount && toNumber(order.tax_amount) > 0) ||
                      (order.platform_fee && toNumber(order.platform_fee) > 0)) ? (
                      <>
                        <div className="flex justify-between text-stone-600">
                          <span>Subtotal Produk</span>
                          <span className="font-semibold text-stone-900">
                            {formatRupiah(order.subtotal ?? 0)}
                          </span>
                        </div>
                        {order.tax_amount && toNumber(order.tax_amount) > 0 ? (
                          <div className="flex justify-between text-stone-600">
                            <span>
                              PPN ({Math.round(toNumber(order.tax_pct ?? 0) * 100)}%)
                            </span>
                            <span className="font-semibold text-stone-900">
                              {formatRupiah(order.tax_amount)}
                            </span>
                          </div>
                        ) : null}
                        {order.platform_fee && toNumber(order.platform_fee) > 0 ? (
                          <div className="flex justify-between text-stone-600">
                            <span>Biaya Layanan Aplikasi</span>
                            <span className="font-semibold text-stone-900">
                              {formatRupiah(order.platform_fee)}
                            </span>
                          </div>
                        ) : null}
                      </>
                    ) : null}
                    <div className="flex justify-between text-stone-600">
                      <span>Total Nilai Pesanan</span>
                      <span className="font-semibold text-stone-900">
                        {formatRupiah(order.total)}
                      </span>
                    </div>
                    <div className="flex justify-between text-stone-600">
                      <span>Tagihan Uang Muka (DP Minimal)</span>
                      <span className="font-semibold text-amber-800">
                        {formatRupiah(order.dp_required)}
                      </span>
                    </div>
                    <div className="flex justify-between text-stone-600">
                      <span>Pembayaran Tercatat</span>
                      <span className="font-semibold text-emerald-700">
                        {formatRupiah(order.paid)}
                      </span>
                    </div>
                    <div className="border-t border-stone-200 pt-2 flex justify-between font-semibold">
                      <span>Sisa Pelunasan</span>
                      <span className="text-stone-900">
                        {formatRupiah(remainingTotal)}
                      </span>
                    </div>

                    {isFullyPaid ? (
                      <p className="pt-1 text-center text-xs font-semibold text-emerald-800">
                        ✓ Pembayaran telah Lunas. Anda dapat mencetak Nota Lunas sebagai bukti serah terima pesanan.
                      </p>
                    ) : (
                      <p className="pt-1 text-center text-[11px] text-stone-500">
                        *Nota resmi pengambilan / pengantaran dapat dicetak setelah pembayaran Lunas.
                      </p>
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card className="lg:col-span-5">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-2">
                    <CreditCard className="h-4 w-4 text-amber-700" />
                    <CardTitle className="text-base sm:text-lg">
                      Informasi &amp; Instruksi Pembayaran
                    </CardTitle>
                  </div>
                </CardHeader>
                <CardContent className="space-y-4 text-sm">
                  {isFullyPaid ? (
                    <div className="rounded-xl border border-emerald-200 bg-emerald-50/80 p-4 text-center">
                      <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
                        <CheckCircle2 className="h-5 w-5" />
                      </div>
                      <p className="mt-2 text-sm font-bold text-emerald-900">
                        Pembayaran Lunas (100%)
                      </p>
                      <p className="mt-1 text-xs text-emerald-800">
                        Seluruh tagihan pesanan sebesar <strong>{formatRupiah(order.total)}</strong> telah dibayar lunas.
                      </p>
                      <p className="mt-2 text-[11px] text-stone-600">
                        Terima kasih! Pesanan Anda saat ini sedang diproses oleh tim dapur kami sesuai jadwal.
                      </p>
                    </div>
                  ) : remainingDP <= 0 ? (
                    <div className="rounded-xl border border-teal-200 bg-teal-50/80 p-4">
                      <div className="flex items-center gap-2">
                        <CheckCircle2 className="h-4 w-4 text-teal-700" />
                        <span className="text-xs font-bold text-teal-900">Uang Muka (DP) Terverifikasi</span>
                      </div>
                      <p className="mt-1 text-xs text-stone-700">
                        Pembayaran DP sebesar <strong>{formatRupiah(order.paid)}</strong> telah diterima. Sisa pelunasan sebesar <strong>{formatRupiah(remainingTotal)}</strong> dapat dibayarkan saat pesanan siap diambil/diantar.
                      </p>
                    </div>
                  ) : order.dp_deadline ? (
                    <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3.5">
                      <p className="text-xs font-medium text-amber-800">
                        Tenggat Pembayaran DP
                      </p>
                      <p className="mt-0.5 text-sm font-bold text-amber-950">
                        {formatDateTimeWIB(order.dp_deadline)}
                      </p>
                      {remainingDP > 0 && order.status === "accepted" ? (
                        <p className="mt-1 text-xs text-amber-900/90">
                          Kurang bayar DP:{" "}
                          <strong>{formatRupiah(remainingDP)}</strong>. Harap
                          lunasi DP sebelum tenggat agar reservasi slot tidak
                          kedaluwarsa.
                        </p>
                      ) : null}
                    </div>
                  ) : null}

                  {!isFullyPaid ? (
                    <div className="space-y-2.5 rounded-xl border border-stone-200 bg-stone-50/70 p-4 text-xs leading-relaxed text-stone-700">
                      <p className="font-semibold text-stone-900">
                        Instruksi Pembayaran Manual (Simulasi):
                      </p>
                      <ol className="list-decimal space-y-1.5 pl-4">
                        <li>
                          Transfer sesuai nominal DP (
                          <strong>{formatRupiah(order.dp_required)}</strong>) atau
                          pelunasan ke rekening simulasi{" "}
                          <strong>Bank Nusantara 123-456-7890</strong> a.n.{" "}
                          <strong>Dapur Kalkurasa</strong>.
                        </li>
                        <li>
                          Cantumkan kode pesanan{" "}
                          <strong className="font-mono">{order.code}</strong> pada
                          berita transfer.
                        </li>
                        <li>
                          Konfirmasikan bukti bayar kepada Manajer/Admin agar
                          pembayaran dicatat ke sistem dan status pesanan berubah
                          menjadi <strong>Terkonfirmasi</strong>.
                        </li>
                      </ol>
                    </div>
                  ) : null}

                  {order.status === "pending_decision" ? (
                    <div className="flex items-start gap-2 rounded-xl border border-sky-200 bg-sky-50/80 p-3 text-xs text-sky-900">
                      <Info className="mt-0.5 h-4 w-4 shrink-0 text-sky-700" />
                      <span>
                        Anda belum perlu melakukan transfer DP saat ini. Tunggu
                        hingga pesanan disetujui atau tawaran balik Anda terima.
                      </span>
                    </div>
                  ) : null}
                </CardContent>
              </Card>
            </div>

            <div className="flex justify-center pt-2">
              <Link href="/pesan">
                <Button variant="outline" className="gap-2">
                  <ShoppingBag className="h-4 w-4" />
                  Buat Pesanan Baru Lainnya
                </Button>
              </Link>
            </div>
          </div>

          {/* Nota Invoice Cetak Resmi (@media print) */}
          <OrderInvoicePrint order={order} />
        </>
      ) : null}
      </main>
    </div>
  );
}

