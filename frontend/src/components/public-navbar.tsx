"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUpRight,
  ClipboardList,
  LayoutDashboard,
  LogIn,
  LogOut,
  Menu,
  Search,
  ShoppingBag,
  UserPlus,
  X,
} from "lucide-react";
import { getMe, logout } from "@/lib/api";
import { ROLE_LABELS } from "@/lib/format";
import { useToast } from "@/components/providers";
import { KalkurasaLogoIcon } from "@/components/kalkurasa-logo";

export function PublicNavbar() {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const { data: user, isLoading } = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    retry: false,
    staleTime: 60 * 1000,
  });

  const logoutMutation = useMutation({
    mutationFn: () => logout(),
    onSuccess: () => {
      queryClient.setQueryData(["auth-me"], null);
      void queryClient.invalidateQueries({ queryKey: ["auth-me"] });
      toast({
        title: "Sesi berakhir",
        description: "Anda telah keluar dari sesi.",
        variant: "info",
      });
      setMobileMenuOpen(false);
      router.push("/");
    },
    onError: () => {
      queryClient.setQueryData(["auth-me"], null);
      setMobileMenuOpen(false);
      router.push("/");
    },
  });

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && mobileMenuOpen) {
        setMobileMenuOpen(false);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [mobileMenuOpen]);

  const isInternalUser = Boolean(user && user.role !== "customer");

  return (
    <header className="sticky top-0 z-40 w-full">
      {/* Floating Capsule Navbar (Slight Twist Aesthetic, Overflow-Safe) */}
      <div className="mx-auto max-w-7xl px-3 pt-3 pb-2 sm:px-6 lg:px-8">
        <div className="flex min-h-14 w-full items-center justify-between gap-2 rounded-full border-2 border-[#12100E] bg-[#FFFDF9] px-3 py-1.5 shadow-[4px_4px_0px_#12100E] sm:px-4">
          {/* Brand Identity */}
          <Link
            href="/"
            className="group flex shrink-0 items-center gap-2.5 rounded-full pr-1 focus-visible:outline-2 focus-visible:outline-[#C2410C]"
          >
            <div className="relative flex h-10 w-10 shrink-0 items-center justify-center transition-transform duration-200 group-hover:-rotate-6">
              <KalkurasaLogoIcon className="h-10 w-10" />
            </div>
            <div className="flex flex-col leading-none">
              <span className="font-display text-base font-extrabold tracking-tight text-[#12100E] uppercase sm:text-lg">
                Kalku<span className="text-[#EA580C]">rasa</span>
              </span>
              <span className="mt-0.5 hidden text-[10px] font-bold text-[#5C5046] xl:block">
                Little Chef · Kitchen Engine
              </span>
            </div>
          </Link>

          {/* Center Capsule Links (Desktop lg+) */}
          <nav
            aria-label="Navigasi Utama"
            className="hidden min-w-0 items-center gap-1 rounded-full border-2 border-[#12100E] bg-[#F7EFE2] p-1 lg:flex"
          >
            <Link
              href="/#panggung-rasa"
              className={`rounded-full px-3 py-1.5 text-xs font-extrabold whitespace-nowrap uppercase transition-all ${
                pathname === "/"
                  ? "bg-[#12100E] text-[#D9F99D]"
                  : "text-[#12100E] hover:bg-[#FFEDD5]"
              }`}
            >
              Panggung Rasa
            </Link>
            <Link
              href="/#katalog"
              className="rounded-full px-3 py-1.5 text-xs font-extrabold whitespace-nowrap text-[#12100E] uppercase transition-all hover:bg-[#D9F99D]"
            >
              Menu
            </Link>
            <Link
              href="/#dapur-kami"
              className="inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-xs font-extrabold whitespace-nowrap text-[#12100E] uppercase transition-all hover:bg-[#FDE047]"
            >
              <span>Cerita Dapur</span>
            </Link>
            <Link
              href="/lacak"
              className={`inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-xs font-extrabold whitespace-nowrap uppercase transition-all ${
                pathname?.startsWith("/lacak")
                  ? "bg-[#12100E] text-[#D9F99D]"
                  : "text-[#12100E] hover:bg-[#FFEDD5]"
              }`}
            >
              <Search className="h-3.5 w-3.5 shrink-0" />
              <span>Lacak Tiket</span>
            </Link>
            {user && user.role === "customer" ? (
              <Link
                href="/pesanan-saya"
                className={`inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-xs font-extrabold whitespace-nowrap uppercase transition-all ${
                  pathname?.startsWith("/pesanan-saya")
                    ? "bg-[#12100E] text-[#D9F99D]"
                    : "text-[#12100E] hover:bg-[#FFEDD5]"
                }`}
              >
                <ClipboardList className="h-3.5 w-3.5 shrink-0" />
                <span>Pesanan Saya</span>
              </Link>
            ) : null}
            {isInternalUser ? (
              <Link
                href="/app"
                className="inline-flex items-center gap-1 rounded-full border border-[#12100E] bg-[#D9F99D] px-3 py-1 text-xs font-extrabold whitespace-nowrap text-[#12100E] uppercase transition-all hover:bg-[#FDE047]"
              >
                <LayoutDashboard className="h-3.5 w-3.5 shrink-0" />
                <span>Portal Dapur</span>
              </Link>
            ) : null}
          </nav>

          {/* Right Actions (Shrink-0, Compact) */}
          <div className="flex shrink-0 items-center gap-2">
            {isLoading ? (
              <div className="hidden h-9 w-20 animate-pulse rounded-full bg-[#F7EFE2] sm:block" />
            ) : user ? (
              <div className="hidden items-center gap-1.5 md:flex">
                <div className="max-w-[140px] rounded-full border-2 border-[#12100E] bg-[#F7EFE2] px-3 py-1 text-right">
                  <p className="truncate text-xs font-extrabold text-[#12100E]">
                    {user.name}
                  </p>
                  <p className="truncate text-[10px] font-bold text-[#C2410C] uppercase">
                    {ROLE_LABELS[user.role]}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => logoutMutation.mutate()}
                  disabled={logoutMutation.isPending}
                  title="Keluar Sesi"
                  aria-label="Keluar Sesi"
                  className="twist-btn inline-flex h-10 items-center gap-1 rounded-full bg-[#FFE4E6] px-3 text-xs font-extrabold text-[#9F1239] uppercase"
                >
                  <LogOut className="h-3.5 w-3.5 shrink-0" />
                  <span className="hidden xl:inline">Keluar</span>
                </button>
              </div>
            ) : (
              <div className="hidden items-center gap-1.5 sm:flex">
                <Link
                  href="/login"
                  className="twist-btn inline-flex h-10 items-center gap-1.5 rounded-full bg-[#F7EFE2] px-3.5 text-xs font-extrabold text-[#12100E] uppercase"
                >
                  <LogIn className="h-3.5 w-3.5 shrink-0" />
                  <span>Masuk</span>
                </Link>
              </div>
            )}

            <Link
              href="/pesan"
              className="twist-btn inline-flex h-10 shrink-0 items-center gap-1.5 rounded-full bg-[#C2410C] px-3.5 text-xs font-extrabold tracking-wide text-white uppercase sm:px-4"
            >
              <ShoppingBag className="h-3.5 w-3.5 shrink-0" />
              <span>Pesan</span>
              <ArrowUpRight className="hidden h-3.5 w-3.5 shrink-0 sm:inline" />
            </Link>

            {/* Mobile Hamburger Button */}
            <button
              type="button"
              onClick={() => setMobileMenuOpen((prev) => !prev)}
              aria-expanded={mobileMenuOpen}
              aria-controls="public-mobile-drawer"
              aria-label={
                mobileMenuOpen ? "Tutup menu navigasi" : "Buka menu navigasi"
              }
              className="twist-btn inline-flex h-10 shrink-0 items-center justify-center gap-1 rounded-full bg-[#D9F99D] px-3 text-xs font-extrabold text-[#12100E] uppercase lg:hidden"
            >
              {mobileMenuOpen ? (
                <X className="h-4 w-4" />
              ) : (
                <Menu className="h-4 w-4" />
              )}
              <span className="sr-only sm:not-sr-only">Menu</span>
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Navigation Drawer */}
      {mobileMenuOpen ? (
        <div
          id="public-mobile-drawer"
          className="mx-3 mt-1 rounded-3xl border-2 border-[#12100E] bg-[#FFFDF9] p-4 shadow-[6px_6px_0px_#12100E] lg:hidden"
        >
          <nav className="flex flex-col gap-2" aria-label="Navigasi Mobile">
            <Link
              href="/#panggung-rasa"
              className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#FFEDD5] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
            >
              <span>01 · Panggung Rasa Interaktif</span>
              <ArrowUpRight className="h-4 w-4 shrink-0" />
            </Link>
            <Link
              href="/#katalog"
              className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#ECFCCB] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
            >
              <span>02 · Katalog Menu &amp; Foto Asli</span>
              <ArrowUpRight className="h-4 w-4 shrink-0" />
            </Link>
            <Link
              href="/#dapur-kami"
              className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#FEF9C3] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
            >
              <span>03 · Cerita Dapur Kalkurasa</span>
              <ArrowUpRight className="h-4 w-4 shrink-0" />
            </Link>
            <Link
              href="/lacak"
              className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#FFE4E6] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
            >
              <span>04 · Lacak Tiket Pesanan</span>
              <Search className="h-4 w-4 shrink-0" />
            </Link>

            {user && user.role === "customer" ? (
              <Link
                href="/pesanan-saya"
                className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#F7EFE2] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
              >
                <span>Pesanan Saya</span>
                <ClipboardList className="h-4 w-4 shrink-0" />
              </Link>
            ) : null}

            {isInternalUser && user ? (
              <Link
                href="/app"
                className="flex min-h-11 items-center justify-between rounded-2xl border-2 border-[#12100E] bg-[#D9F99D] px-4 py-2.5 text-xs font-extrabold text-[#12100E] uppercase"
              >
                <span>Portal Dapur ({ROLE_LABELS[user.role]})</span>
                <LayoutDashboard className="h-4 w-4 shrink-0" />
              </Link>
            ) : null}

            <div className="mt-2 flex flex-wrap items-center gap-2 border-t-2 border-dashed border-[#12100E] pt-3">
              {user ? (
                <button
                  type="button"
                  onClick={() => logoutMutation.mutate()}
                  className="twist-btn flex min-h-11 w-full items-center justify-center gap-2 rounded-full bg-[#FFE4E6] px-4 text-xs font-extrabold text-[#9F1239] uppercase"
                >
                  <LogOut className="h-4 w-4" />
                  <span>Keluar ({user.name})</span>
                </button>
              ) : (
                <>
                  <Link
                    href="/login"
                    className="twist-btn flex min-h-11 flex-1 items-center justify-center gap-1.5 rounded-full bg-[#F7EFE2] px-4 text-xs font-extrabold text-[#12100E] uppercase"
                  >
                    <LogIn className="h-4 w-4" />
                    <span>Masuk</span>
                  </Link>
                  <Link
                    href="/daftar"
                    className="twist-btn flex min-h-11 flex-1 items-center justify-center gap-1.5 rounded-full bg-[#D9F99D] px-4 text-xs font-extrabold text-[#12100E] uppercase"
                  >
                    <UserPlus className="h-4 w-4" />
                    <span>Daftar</span>
                  </Link>
                </>
              )}
            </div>
          </nav>
        </div>
      ) : null}
    </header>
  );
}
