"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertTriangle,
  Calendar,
  ExternalLink,
  Filter,
  Loader2,
  Plus,
  RefreshCw,
  ShoppingBag,
  Trash2,
  X,
} from "lucide-react";
import { useInternalSession } from "@/app/app/layout";
import { OrderStatusBadge } from "@/components/internal/order-timeline";
import {
  createManualOrder,
  getErrorMessage,
  getOrders,
  getPublicProducts,
} from "@/lib/api";
import {
  addDaysDateKey,
  formatDateShort,
  formatRupiah,
  toDateKeyWIB,
} from "@/lib/format";
import type {
  DeliveryType,
  OrderDetail,
  PublicProduct,
} from "@/lib/types";

const STATUS_FILTER_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "Semua Status" },
  { value: "pending_decision", label: "Menunggu Keputusan" },
  { value: "countered", label: "Tawaran Balik" },
  { value: "accepted", label: "Disetujui (Menunggu DP)" },
  { value: "confirmed", label: "Terkonfirmasi (DP Lunas)" },
  { value: "in_production", label: "Dalam Produksi" },
  { value: "ready", label: "Siap" },
  { value: "delivering", label: "Sedang Diantar / Diambil" },
  { value: "completed", label: "Selesai" },
  { value: "cancelled", label: "Dibatalkan" },
  { value: "rejected", label: "Ditolak" },
  { value: "expired", label: "Kedaluwarsa" },
];

export default function OrdersListPage() {
  const router = useRouter();
  const { user } = useInternalSession();
  const isStaff = user?.role === "staff";
  const canCreateManual = user?.role === "owner" || user?.role === "manager";

  const [orders, setOrders] = useState<OrderDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState("");
  const [dateFilter, setDateFilter] = useState("");

  // Modal buat pesanan manual
  const [showManualModal, setShowManualModal] = useState(false);
  const [products, setProducts] = useState<PublicProduct[]>([]);
  const [custName, setCustName] = useState("");
  const [custContact, setCustContact] = useState("");
  const [deliveryType, setDeliveryType] = useState<DeliveryType>("pickup");
  const [address, setAddress] = useState("");
  const [notes, setNotes] = useState("");
  const [eventDate, setEventDate] = useState(() =>
    addDaysDateKey(toDateKeyWIB(new Date()), 3)
  );
  const [prodDate, setProdDate] = useState("");
  const [manualItems, setManualItems] = useState<
    { product_id: string; qty: number; custom_price: string }[]
  >([]);
  const [submittingManual, setSubmittingManual] = useState(false);
  const [manualError, setManualError] = useState<string | null>(null);

  const loadOrders = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const data = await getOrders({
        status: statusFilter || undefined,
        date: dateFilter || undefined,
      });
      setOrders(data);
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat daftar pesanan."));
    } finally {
      setLoading(false);
    }
  }, [statusFilter, dateFilter]);

  useEffect(() => {
    void loadOrders();
  }, [loadOrders]);

  async function openManualOrderModal() {
    setManualError(null);
    setShowManualModal(true);
    if (products.length === 0) {
      try {
        const prods = await getPublicProducts();
        setProducts(prods);
        if (prods.length > 0 && manualItems.length === 0) {
          setManualItems([
            { product_id: prods[0].id, qty: 10, custom_price: "" },
          ]);
        }
      } catch (err) {
        setManualError(getErrorMessage(err, "Gagal memuat katalog produk."));
      }
    } else if (manualItems.length === 0 && products.length > 0) {
      setManualItems([
        { product_id: products[0].id, qty: 10, custom_price: "" },
      ]);
    }
  }

  async function handleCreateManualOrder(e: React.FormEvent) {
    e.preventDefault();
    setManualError(null);

    if (!custName.trim() || !custContact.trim()) {
      setManualError("Nama dan kontak pelanggan wajib diisi.");
      return;
    }
    if (deliveryType === "delivery" && !address.trim()) {
      setManualError("Alamat pengantaran wajib diisi untuk pesanan diantar.");
      return;
    }
    if (!eventDate) {
      setManualError("Tanggal acara wajib dipilih.");
      return;
    }
    if (manualItems.length === 0) {
      setManualError("Minimal satu item produk wajib dipilih.");
      return;
    }

    setSubmittingManual(true);
    try {
      const created = await createManualOrder({
        customer_name: custName.trim(),
        customer_contact: custContact.trim(),
        delivery_type: deliveryType,
        address: address.trim() || undefined,
        notes: notes.trim() || undefined,
        event_date: eventDate,
        production_date: prodDate.trim() || undefined,
        items: manualItems.map((it) => ({
          product_id: it.product_id,
          qty: Number(it.qty) || 1,
          unit_price: it.custom_price.trim()
            ? it.custom_price.trim()
            : undefined,
        })),
      });
      setShowManualModal(false);
      router.push(`/app/pesanan/${created.id}`);
    } catch (err) {
      setManualError(getErrorMessage(err, "Gagal membuat pesanan manual."));
    } finally {
      setSubmittingManual(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
            <ShoppingBag className="h-3.5 w-3.5" />
            Manajemen Pesanan Internal
          </span>
          <h1 className="mt-2 text-2xl font-bold text-stone-900">
            Daftar Pesanan
          </h1>
          <p className="mt-1 text-sm text-stone-600">
            Pantau seluruh siklus pesanan dari pengajuan hingga selesai.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2.5">
          <button
            type="button"
            onClick={() => void loadOrders()}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-3.5 py-2 text-xs font-semibold text-stone-700 shadow-2xs transition hover:bg-stone-50"
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`}
            />
            Muat Ulang
          </button>

          {canCreateManual && (
            <button
              type="button"
              onClick={() => void openManualOrderModal()}
              className="inline-flex items-center gap-1.5 rounded-xl bg-amber-600 px-4 py-2 text-xs font-bold text-white shadow-xs transition hover:bg-amber-700"
            >
              <Plus className="h-4 w-4" />
              Buat Pesanan Manual
            </button>
          )}
        </div>
      </div>

      {/* Bar Filter Status & Tanggal */}
      <div className="flex flex-wrap items-end gap-3 rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
        <div className="min-w-48 flex-1">
          <label className="flex items-center gap-1.5 text-xs font-semibold text-stone-600">
            <Filter className="h-3.5 w-3.5 text-stone-400" />
            Filter Status Pesanan
          </label>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3 py-2 text-xs font-medium text-stone-800 focus:border-amber-500 focus:bg-white focus:outline-none"
          >
            {STATUS_FILTER_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </div>

        <div className="min-w-44 flex-1">
          <label className="flex items-center gap-1.5 text-xs font-semibold text-stone-600">
            <Calendar className="h-3.5 w-3.5 text-stone-400" />
            Filter Tanggal Acara / Produksi
          </label>
          <input
            type="date"
            value={dateFilter}
            onChange={(e) => setDateFilter(e.target.value)}
            className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/60 px-3 py-2 text-xs font-medium text-stone-800 focus:border-amber-500 focus:bg-white focus:outline-none"
          />
        </div>

        {(statusFilter || dateFilter) && (
          <button
            type="button"
            onClick={() => {
              setStatusFilter("");
              setDateFilter("");
            }}
            className="rounded-xl border border-stone-200 px-3 py-2 text-xs font-semibold text-stone-600 hover:bg-stone-100"
          >
            Reset Filter
          </button>
        )}
      </div>

      {errorMsg && (
        <div className="flex items-center gap-2 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <AlertTriangle className="h-4 w-4 shrink-0 text-rose-600" />
          <span>{errorMsg}</span>
        </div>
      )}

      {/* Tabel Daftar Pesanan */}
      {loading ? (
        <div className="flex items-center justify-center rounded-2xl border border-stone-200 bg-white py-16 text-sm text-stone-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin text-amber-600" />
          Memuat daftar pesanan...
        </div>
      ) : orders.length === 0 ? (
        <div className="rounded-2xl border border-stone-200 bg-white p-12 text-center shadow-xs">
          <p className="text-sm font-semibold text-stone-800">
            Tidak ada pesanan yang cocok dengan filter saat ini.
          </p>
          <p className="mt-1 text-xs text-stone-500">
            Coba ubah filter status/tanggal atau buat pesanan manual baru.
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-xs">
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-stone-200 text-xs">
              <thead className="bg-stone-50 text-left font-bold text-stone-600 uppercase">
                <tr>
                  <th className="px-4 py-3">Kode</th>
                  <th className="px-4 py-3">Pelanggan</th>
                  <th className="px-4 py-3">Jadwal</th>
                  <th className="px-4 py-3">Item</th>
                  <th className="px-4 py-3">Status</th>
                  {!isStaff && (
                    <>
                      <th className="px-4 py-3 text-right">Total</th>
                      <th className="px-4 py-3 text-right">DP / Dibayar</th>
                    </>
                  )}
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100 bg-white text-stone-800">
                {orders.map((ord) => (
                  <tr
                    key={ord.id}
                    className="transition hover:bg-stone-50/70"
                  >
                    <td className="px-4 py-3 font-mono font-bold text-stone-900">
                      #{ord.code}
                      <div className="mt-0.5 font-sans text-[10px] font-medium text-stone-400">
                        {ord.source === "manual" ? "Manual" : "Publik"}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-semibold text-stone-900">
                        {ord.customer_name}
                      </div>
                      <div className="text-[11px] text-stone-500">
                        {ord.customer_contact} •{" "}
                        {ord.delivery_type === "delivery"
                          ? "Diantar"
                          : "Ambil Sendiri"}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-medium text-stone-900">
                        Acara: {formatDateShort(ord.event_date)}
                      </div>
                      <div className="text-[11px] text-stone-500">
                        Produksi: {formatDateShort(ord.production_date)}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="font-medium text-stone-800">
                        {ord.total_units} unit ({ord.items.length} jenis)
                      </div>
                      <div className="max-w-52 truncate text-[11px] text-stone-500">
                        {ord.items
                          .map((it) => `${it.product_name} (${it.qty})`)
                          .join(", ")}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <OrderStatusBadge status={ord.status} />
                    </td>
                    {!isStaff && (
                      <>
                        <td className="px-4 py-3 text-right font-bold text-stone-900">
                          {formatRupiah(ord.total)}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <div className="font-semibold text-emerald-700">
                            Dibayar: {formatRupiah(ord.paid)}
                          </div>
                          <div className="text-[11px] text-stone-500">
                            Wajib DP: {formatRupiah(ord.dp_required)}
                          </div>
                        </td>
                      </>
                    )}
                    <td className="px-4 py-3 text-right">
                      <Link
                        href={`/app/pesanan/${ord.id}`}
                        className="inline-flex items-center gap-1 rounded-lg border border-stone-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-stone-700 hover:border-amber-300 hover:bg-amber-50 hover:text-amber-900"
                      >
                        Detail
                        <ExternalLink className="h-3 w-3" />
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Modal Buat Pesanan Manual */}
      {showManualModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-950/50 p-4">
          <div className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-stone-200 bg-white p-6 shadow-xl">
            <div className="flex items-center justify-between border-b border-stone-100 pb-4">
              <div>
                <h2 className="text-lg font-bold text-stone-900">
                  Buat Pesanan Manual (Internal)
                </h2>
                <p className="text-xs text-stone-500">
                  Mendukung pengaturan tanggal produksi terpisah dan harga
                  satuan khusus (nego/korporat).
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowManualModal(false)}
                className="rounded-lg p-1.5 text-stone-400 hover:bg-stone-100 hover:text-stone-700"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleCreateManualOrder} className="mt-4 space-y-4">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Nama Pelanggan / Instansi *
                  </label>
                  <input
                    type="text"
                    required
                    value={custName}
                    onChange={(e) => setCustName(e.target.value)}
                    placeholder="Mis. PT Nusantara / Ibu Rina"
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Kontak WhatsApp / Telepon *
                  </label>
                  <input
                    type="text"
                    required
                    value={custContact}
                    onChange={(e) => setCustContact(e.target.value)}
                    placeholder="0812xxxxxxxx"
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Metode Penyerahan *
                  </label>
                  <select
                    value={deliveryType}
                    onChange={(e) =>
                      setDeliveryType(e.target.value as DeliveryType)
                    }
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  >
                    <option value="pickup">Ambil Sendiri (Pickup)</option>
                    <option value="delivery">Diantar Kurir (Delivery)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Tanggal Acara *
                  </label>
                  <input
                    type="date"
                    required
                    value={eventDate}
                    onChange={(e) => setEventDate(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Tanggal Produksi (Opsional)
                  </label>
                  <input
                    type="date"
                    value={prodDate}
                    onChange={(e) => setProdDate(e.target.value)}
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  />
                </div>
              </div>

              {deliveryType === "delivery" && (
                <div>
                  <label className="block text-xs font-semibold text-stone-700">
                    Alamat Pengantaran Lengkap *
                  </label>
                  <input
                    type="text"
                    required
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                    placeholder="Jalan, nomor gedung, patokan..."
                    className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                  />
                </div>
              )}

              <div>
                <div className="flex items-center justify-between">
                  <label className="block text-xs font-semibold text-stone-700">
                    Daftar Item Produk *
                  </label>
                  <button
                    type="button"
                    onClick={() => {
                      if (products.length > 0) {
                        setManualItems((prev) => [
                          ...prev,
                          {
                            product_id: products[0].id,
                            qty: 10,
                            custom_price: "",
                          },
                        ]);
                      }
                    }}
                    className="text-xs font-bold text-amber-700 hover:text-amber-800"
                  >
                    + Tambah Baris Produk
                  </button>
                </div>

                <div className="mt-2 space-y-2">
                  {manualItems.map((it, idx) => (
                    <div
                      key={idx}
                      className="grid grid-cols-1 gap-2 rounded-xl border border-stone-200 bg-stone-50/70 p-2.5 sm:grid-cols-12"
                    >
                      <div className="sm:col-span-5">
                        <label className="block text-[10px] font-semibold text-stone-500">
                          Produk
                        </label>
                        <select
                          value={it.product_id}
                          onChange={(e) => {
                            const val = e.target.value;
                            setManualItems((prev) =>
                              prev.map((row, i) =>
                                i === idx ? { ...row, product_id: val } : row
                              )
                            );
                          }}
                          className="mt-0.5 w-full rounded-lg border border-stone-200 bg-white px-2.5 py-1.5 text-xs text-stone-900"
                        >
                          {products.map((p) => (
                            <option key={p.id} value={p.id}>
                              {p.name} ({formatRupiah(p.price)})
                            </option>
                          ))}
                        </select>
                      </div>

                      <div className="sm:col-span-2">
                        <label className="block text-[10px] font-semibold text-stone-500">
                          Jumlah (Qty)
                        </label>
                        <input
                          type="number"
                          min={1}
                          required
                          value={it.qty}
                          onChange={(e) => {
                            const val = Number.parseInt(e.target.value, 10) || 1;
                            setManualItems((prev) =>
                              prev.map((row, i) =>
                                i === idx ? { ...row, qty: val } : row
                              )
                            );
                          }}
                          className="mt-0.5 w-full rounded-lg border border-stone-200 bg-white px-2.5 py-1.5 text-xs text-stone-900"
                        />
                      </div>

                      <div className="sm:col-span-4">
                        <label className="block text-[10px] font-semibold text-stone-500">
                          Harga Satuan Khusus (Kosong = Normal)
                        </label>
                        <input
                          type="number"
                          min={1}
                          placeholder="Default katalog"
                          value={it.custom_price}
                          onChange={(e) => {
                            const val = e.target.value;
                            setManualItems((prev) =>
                              prev.map((row, i) =>
                                i === idx ? { ...row, custom_price: val } : row
                              )
                            );
                          }}
                          className="mt-0.5 w-full rounded-lg border border-stone-200 bg-white px-2.5 py-1.5 text-xs text-stone-900"
                        />
                      </div>

                      <div className="flex items-end justify-end sm:col-span-1">
                        {manualItems.length > 1 && (
                          <button
                            type="button"
                            onClick={() =>
                              setManualItems((prev) =>
                                prev.filter((_, i) => i !== idx)
                              )
                            }
                            className="rounded-lg p-1.5 text-rose-600 hover:bg-rose-50"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-stone-700">
                  Catatan Pesanan (Opsional)
                </label>
                <textarea
                  rows={2}
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="Catatan khusus kemasan, jam pengantaran, dsb."
                  className="mt-1 w-full rounded-xl border border-stone-200 px-3 py-2 text-sm text-stone-900"
                />
              </div>

              {manualError && (
                <div className="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {manualError}
                </div>
              )}

              <div className="flex items-center justify-end gap-2 border-t border-stone-100 pt-4">
                <button
                  type="button"
                  onClick={() => setShowManualModal(false)}
                  className="rounded-xl border border-stone-200 px-4 py-2 text-xs font-semibold text-stone-700 hover:bg-stone-50"
                >
                  Batal
                </button>
                <button
                  type="submit"
                  disabled={submittingManual}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-amber-600 px-4 py-2 text-xs font-bold text-white hover:bg-amber-700 disabled:opacity-50"
                >
                  {submittingManual && (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  )}
                  Simpan & Evaluasi Pesanan
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
