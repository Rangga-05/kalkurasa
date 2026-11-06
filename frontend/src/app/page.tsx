"use client";

import { useState, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowRight,
  CalendarCheck,
  ChefHat,
  Clock,
  PackageSearch,
  ShieldCheck,
  ShoppingBag,
  Sparkles,
} from "lucide-react";
import { getPublicProducts, getErrorMessage } from "@/lib/api";
import { formatCategoryName, formatRupiah } from "@/lib/format";
import { PublicNavbar } from "@/components/public-navbar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState } from "@/components/ui/states";

export default function LandingCatalogPage() {
  const router = useRouter();
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [trackCode, setTrackCode] = useState("");

  const productsQuery = useQuery({
    queryKey: ["public-products"],
    queryFn: ({ signal }) => getPublicProducts(signal),
  });

  const categories = useMemo(() => {
    const list = productsQuery.data ?? [];
    const set = new Set<string>();
    for (const p of list) {
      if (p.category) set.add(p.category);
    }
    return Array.from(set);
  }, [productsQuery.data]);

  const filteredProducts = useMemo(() => {
    const list = productsQuery.data ?? [];
    if (selectedCategory === "all") return list;
    return list.filter((p) => p.category === selectedCategory);
  }, [productsQuery.data, selectedCategory]);

  const handleTrackSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const cleaned = trackCode.trim().toUpperCase();
    if (!cleaned) return;
    router.push(`/lacak/${encodeURIComponent(cleaned)}`);
  };

  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />

      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-10">
        {/* Hero / Landing Singkat */}
        <section className="overflow-hidden rounded-2xl border border-amber-200/80 bg-gradient-to-br from-amber-50 via-white to-stone-100 p-6 shadow-xs sm:p-10">
          <div className="grid grid-cols-1 items-center gap-8 lg:grid-cols-12">
            <div className="space-y-4 lg:col-span-7">
              <Badge variant="amber" className="gap-1.5">
                <Sparkles className="h-3.5 w-3.5" />
                Pre-Order Terukur &amp; Transparan
              </Badge>
              <h1 className="text-2xl font-bold tracking-tight text-stone-900 sm:text-4xl">
                Pesan Sajian Kuliner Terbaik dengan Kepastian Jadwal Dapur
              </h1>
              <p className="max-w-2xl text-sm leading-relaxed text-stone-600 sm:text-base">
                <strong>Kalkurasa</strong> memadukan cita rasa autentik dengan
                perhitungan kapasitas oven dan tenaga dapur secara langsung.
                Pilih menu, cek tanggal yang tersedia, dan pantau progres
                pesanan Anda setiap saat.
              </p>

              <div className="flex flex-wrap items-center gap-3 pt-1">
                <Link href="/pesan">
                  <Button size="lg" className="gap-2">
                    <ShoppingBag className="h-4 w-4" />
                    Buat Pesanan Sekarang
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
                <a href="#katalog">
                  <Button variant="outline" size="lg">
                    Lihat Katalog Menu
                  </Button>
                </a>
              </div>

              <div className="grid grid-cols-1 gap-3 pt-3 text-xs text-stone-600 sm:grid-cols-3">
                <div className="flex items-center gap-2 rounded-lg bg-white/80 p-2.5 ring-1 ring-stone-200/80">
                  <CalendarCheck className="h-4 w-4 shrink-0 text-amber-700" />
                  <span>Ketersediaan slot dicek otomatis</span>
                </div>
                <div className="flex items-center gap-2 rounded-lg bg-white/80 p-2.5 ring-1 ring-stone-200/80">
                  <Clock className="h-4 w-4 shrink-0 text-amber-700" />
                  <span>Lead time jelas &amp; tepat waktu</span>
                </div>
                <div className="flex items-center gap-2 rounded-lg bg-white/80 p-2.5 ring-1 ring-stone-200/80">
                  <ShieldCheck className="h-4 w-4 shrink-0 text-amber-700" />
                  <span>Lacak status dengan kode unik</span>
                </div>
              </div>
            </div>

            {/* Kotak Lacak Cepat */}
            <div className="lg:col-span-5">
              <Card className="border-stone-200/90 bg-white/95 shadow-sm">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-2 text-amber-800">
                    <PackageSearch className="h-5 w-5" />
                    <CardTitle className="text-base">
                      Sudah Punya Kode Pesanan?
                    </CardTitle>
                  </div>
                  <CardDescription>
                    Masukkan 10 karakter kode pesanan Anda untuk melihat status
                    produksi, tagihan DP, atau merespons tawaran balik.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <form
                    onSubmit={handleTrackSubmit}
                    className="flex flex-col gap-2.5 sm:flex-row"
                  >
                    <Input
                      value={trackCode}
                      onChange={(e) =>
                        setTrackCode(e.target.value.toUpperCase())
                      }
                      placeholder="Contoh: K8M4P2X9TQ"
                      maxLength={12}
                      aria-label="Kode pesanan"
                      className="font-mono uppercase tracking-wider"
                    />
                    <Button
                      type="submit"
                      variant="default"
                      disabled={!trackCode.trim()}
                      className="shrink-0"
                    >
                      Lacak
                    </Button>
                  </form>
                </CardContent>
              </Card>
            </div>
          </div>
        </section>

        {/* Bagian Katalog Produk */}
        <section id="katalog" className="mt-10 space-y-5">
          <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-end">
            <div>
              <h2 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
                Katalog Produk Pre-Order
              </h2>
              <p className="text-sm text-stone-500">
                Harga transparan per unit. Pilih produk untuk langsung mengatur
                jumlah dan jadwal acara Anda.
              </p>
            </div>

            {/* Filter Kategori */}
            {categories.length > 1 ? (
              <div
                role="tablist"
                aria-label="Filter kategori produk"
                className="flex flex-wrap gap-1.5"
              >
                <button
                  type="button"
                  onClick={() => setSelectedCategory("all")}
                  className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer ${
                    selectedCategory === "all"
                      ? "bg-amber-700 text-white"
                      : "bg-stone-200/70 text-stone-700 hover:bg-stone-200"
                  }`}
                >
                  Semua Menu ({productsQuery.data?.length ?? 0})
                </button>
                {categories.map((cat) => (
                  <button
                    key={cat}
                    type="button"
                    onClick={() => setSelectedCategory(cat)}
                    className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors cursor-pointer ${
                      selectedCategory === cat
                        ? "bg-amber-700 text-white"
                        : "bg-stone-200/70 text-stone-700 hover:bg-stone-200"
                    }`}
                  >
                    {formatCategoryName(cat)}
                  </button>
                ))}
              </div>
            ) : null}
          </div>

          {/* State Loading */}
          {productsQuery.isLoading ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {Array.from({ length: 6 }).map((_, idx) => (
                <Card key={idx} className="p-5">
                  <div className="space-y-3">
                    <Skeleton className="h-5 w-24 rounded-full" />
                    <Skeleton className="h-6 w-3/4" />
                    <Skeleton className="h-7 w-1/2" />
                    <Skeleton className="mt-4 h-9 w-full" />
                  </div>
                </Card>
              ))}
            </div>
          ) : null}

          {/* State Error */}
          {productsQuery.isError ? (
            <ErrorState
              title="Gagal Memuat Katalog Produk"
              message={getErrorMessage(productsQuery.error)}
              onRetry={() => productsQuery.refetch()}
            />
          ) : null}

          {/* State Kosong */}
          {!productsQuery.isLoading &&
          !productsQuery.isError &&
          filteredProducts.length === 0 ? (
            <EmptyState
              title="Belum Ada Produk Aktif"
              description="Saat ini belum ada produk yang tersedia pada kategori ini."
            />
          ) : null}

          {/* Daftar Produk */}
          {!productsQuery.isLoading &&
          !productsQuery.isError &&
          filteredProducts.length > 0 ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {filteredProducts.map((product) => (
                <Card
                  key={product.id}
                  className="flex flex-col justify-between transition-shadow hover:shadow-md"
                >
                  <CardHeader className="pb-3">
                    <div className="flex items-center justify-between gap-2">
                      <Badge variant="slate">
                        {formatCategoryName(product.category)}
                      </Badge>
                      <ChefHat className="h-4 w-4 text-stone-400" />
                    </div>
                    <CardTitle className="pt-2 text-lg">
                      {product.name}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="pb-4">
                    <div className="rounded-lg bg-stone-50 p-3 ring-1 ring-stone-200/70">
                      <p className="text-[11px] font-medium uppercase tracking-wider text-stone-500">
                        Harga Satuan
                      </p>
                      <p className="mt-0.5 text-xl font-bold text-amber-800">
                        {formatRupiah(product.price)}
                        <span className="text-xs font-normal text-stone-500">
                          {" "}
                          / unit
                        </span>
                      </p>
                    </div>
                  </CardContent>
                  <CardFooter>
                    <Link
                      href={`/pesan?product=${encodeURIComponent(product.id)}`}
                      className="w-full"
                    >
                      <Button variant="outline" className="w-full gap-2">
                        <ShoppingBag className="h-4 w-4 text-amber-700" />
                        Pesan Produk Ini
                      </Button>
                    </Link>
                  </CardFooter>
                </Card>
              ))}
            </div>
          ) : null}
        </section>
      </main>

      <footer className="border-t border-stone-200 bg-white py-5 text-center text-xs text-stone-500">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-2 px-4 sm:flex-row sm:px-6">
          <span>
            <strong>Kalkurasa</strong> — Kalkulasi + Rasa · Sistem Pre-Order
            UMKM Kuliner
          </span>
          <span>Data simulasi — Proyek GTNIC 2026</span>
        </div>
      </footer>
    </div>
  );
}
