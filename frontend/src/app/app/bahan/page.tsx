"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Boxes,
  Edit3,
  Plus,
  RefreshCw,
  Scale,
  Search,
  Trash2,
  X,
} from "lucide-react";
import {
  getIngredients,
  createIngredient,
  updateIngredient,
  deleteIngredient,
  adjustStock,
  getErrorMessage,
} from "@/lib/api";
import { formatNumber, formatRupiah, toNumber } from "@/lib/format";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import type { Ingredient, RoundingRule } from "@/lib/types";

export default function BahanPage() {
  const { user } = useInternalSession();
  const isStaff = user?.role === "staff";
  const canEditMaster =
    user?.role === "owner" ||
    user?.role === "manager" ||
    user?.role === "admin";

  const toast = useToast();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [onlyLowStock, setOnlyLowStock] = useState(false);

  // Modal CRUD Bahan
  const [crudOpen, setCrudOpen] = useState(false);
  const [editingIng, setEditingIng] = useState<Ingredient | null>(null);
  const [name, setName] = useState("");
  const [useUnit, setUseUnit] = useState("kg");
  const [buyUnit, setBuyUnit] = useState("sak");
  const [buyUnitSize, setBuyUnitSize] = useState("1");
  const [buyPrice, setBuyPrice] = useState("0");
  const [stock, setStock] = useState("0");
  const [minStock, setMinStock] = useState("0");
  const [roundingRule, setRoundingRule] = useState<RoundingRule>("loose");
  const [isActive, setIsActive] = useState(true);
  const [crudError, setCrudError] = useState<string | null>(null);

  // Modal Koreksi Stok
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [adjustTarget, setAdjustTarget] = useState<Ingredient | null>(null);
  const [adjustMode, setAdjustMode] = useState<"delta" | "set">("delta");
  const [adjustValue, setAdjustValue] = useState("");
  const [adjustReason, setAdjustReason] = useState("");
  const [adjustError, setAdjustError] = useState<string | null>(null);

  const ingredientsQuery = useQuery({
    queryKey: ["ingredients"],
    queryFn: ({ signal }) => getIngredients(signal),
  });

  const createMutation = useMutation({
    mutationFn: createIngredient,
    onSuccess: (res) => {
      toast.success("Bahan Ditambahkan", `${res.name} berhasil dibuat.`);
      setCrudOpen(false);
      queryClient.invalidateQueries({ queryKey: ["ingredients"] });
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
    },
    onError: (err) => {
      setCrudError(getErrorMessage(err));
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: Parameters<typeof updateIngredient>[1];
    }) => updateIngredient(id, payload),
    onSuccess: (res) => {
      toast.success("Bahan Diperbarui", `${res.name} berhasil disimpan.`);
      setCrudOpen(false);
      queryClient.invalidateQueries({ queryKey: ["ingredients"] });
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
    },
    onError: (err) => {
      setCrudError(getErrorMessage(err));
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteIngredient,
    onSuccess: (res) => {
      toast.info(
        res.deactivated ? "Bahan Dinonaktifkan" : "Bahan Dihapus",
        res.deactivated
          ? "Bahan sedang dipakai di resep/riwayat sehingga dinonaktifkan."
          : "Bahan berhasil dihapus dari master."
      );
      queryClient.invalidateQueries({ queryKey: ["ingredients"] });
    },
    onError: (err) => {
      toast.error("Gagal Menghapus Bahan", getErrorMessage(err));
    },
  });

  const adjustMutation = useMutation({
    mutationFn: adjustStock,
    onSuccess: (res) => {
      toast.success(
        "Stok Bahan Dikoreksi",
        `Stok ${res.ingredient_name} berubah dari ${formatNumber(res.previous_stock)} menjadi ${formatNumber(res.new_stock)} ${res.use_unit}.`
      );
      setAdjustOpen(false);
      queryClient.invalidateQueries({ queryKey: ["ingredients"] });
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
    },
    onError: (err) => {
      setAdjustError(getErrorMessage(err));
    },
  });

  const openCreateModal = () => {
    setEditingIng(null);
    setName("");
    setUseUnit("kg");
    setBuyUnit("kg");
    setBuyUnitSize("1");
    setBuyPrice("0");
    setStock("0");
    setMinStock("1");
    setRoundingRule("loose");
    setIsActive(true);
    setCrudError(null);
    setCrudOpen(true);
  };

  const openEditModal = (ing: Ingredient) => {
    setEditingIng(ing);
    setName(ing.name);
    setUseUnit(ing.use_unit);
    setBuyUnit(ing.buy_unit);
    setBuyUnitSize(String(toNumber(ing.buy_unit_size)));
    setBuyPrice(String(toNumber(ing.buy_price)));
    setStock(String(toNumber(ing.stock)));
    setMinStock(String(toNumber(ing.min_stock)));
    setRoundingRule(ing.rounding_rule);
    setIsActive(ing.is_active);
    setCrudError(null);
    setCrudOpen(true);
  };

  const openAdjustModal = (ing: Ingredient) => {
    setAdjustTarget(ing);
    setAdjustMode("delta");
    setAdjustValue("");
    setAdjustReason("");
    setAdjustError(null);
    setAdjustOpen(true);
  };

  const handleCrudSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCrudError(null);
    if (!name.trim()) {
      setCrudError("Nama bahan wajib diisi.");
      return;
    }
    if (!editingIng) {
      createMutation.mutate({
        name: name.trim(),
        use_unit: useUnit.trim(),
        buy_unit: buyUnit.trim(),
        buy_unit_size: buyUnitSize,
        buy_price: buyPrice,
        stock,
        min_stock: minStock,
        rounding: roundingRule,
        rounding_rule: roundingRule,
        is_active: isActive,
      });
    } else {
      updateMutation.mutate({
        id: editingIng.id,
        payload: {
          name: name.trim(),
          use_unit: useUnit.trim(),
          buy_unit: buyUnit.trim(),
          buy_unit_size: buyUnitSize,
          buy_price: buyPrice,
          min_stock: minStock,
          rounding: roundingRule,
          rounding_rule: roundingRule,
          is_active: isActive,
        },
      });
    }
  };

  const handleAdjustSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setAdjustError(null);
    if (!adjustTarget) return;
    if (!adjustReason.trim()) {
      setAdjustError("Alasan koreksi stok wajib diisi.");
      return;
    }
    const val = Number.parseFloat(adjustValue);
    if (!Number.isFinite(val)) {
      setAdjustError("Nilai koreksi stok tidak valid.");
      return;
    }
    const currentStock = toNumber(adjustTarget.stock);
    const delta = adjustMode === "set" ? val - currentStock : val;
    if (delta === 0) {
      setAdjustError("Perubahan stok tidak boleh 0.");
      return;
    }

    adjustMutation.mutate({
      ingredient_id: adjustTarget.id,
      delta: String(delta),
      qty: String(delta),
      reason: adjustReason.trim(),
    });
  };

  const ingredients = ingredientsQuery.data ?? [];
  const lowStockCount = ingredients.filter(
    (i) => i.is_active && i.below_min_stock
  ).length;

  const filtered = ingredients.filter((ing) => {
    if (onlyLowStock && !ing.below_min_stock) return false;
    if (
      search.trim() &&
      !ing.name.toLowerCase().includes(search.trim().toLowerCase())
    ) {
      return false;
    }
    return true;
  });

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-stone-900">
            Master Bahan Baku & Koreksi Stok
          </h1>
          <p className="text-sm text-stone-600">
            Kelola konversi satuan pakai vs satuan beli, aturan pembulatan (
            <code className="rounded bg-stone-200/70 px-1 py-0.5 text-xs">
              loose
            </code>{" "}
            vs{" "}
            <code className="rounded bg-stone-200/70 px-1 py-0.5 text-xs">
              pack
            </code>
            ), batas stok minimum, dan stok opname.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => ingredientsQuery.refetch()}
            disabled={ingredientsQuery.isFetching}
            className="gap-1.5"
          >
            <RefreshCw
              className={`h-4 w-4 ${ingredientsQuery.isFetching ? "animate-spin" : ""}`}
            />
            Segarkan
          </Button>
          {canEditMaster ? (
            <Button
              type="button"
              size="sm"
              onClick={openCreateModal}
              className="gap-1.5"
            >
              <Plus className="h-4 w-4" />
              Tambah Bahan Baru
            </Button>
          ) : null}
        </div>
      </div>

      {/* Filter & Peringatan */}
      <Card className="p-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative flex-1 max-w-sm">
            <Search className="pointer-events-none absolute top-2.5 left-3 h-4 w-4 text-stone-400" />
            <Input
              type="text"
              placeholder="Cari nama bahan baku..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => setOnlyLowStock((v) => !v)}
              className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
                onlyLowStock
                  ? "border-rose-500 bg-rose-50 text-rose-800"
                  : "border-stone-200 bg-white text-stone-700 hover:bg-stone-50"
              }`}
            >
              <AlertTriangle className="h-3.5 w-3.5 text-rose-600" />
              Stok di Bawah Minimum ({lowStockCount})
            </button>
          </div>
        </div>
      </Card>

      {/* Tabel Bahan */}
      {ingredientsQuery.isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-12 w-full rounded-xl" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
      ) : ingredientsQuery.isError ? (
        <Card className="border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          Gagal memuat bahan baku: {getErrorMessage(ingredientsQuery.error)}
        </Card>
      ) : filtered.length === 0 ? (
        <Card className="p-8 text-center">
          <Boxes className="mx-auto h-10 w-10 text-stone-400" />
          <p className="mt-2 text-sm font-semibold text-stone-700">
            Tidak ada bahan baku yang cocok dengan pencarian.
          </p>
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-stone-200 bg-stone-100/80 text-[11px] font-semibold uppercase tracking-wider text-stone-600">
                <tr>
                  <th className="px-4 py-3">Nama Bahan</th>
                  <th className="px-3 py-3">Konversi Satuan</th>
                  <th className="px-3 py-3">Aturan Beli</th>
                  {!isStaff ? (
                    <th className="px-3 py-3 text-right">Harga / Satuan Beli</th>
                  ) : null}
                  <th className="px-3 py-3 text-right">Stok Aktual</th>
                  <th className="px-3 py-3 text-right">Stok Min</th>
                  <th className="px-4 py-3 text-right">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-200 bg-white">
                {filtered.map((ing) => (
                  <tr
                    key={ing.id}
                    className={
                      !ing.is_active
                        ? "bg-stone-100/60 opacity-60"
                        : ing.below_min_stock
                          ? "bg-rose-50/30 hover:bg-rose-50/50"
                          : "hover:bg-stone-50"
                    }
                  >
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-stone-900">
                          {ing.name}
                        </span>
                        {!ing.is_active ? (
                          <Badge variant="slate" className="text-[10px]">
                            Nonaktif
                          </Badge>
                        ) : null}
                        {ing.below_min_stock ? (
                          <Badge variant="rose" className="text-[10px]">
                            Stok Menipis
                          </Badge>
                        ) : null}
                      </div>
                    </td>

                    <td className="px-3 py-3 text-stone-700 whitespace-nowrap">
                      1 <strong>{ing.buy_unit}</strong> ={" "}
                      <strong>{formatNumber(ing.buy_unit_size)}</strong>{" "}
                      {ing.use_unit}
                    </td>

                    <td className="px-3 py-3 whitespace-nowrap">
                      <Badge
                        variant={
                          ing.rounding_rule === "pack" ? "amber" : "slate"
                        }
                        className="text-[10px]"
                      >
                        {ing.rounding_rule === "pack"
                          ? "Pack (Kemasan Utuh)"
                          : "Loose (Curah)"}
                      </Badge>
                    </td>

                    {!isStaff ? (
                      <td className="px-3 py-3 text-right font-medium text-stone-900 whitespace-nowrap">
                        {formatRupiah(ing.buy_price)} / {ing.buy_unit}
                      </td>
                    ) : null}

                    <td className="px-3 py-3 text-right whitespace-nowrap">
                      <span
                        className={
                          ing.below_min_stock
                            ? "font-bold text-rose-700"
                            : "font-bold text-stone-900"
                        }
                      >
                        {formatNumber(ing.stock)} {ing.use_unit}
                      </span>
                    </td>

                    <td className="px-3 py-3 text-right text-stone-600 whitespace-nowrap">
                      {formatNumber(ing.min_stock)} {ing.use_unit}
                    </td>

                    <td className="px-4 py-3 text-right whitespace-nowrap">
                      <div className="inline-flex items-center gap-1.5">
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          onClick={() => openAdjustModal(ing)}
                          className="gap-1 text-xs"
                        >
                          <Scale className="h-3.5 w-3.5" />
                          Koreksi Stok
                        </Button>
                        {canEditMaster ? (
                          <>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => openEditModal(ing)}
                              className="p-1.5"
                              title="Ubah Bahan"
                            >
                              <Edit3 className="h-3.5 w-3.5" />
                            </Button>
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => {
                                if (
                                  window.confirm(
                                    `Hapus atau nonaktifkan bahan "${ing.name}"?`
                                  )
                                ) {
                                  deleteMutation.mutate(ing.id);
                                }
                              }}
                              className="p-1.5 text-rose-700 hover:bg-rose-50"
                              title="Hapus Bahan"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Modal Koreksi Stok */}
      {adjustOpen && adjustTarget ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/50 p-4 backdrop-blur-xs">
          <Card className="w-full max-w-md overflow-hidden shadow-xl">
            <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-5 py-3.5">
              <div>
                <h3 className="text-base font-bold text-stone-900">
                  Koreksi Stok: {adjustTarget.name}
                </h3>
                <p className="text-xs text-stone-500">
                  Stok saat ini:{" "}
                  <strong>
                    {formatNumber(adjustTarget.stock)} {adjustTarget.use_unit}
                  </strong>
                </p>
              </div>
              <button
                type="button"
                onClick={() => setAdjustOpen(false)}
                className="rounded-lg p-1 text-stone-500 hover:bg-stone-200/60"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleAdjustSubmit} className="p-5 space-y-4">
              {adjustError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {adjustError}
                </div>
              ) : null}

              <div>
                <label className="mb-1 block text-xs font-semibold text-stone-700">
                  Cara Koreksi
                </label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setAdjustMode("delta")}
                    className={`rounded-lg border px-3 py-2 text-xs font-semibold ${
                      adjustMode === "delta"
                        ? "border-amber-700 bg-amber-50 text-amber-950"
                        : "border-stone-200 bg-white text-stone-600"
                    }`}
                  >
                    Selisih (+/- {adjustTarget.use_unit})
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setAdjustMode("set");
                      setAdjustValue(String(toNumber(adjustTarget.stock)));
                    }}
                    className={`rounded-lg border px-3 py-2 text-xs font-semibold ${
                      adjustMode === "set"
                        ? "border-amber-700 bg-amber-50 text-amber-950"
                        : "border-stone-200 bg-white text-stone-600"
                    }`}
                  >
                    Stok Fisik Baru ({adjustTarget.use_unit})
                  </button>
                </div>
              </div>

              <div>
                <label
                  htmlFor="adjust-val"
                  className="mb-1 block text-xs font-semibold text-stone-700"
                >
                  {adjustMode === "delta"
                    ? `Perubahan Stok (${adjustTarget.use_unit}, gunakan tanda minus untuk pengurangan)`
                    : `Jumlah Stok Fisik Hasil Opname (${adjustTarget.use_unit})`}
                </label>
                <Input
                  id="adjust-val"
                  type="number"
                  step="0.01"
                  value={adjustValue}
                  onChange={(e) => setAdjustValue(e.target.value)}
                  placeholder={adjustMode === "delta" ? "Mis. -1.5 atau 2" : "Mis. 10"}
                  required
                />
              </div>

              <div>
                <label
                  htmlFor="adjust-reason"
                  className="mb-1 block text-xs font-semibold text-stone-700"
                >
                  Alasan Koreksi Stok (Wajib)
                </label>
                <Input
                  id="adjust-reason"
                  type="text"
                  value={adjustReason}
                  onChange={(e) => setAdjustReason(e.target.value)}
                  placeholder="Contoh: Stok opname mingguan / bahan rusak"
                  required
                />
              </div>

              <div className="flex items-center justify-end gap-2 border-t border-stone-200 pt-4">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setAdjustOpen(false)}
                >
                  Batal
                </Button>
                <Button type="submit" disabled={adjustMutation.isPending}>
                  {adjustMutation.isPending
                    ? "Menyimpan..."
                    : "Simpan Koreksi"}
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}

      {/* Modal Tambah / Ubah Bahan */}
      {crudOpen && canEditMaster ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/50 p-4 backdrop-blur-xs">
          <Card className="w-full max-w-lg overflow-hidden shadow-xl">
            <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-5 py-3.5">
              <h3 className="text-base font-bold text-stone-900">
                {editingIng
                  ? `Ubah Bahan: ${editingIng.name}`
                  : "Tambah Bahan Baku Baru"}
              </h3>
              <button
                type="button"
                onClick={() => setCrudOpen(false)}
                className="rounded-lg p-1 text-stone-500 hover:bg-stone-200/60"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleCrudSubmit} className="p-5 space-y-4">
              {crudError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {crudError}
                </div>
              ) : null}

              <div>
                <label className="mb-1 block text-xs font-semibold text-stone-700">
                  Nama Bahan Baku
                </label>
                <Input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Contoh: Tepung Terigu Protein Sedang"
                  required
                />
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Satuan Pakai Resep
                  </label>
                  <Input
                    type="text"
                    value={useUnit}
                    onChange={(e) => setUseUnit(e.target.value)}
                    placeholder="kg / liter / butir"
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Satuan Beli Pasar
                  </label>
                  <Input
                    type="text"
                    value={buyUnit}
                    onChange={(e) => setBuyUnit(e.target.value)}
                    placeholder="sak / dus / peti"
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Isi per Satuan Beli
                  </label>
                  <Input
                    type="number"
                    step="0.01"
                    min="0.01"
                    value={buyUnitSize}
                    onChange={(e) => setBuyUnitSize(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Harga per {buyUnit || "Satuan Beli"} (Rp)
                  </label>
                  <Input
                    type="number"
                    step="1"
                    min="0"
                    value={buyPrice}
                    onChange={(e) => setBuyPrice(e.target.value)}
                    required
                  />
                </div>
                {!editingIng ? (
                  <div>
                    <label className="mb-1 block text-xs font-semibold text-stone-700">
                      Stok Awal ({useUnit})
                    </label>
                    <Input
                      type="number"
                      step="0.01"
                      min="0"
                      value={stock}
                      onChange={(e) => setStock(e.target.value)}
                      required
                    />
                  </div>
                ) : null}
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Stok Minimum ({useUnit})
                  </label>
                  <Input
                    type="number"
                    step="0.01"
                    min="0"
                    value={minStock}
                    onChange={(e) => setMinStock(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Aturan Pembulatan Belanja
                  </label>
                  <select
                    value={roundingRule}
                    onChange={(e) =>
                      setRoundingRule(e.target.value as RoundingRule)
                    }
                    className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900"
                  >
                    <option value="loose">
                      Loose (Curah : boleh pecahan satuan beli)
                    </option>
                    <option value="pack">
                      Pack (Kemasan Utuh : dibulatkan ke atas)
                    </option>
                  </select>
                </div>

                <div className="flex items-end pb-1">
                  <label className="inline-flex items-center gap-2 text-xs font-semibold text-stone-700">
                    <input
                      type="checkbox"
                      checked={isActive}
                      onChange={(e) => setIsActive(e.target.checked)}
                      className="h-4 w-4 rounded border-stone-300 text-amber-700"
                    />
                    Bahan Aktif Digunakan
                  </label>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 border-t border-stone-200 pt-4">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setCrudOpen(false)}
                >
                  Batal
                </Button>
                <Button
                  type="submit"
                  disabled={
                    createMutation.isPending || updateMutation.isPending
                  }
                >
                  {createMutation.isPending || updateMutation.isPending
                    ? "Menyimpan..."
                    : "Simpan Bahan"}
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}
    </div>
  );
}
