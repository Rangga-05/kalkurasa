"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Calendar,
  CheckCircle2,
  PackagePlus,
  RefreshCw,
  ShoppingBag,
  ShoppingCart,
  X,
} from "lucide-react";
import {
  getShoppingList,
  getIngredients,
  recordPurchase,
  getErrorMessage,
} from "@/lib/api";
import {
  addDaysDateKey,
  formatDateShort,
  formatNumber,
  formatPercent,
  formatRupiah,
  toDateKeyWIB,
  toNumber,
} from "@/lib/format";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import type { ShoppingListItem } from "@/lib/types";

export default function BelanjaPage() {
  const { user } = useInternalSession();
  const isStaff = user?.role === "staff";
  const toast = useToast();
  const queryClient = useQueryClient();

  const todayStr = toDateKeyWIB(new Date());
  const defaultToStr = addDaysDateKey(todayStr, 14);

  const [fromDate, setFromDate] = useState(todayStr);
  const [toDate, setToDate] = useState(defaultToStr);

  // Modal Catat Pembelian
  const [purchaseModalOpen, setPurchaseModalOpen] = useState(false);
  const [selectedIngredientId, setSelectedIngredientId] = useState("");
  const [qtyBuyUnits, setQtyBuyUnits] = useState("");
  const [totalCost, setTotalCost] = useState("");
  const [purchasedAt, setPurchasedAt] = useState(todayStr);
  const [supplier, setSupplier] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const shoppingQuery = useQuery({
    queryKey: ["shopping-list", fromDate, toDate],
    queryFn: ({ signal }) =>
      getShoppingList({ from: fromDate, to: toDate }, signal),
  });

  const ingredientsQuery = useQuery({
    queryKey: ["ingredients"],
    queryFn: ({ signal }) => getIngredients(signal),
  });

  const purchaseMutation = useMutation({
    mutationFn: recordPurchase,
    onSuccess: (res) => {
      toast.success(
        "Pembelian Bahan Tercatat",
        `Stok ${res.ingredient_name} bertambah +${formatNumber(res.added_use_units)} menjadi ${formatNumber(res.new_stock)}.`
      );
      setPurchaseModalOpen(false);
      setFormError(null);
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
      queryClient.invalidateQueries({ queryKey: ["ingredients"] });
      queryClient.invalidateQueries({ queryKey: ["cash-projection"] });
    },
    onError: (err) => {
      const msg = getErrorMessage(err);
      setFormError(msg);
      toast.error("Gagal Mencatat Pembelian", msg);
    },
  });

  const openPurchaseForItem = (item?: ShoppingListItem) => {
    setFormError(null);
    if (item) {
      setSelectedIngredientId(item.ingredient_id);
      const recQty = toNumber(item.qty_buy_units);
      setQtyBuyUnits(recQty > 0 ? String(recQty) : "1");
      if (!isStaff && item.estimated_cost !== undefined) {
        const est = toNumber(item.estimated_cost);
        setTotalCost(est > 0 ? String(Math.round(est)) : "");
      } else {
        setTotalCost("");
      }
    } else {
      const firstIng = ingredientsQuery.data?.[0];
      setSelectedIngredientId(firstIng?.id ?? "");
      setQtyBuyUnits("1");
      setTotalCost("");
    }
    setPurchasedAt(todayStr);
    setSupplier("");
    setPurchaseModalOpen(true);
  };

  const handlePurchaseSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!selectedIngredientId) {
      setFormError("Pilih bahan baku terlebih dahulu.");
      return;
    }
    const qtyNum = Number.parseFloat(qtyBuyUnits);
    if (!Number.isFinite(qtyNum) || qtyNum <= 0) {
      setFormError("Jumlah satuan beli harus lebih dari 0.");
      return;
    }
    const costNum = Number.parseFloat(totalCost);
    if (!Number.isFinite(costNum) || costNum < 0) {
      setFormError("Total harga beli aktual tidak valid.");
      return;
    }

    purchaseMutation.mutate({
      ingredient_id: selectedIngredientId,
      qty_buy_units: String(qtyNum),
      total_cost: String(costNum),
      purchased_at: purchasedAt || todayStr,
      supplier: supplier.trim() || undefined,
    });
  };

  const data = shoppingQuery.data;
  const items = data?.items ?? [];
  const lowStockItems = data?.low_stock_items ?? [];
  const needToBuyItems = items.filter((it) => toNumber(it.qty_buy_units) > 0);
  const selectedIngObj = ingredientsQuery.data?.find(
    (i) => i.id === selectedIngredientId
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-stone-900">
            Daftar Belanja Bahan & Penerimaan Stok
          </h1>
          <p className="text-sm text-stone-600">
            Kebutuhan bahan otomatis dari pesanan aktif (
            <code className="rounded bg-stone-200/70 px-1 py-0.5 text-xs">
              confirmed
            </code>{" "}
            &{" "}
            <code className="rounded bg-stone-200/70 px-1 py-0.5 text-xs">
              in_production
            </code>
            ) menggunakan aturan pembulatan satuan beli.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => shoppingQuery.refetch()}
            disabled={shoppingQuery.isFetching}
            className="gap-1.5"
          >
            <RefreshCw
              className={`h-4 w-4 ${shoppingQuery.isFetching ? "animate-spin" : ""}`}
            />
            Segarkan
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => openPurchaseForItem()}
            className="gap-1.5"
          >
            <PackagePlus className="h-4 w-4" />
            Catat Pembelian Bahan
          </Button>
        </div>
      </div>

      {/* Filter Rentang Tanggal Acara */}
      <Card className="p-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div className="flex flex-wrap items-end gap-3">
            <div>
              <label
                htmlFor="filter-from"
                className="mb-1 block text-xs font-semibold text-stone-700"
              >
                Dari Tanggal Acara
              </label>
              <Input
                id="filter-from"
                type="date"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
                className="w-44"
              />
            </div>
            <div>
              <label
                htmlFor="filter-to"
                className="mb-1 block text-xs font-semibold text-stone-700"
              >
                Sampai Tanggal Acara
              </label>
              <Input
                id="filter-to"
                type="date"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
                className="w-44"
              />
            </div>
          </div>

          {data ? (
            <div className="flex flex-wrap items-center gap-3 text-xs text-stone-600">
              <Badge variant="slate">
                Pesanan Aktif: <strong>{data.order_count}</strong>
              </Badge>
              <Badge variant="amber">
                Buffer Belanja: <strong>{formatPercent(data.buffer_pct)}</strong>
              </Badge>
              <Badge variant="sky">
                Lead Belanja: <strong>H-{data.lead_days} hari</strong>
              </Badge>
              {!isStaff && data.total_estimated !== undefined ? (
                <Badge variant="emerald">
                  Total Estimasi Belanja:{" "}
                  <strong>{formatRupiah(data.total_estimated)}</strong>
                </Badge>
              ) : null}
            </div>
          ) : null}
        </div>
      </Card>

      {/* Peringatan Stok di Bawah Minimum */}
      {lowStockItems.length > 0 ? (
        <Card className="border-amber-300 bg-amber-50/70 p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-700" />
            <div className="flex-1 space-y-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-sm font-bold text-amber-950">
                  Peringatan Stok di Bawah Minimum ({lowStockItems.length} Bahan)
                </h2>
                <span className="text-xs text-amber-800">
                  Segera lakukan pembelian ulang agar produksi tidak terhambat
                </span>
              </div>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {lowStockItems.map((low) => (
                  <div
                    key={low.ingredient_id}
                    className="flex items-center justify-between gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-xs shadow-2xs"
                  >
                    <div>
                      <p className="font-semibold text-stone-900">
                        {low.ingredient_name}
                      </p>
                      <p className="text-stone-600">
                        Stok:{" "}
                        <strong className="text-rose-700">
                          {formatNumber(low.stock)} {low.use_unit}
                        </strong>{" "}
                        / Min: {formatNumber(low.min_stock)} {low.use_unit}
                      </p>
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setSelectedIngredientId(low.ingredient_id);
                        setQtyBuyUnits("1");
                        setTotalCost(
                          !isStaff && low.buy_price
                            ? String(Math.round(toNumber(low.buy_price)))
                            : ""
                        );
                        setPurchasedAt(todayStr);
                        setSupplier("");
                        setFormError(null);
                        setPurchaseModalOpen(true);
                      }}
                      className="text-xs"
                    >
                      Beli
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Card>
      ) : null}

      {/* Tabel Daftar Belanja */}
      {shoppingQuery.isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-12 w-full rounded-xl" />
          <Skeleton className="h-48 w-full rounded-xl" />
        </div>
      ) : shoppingQuery.isError ? (
        <Card className="border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          Gagal memuat daftar belanja: {getErrorMessage(shoppingQuery.error)}
        </Card>
      ) : items.length === 0 ? (
        <Card className="p-8 text-center">
          <ShoppingBag className="mx-auto h-10 w-10 text-stone-400" />
          <h3 className="mt-3 text-base font-bold text-stone-900">
            Belum Ada Kebutuhan Bahan pada Rentang Ini
          </h3>
          <p className="mt-1 text-sm text-stone-600">
            Tidak ditemukan pesanan berstatus{" "}
            <span className="font-semibold">Terkonfirmasi (confirmed)</span> atau{" "}
            <span className="font-semibold">Dalam Produksi (in_production)</span>{" "}
            antara {formatDateShort(fromDate)} hingga {formatDateShort(toDate)}.
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-stone-200 bg-stone-50 px-4 py-3">
            <div className="flex items-center gap-2">
              <ShoppingCart className="h-4 w-4 text-amber-800" />
              <h2 className="text-sm font-bold text-stone-900">
                Rincian Kebutuhan Bahan ({items.length} jenis bahan,{" "}
                {needToBuyItems.length} perlu dibeli)
              </h2>
            </div>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-stone-200 bg-stone-100/80 text-[11px] font-semibold uppercase tracking-wider text-stone-600">
                <tr>
                  <th className="px-4 py-3">Bahan Baku</th>
                  <th className="px-3 py-3">Jadwal Beli</th>
                  <th className="px-3 py-3 text-right">Kebutuhan Resep</th>
                  <th className="px-3 py-3 text-right">Stok Saat Ini</th>
                  <th className="px-3 py-3 text-right">
                    Kekurangan (+Buffer)
                  </th>
                  <th className="px-3 py-3 text-right">Rekomendasi Beli</th>
                  {!isStaff ? (
                    <th className="px-3 py-3 text-right">Estimasi Biaya</th>
                  ) : null}
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-200 bg-white">
                {items.map((item) => {
                  const needBuy = toNumber(item.qty_buy_units) > 0;
                  return (
                    <tr
                      key={item.ingredient_id}
                      className={
                        needBuy ? "bg-amber-50/25 hover:bg-amber-50/50" : "hover:bg-stone-50"
                      }
                    >
                      <td className="px-4 py-3 align-top">
                        <div className="font-semibold text-stone-900">
                          {item.ingredient_name}
                        </div>
                        <div className="mt-1 flex flex-wrap items-center gap-1.5">
                          <Badge
                            variant={
                              item.rounding_rule === "pack" ? "amber" : "slate"
                            }
                            className="text-[10px]"
                          >
                            {item.rounding_rule === "pack"
                              ? `Kemasan Utuh (1 ${item.buy_unit} = ${formatNumber(item.buy_unit_size)} ${item.use_unit})`
                              : `Curah / Loose (${item.use_unit})`}
                          </Badge>
                          {item.below_min_stock ? (
                            <Badge variant="rose" className="text-[10px]">
                              Di Bawah Stok Min ({formatNumber(item.min_stock)}{" "}
                              {item.use_unit})
                            </Badge>
                          ) : null}
                        </div>
                        {item.order_codes?.length > 0 ? (
                          <p className="mt-1 text-[11px] text-stone-500">
                            Pesanan: {item.order_codes.join(", ")}
                          </p>
                        ) : null}
                      </td>

                      <td className="px-3 py-3 align-top whitespace-nowrap">
                        <div className="inline-flex items-center gap-1 font-medium text-stone-900">
                          <Calendar className="h-3.5 w-3.5 text-amber-700" />
                          {formatDateShort(item.purchase_date)}
                        </div>
                        <div className="text-[11px] text-stone-500">
                          Acara: {formatDateShort(item.earliest_event)}
                        </div>
                      </td>

                      <td className="px-3 py-3 align-top text-right font-medium text-stone-900 whitespace-nowrap">
                        {formatNumber(item.required_qty)} {item.use_unit}
                      </td>

                      <td className="px-3 py-3 align-top text-right whitespace-nowrap">
                        <span
                          className={
                            item.below_min_stock
                              ? "font-bold text-rose-700"
                              : "font-medium text-stone-800"
                          }
                        >
                          {formatNumber(item.stock)} {item.use_unit}
                        </span>
                      </td>

                      <td className="px-3 py-3 align-top text-right whitespace-nowrap">
                        {toNumber(item.need_qty) > 0 ? (
                          <span className="font-semibold text-amber-900">
                            {formatNumber(item.need_qty)} {item.use_unit}
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-emerald-700">
                            <CheckCircle2 className="h-3.5 w-3.5" />
                            Stok Cukup
                          </span>
                        )}
                      </td>

                      <td className="px-3 py-3 align-top text-right whitespace-nowrap">
                        {needBuy ? (
                          <div>
                            <span className="font-bold text-amber-900">
                              {formatNumber(item.qty_buy_units)} {item.buy_unit}
                            </span>
                            <div className="text-[11px] text-stone-500">
                              Setara {formatNumber(item.buy_use_units)}{" "}
                              {item.use_unit}
                            </div>
                          </div>
                        ) : (
                          <span className="text-stone-400">0 {item.buy_unit}</span>
                        )}
                      </td>

                      {!isStaff ? (
                        <td className="px-3 py-3 align-top text-right whitespace-nowrap">
                          <div className="font-bold text-stone-900">
                            {formatRupiah(item.estimated_cost)}
                          </div>
                          <div className="text-[11px] text-stone-500">
                            @ {formatRupiah(item.buy_price)} / {item.buy_unit}
                          </div>
                        </td>
                      ) : null}

                      <td className="px-4 py-3 align-top text-right whitespace-nowrap">
                        <Button
                          type="button"
                          size="sm"
                          variant={needBuy ? "default" : "outline"}
                          onClick={() => openPurchaseForItem(item)}
                          className="text-xs"
                        >
                          Catat Beli
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Modal Catat Pembelian Bahan */}
      {purchaseModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/50 p-4 backdrop-blur-xs">
          <Card className="w-full max-w-md overflow-hidden shadow-xl">
            <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-5 py-3.5">
              <h3 className="text-base font-bold text-stone-900">
                Catat Pembelian & Penerimaan Bahan
              </h3>
              <button
                type="button"
                onClick={() => setPurchaseModalOpen(false)}
                className="rounded-lg p-1 text-stone-500 hover:bg-stone-200/60"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handlePurchaseSubmit} className="p-5 space-y-4">
              {formError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {formError}
                </div>
              ) : null}

              <div>
                <label
                  htmlFor="purchase-ingredient"
                  className="mb-1 block text-xs font-semibold text-stone-700"
                >
                  Pilih Bahan Baku
                </label>
                <select
                  id="purchase-ingredient"
                  value={selectedIngredientId}
                  onChange={(e) => setSelectedIngredientId(e.target.value)}
                  className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 focus:border-amber-600 focus:outline-none"
                  required
                >
                  <option value="">-- Pilih Bahan --</option>
                  {(ingredientsQuery.data ?? []).map((ing) => (
                    <option key={ing.id} value={ing.id}>
                      {ing.name} (1 {ing.buy_unit} ={" "}
                      {formatNumber(ing.buy_unit_size)} {ing.use_unit})
                    </option>
                  ))}
                </select>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label
                    htmlFor="purchase-qty"
                    className="mb-1 block text-xs font-semibold text-stone-700"
                  >
                    Jumlah Beli ({selectedIngObj?.buy_unit || "satuan beli"})
                  </label>
                  <Input
                    id="purchase-qty"
                    type="number"
                    step="0.01"
                    min="0.01"
                    value={qtyBuyUnits}
                    onChange={(e) => setQtyBuyUnits(e.target.value)}
                    required
                  />
                  {selectedIngObj && Number.parseFloat(qtyBuyUnits) > 0 ? (
                    <p className="mt-1 text-[11px] text-stone-500">
                      Menambah stok +
                      {formatNumber(
                        Number.parseFloat(qtyBuyUnits) *
                          toNumber(selectedIngObj.buy_unit_size)
                      )}{" "}
                      {selectedIngObj.use_unit}
                    </p>
                  ) : null}
                </div>

                <div>
                  <label
                    htmlFor="purchase-cost"
                    className="mb-1 block text-xs font-semibold text-stone-700"
                  >
                    Total Biaya Aktual (Rp)
                  </label>
                  <Input
                    id="purchase-cost"
                    type="number"
                    step="1"
                    min="0"
                    value={totalCost}
                    onChange={(e) => setTotalCost(e.target.value)}
                    placeholder="Contoh: 250000"
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label
                    htmlFor="purchase-date"
                    className="mb-1 block text-xs font-semibold text-stone-700"
                  >
                    Tanggal Pembelian
                  </label>
                  <Input
                    id="purchase-date"
                    type="date"
                    value={purchasedAt}
                    onChange={(e) => setPurchasedAt(e.target.value)}
                    required
                  />
                </div>

                <div>
                  <label
                    htmlFor="purchase-supplier"
                    className="mb-1 block text-xs font-semibold text-stone-700"
                  >
                    Pemasok / Toko (Opsional)
                  </label>
                  <Input
                    id="purchase-supplier"
                    type="text"
                    value={supplier}
                    onChange={(e) => setSupplier(e.target.value)}
                    placeholder="Mis. Toko Bahan Kue Makmur"
                  />
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 border-t border-stone-200 pt-4">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setPurchaseModalOpen(false)}
                >
                  Batal
                </Button>
                <Button type="submit" disabled={purchaseMutation.isPending}>
                  {purchaseMutation.isPending
                    ? "Menyimpan..."
                    : "Simpan Pembelian"}
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}
    </div>
  );
}
