"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowRight,
  ArrowUpRight,
  CheckCircle2,
  Flame,
  PackageSearch,
  ShoppingBag,
  Utensils,
  XCircle,
} from "lucide-react";
import { getPublicProducts, getErrorMessage } from "@/lib/api";
import {
  formatCategoryName,
  formatRupiah,
  normalizeProductCategoryValue,
} from "@/lib/format";
import {
  getProductVisualMeta,
  SIGNATURE_SHOWCASE_ITEMS,
} from "@/lib/product-visuals";
import { PublicNavbar } from "@/components/public-navbar";
import { KalkurasaLogoIcon } from "@/components/kalkurasa-logo";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState } from "@/components/ui/states";

export default function KatalogHomePage() {
  const router = useRouter();
  const [selectedCategory, setSelectedCategory] = useState<string>("ALL");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [expandedProductId, setExpandedProductId] = useState<string | null>(
    null
  );

  // Dynamic Flavor/Dish Stage Switcher (Slight Twist Hero Stage)
  const [activeFlavorIndex, setActiveFlavorIndex] = useState<number>(0);
  const activeShowcase =
    SIGNATURE_SHOWCASE_ITEMS[activeFlavorIndex] ?? SIGNATURE_SHOWCASE_ITEMS[0];

  // Quick Order Tracker State
  const [trackCode, setTrackCode] = useState<string>("");
  const [trackError, setTrackError] = useState<string | null>(null);

  const {
    data: products,
    isLoading,
    isError,
    error,
    refetch,
  } = useQuery({
    queryKey: ["public", "products"],
    queryFn: ({ signal }) => getPublicProducts(signal),
  });

  const activeProducts = useMemo(() => {
    return products ?? [];
  }, [products]);

  const categories = useMemo(() => {
    const unique = Array.from(
      new Set(
        activeProducts.map((p) => normalizeProductCategoryValue(p.category))
      )
    );
    return unique;
  }, [activeProducts]);

  const filteredProducts = useMemo(() => {
    return activeProducts.filter((product) => {
      const normalizedCat = normalizeProductCategoryValue(product.category);
      const matchesCategory =
        selectedCategory === "ALL" || normalizedCat === selectedCategory;
      const q = searchQuery.trim().toLowerCase();
      const visual = getProductVisualMeta(product);
      const matchesSearch =
        !q ||
        product.name.toLowerCase().includes(q) ||
        visual.story.toLowerCase().includes(q) ||
        formatCategoryName(product.category).toLowerCase().includes(q);
      return matchesCategory && matchesSearch;
    });
  }, [activeProducts, selectedCategory, searchQuery]);

  const handleTrackSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const cleaned = trackCode.trim().toUpperCase();
    if (!cleaned) {
      setTrackError("Masukkan kode pesanan Anda (contoh: KLK-240901).");
      return;
    }
    setTrackError(null);
    router.push(`/lacak/${encodeURIComponent(cleaned)}`);
  };

  return (
    <div className="flex min-h-screen flex-col bg-[var(--bg-canvas)] text-[var(--ink-primary)]">
      <PublicNavbar />

      {/* =====================================================================
          COMPOSITION 1: SLIGHT-TWIST DYNAMIC COLOR-BLOCKED FLAVOR STAGE
          ===================================================================== */}
      <section
        id="panggung-rasa"
        style={{ backgroundColor: activeShowcase.meta.stageBg }}
        className="bg-halftone-dots relative overflow-hidden border-b-2 border-[#12100E] pt-4 pb-14 transition-colors duration-300 sm:pt-8 sm:pb-20"
      >
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          {/* Flavor Switcher Pill Rail (Slight Twist Top Selector) */}
          <div className="mb-8 flex flex-wrap items-center justify-between gap-3 rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-2.5 shadow-[5px_5px_0px_#12100E]">
            <div className="flex items-center gap-2 pl-2">
              <span className="inline-block h-3 w-3 rounded-full border border-[#12100E] bg-[#C2410C]" />
              <span className="font-mono-num text-xs font-extrabold tracking-wider text-[#12100E] uppercase">
                Pilih Sajian Dapur ({activeFlavorIndex + 1}/
                {SIGNATURE_SHOWCASE_ITEMS.length}):
              </span>
            </div>
            <div
              role="tablist"
              aria-label="Pilih sajian utama Kalkurasa"
              className="flex flex-wrap items-center gap-1.5"
            >
              {SIGNATURE_SHOWCASE_ITEMS.map((item, idx) => {
                const isSelected = idx === activeFlavorIndex;
                return (
                  <button
                    key={item.key}
                    type="button"
                    role="tab"
                    aria-selected={isSelected}
                    onClick={() => setActiveFlavorIndex(idx)}
                    style={{
                      backgroundColor: isSelected
                        ? item.meta.stickerBg
                        : "#F7EFE2",
                    }}
                    className={`rounded-full border-2 border-[#12100E] px-3.5 py-1.5 text-xs font-extrabold tracking-tight text-[#12100E] uppercase transition-all ${
                      isSelected
                        ? "-translate-y-0.5 shadow-[3px_3px_0px_#12100E]"
                        : "opacity-85 hover:opacity-100"
                    }`}
                  >
                    <span className="font-mono-num mr-1 text-[10px] opacity-70">
                      0{idx + 1}
                    </span>
                    {item.name.split(" ").slice(0, 2).join(" ")}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Main Asymmetric Split Stage */}
          <div className="grid grid-cols-1 items-center gap-10 lg:grid-cols-12 lg:gap-8">
            {/* Left Column: Oversized Expressive Editorial Typography */}
            <div className="lg:col-span-7">
              <div className="mb-4 flex flex-wrap items-center gap-2.5">
                <span
                  style={{ backgroundColor: activeShowcase.meta.stickerBg }}
                  className="twist-sticker -rotate-2 rounded-full px-3.5 py-1 text-xs font-extrabold tracking-wider text-[#12100E] uppercase"
                >
                  {activeShowcase.meta.flavorNumber} ·{" "}
                  {activeShowcase.categoryLabel}
                </span>
                <span className="rounded-full border-2 border-[#12100E] bg-[#12100E] px-3.5 py-1 text-xs font-extrabold tracking-wider text-[#D9F99D] uppercase">
                  {activeShowcase.meta.badgeText}
                </span>
              </div>

              <h1 className="font-display text-[clamp(2.5rem,5.6vw,4.9rem)] leading-[0.94] font-extrabold tracking-tight text-[#12100E] uppercase">
                Masakan Jujur,{" "}
                <span
                  style={{ backgroundColor: activeShowcase.meta.stickerBg }}
                  className="font-twist inline-block -rotate-1 rounded-full border-2 border-[#12100E] px-4 py-0.5 font-bold text-[#12100E] normal-case shadow-[4px_4px_0px_#12100E]"
                >
                  {activeShowcase.meta.twistWord}
                </span>{" "}
                Terjadwal Presisi.
              </h1>

              <p className="mt-5 max-w-2xl text-base leading-relaxed font-semibold text-[#12100E]/85 sm:text-lg">
                {activeShowcase.meta.story} Setiap pesanan dievaluasi otomatis
                oleh <strong>Mesin 4-Gerbang</strong> agar kualitas masakan
                tetap prima tanpa membuat oven dan tim dapur kewalahan.
              </p>

              {/* Recipe & Kitchen Telemetry Chips */}
              <div className="mt-6 grid grid-cols-1 gap-2.5 sm:grid-cols-3">
                {activeShowcase.meta.recipeHighlights.map((highlight, i) => (
                  <div
                    key={highlight}
                    className="flex items-start gap-2 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] p-3 shadow-[3px_3px_0px_#12100E]"
                  >
                    <span
                      style={{ backgroundColor: activeShowcase.meta.stickerBg }}
                      className="font-mono-num flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-[#12100E] text-xs font-extrabold text-[#12100E]"
                    >
                      0{i + 1}
                    </span>
                    <span className="text-xs leading-snug font-bold text-[#12100E]">
                      {highlight}
                    </span>
                  </div>
                ))}
              </div>

              {/* Primary CTA Row */}
              <div className="mt-7 flex flex-col items-stretch gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-3.5">
                <Link
                  href={`/pesan?product=${encodeURIComponent(
                    activeProducts.find(
                      (p) =>
                        getProductVisualMeta(p).flavorNumber ===
                        activeShowcase.meta.flavorNumber
                    )?.id ?? activeShowcase.key
                  )}`}
                  className="twist-btn inline-flex min-h-13 w-full items-center justify-center gap-2.5 rounded-full bg-[#C2410C] px-5 py-3.5 text-center text-sm font-extrabold tracking-wide text-white uppercase sm:w-auto sm:px-7"
                >
                  <ShoppingBag className="h-4 w-4 shrink-0" />
                  <span>Pesan {activeShowcase.name}</span>
                  <ArrowUpRight className="h-4 w-4 shrink-0" />
                </Link>
                <a
                  href="#katalog"
                  className="twist-btn inline-flex min-h-13 w-full items-center justify-center gap-2 rounded-full bg-[#FFFDF9] px-6 py-3.5 text-center text-sm font-extrabold tracking-wide text-[#12100E] uppercase sm:w-auto"
                >
                  <Utensils className="h-4 w-4 shrink-0 text-[#C2410C]" />
                  <span>Lihat Semua Foto Menu</span>
                </a>
              </div>

              {/* Embedded Tactile Order Ticket Tracker */}
              <form
                onSubmit={handleTrackSubmit}
                aria-label="Lacak kode tiket pesanan"
                className="mt-8 rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[5px_5px_0px_#12100E]"
              >
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <div className="min-w-0 flex-1">
                    <label
                      htmlFor="hero-track-code"
                      className="block text-xs font-extrabold tracking-wider text-[#12100E] uppercase"
                    >
                      Sudah Punya Tiket Pre-Order? Lacak Status Dapur:
                    </label>
                    <p className="text-xs font-medium text-[#5C5046]">
                      Pantau evaluasi 4-Gerbang, jadwal panggang, dan verifikasi
                      DP secara langsung.
                    </p>
                  </div>
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <Input
                      id="hero-track-code"
                      type="text"
                      value={trackCode}
                      onChange={(e) => {
                        setTrackCode(e.target.value);
                        if (trackError) setTrackError(null);
                      }}
                      placeholder="Contoh: KLK-240901"
                      className="font-mono-num h-11 w-full rounded-full border-2 border-[#12100E] bg-[#FFF9F0] px-4 text-sm font-bold text-[#12100E] uppercase sm:w-48"
                    />
                    <button
                      type="submit"
                      className="twist-btn inline-flex h-11 shrink-0 items-center justify-center gap-1.5 rounded-full bg-[#D9F99D] px-5 text-xs font-extrabold tracking-wider text-[#12100E] uppercase"
                    >
                      <span>Cek Tiket</span>
                      <ArrowRight className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>
                {trackError && (
                  <p
                    role="alert"
                    className="mt-2 text-xs font-extrabold text-[#BE123C]"
                  >
                    {trackError}
                  </p>
                )}
              </form>
            </div>

            {/* Right Column: Arched Studio Food Photography Showcase Card */}
            <div className="mt-4 sm:mt-0 lg:col-span-5">
              <div className="relative mx-auto max-w-lg">
                {/* Top-Left Floating Price Sticker */}
                <div
                  style={{ backgroundColor: activeShowcase.meta.stickerBg }}
                  className="twist-sticker absolute -top-5 left-2 z-20 -rotate-3 rounded-2xl px-3 py-1.5 text-[#12100E] sm:-top-4 sm:-left-4 sm:px-4 sm:py-2"
                >
                  <p className="text-[9px] font-extrabold tracking-widest uppercase sm:text-[10px]">
                    Harga Pre-Order
                  </p>
                  <p className="font-mono-num text-base font-extrabold sm:text-xl">
                    {formatRupiah(activeShowcase.price)}
                    <span className="text-[11px] font-bold sm:text-xs">
                      /{activeShowcase.unit}
                    </span>
                  </p>
                </div>

                {/* Top-Right Kitchen Capacity Pill */}
                <div className="twist-sticker absolute -top-3 right-2 z-20 rotate-2 rounded-full bg-[#12100E] px-2.5 py-1 text-[#D9F99D] sm:-right-3 sm:px-3.5 sm:py-1.5">
                  <p className="font-mono-num text-[11px] font-extrabold sm:text-xs">
                    🔥 {activeShowcase.meta.ovenHoursPerUnit}j{" "}
                    {activeShowcase.meta.heatStationLabel} · ⏱️{" "}
                    {activeShowcase.meta.laborMinsPerUnit}m Tim
                  </p>
                </div>

                {/* Main Photo Frame */}
                <div className="overflow-hidden rounded-[2.25rem] border-3 border-[#12100E] bg-[#FFFDF9] p-3 shadow-[10px_10px_0px_#12100E]">
                  <div className="photo-frame relative aspect-[4/3] w-full overflow-hidden rounded-[1.65rem] border-2 border-[#12100E] bg-[#12100E]">
                    <img
                      src={activeShowcase.meta.illustration}
                      alt={activeShowcase.name}
                      width={900}
                      height={675}
                      fetchPriority="high"
                      className="h-full w-full object-cover"
                    />
                    <div className="absolute right-3 bottom-3 left-3 flex items-center justify-between gap-2 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9]/95 px-3.5 py-2 backdrop-blur-xs">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-extrabold text-[#12100E]">
                          {activeShowcase.name}
                        </p>
                        <p className="truncate text-[11px] font-semibold text-[#5C5046]">
                          {activeShowcase.meta.pairingNote}
                        </p>
                      </div>
                      <span className="font-mono-num shrink-0 rounded-full border border-[#12100E] bg-[#D9F99D] px-2.5 py-1 text-[11px] font-extrabold text-[#12100E]">
                        Min. {activeShowcase.minOrder} {activeShowcase.unit}
                      </span>
                    </div>
                  </div>

                  {/* Bottom Telemetry Strip Inside Stage Card */}
                  <div className="mt-3 grid grid-cols-3 gap-2 rounded-2xl border-2 border-[#12100E] bg-[#12100E] p-3 text-[#FAF5EE]">
                    <div>
                      <p className="text-[10px] font-bold tracking-wider text-[#FAF5EE]/70 uppercase">
                        G1 Kapasitas
                      </p>
                      <p className="font-mono-num text-sm font-extrabold text-[#D9F99D]">
                        ≤ 90% Aman
                      </p>
                    </div>
                    <div className="border-x border-[#FAF5EE]/20 px-2">
                      <p className="text-[10px] font-bold tracking-wider text-[#FAF5EE]/70 uppercase">
                        G2 Margin
                      </p>
                      <p className="font-mono-num text-sm font-extrabold text-[#FDE047]">
                        ≥ 20% Sehat
                      </p>
                    </div>
                    <div>
                      <p className="text-[10px] font-bold tracking-wider text-[#FAF5EE]/70 uppercase">
                        G4 Lead Time
                      </p>
                      <p className="font-mono-num text-sm font-extrabold text-[#FAF5EE]">
                        H-2 Standar
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* =====================================================================
          HIGH-CONTRAST CRAFT TICKER STRIP (SLIGHT TWIST DIVIDER)
          ===================================================================== */}
      <div className="border-b-2 border-[#12100E] bg-[#12100E] py-3.5 text-[#FAF5EE]">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-around gap-4 px-4 text-xs font-extrabold tracking-widest uppercase">
          <span className="inline-flex items-center gap-2 text-[#D9F99D]">
            <span className="h-2.5 w-2.5 rounded-full bg-[#D9F99D]" />
            Nasi Box Daun Pisang Segar
          </span>
          <span className="inline-flex items-center gap-2 text-[#FDE047]">
            <span className="h-2.5 w-2.5 rounded-full bg-[#FDE047]" />
            Snack Box Risoles Ragout & Pastel
          </span>
          <span className="inline-flex items-center gap-2 text-[#FECDD3]">
            <span className="h-2.5 w-2.5 rounded-full bg-[#FECDD3]" />
            Celebration Tart Dark Ganache 70%
          </span>
          <span className="inline-flex items-center gap-2 text-[#FED7AA]">
            <span className="h-2.5 w-2.5 rounded-full bg-[#FED7AA]" />
            Nastar Wisjman & Brownies Shiny Crust
          </span>
        </div>
      </div>

      {/* =====================================================================
          COMPOSITION 2: COLOR-BLOCKED MENU CATALOG WITH REAL FOOD PHOTOS
          ===================================================================== */}
      <section
        id="katalog"
        className="border-b-2 border-[var(--ink-border)] bg-[var(--bg-canvas)] py-16 sm:py-24"
      >
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          {/* Section Header */}
          <div className="flex flex-col justify-between gap-6 lg:flex-row lg:items-end">
            <div>
              <div className="mb-3 inline-flex items-center gap-2 rounded-full border-2 border-[#12100E] bg-[#FDE047] px-4 py-1 text-xs font-extrabold tracking-wider text-[#12100E] uppercase shadow-[3px_3px_0px_#12100E]">
                <Utensils className="h-3.5 w-3.5" />
                <span>Menu Pre-Order · Foto Asli Dapur Kalkurasa</span>
              </div>
              <h2 className="font-display text-3xl font-extrabold tracking-tight text-[var(--ink-primary)] uppercase sm:text-5xl">
                Pilih Sajian,{" "}
                <span className="font-twist inline-block -rotate-1 rounded-2xl border-2 border-[#12100E] bg-[#D9F99D] px-3 py-0.5 text-[#12100E] normal-case shadow-[3px_3px_0px_#12100E]">
                  Kunci Kuota Oven
                </span>{" "}
                Anda.
              </h2>
            </div>

            {/* Search Input */}
            <div className="w-full max-w-md">
              <label
                htmlFor="catalog-search"
                className="mb-1.5 block text-xs font-extrabold tracking-wider text-[var(--ink-primary)] uppercase"
              >
                Cari Sajian atau Bahan Resep
              </label>
              <div className="relative">
                <PackageSearch className="pointer-events-none absolute top-1/2 left-4 h-4 w-4 -translate-y-1/2 text-[#12100E]" />
                <Input
                  id="catalog-search"
                  type="search"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Ketik nasi box, risoles, tart, nastar, brownies..."
                  className="h-12 rounded-full border-2 border-[#12100E] bg-[#FFFDF9] pl-11 text-sm font-bold text-[#12100E] shadow-[4px_4px_0px_#12100E]"
                />
              </div>
            </div>
          </div>

          {/* Category Filter Capsules */}
          {!isLoading && !isError && activeProducts.length > 0 && (
            <div
              role="tablist"
              aria-label="Filter kategori produk"
              className="mt-8 flex flex-wrap items-center gap-2.5"
            >
              <button
                type="button"
                role="tab"
                aria-selected={selectedCategory === "ALL"}
                onClick={() => setSelectedCategory("ALL")}
                className={`twist-btn min-h-11 rounded-full px-5 py-2 text-xs font-extrabold tracking-wider uppercase ${
                  selectedCategory === "ALL"
                    ? "bg-[#12100E] text-[#D9F99D]"
                    : "bg-[#FFFDF9] text-[#12100E] hover:bg-[#FFEDD5]"
                }`}
              >
                Semua Menu ({activeProducts.length})
              </button>
              {categories.map((cat) => {
                const count = activeProducts.filter(
                  (p) => normalizeProductCategoryValue(p.category) === cat
                ).length;
                const isSelected = selectedCategory === cat;
                return (
                  <button
                    key={cat}
                    type="button"
                    role="tab"
                    aria-selected={isSelected}
                    onClick={() => setSelectedCategory(cat)}
                    className={`twist-btn min-h-11 rounded-full px-5 py-2 text-xs font-extrabold tracking-wider uppercase ${
                      isSelected
                        ? "bg-[#C2410C] text-white"
                        : "bg-[#FFFDF9] text-[#12100E] hover:bg-[#D9F99D]"
                    }`}
                  >
                    {formatCategoryName(cat)} ({count})
                  </button>
                );
              })}
            </div>
          )}

          {/* Product Grid */}
          <div className="mt-10">
            {isLoading ? (
              <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
                {Array.from({ length: 6 }).map((_, idx) => (
                  <div
                    key={idx}
                    className="rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-5 shadow-[6px_6px_0px_#12100E]"
                  >
                    <Skeleton className="mb-4 h-56 w-full rounded-2xl" />
                    <Skeleton className="h-6 w-3/4" />
                    <Skeleton className="mt-2 h-4 w-1/2" />
                    <Skeleton className="mt-6 h-12 w-full rounded-full" />
                  </div>
                ))}
              </div>
            ) : isError ? (
              <ErrorState
                title="Gagal Memuat Katalog Dapur"
                message={getErrorMessage(error)}
                onRetry={() => refetch()}
              />
            ) : filteredProducts.length === 0 ? (
              <EmptyState
                title="Menu Tidak Ditemukan"
                description={
                  searchQuery || selectedCategory !== "ALL"
                    ? "Tidak ada menu yang cocok dengan filter pencarian Anda."
                    : "Belum ada produk aktif yang tersedia saat ini."
                }
                action={
                  (searchQuery || selectedCategory !== "ALL") && (
                    <button
                      type="button"
                      onClick={() => {
                        setSelectedCategory("ALL");
                        setSearchQuery("");
                      }}
                      className="twist-btn rounded-full bg-[#D9F99D] px-5 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
                    >
                      Reset Filter Pencarian
                    </button>
                  )
                }
              />
            ) : (
              <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-3">
                {filteredProducts.map((product, index) => {
                  const visual = getProductVisualMeta(product);
                  const isExpanded = expandedProductId === product.id;

                  return (
                    <article
                      key={product.id}
                      style={{ backgroundColor: visual.stageBg }}
                      className="twist-card flex flex-col justify-between overflow-hidden rounded-[2rem] text-[#12100E]"
                    >
                      <div>
                        {/* Card Top Color-Blocked Header + Real Studio Food Photo */}
                        <div className="relative p-3.5 pb-0">
                          <div className="mb-2.5 flex items-center justify-between gap-2">
                            <span
                              style={{ backgroundColor: visual.stickerBg }}
                              className="font-mono-num rounded-full border-2 border-[#12100E] px-3 py-0.5 text-[11px] font-extrabold text-[#12100E] uppercase shadow-[2px_2px_0px_#12100E]"
                            >
                              0{index + 1} ·{" "}
                              {formatCategoryName(product.category)}
                            </span>
                            <span className="font-mono-num rounded-full border-2 border-[#12100E] bg-[#12100E] px-3 py-0.5 text-[11px] font-extrabold text-[#D9F99D]">
                              {visual.flavorNumber}
                            </span>
                          </div>

                          {/* Real Food Photograph Frame */}
                          <div className="photo-frame relative aspect-[4/3] w-full overflow-hidden rounded-2xl border-2 border-[#12100E] bg-[#12100E] shadow-[4px_4px_0px_#12100E]">
                            <img
                              src={visual.illustration}
                              alt={`Foto sajian ${product.name}`}
                              width={640}
                              height={480}
                              loading="lazy"
                              className="h-full w-full object-cover"
                            />
                            <div
                              style={{ backgroundColor: visual.stickerBg }}
                              className="twist-sticker absolute right-2.5 bottom-2.5 -rotate-2 rounded-full px-3 py-1 text-[11px] font-extrabold text-[#12100E]"
                            >
                              {visual.badgeText}
                            </div>
                          </div>
                        </div>

                        {/* Card Body */}
                        <div className="p-5">
                          <h3 className="font-display text-2xl leading-tight font-extrabold text-[#12100E]">
                            {product.name}
                          </h3>
                          <p className="mt-2 text-sm leading-relaxed font-semibold text-[#12100E]/80">
                            {visual.story}
                          </p>

                          {/* Kitchen Load Telemetry Pills */}
                          <div className="mt-4 flex flex-wrap items-center gap-2">
                            <span className="font-mono-num rounded-full border border-[#12100E] bg-[#FFFDF9] px-3 py-1 text-xs font-extrabold text-[#12100E]">
                              🔥 Jam {visual.heatStationLabel}{" "}
                              {visual.ovenHoursPerUnit} jam/unit
                            </span>
                            <span className="font-mono-num rounded-full border border-[#12100E] bg-[#FFFDF9] px-3 py-1 text-xs font-extrabold text-[#12100E]">
                              ⏱️ Tim {visual.laborMinsPerUnit} mnt/unit
                            </span>
                          </div>

                          {/* Expandable Recipe Story Drawer */}
                          <div className="mt-4">
                            <button
                              type="button"
                              onClick={() =>
                                setExpandedProductId(
                                  isExpanded ? null : product.id
                                )
                              }
                              aria-expanded={isExpanded}
                              className="inline-flex items-center gap-1.5 rounded-full border border-[#12100E] bg-[#FFFDF9] px-3.5 py-1.5 text-xs font-extrabold text-[#12100E] transition-colors hover:bg-[#FDE047]"
                            >
                              <span>
                                {isExpanded
                                  ? "Tutup Detail Komposisi"
                                  : "Intip Komposisi & Cerita Resep"}
                              </span>
                              <ArrowRight
                                className={`h-3.5 w-3.5 transition-transform duration-200 ${
                                  isExpanded ? "rotate-90" : ""
                                }`}
                              />
                            </button>

                            {isExpanded && (
                              <div className="mt-3 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] p-3.5 text-xs shadow-[3px_3px_0px_#12100E]">
                                <p className="font-extrabold tracking-wider text-[#C2410C] uppercase">
                                  Komposisi Utama Dapur:
                                </p>
                                <ul className="mt-2 space-y-1.5 font-semibold text-[#12100E]">
                                  {visual.recipeHighlights.map((item) => (
                                    <li
                                      key={item}
                                      className="flex items-start gap-2"
                                    >
                                      <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[#15803D]" />
                                      <span>{item}</span>
                                    </li>
                                  ))}
                                </ul>
                                <p className="mt-2.5 border-t border-dashed border-[#12100E]/30 pt-2 text-[11px] font-bold text-[#5C5046]">
                                  {visual.pairingNote}
                                </p>
                              </div>
                            )}
                          </div>
                        </div>
                      </div>

                      {/* Card Footer: Price & Order Button */}
                      <div className="mt-2 flex items-center justify-between gap-3 border-t-2 border-[#12100E] bg-[#FFFDF9] px-5 py-4">
                        <div>
                          <p className="text-[10px] font-extrabold tracking-widest text-[#5C5046] uppercase">
                            Harga Satuan
                          </p>
                          <p className="font-mono-num text-xl font-extrabold text-[#12100E]">
                            {formatRupiah(product.price)}
                            <span className="text-xs font-bold text-[#5C5046]">
                              /unit
                            </span>
                          </p>
                        </div>

                        <Link
                          href={`/pesan?product=${encodeURIComponent(product.id)}`}
                          className="twist-btn inline-flex min-h-11 items-center gap-1.5 rounded-full bg-[#C2410C] px-5 py-2.5 text-xs font-extrabold tracking-wider text-white uppercase"
                        >
                          <span>Pesan</span>
                          <ArrowUpRight className="h-4 w-4" />
                        </Link>
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </section>

      {/* =====================================================================
          COMPOSITION 3: CRAFT KITCHEN STORY & BENTO GALLERY (#dapur-kami)
          ===================================================================== */}
      <section
        id="dapur-kami"
        className="bg-halftone-dots border-b-2 border-[#12100E] bg-[#FFEDD5] py-16 text-[#12100E] sm:py-24"
      >
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="mb-12 max-w-3xl">
            <span className="twist-sticker inline-flex -rotate-2 items-center gap-2 rounded-full bg-[#D9F99D] px-4 py-1 text-xs font-extrabold tracking-wider text-[#12100E] uppercase">
              <Flame className="h-3.5 w-3.5 text-[#C2410C]" />
              <span>Di Balik Dapur Kalkurasa</span>
            </span>
            <h2 className="font-display mt-3 text-3xl font-extrabold tracking-tight uppercase sm:text-5xl">
              Kenapa Pre-Order Kami{" "}
              <span className="font-twist inline-block rotate-1 rounded-2xl border-2 border-[#12100E] bg-[#FFFDF9] px-3 py-0.5 normal-case shadow-[3px_3px_0px_#12100E]">
                Tidak Pernah
              </span>{" "}
              Telat atau Asal Jadi?
            </h2>
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
            {/* Photo Bento 1: Real Nasi Box & Snack Box Showcase */}
            <div className="overflow-hidden rounded-[2rem] border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[6px_6px_0px_#12100E] lg:col-span-5">
              <div className="photo-frame aspect-[4/3] overflow-hidden rounded-2xl border-2 border-[#12100E]">
                <img
                  src="/assets/foods/nasi-box-photo.jpg"
                  alt="Nasi Box Ayam Bakar Rempah di atas daun pisang segar"
                  width={800}
                  height={600}
                  loading="lazy"
                  className="h-full w-full object-cover"
                />
              </div>
              <div className="mt-4">
                <span className="font-mono-num rounded-full border border-[#12100E] bg-[#FDE047] px-3 py-0.5 text-xs font-extrabold">
                  PRINSIP 01 · FRESH BATCH SUBUH
                </span>
                <h3 className="font-display mt-2 text-2xl font-extrabold">
                  Dimasak Pagi Hari, Bukan Stok Beku Kemarin.
                </h3>
                <p className="mt-1.5 text-sm font-semibold text-[#3D352E]">
                  Setiap porsi nasi pandan, ayam bakar bumbu rempah, dan risoles
                  ragout disiapkan sesuai kuota jam masak dapur hari tersebut.
                </p>
              </div>
            </div>

            {/* Photo Bento 2: Bakery Proofing & Oven */}
            <div className=" flex flex-col justify-between gap-6 lg:col-span-7">
              <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
                <div className="overflow-hidden rounded-[2rem] border-2 border-[#12100E] bg-[#ECFCCB] p-4 shadow-[6px_6px_0px_#12100E]">
                  <div className="photo-frame aspect-[4/3] overflow-hidden rounded-2xl border-2 border-[#12100E]">
                    <img
                      src="/assets/foods/tart-ulang-tahun-photo.jpg"
                      alt="Tart Ulang Tahun Dark Chocolate Ganache"
                      width={600}
                      height={450}
                      loading="lazy"
                      className="h-full w-full object-cover"
                    />
                  </div>
                  <h3 className="font-display mt-3 text-xl font-extrabold">
                    02 · Margin & Bahan Transparan
                  </h3>
                  <p className="mt-1 text-xs font-bold text-[#3D352E]">
                    Kalkulator resep otomatis menjaga porsi cokelat couverture,
                    mentega Wisjman, dan bumbu rempah tidak pernah dikurangi.
                  </p>
                </div>

                <div className="overflow-hidden rounded-[2rem] border-2 border-[#12100E] bg-[#FFE4E6] p-4 shadow-[6px_6px_0px_#12100E]">
                  <div className="photo-frame aspect-[4/3] overflow-hidden rounded-2xl border-2 border-[#12100E]">
                    <img
                      src="/assets/foods/kue-kering-nastar-photo.jpg"
                      alt="Kue Kering Nastar Klasik"
                      width={600}
                      height={450}
                      loading="lazy"
                      className="h-full w-full object-cover"
                    />
                  </div>
                  <h3 className="font-display mt-3 text-xl font-extrabold">
                    03 · Jadwal Terlindungi 90%
                  </h3>
                  <p className="mt-1 text-xs font-bold text-[#3D352E]">
                    Sisa 10% kapasitas dapur dikunci sebagai cadangan kualitas
                    agar pesanan Anda tiba tepat waktu.
                  </p>
                </div>
              </div>

              {/* 4-Gate Architecture Interactive Board (Slight Twist Style) */}
              <div className="overflow-hidden rounded-[2rem] border-2 border-[#12100E] bg-[#FFFDF9] p-5 shadow-[6px_6px_0px_#12100E]">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b-2 border-[#12100E]/15 pb-3">
                  <div className="flex items-center gap-2">
                    <span className="inline-flex h-7 w-7 items-center justify-center rounded-full border-2 border-[#12100E] bg-[#D9F99D] text-xs font-extrabold">
                      4G
                    </span>
                    <div>
                      <h3 className="font-display text-base font-extrabold tracking-tight uppercase sm:text-lg">
                        Alur Mesin Keputusan 4-Gerbang Kalkurasa
                      </h3>
                      <p className="text-[11px] font-bold text-[#5C5046]">
                        Setiap pesanan masuk diuji otomatis melalui 4 filter sebelum dikunci ke jadwal dapur
                      </p>
                    </div>
                  </div>
                  <span className="font-mono-num rounded-full border-2 border-[#12100E] bg-[#12100E] px-3 py-1 text-[10px] font-extrabold tracking-wider text-[#D9F99D] uppercase">
                    Real-Time Engine
                  </span>
                </div>

                {/* 4-Gate Visual Pipeline Grid */}
                <div className="mt-4 grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-4">
                  {[
                    {
                      gate: "G1",
                      title: "Kapasitas Dapur",
                      rule: "Beban ≤ 90%",
                      desc: "Cek jam masak, oven, & kru harian",
                      bg: "#FFEDD5",
                      badgeBg: "#EA580C",
                      badgeText: "#FFFDF9",
                    },
                    {
                      gate: "G2",
                      title: "Margin Resep",
                      rule: "Margin ≥ 20%",
                      desc: "Cek HPP bahan, kemasan, & diskon",
                      bg: "#FEF9C3",
                      badgeBg: "#FDE047",
                      badgeText: "#12100E",
                    },
                    {
                      gate: "G3",
                      title: "Arus Kas 14H",
                      rule: "Kas ≥ Buffer",
                      desc: "Pastikan belanja bahan tidak minus",
                      bg: "#ECFCCB",
                      badgeBg: "#D9F99D",
                      badgeText: "#12100E",
                    },
                    {
                      gate: "G4",
                      title: "Stok & Supplier",
                      rule: "H-2 Tersedia",
                      desc: "Cek gudang & waktu kirim pemasok",
                      bg: "#FFE4E6",
                      badgeBg: "#FECDD3",
                      badgeText: "#12100E",
                    },
                  ].map((item, idx) => (
                    <div
                      key={item.gate}
                      style={{ backgroundColor: item.bg }}
                      className="group relative flex flex-col justify-between rounded-2xl border-2 border-[#12100E] p-3 shadow-[3px_3px_0px_#12100E] transition-transform duration-150 hover:-translate-y-0.5"
                    >
                      <div className="flex items-center justify-between gap-1.5">
                        <span
                          style={{
                            backgroundColor: item.badgeBg,
                            color: item.badgeText,
                          }}
                          className="font-mono-num rounded-lg border-2 border-[#12100E] px-2 py-0.5 text-xs font-extrabold"
                        >
                          {item.gate}
                        </span>
                        <span className="font-mono-num rounded-full border border-[#12100E]/40 bg-[#FFFDF9]/90 px-2 py-0.5 text-[10px] font-extrabold text-[#12100E]">
                          {item.rule}
                        </span>
                      </div>
                      <div className="mt-2.5">
                        <p className="font-display text-sm font-extrabold text-[#12100E]">
                          {idx + 1}. {item.title}
                        </p>
                        <p className="mt-0.5 text-[11px] leading-snug font-semibold text-[#3D352E]">
                          {item.desc}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>

                {/* Output Verdict Footer Strip */}
                <div className="mt-3.5 flex flex-wrap items-center justify-between gap-2 rounded-2xl border-2 border-[#12100E] bg-[#12100E] px-3.5 py-2.5 text-[#FFFDF9]">
                  <span className="font-mono-num text-[11px] font-extrabold tracking-wider text-[#FDE047] uppercase">
                    Hasil Rekomendasi Mesin →
                  </span>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className="inline-flex items-center gap-1 rounded-full border border-[#12100E] bg-[#D9F99D] px-2.5 py-0.5 text-[11px] font-extrabold text-[#12100E]">
                      <CheckCircle2 className="h-3 w-3 text-[#15803D]" />
                      TERIMA (4 Lolos)
                    </span>
                    <span className="inline-flex items-center gap-1 rounded-full border border-[#12100E] bg-[#FDE047] px-2.5 py-0.5 text-[11px] font-extrabold text-[#12100E]">
                      <AlertTriangle className="h-3 w-3 text-[#B45309]" />
                      NEGOSIASI (Jadwal/DP)
                    </span>
                    <span className="inline-flex items-center gap-1 rounded-full border border-[#12100E] bg-[#FECDD3] px-2.5 py-0.5 text-[11px] font-extrabold text-[#12100E]">
                      <XCircle className="h-3 w-3 text-[#BE123C]" />
                      TOLAK (Overload)
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* =====================================================================
          FOOTER (SLIGHT TWIST BOLD FOOTER)
          ===================================================================== */}
      <footer className="bg-[#12100E] py-14 text-[#FAF5EE]">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex flex-col justify-between gap-8 border-b-2 border-[#FAF5EE]/15 pb-10 lg:flex-row lg:items-center">
            <div className="flex items-center gap-4">
              <div className="h-14 w-14 shrink-0">
                <KalkurasaLogoIcon className="h-14 w-14" />
              </div>
              <div>
                <p className="font-display text-2xl font-extrabold tracking-tight text-[#FFFDF9] uppercase">
                  Kalku<span className="text-[#FB923C]">rasa</span> · Little Chef Kitchen
                </p>
                <p className="text-sm font-medium text-[#FAF5EE]/75">
                  Rasa Artisan dengan Kalkulasi Kapasitas Oven, Margin Resep, &
                  Arus Kas Presisi.
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-3">
              <Link
                href="/pesan"
                className="twist-btn inline-flex min-h-12 items-center gap-2 rounded-full bg-[#D9F99D] px-6 py-3 text-xs font-extrabold tracking-wider text-[#12100E] uppercase"
              >
                <ShoppingBag className="h-4 w-4" />
                <span>Buat Pesanan Pre-Order</span>
              </Link>
              <Link
                href="/lacak"
                className="twist-btn inline-flex min-h-12 items-center gap-2 rounded-full bg-[#FFFDF9] px-6 py-3 text-xs font-extrabold tracking-wider text-[#12100E] uppercase"
              >
                <span>Lacak Tiket KLK</span>
              </Link>
              <Link
                href="/login"
                className="twist-btn inline-flex min-h-12 items-center gap-2 rounded-full bg-[#C2410C] px-6 py-3 text-xs font-extrabold tracking-wider text-white uppercase"
              >
                <span>Portal Tim & Pelanggan</span>
              </Link>
            </div>
          </div>

          <div className="mt-6 flex flex-col justify-between gap-2 text-xs text-[#FAF5EE]/65 sm:flex-row">
            <p>
              © {new Date().getFullYear()} Kalkurasa. Sistem Pre-Order Kuliner
              dengan Evaluasi 4-Gerbang (`G1–G4`).
            </p>
            <p>
              Fotografi Sajian Studio Kalkurasa & Dokumentasi Oven Roti
              Wikimedia Commons (CC BY-SA).
            </p>
          </div>
        </div>
      </footer>
    </div>
  );
}
