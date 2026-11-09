"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import {
  AlertCircle,
  ArrowRight,
  CalendarClock,
  ClipboardList,
  Clock,
  Info,
  PackageSearch,
  Phone,
  Plus,
  RefreshCw,
  Search,
  ShoppingBag,
  Sparkles,
  Truck,
  UserCheck,
  X,
} from "lucide-react";
import { getErrorMessage, getMe, getMyOrders } from "@/lib/api";
import {
  formatDateLong,
  formatDateTimeWIB,
  formatRupiah,
  getOrderStatusMeta,
  toNumber,
} from "@/lib/format";
import { PublicNavbar } from "@/components/public-navbar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Pagination } from "@/components/ui/pagination";

const ACTIVE_UNPAID_STATUSES = new Set([
  "pending_decision",
  "countered",
  "accepted",
]);

export default function MyOrdersPage() {
  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    retry: false,
  });

  const ordersQuery = useQuery({
    queryKey: ["my-orders"],
    queryFn: ({ signal }) => getMyOrders(signal),
    enabled: meQuery.data?.role === "customer",
  });

  const user = meQuery.data;
  const orders = useMemo(() => ordersQuery.data ?? [], [ordersQuery.data]);
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const PAGE_SIZE = 5;

  const filteredOrders = useMemo(() => {
    const q = searchQuery.trim().toLowerCase().replace(/^#/, "");
    if (!q) return orders;
    return orders.filter((o) => {
      const code = (o.code || "").toLowerCase();
      const statusMeta = getOrderStatusMeta(o.status);
      const statusLabel = statusMeta.label.toLowerCase();
      const items = (o.items || [])
        .map((i) => (i.product_name || "").toLowerCase())
        .join(" ");
      return (
        code.includes(q) ||
        statusLabel.includes(q) ||
        items.includes(q)
      );
    });
  }, [orders, searchQuery]);

  const totalPages = Math.ceil(filteredOrders.length / PAGE_SIZE) || 1;
  const paginatedOrders = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE;
    return filteredOrders.slice(start, start + PAGE_SIZE);
  }, [filteredOrders, page, PAGE_SIZE]);

  const activeUnpaidCount = orders.filter(
    (o) =>
      ACTIVE_UNPAID_STATUSES.has(o.status) &&
      toNumber(o.paid) < toNumber(o.dp_required)
  ).length;

  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />

      <main className="mx-auto w-full max-w-5xl flex-1 space-y-6 px-4 py-6 sm:px-6 sm:py-8">
        {/* Header Profil Pelanggan */}
        <div className="flex flex-col justify-between gap-4 rounded-2xl border border-stone-200 bg-white p-5 shadow-xs sm:flex-row sm:items-center">
          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="amber" className="gap-1">
                <UserCheck className="h-3 w-3" />
                Akun Pelanggan
              </Badge>
              {user?.contact ? (
                <span className="inline-flex items-center gap-1 text-xs text-stone-500">
                  <Phone className="h-3 w-3" />
                  {user.contact}
                </span>
              ) : null}
            </div>
            <h1 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
              Pesanan Saya{user?.name ? ` · ${user.name}` : ""}
            </h1>
            <p className="text-xs text-stone-600 sm:text-sm">
              Pantau status persetujuan jadwal, pembayaran DP, hingga pengiriman
              seluruh pesanan pre-order Anda.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => ordersQuery.refetch()}
              disabled={ordersQuery.isFetching}
              className="gap-1.5"
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${ordersQuery.isFetching ? "animate-spin" : ""}`}
              />
              Segarkan
            </Button>
            <Link href="/pesan">
              <Button size="sm" className="gap-1.5">
                <Plus className="h-4 w-4" />
                Buat Pesanan Baru
              </Button>
            </Link>
          </div>
        </div>

        {/* Info Batas Pesanan Aktif Belum Bayar DP */}
        <div className="flex flex-col justify-between gap-3 rounded-xl border border-stone-200 bg-stone-50/90 px-4 py-3 text-xs text-stone-700 sm:flex-row sm:items-center">
          <div className="flex items-start gap-2.5">
            <Info className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
            <div>
              <span className="font-semibold text-stone-900">
                Ketentuan Slot Pesanan Aktif:
              </span>{" "}
              Setiap pelanggan dapat memiliki maksimal{" "}
              <strong>2 pesanan aktif yang belum melunasi DP</strong> (status{" "}
              <em>Menunggu Konfirmasi</em>, <em>Ada Tawaran Balik</em>, atau{" "}
              <em>Disetujui : Menunggu DP</em>). Setelah DP terverifikasi, Anda
              dapat membuat pesanan baru kembali.
            </div>
          </div>
          <Badge
            variant={activeUnpaidCount >= 2 ? "amber" : "slate"}
            className="shrink-0 self-start sm:self-center"
          >
            Slot Aktif Belum DP: {activeUnpaidCount} / 2
          </Badge>
        </div>

        {/* Daftar Pesanan */}
        {meQuery.isLoading || ordersQuery.isLoading ? (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, idx) => (
              <Skeleton key={idx} className="h-40 w-full rounded-xl" />
            ))}
          </div>
        ) : ordersQuery.isError ? (
          <Card className="border-rose-200 bg-rose-50/50">
            <CardContent className="flex flex-col items-center justify-between gap-3 p-6 sm:flex-row">
              <div className="flex items-start gap-3 text-rose-900">
                <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-rose-600" />
                <div>
                  <p className="text-sm font-semibold">
                    Gagal Memuat Daftar Pesanan
                  </p>
                  <p className="text-xs text-rose-700">
                    {getErrorMessage(ordersQuery.error)}
                  </p>
                </div>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => ordersQuery.refetch()}
              >
                Coba Lagi
              </Button>
            </CardContent>
          </Card>
        ) : orders.length === 0 ? (
          <Card className="border-dashed border-stone-300 bg-white">
            <CardContent className="flex flex-col items-center justify-center py-12 text-center">
              <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-50 text-amber-700">
                <ClipboardList className="h-6 w-6" />
              </div>
              <h2 className="text-base font-bold text-stone-900">
                Belum Ada Pesanan Pre-Order
              </h2>
              <p className="mt-1 max-w-md text-xs text-stone-500 sm:text-sm">
                Anda belum pernah mengirim pesanan dari akun ini. Pilih produk
                favorit dan cek ketersediaan jadwal dapur di halaman pemesanan.
              </p>
              <div className="mt-5 flex flex-wrap justify-center gap-2.5">
                <Link href="/pesan">
                  <Button className="gap-1.5">
                    <ShoppingBag className="h-4 w-4" />
                    Mulai Buat Pesanan
                  </Button>
                </Link>
                <Link href="/lacak">
                  <Button variant="outline" className="gap-1.5">
                    <PackageSearch className="h-4 w-4" />
                    Cari dengan Kode Lacak
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-4">
            {/* Search Bar Pelanggan */}
            <div className="relative">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-stone-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => {
                  setSearchQuery(e.target.value);
                  setPage(1);
                }}
                placeholder="Cari kode pesanan (#DEMO...) atau nama menu..."
                className="w-full rounded-xl border border-stone-200 bg-white py-2.5 pl-10 pr-10 text-xs font-medium text-stone-900 placeholder:text-stone-400 focus:border-amber-500 focus:outline-none focus:ring-2 focus:ring-amber-500/20"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => {
                    setSearchQuery("");
                    setPage(1);
                  }}
                  className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full p-1 text-stone-400 hover:bg-stone-100 hover:text-stone-700"
                  title="Hapus pencarian"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>

            {filteredOrders.length === 0 ? (
              <Card className="border-stone-200 bg-white">
                <CardContent className="flex flex-col items-center justify-center py-10 text-center">
                  <Search className="h-8 w-8 text-stone-400" />
                  <p className="mt-3 text-sm font-semibold text-stone-800">
                    Tidak ada pesanan yang sesuai dengan &ldquo;{searchQuery}&rdquo;
                  </p>
                  <Button
                    variant="outline"
                    size="sm"
                    className="mt-3"
                    onClick={() => {
                      setSearchQuery("");
                      setPage(1);
                    }}
                  >
                    Hapus Pencarian
                  </Button>
                </CardContent>
              </Card>
            ) : (
              <>
                <div className="space-y-4">
                  {paginatedOrders.map((order) => {
              const statusMeta = getOrderStatusMeta(order.status);
              const totalNum = toNumber(order.total);
              const dpRequiredNum = toNumber(order.dp_required);
              const paidNum = toNumber(order.paid);
              const remainingDp = Math.max(0, dpRequiredNum - paidNum);
              const totalUnits = order.items.reduce(
                (acc, item) => acc + item.qty,
                0
              );

              return (
                <Card
                  key={order.code}
                  className="border-stone-200/90 transition-shadow hover:shadow-sm"
                >
                  <CardHeader className="flex flex-col justify-between gap-2 border-b border-stone-100 pb-3 sm:flex-row sm:items-center">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="rounded-lg bg-stone-900 px-2.5 py-1 font-mono text-xs font-bold tracking-wider text-white">
                        {order.code}
                      </span>
                      <Badge variant={statusMeta.tone}>{statusMeta.label}</Badge>
                      <span className="text-xs text-stone-500">
                        Dibuat {formatDateTimeWIB(order.created_at)}
                      </span>
                    </div>

                    <Link href={`/lacak/${order.code}`}>
                      <Button variant="outline" size="sm" className="gap-1.5">
                        Lihat Detail & Lacak
                        <ArrowRight className="h-3.5 w-3.5" />
                      </Button>
                    </Link>
                  </CardHeader>

                  <CardContent className="space-y-3 pt-4">
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                      <div className="rounded-xl bg-stone-50 p-3">
                        <span className="flex items-center gap-1 text-[11px] font-medium text-stone-500">
                          <CalendarClock className="h-3.5 w-3.5 text-amber-700" />
                          Tanggal Acara / Pengambilan
                        </span>
                        <p className="mt-0.5 text-sm font-semibold text-stone-900">
                          {formatDateLong(order.event_date)}
                        </p>
                        <p className="mt-0.5 flex items-center gap-1 text-xs text-stone-600">
                          <Truck className="h-3 w-3" />
                          {order.delivery_type === "delivery"
                            ? "Diantar Kurir"
                            : "Ambil di Dapur (Pickup)"}
                        </p>
                      </div>

                      <div className="rounded-xl bg-stone-50 p-3">
                        <span className="text-[11px] font-medium text-stone-500">
                          Ringkasan Item ({totalUnits} unit)
                        </span>
                        <ul className="mt-1 space-y-0.5 text-xs text-stone-800">
                          {order.items.map((item) => (
                            <li
                              key={item.product_id}
                              className="flex justify-between gap-2"
                            >
                              <span className="truncate font-medium">
                                {item.product_name} × {item.qty}
                              </span>
                              <span className="shrink-0 text-stone-600">
                                {formatRupiah(item.subtotal)}
                              </span>
                            </li>
                          ))}
                        </ul>
                      </div>

                      <div className="rounded-xl bg-amber-50/60 p-3 ring-1 ring-amber-200/70">
                        <div className="flex items-center justify-between text-xs text-stone-600">
                          <span>Total Pesanan</span>
                          <span className="font-bold text-stone-900">
                            {formatRupiah(totalNum)}
                          </span>
                        </div>
                        <div className="mt-1 flex items-center justify-between text-xs text-stone-600">
                          <span>Tagihan DP Minimal</span>
                          <span className="font-semibold text-amber-900">
                            {formatRupiah(dpRequiredNum)}
                          </span>
                        </div>
                        <div className="mt-1 flex items-center justify-between text-xs text-stone-600">
                          <span>Sudah Dibayar</span>
                          <span className="font-semibold text-emerald-700">
                            {formatRupiah(paidNum)}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Info kontekstual untuk tindakan pelanggan */}
                    {order.status === "countered" && order.counter_offer ? (
                      <div className="flex flex-col justify-between gap-2 rounded-xl border border-amber-300 bg-amber-50 p-3 text-xs text-amber-950 sm:flex-row sm:items-center">
                        <div className="flex items-start gap-2">
                          <Sparkles className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
                          <span>
                            <strong>Ada Tawaran Balik dari Dapur:</strong> Kami
                            menawarkan penyesuaian tanggal acara ke{" "}
                            <strong>
                              {formatDateLong(order.counter_offer.date)}
                            </strong>{" "}
                            dengan DP{" "}
                            <strong>
                              {formatRupiah(order.counter_offer.dp)}
                            </strong>
                            . Silakan buka halaman detail untuk menerima atau
                            menolak tawaran ini.
                          </span>
                        </div>
                        <Link href={`/lacak/${order.code}`} className="shrink-0">
                          <Button size="sm" className="w-full sm:w-auto">
                            Tanggapi Tawaran
                          </Button>
                        </Link>
                      </div>
                    ) : null}

                    {order.status === "accepted" && remainingDp > 0 ? (
                      <div className="flex flex-col justify-between gap-2 rounded-xl border border-sky-200 bg-sky-50/80 p-3 text-xs text-sky-950 sm:flex-row sm:items-center">
                        <div className="flex items-start gap-2">
                          <Clock className="mt-0.5 h-4 w-4 shrink-0 text-sky-700" />
                          <span>
                            <strong>Menunggu Pembayaran DP:</strong> Slot Anda
                            telah direservasi. Sisa DP sebesar{" "}
                            <strong>{formatRupiah(remainingDp)}</strong> perlu
                            dibayarkan sebelum{" "}
                            <strong>
                              {formatDateTimeWIB(order.dp_deadline)}
                            </strong>{" "}
                            agar pesanan tidak kedaluwarsa.
                          </span>
                        </div>
                        <Link href={`/lacak/${order.code}`} className="shrink-0">
                          <Button
                            size="sm"
                            variant="outline"
                            className="w-full border-sky-300 bg-white text-sky-900 hover:bg-sky-100 sm:w-auto"
                          >
                            Instruksi Bayar DP
                          </Button>
                        </Link>
                      </div>
                    ) : null}

                    {order.status === "pending_decision" ? (
                      <CardDescription className="text-xs text-stone-600">
                        Pesanan Anda sedang ditinjau oleh Manajer/Pemilik usaha
                        (belum perlu membayar DP sekarang). Anda akan melihat
                        status berubah menjadi{" "}
                        <strong>Disetujui : Menunggu DP</strong> atau{" "}
                        <strong>Ada Tawaran Balik</strong> setelah ditinjau.
                      </CardDescription>
                    ) : null}
                  </CardContent>
                </Card>
              );
            })}
          </div>

          <Pagination
            currentPage={page}
            totalPages={totalPages}
            totalItems={filteredOrders.length}
            pageSize={PAGE_SIZE}
            onPageChange={setPage}
            itemName="pesanan"
          />
        </>
      )}
    </div>
  )}
      </main>
    </div>
  );
}
