"use client";

import { useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Calculator,
  ChefHat,
  Edit3,
  Filter,
  Flame,
  Plus,
  RefreshCw,
  Search,
  Trash2,
  Utensils,
  X,
} from "lucide-react";
import {
  getProducts,
  getProductsCosting,
  createProduct,
  updateProduct,
  deleteProduct,
  getProductRecipes,
  upsertProductRecipes,
  getProductLoads,
  upsertProductLoads,
  getIngredients,
  getResources,
  getErrorMessage,
} from "@/lib/api";
import {
  formatCategoryName,
  formatNumber,
  formatPercent,
  formatRupiah,
  normalizeProductCategoryValue,
  PRODUCT_CATEGORY_OPTIONS,
  toNumber,
} from "@/lib/format";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Pagination } from "@/components/ui/pagination";
import type { Product, ProductCostingDTO } from "@/lib/types";

interface DraftRecipeRow {
  ingredient_id: string;
  qty_per_unit: string;
}

interface DraftLoadRow {
  resource_id: string;
  load_per_unit: string;
}

export default function ProdukPage() {
  const toast = useToast();
  const queryClient = useQueryClient();

  // Modal CRUD Produk
  const [crudOpen, setCrudOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<Product | null>(null);
  const [name, setName] = useState("");
  const [category, setCategory] = useState("snack_box");
  const [price, setPrice] = useState("25000");
  const [packagingCost, setPackagingCost] = useState("2000");
  const [overheadPerUnit, setOverheadPerUnit] = useState("1000");
  const [wastePct, setWastePct] = useState("0.05");
  const [leadTimeDays, setLeadTimeDays] = useState("2");
  const [isActive, setIsActive] = useState(true);
  const [crudError, setCrudError] = useState<string | null>(null);

  // Modal Resep & Beban Produksi
  const [recipeModalProduct, setRecipeModalProduct] =
    useState<Product | null>(null);
  const [draftRecipes, setDraftRecipes] = useState<DraftRecipeRow[]>([]);
  const [draftLoads, setDraftLoads] = useState<DraftLoadRow[]>([]);
  const [recipeModalLoading, setRecipeModalLoading] = useState(false);
  const [recipeModalError, setRecipeModalError] = useState<string | null>(null);

  // Filter, Pencarian, & Paginasi
  const [searchQuery, setSearchQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("all");
  const [currentPage, setCurrentPage] = useState(1);
  const PAGE_SIZE = 6;

  const productsQuery = useQuery({
    queryKey: ["products"],
    queryFn: ({ signal }) => getProducts(signal),
  });

  const costingQuery = useQuery({
    queryKey: ["costing-products"],
    queryFn: ({ signal }) => getProductsCosting(signal),
  });

  const ingredientsQuery = useQuery({
    queryKey: ["ingredients"],
    queryFn: ({ signal }) => getIngredients(signal),
  });

  const resourcesQuery = useQuery({
    queryKey: ["resources"],
    queryFn: ({ signal }) => getResources(signal),
  });

  const createMutation = useMutation({
    mutationFn: createProduct,
    onSuccess: (res) => {
      toast.success("Produk Ditambahkan", `${res.name} berhasil dibuat.`);
      setCrudOpen(false);
      queryClient.invalidateQueries({ queryKey: ["products"] });
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
    },
    onError: (err) => setCrudError(getErrorMessage(err)),
  });

  const updateMutation = useMutation({
    mutationFn: ({
      id,
      payload,
    }: {
      id: string;
      payload: Parameters<typeof updateProduct>[1];
    }) => updateProduct(id, payload),
    onSuccess: (res) => {
      toast.success("Produk Diperbarui", `${res.name} berhasil disimpan.`);
      setCrudOpen(false);
      queryClient.invalidateQueries({ queryKey: ["products"] });
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
    },
    onError: (err) => setCrudError(getErrorMessage(err)),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProduct,
    onSuccess: (res) => {
      toast.info(
        res.deactivated ? "Produk Dinonaktifkan" : "Produk Dihapus",
        res.deactivated
          ? "Produk sudah tercatat di pesanan sehingga dinonaktifkan."
          : "Produk berhasil dihapus."
      );
      queryClient.invalidateQueries({ queryKey: ["products"] });
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
    },
    onError: (err) => {
      toast.error("Gagal Menghapus Produk", getErrorMessage(err));
    },
  });

  const saveRecipesAndLoadsMutation = useMutation({
    mutationFn: async ({
      productId,
      recipes,
      loads,
    }: {
      productId: string;
      recipes: DraftRecipeRow[];
      loads: DraftLoadRow[];
    }) => {
      const cleanRecipes = recipes
        .filter(
          (r) => r.ingredient_id && Number.parseFloat(r.qty_per_unit) > 0
        )
        .map((r) => ({
          ingredient_id: r.ingredient_id,
          qty_per_unit: r.qty_per_unit,
        }));
      await upsertProductRecipes(productId, {
        replace_all: true,
        items: cleanRecipes,
        recipes: cleanRecipes,
      });

      const cleanLoads = loads
        .filter(
          (l) => l.resource_id && Number.parseFloat(l.load_per_unit) >= 0
        )
        .map((l) => ({
          resource_id: l.resource_id,
          load: l.load_per_unit,
          load_per_unit: l.load_per_unit,
        }));
      await upsertProductLoads(productId, {
        replace_all: true,
        items: cleanLoads,
        loads: cleanLoads,
      });
    },
    onSuccess: () => {
      toast.success(
        "Resep & Beban Produksi Disimpan",
        "Komposisi bahan dan beban kapasitas produk telah diperbarui; HPP dihitung ulang otomatis."
      );
      setRecipeModalProduct(null);
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
    },
    onError: (err) => {
      setRecipeModalError(getErrorMessage(err));
    },
  });

  const openCreateModal = () => {
    setEditingProduct(null);
    setName("");
    setCategory("snack_box");
    setPrice("25000");
    setPackagingCost("2000");
    setOverheadPerUnit("1000");
    setWastePct("0.05");
    setLeadTimeDays("2");
    setIsActive(true);
    setCrudError(null);
    setCrudOpen(true);
  };

  const openEditModal = (prod: Product) => {
    setEditingProduct(prod);
    setName(prod.name);
    setCategory(normalizeProductCategoryValue(prod.category));
    setPrice(String(toNumber(prod.price)));
    setPackagingCost(String(toNumber(prod.packaging_cost)));
    const ovh = prod.overhead_per_unit;
    setOverheadPerUnit(String(toNumber(ovh)));
    setWastePct(String(prod.waste_pct !== undefined ? toNumber(prod.waste_pct) : 0.05));
    setLeadTimeDays(String(prod.lead_time_days !== undefined ? prod.lead_time_days : 2));
    setIsActive(prod.is_active);
    setCrudError(null);
    setCrudOpen(true);
  };

  const openRecipeAndLoadModal = async (prod: Product) => {
    setRecipeModalProduct(prod);
    setRecipeModalError(null);
    setRecipeModalLoading(true);
    try {
      const [recs, lds] = await Promise.all([
        getProductRecipes(prod.id),
        getProductLoads(prod.id),
      ]);
      setDraftRecipes(
        recs.map((r) => ({
          ingredient_id: r.ingredient_id,
          qty_per_unit: String(toNumber(r.qty_per_unit)),
        }))
      );
      const allResources = resourcesQuery.data ?? [];
      if (allResources.length > 0) {
        setDraftLoads(
          allResources.map((res) => {
            const found = lds.find((l) => l.resource_id === res.id);
            return {
              resource_id: res.id,
              load_per_unit: found
                ? String(toNumber(found.load_per_unit))
                : "0",
            };
          })
        );
      } else {
        setDraftLoads(
          lds.map((l) => ({
            resource_id: l.resource_id,
            load_per_unit: String(toNumber(l.load_per_unit)),
          }))
        );
      }
    } catch (err) {
      setRecipeModalError(getErrorMessage(err));
    } finally {
      setRecipeModalLoading(false);
    }
  };

  const handleCrudSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setCrudError(null);
    if (!name.trim()) {
      setCrudError("Nama produk wajib diisi.");
      return;
    }
    const leadNum = Number.parseInt(leadTimeDays, 10);
    const cleanWaste = wastePct.replace(",", ".");
    const wasteNum = Number.parseFloat(cleanWaste);
    const validWaste = Number.isFinite(wasteNum) && wasteNum >= 0 ? cleanWaste : "0.05";
    const validLead = Number.isFinite(leadNum) && leadNum >= 0 ? leadNum : 2;

    if (!editingProduct) {
      createMutation.mutate({
        name: name.trim(),
        category,
        price,
        packaging_cost: packagingCost,
        overhead_cost: overheadPerUnit,
        overhead_per_unit: overheadPerUnit,
        waste_pct: validWaste,
        lead_time_days: validLead,
        is_active: isActive,
      });
    } else {
      updateMutation.mutate({
        id: editingProduct.id,
        payload: {
          name: name.trim(),
          category,
          price,
          packaging_cost: packagingCost,
          overhead_cost: overheadPerUnit,
          overhead_per_unit: overheadPerUnit,
          waste_pct: validWaste,
          lead_time_days: validLead,
          is_active: isActive,
        },
      });
    }
  };

  const products = useMemo(() => productsQuery.data ?? [], [productsQuery.data]);
  const costingMap = new Map<string, ProductCostingDTO>();
  for (const c of costingQuery.data ?? []) {
    costingMap.set(c.product_id, c);
  }

  const filteredProducts = useMemo(() => {
    return products.filter((prod) => {
      if (categoryFilter !== "all") {
        const normCat = normalizeProductCategoryValue(prod.category);
        if (normCat !== categoryFilter) return false;
      }
      if (searchQuery.trim()) {
        const q = searchQuery.trim().toLowerCase();
        const matchName = prod.name.toLowerCase().includes(q);
        const matchCat = formatCategoryName(prod.category).toLowerCase().includes(q);
        if (!matchName && !matchCat) return false;
      }
      return true;
    });
  }, [products, categoryFilter, searchQuery]);

  const totalPages = Math.ceil(filteredProducts.length / PAGE_SIZE) || 1;
  const paginatedProducts = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return filteredProducts.slice(start, start + PAGE_SIZE);
  }, [filteredProducts, currentPage, PAGE_SIZE]);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-stone-900">
            Master Produk, Resep, Beban & Kalkulasi HPP
          </h1>
          <p className="text-sm text-stone-600">
            Kelola katalog produk, komposisi bahan per unit, beban oven & tenaga
            kerja, serta rincian HPP (
            <code className="rounded bg-stone-200/70 px-1 py-0.5 text-xs">
              UnitCost
            </code>
            ), margin, harga minimum, dan diskon maksimum.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              productsQuery.refetch();
              costingQuery.refetch();
            }}
            disabled={productsQuery.isFetching || costingQuery.isFetching}
            className="gap-1.5"
          >
            <RefreshCw
              className={`h-4 w-4 ${
                productsQuery.isFetching || costingQuery.isFetching
                  ? "animate-spin"
                  : ""
              }`}
            />
            Segarkan
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={openCreateModal}
            className="gap-1.5"
          >
            <Plus className="h-4 w-4" />
            Tambah Produk Baru
          </Button>
        </div>
      </div>

      {/* Toolbar Pencarian & Filter Kategori */}
      {products.length > 0 && (
        <div className="space-y-3">
          <div className="flex flex-col gap-3 rounded-2xl border border-stone-200 bg-white p-3.5 shadow-2xs sm:flex-row sm:items-center sm:justify-between">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-stone-400" />
              <Input
                type="text"
                placeholder="Cari nama produk atau kategori..."
                value={searchQuery}
                onChange={(e) => {
                  setSearchQuery(e.target.value);
                  setCurrentPage(1);
                }}
                className="pl-9 pr-9 text-xs sm:text-sm bg-stone-50/50 focus:bg-white"
              />
              {searchQuery && (
                <button
                  type="button"
                  onClick={() => {
                    setSearchQuery("");
                    setCurrentPage(1);
                  }}
                  className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full p-1 text-stone-400 hover:bg-stone-100 hover:text-stone-700"
                  title="Hapus pencarian"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center gap-1.5">
                <Filter className="h-4 w-4 text-stone-500 shrink-0" />
                <select
                  value={categoryFilter}
                  onChange={(e) => {
                    setCategoryFilter(e.target.value);
                    setCurrentPage(1);
                  }}
                  className="rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-semibold text-stone-800 focus:border-amber-500 focus:bg-white focus:outline-none"
                >
                  <option value="all">Semua Kategori ({products.length})</option>
                  {PRODUCT_CATEGORY_OPTIONS.map((cat) => {
                    const count = products.filter(
                      (p) => normalizeProductCategoryValue(p.category) === cat.value
                    ).length;
                    return (
                      <option key={cat.value} value={cat.value}>
                        {cat.label} ({count})
                      </option>
                    );
                  })}
                </select>
              </div>

              {(searchQuery || categoryFilter !== "all") && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setSearchQuery("");
                    setCategoryFilter("all");
                    setCurrentPage(1);
                  }}
                  className="text-xs"
                >
                  Reset Filter
                </Button>
              )}
            </div>
          </div>

          {/* Quick Filter Pills */}
          <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
            <button
              type="button"
              onClick={() => {
                setCategoryFilter("all");
                setCurrentPage(1);
              }}
              className={`shrink-0 rounded-full px-3 py-1 text-xs font-semibold transition cursor-pointer ${
                categoryFilter === "all"
                  ? "bg-stone-900 text-white shadow-2xs"
                  : "bg-white border border-stone-200 text-stone-700 hover:bg-stone-100"
              }`}
            >
              Semua ({products.length})
            </button>
            {PRODUCT_CATEGORY_OPTIONS.map((cat) => {
              const count = products.filter(
                (p) => normalizeProductCategoryValue(p.category) === cat.value
              ).length;
              if (count === 0) return null;
              const isSelected = categoryFilter === cat.value;
              return (
                <button
                  key={cat.value}
                  type="button"
                  onClick={() => {
                    setCategoryFilter(cat.value);
                    setCurrentPage(1);
                  }}
                  className={`shrink-0 rounded-full px-3 py-1 text-xs font-semibold transition cursor-pointer ${
                    isSelected
                      ? "bg-[#E05A47] text-white shadow-2xs"
                      : "bg-white border border-stone-200 text-stone-700 hover:bg-stone-100"
                  }`}
                >
                  {cat.label} ({count})
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Daftar Produk & Costing */}
      {productsQuery.isLoading || costingQuery.isLoading ? (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <Skeleton className="h-64 w-full rounded-xl" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
      ) : productsQuery.isError ? (
        <Card className="border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          Gagal memuat produk: {getErrorMessage(productsQuery.error)}
        </Card>
      ) : products.length === 0 ? (
        <Card className="p-8 text-center">
          <ChefHat className="mx-auto h-10 w-10 text-stone-400" />
          <p className="mt-2 text-sm font-semibold text-stone-700">
            Belum ada produk terdaftar.
          </p>
        </Card>
      ) : filteredProducts.length === 0 ? (
        <Card className="p-8 text-center">
          <Search className="mx-auto h-10 w-10 text-stone-400" />
          <p className="mt-2 text-sm font-bold text-stone-800">
            Tidak ada produk yang cocok
          </p>
          <p className="mt-1 text-xs text-stone-500">
            Tidak ditemukan produk dengan kata kunci &ldquo;{searchQuery}&rdquo;
            {categoryFilter !== "all" ? ` pada kategori ini` : ""}.
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-4"
            onClick={() => {
              setSearchQuery("");
              setCategoryFilter("all");
              setCurrentPage(1);
            }}
          >
            Reset Filter
          </Button>
        </Card>
      ) : (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
            {paginatedProducts.map((prod) => {
            const cost = costingMap.get(prod.id);
            const marginNum = toNumber(cost?.margin);
            const minMarginNum = toNumber(cost?.min_margin);
            const targetMarginNum = toNumber(cost?.target_margin);

            return (
              <Card
                key={prod.id}
                className={`flex flex-col justify-between p-5 ${
                  !prod.is_active ? "opacity-65 bg-stone-50" : ""
                }`}
              >
                <div className="space-y-4">
                  {/* Baris Atas: Judul & Harga */}
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <h2 className="text-base font-bold text-stone-900">
                          {prod.name}
                        </h2>
                        <Badge variant="slate" className="text-[10px]">
                          {formatCategoryName(prod.category)}
                        </Badge>
                        {!prod.is_active ? (
                          <Badge variant="rose" className="text-[10px]">
                            Nonaktif
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mt-1 text-xs text-stone-500">
                        Lead Time Minimum:{" "}
                        <strong>H-{prod.lead_time_days !== undefined ? prod.lead_time_days : 2} hari</strong> · Waste
                        Bahan: <strong>{formatPercent(prod.waste_pct !== undefined ? prod.waste_pct : (cost?.waste_pct || 0.05))}</strong>
                      </p>
                    </div>

                    <div className="text-right">
                      <span className="block text-[11px] font-medium text-stone-500">
                        Harga Jual Katalog
                      </span>
                      <span className="text-lg font-extrabold text-amber-900">
                        {formatRupiah(prod.price)}
                      </span>
                    </div>
                  </div>

                  {/* Rincian Costing dari /api/v1/costing/products */}
                  {cost ? (
                    <div className="rounded-xl border border-stone-200 bg-stone-50/80 p-3.5 space-y-3">
                      <div className="flex items-center justify-between border-b border-stone-200 pb-2">
                        <div className="flex items-center gap-1.5 text-xs font-bold text-stone-800">
                          <Calculator className="h-3.5 w-3.5 text-amber-700" />
                          HPP per Unit:{" "}
                          <span className="text-stone-950">
                            {formatRupiah(cost.unit_cost)}
                          </span>
                        </div>
                        <Badge
                          variant={
                            marginNum < minMarginNum
                              ? "rose"
                              : marginNum < targetMarginNum
                                ? "amber"
                                : "emerald"
                          }
                        >
                          Margin: {formatPercent(cost.margin)}
                        </Badge>
                      </div>

                      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs sm:grid-cols-4">
                        <div>
                          <span className="block text-[10px] text-stone-500">
                            Bahan (+Waste)
                          </span>
                          <span className="font-semibold text-stone-800">
                            {formatRupiah(cost.ingredient_cost)}
                          </span>
                        </div>
                        <div>
                          <span className="block text-[10px] text-stone-500">
                            Kemasan
                          </span>
                          <span className="font-semibold text-stone-800">
                            {formatRupiah(cost.packaging_cost)}
                          </span>
                        </div>
                        <div>
                          <span className="block text-[10px] text-stone-500">
                            Tenaga ({formatNumber(cost.labor_minutes)} mnt)
                          </span>
                          <span className="font-semibold text-stone-800">
                            {formatRupiah(cost.labor_cost)}
                          </span>
                        </div>
                        <div>
                          <span className="block text-[10px] text-stone-500">
                            Overhead
                          </span>
                          <span className="font-semibold text-stone-800">
                            {formatRupiah(cost.overhead_per_unit ?? prod.overhead_per_unit ?? 0)}
                          </span>
                        </div>
                      </div>

                      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-stone-200/80 pt-2 text-[11px] text-stone-600">
                        <span>
                          Harga Min (G2 {formatPercent(cost.min_margin)}):{" "}
                          <strong className="text-stone-900">
                            {formatRupiah(cost.min_price)}
                          </strong>
                        </span>
                        <span>
                          Diskon Maks:{" "}
                          <strong className="text-emerald-800">
                            {formatPercent(cost.max_discount)}
                          </strong>
                        </span>
                      </div>
                    </div>
                  ) : null}
                </div>

                {/* Tombol Aksi */}
                <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-stone-200 pt-3">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => openRecipeAndLoadModal(prod)}
                    className="gap-1.5 text-xs"
                  >
                    <Utensils className="h-3.5 w-3.5 text-amber-700" />
                    Atur Resep & Beban
                  </Button>

                  <div className="flex items-center gap-1.5">
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => openEditModal(prod)}
                      className="gap-1 text-xs"
                    >
                      <Edit3 className="h-3.5 w-3.5" />
                      Ubah
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        if (
                          window.confirm(
                            `Hapus atau nonaktifkan produk "${prod.name}"?`
                          )
                        ) {
                          deleteMutation.mutate(prod.id);
                        }
                      }}
                      className="p-1.5 text-rose-700 hover:bg-rose-50"
                      title="Hapus Produk"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>
              </Card>
            );
          })}
          </div>

          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            totalItems={filteredProducts.length}
            pageSize={PAGE_SIZE}
            onPageChange={setCurrentPage}
            itemName="produk"
          />
        </div>
      )}

      {/* Modal Tambah / Ubah Produk */}
      {crudOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/50 p-4 backdrop-blur-xs">
          <Card className="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden shadow-xl">
            <div className="flex shrink-0 items-center justify-between border-b border-stone-200 bg-stone-50 px-5 py-3.5">
              <h3 className="text-base font-bold text-stone-900">
                {editingProduct
                  ? `Ubah Produk: ${editingProduct.name}`
                  : "Tambah Produk Baru"}
              </h3>
              <button
                type="button"
                onClick={() => setCrudOpen(false)}
                className="rounded-lg p-1 text-stone-500 hover:bg-stone-200/60"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleCrudSubmit} className="space-y-4 overflow-y-auto p-5">
              {crudError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {crudError}
                </div>
              ) : null}

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Nama Produk
                  </label>
                  <Input
                    type="text"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Contoh: Paket Snack Box Standar"
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Kategori
                  </label>
                  <select
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                    className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900"
                  >
                    {PRODUCT_CATEGORY_OPTIONS.map((opt) => (
                      <option key={opt.value} value={opt.value}>
                        {opt.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Harga Jual (Rp)
                  </label>
                  <Input
                    type="number"
                    step="1"
                    min="1"
                    value={price}
                    onChange={(e) => setPrice(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Biaya Kemasan (Rp)
                  </label>
                  <Input
                    type="number"
                    step="1"
                    min="0"
                    value={packagingCost}
                    onChange={(e) => setPackagingCost(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Overhead / Unit (Rp)
                  </label>
                  <Input
                    type="number"
                    step="1"
                    min="0"
                    value={overheadPerUnit}
                    onChange={(e) => setOverheadPerUnit(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Rasio Waste (0 – 1)
                  </label>
                  <Input
                    type="number"
                    step="0.01"
                    min="0"
                    max="1"
                    value={wastePct}
                    onChange={(e) => setWastePct(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-semibold text-stone-700">
                    Lead Time Minimum (Hari)
                  </label>
                  <Input
                    type="number"
                    step="1"
                    min="0"
                    value={leadTimeDays}
                    onChange={(e) => setLeadTimeDays(e.target.value)}
                    required
                  />
                </div>
                <div className="flex items-end pb-1">
                  <label className="inline-flex items-center gap-2 text-xs font-semibold text-stone-700">
                    <input
                      type="checkbox"
                      checked={isActive}
                      onChange={(e) => setIsActive(e.target.checked)}
                      className="h-4 w-4 rounded border-stone-300 text-amber-700"
                    />
                    Tampil Aktif di Katalog
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
                    : "Simpan Produk"}
                </Button>
              </div>
            </form>
          </Card>
        </div>
      ) : null}

      {/* Modal Resep Bahan & Beban Kapasitas */}
      {recipeModalProduct ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-stone-900/50 p-4 backdrop-blur-xs">
          <Card className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden shadow-xl">
            <div className="flex items-center justify-between border-b border-stone-200 bg-stone-50 px-5 py-3.5">
              <div>
                <h3 className="text-base font-bold text-stone-900">
                  Resep & Beban Produksi: {recipeModalProduct.name}
                </h3>
                <p className="text-xs text-stone-500">
                  Komposisi bahan menentukan HPP & daftar belanja; beban
                  sumber daya menentukan utilisasi kapasitas (G1) & biaya
                  tenaga.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setRecipeModalProduct(null)}
                className="rounded-lg p-1 text-stone-500 hover:bg-stone-200/60"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-5 space-y-6">
              {recipeModalError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800">
                  {recipeModalError}
                </div>
              ) : null}

              {recipeModalLoading ? (
                <div className="space-y-3">
                  <Skeleton className="h-24 w-full" />
                  <Skeleton className="h-24 w-full" />
                </div>
              ) : (
                <>
                  {/* Bagian Beban Kapasitas */}
                  <div className="space-y-3">
                    <div className="flex items-center gap-2">
                      <Flame className="h-4 w-4 text-amber-700" />
                      <h4 className="text-sm font-bold text-stone-900">
                        Beban Sumber Daya per Unit Produk
                      </h4>
                    </div>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      {draftLoads.map((row, idx) => {
                        const resMeta = (resourcesQuery.data ?? []).find(
                          (r) => r.id === row.resource_id
                        );
                        return (
                          <div
                            key={row.resource_id}
                            className="rounded-xl border border-stone-200 bg-stone-50 p-3"
                          >
                            <label className="mb-1 block text-xs font-semibold text-stone-800">
                              {resMeta?.name || "Sumber Daya"} (
                              {resMeta?.unit || "unit"})
                            </label>
                            <Input
                              type="number"
                              step="0.01"
                              min="0"
                              value={row.load_per_unit}
                              onChange={(e) => {
                                const val = e.target.value;
                                setDraftLoads((prev) =>
                                  prev.map((item, i) =>
                                    i === idx
                                      ? { ...item, load_per_unit: val }
                                      : item
                                  )
                                );
                              }}
                            />
                            {resMeta ? (
                              <p className="mt-1 text-[11px] text-stone-500">
                                Kapasitas harian:{" "}
                                {formatNumber(resMeta.daily_capacity)}{" "}
                                {resMeta.unit}/hari
                              </p>
                            ) : null}
                          </div>
                        );
                      })}
                    </div>
                  </div>

                  {/* Bagian Resep Bahan */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Utensils className="h-4 w-4 text-amber-700" />
                        <h4 className="text-sm font-bold text-stone-900">
                          Komposisi Resep Bahan per Unit Produk
                        </h4>
                      </div>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          const firstIng = ingredientsQuery.data?.[0];
                          setDraftRecipes((prev) => [
                            ...prev,
                            {
                              ingredient_id: firstIng?.id ?? "",
                              qty_per_unit: "0.1",
                            },
                          ]);
                        }}
                        className="gap-1 text-xs"
                      >
                        <Plus className="h-3.5 w-3.5" />
                        Tambah Bahan
                      </Button>
                    </div>

                    {draftRecipes.length === 0 ? (
                      <p className="rounded-lg border border-dashed border-stone-200 p-4 text-center text-xs text-stone-500">
                        Belum ada item bahan di resep produk ini. Klik{" "}
                        <strong>Tambah Bahan</strong> untuk menambahkan.
                      </p>
                    ) : (
                      <div className="space-y-2">
                        {draftRecipes.map((row, idx) => {
                          const ingMeta = (ingredientsQuery.data ?? []).find(
                            (i) => i.id === row.ingredient_id
                          );
                          return (
                            <div
                              key={`${row.ingredient_id}-${idx}`}
                              className="flex flex-col gap-2 rounded-lg border border-stone-200 bg-white p-2.5 sm:flex-row sm:items-center"
                            >
                              <select
                                value={row.ingredient_id}
                                onChange={(e) => {
                                  const val = e.target.value;
                                  setDraftRecipes((prev) =>
                                    prev.map((item, i) =>
                                      i === idx
                                        ? { ...item, ingredient_id: val }
                                        : item
                                    )
                                  );
                                }}
                                className="w-full rounded-lg border border-stone-300 bg-white px-2.5 py-2 text-xs text-stone-900 sm:flex-1 sm:py-1.5"
                              >
                                <option value="">-- Pilih Bahan --</option>
                                {(ingredientsQuery.data ?? []).map((ing) => (
                                  <option key={ing.id} value={ing.id}>
                                    {ing.name} ({ing.use_unit})
                                  </option>
                                ))}
                              </select>

                              <div className="flex items-center justify-between gap-2 sm:justify-start">
                                <div className="flex flex-1 items-center gap-1.5 sm:flex-initial">
                                  <Input
                                    type="number"
                                    step="0.001"
                                    min="0.001"
                                    value={row.qty_per_unit}
                                    onChange={(e) => {
                                      const val = e.target.value;
                                      setDraftRecipes((prev) =>
                                        prev.map((item, i) =>
                                          i === idx
                                            ? { ...item, qty_per_unit: val }
                                            : item
                                        )
                                      );
                                    }}
                                    className="w-full text-xs sm:w-28"
                                  />
                                  <span className="w-12 shrink-0 text-xs text-stone-600">
                                    {ingMeta?.use_unit || "unit"}
                                  </span>
                                </div>

                                <button
                                  type="button"
                                  onClick={() =>
                                    setDraftRecipes((prev) =>
                                      prev.filter((_, i) => i !== idx)
                                    )
                                  }
                                  className="rounded p-1.5 text-rose-600 hover:bg-rose-50"
                                  title="Hapus Baris"
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </button>
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-stone-200 bg-stone-50 px-5 py-3.5">
              <Button
                type="button"
                variant="outline"
                onClick={() => setRecipeModalProduct(null)}
              >
                Batal
              </Button>
              <Button
                type="button"
                disabled={
                  recipeModalLoading || saveRecipesAndLoadsMutation.isPending
                }
                onClick={() =>
                  saveRecipesAndLoadsMutation.mutate({
                    productId: recipeModalProduct.id,
                    recipes: draftRecipes,
                    loads: draftLoads,
                  })
                }
              >
                {saveRecipesAndLoadsMutation.isPending
                  ? "Menyimpan..."
                  : "Simpan Resep & Beban"}
              </Button>
            </div>
          </Card>
        </div>
      ) : null}
    </div>
  );
}
