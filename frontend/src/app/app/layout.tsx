"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Boxes,
  CalendarRange,
  CheckSquare,
  ChefHat,
  ClipboardList,
  ExternalLink,
  FileSpreadsheet,
  LayoutDashboard,
  LogOut,
  Menu,
  Scale,
  Settings,
  ShoppingCart,
  Users,
  Wallet,
  X,
} from "lucide-react";
import { ApiError, getMe, logout, getErrorMessage } from "@/lib/api";
import {
  canRoleAccessInternalPath,
  getDashboardPathForRole,
  ROLE_LABELS,
} from "@/lib/format";
import type { UserRole } from "@/lib/types";
import { useToast } from "@/components/providers";
import { KalkurasaLogoIcon } from "@/components/kalkurasa-logo";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/states";
import { cn } from "@/lib/utils";

interface NavItem {
  href: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  roles: UserRole[];
}

const INTERNAL_NAV_ITEMS: NavItem[] = [
  {
    href: "/app/owner",
    label: "Dashboard Pemilik",
    icon: LayoutDashboard,
    roles: ["owner"],
  },
  {
    href: "/app/manajer",
    label: "Dashboard Manajer",
    icon: LayoutDashboard,
    roles: ["manager"],
  },
  {
    href: "/app/karyawan",
    label: "Dashboard Karyawan",
    icon: CheckSquare,
    roles: ["staff"],
  },
  {
    href: "/app/admin",
    label: "Dashboard Admin",
    icon: LayoutDashboard,
    roles: ["admin"],
  },
  {
    href: "/app/pesanan",
    label: "Pesanan Dapur",
    icon: ClipboardList,
    roles: ["owner", "manager", "staff", "admin"],
  },
  {
    href: "/app/keputusan",
    label: "Antrian Keputusan",
    icon: Scale,
    roles: ["owner", "manager"],
  },
  {
    href: "/app/kapasitas",
    label: "Kalender Kapasitas",
    icon: CalendarRange,
    roles: ["owner", "manager"],
  },
  {
    href: "/app/belanja",
    label: "Belanja & Stok",
    icon: ShoppingCart,
    roles: ["owner", "manager", "staff"],
  },
  {
    href: "/app/kas",
    label: "Proyeksi Kas 14 Hari",
    icon: Wallet,
    roles: ["owner", "manager"],
  },
  {
    href: "/app/produk",
    label: "Master Produk & Resep",
    icon: ChefHat,
    roles: ["owner", "manager", "admin"],
  },
  {
    href: "/app/bahan",
    label: "Master Bahan Baku",
    icon: Boxes,
    roles: ["owner", "manager", "admin", "staff"],
  },
  {
    href: "/app/pengaturan",
    label: "Parameter Usaha",
    icon: Settings,
    roles: ["owner"],
  },
  {
    href: "/app/pengguna",
    label: "Pengguna & Peran",
    icon: Users,
    roles: ["admin"],
  },
  {
    href: "/app/audit",
    label: "Log Audit",
    icon: FileSpreadsheet,
    roles: ["owner", "admin"],
  },
];

export default function InternalAppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  });

  const isAuthError =
    meQuery.error instanceof ApiError &&
    (meQuery.error.isUnauthorized || meQuery.error.isForbidden);

  useEffect(() => {
    if (meQuery.isError && isAuthError) {
      void logout().catch(() => undefined);
      router.replace(`/login?next=${encodeURIComponent(pathname)}&reauth=1`);
      return;
    }
    const role = meQuery.data?.role;
    if (!role) return;
    if (role === "customer") {
      router.replace("/pesanan-saya");
      return;
    }
    if (pathname === "/app" || !canRoleAccessInternalPath(role, pathname)) {
      router.replace(getDashboardPathForRole(role));
    }
  }, [meQuery.isError, isAuthError, meQuery.data?.role, pathname, router]);

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!mobileMenuOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMobileMenuOpen(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [mobileMenuOpen]);

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["auth-me"] });
      toast.info("Berhasil Keluar", "Sesi Anda telah diakhiri.");
      window.location.assign("/login");
    },
    onError: (err) => {
      toast.error("Gagal Keluar", getErrorMessage(err));
    },
  });

  const user = meQuery.data;
  const filteredNav = user
    ? INTERNAL_NAV_ITEMS.filter((item) => item.roles.includes(user.role))
    : [];

  return (
    <div className="flex flex-1 bg-[var(--bg-canvas)] print:block print:bg-white">
      {/* Sidebar Desktop Cast-Iron Espresso / Surface */}
      <aside className="sticky top-0 hidden h-screen max-h-screen w-68 shrink-0 flex-col border-r border-[var(--border-warm)] bg-[var(--bg-surface)] lg:flex print:hidden">
        <div className="flex shrink-0 items-center justify-between border-b-2 border-[#12100E] px-5 py-4">
          <Link
            href={user ? getDashboardPathForRole(user.role) : "/app"}
            className="flex min-w-0 items-center gap-3 text-[#12100E]"
          >
            <div className="h-10 w-10 shrink-0">
              <KalkurasaLogoIcon className="h-10 w-10" />
            </div>
            <div className="min-w-0">
              <span className="font-display block truncate text-base font-extrabold tracking-tight uppercase">
                Kalku<span className="text-[#EA580C]">rasa</span>
              </span>
              <span className="block truncate text-[11px] font-bold text-[#5C5046]">
                Little Chef · Kendali Dapur
              </span>
            </div>
          </Link>
        </div>

        <nav
          aria-label="Menu navigasi internal"
          className="flex-1 space-y-1 overflow-y-auto px-3.5 py-4"
        >
          {meQuery.isLoading ? (
            <div className="space-y-2 px-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full rounded-xl" />
              ))}
            </div>
          ) : (
            filteredNav.map((item) => {
              const Icon = item.icon;
              const active =
                pathname === item.href ||
                pathname.startsWith(`${item.href}/`);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  prefetch={true}
                  className={cn(
                    "btn-tactile flex min-h-[40px] items-center gap-2.5 rounded-xl px-3.5 py-2 text-sm font-semibold transition-all",
                    active
                      ? "bg-[#C2410C] text-white shadow-xs"
                      : "text-[var(--ink-secondary)] hover:bg-[var(--bg-subtle)] hover:text-[var(--ink-primary)]"
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                  <span className="truncate">{item.label}</span>
                </Link>
              );
            })
          )}
        </nav>

        <div className="shrink-0 space-y-3 border-t border-[var(--border-warm)] bg-[var(--bg-surface)] p-4">
          {user ? (
            <div className="rounded-2xl border border-[var(--border-warm)] bg-[var(--bg-subtle)]/80 p-3.5">
              <div className="flex items-center justify-between gap-2">
                <p className="truncate text-sm font-bold text-[var(--ink-primary)]">
                  {user.name}
                </p>
                <Badge variant="default">{ROLE_LABELS[user.role]}</Badge>
              </div>
              <p className="mt-0.5 truncate font-mono text-xs text-[var(--ink-muted)]">
                {user.email}
              </p>
            </div>
          ) : (
            <Skeleton className="h-16 w-full rounded-2xl" />
          )}

          <div className="flex flex-col gap-2">
            <Link
              href="/"
              className="btn-tactile inline-flex min-h-[38px] items-center justify-center gap-1.5 rounded-xl border border-[var(--border-strong)] bg-[var(--bg-surface)] px-3 py-1.5 text-xs font-semibold text-[var(--ink-primary)] hover:border-[#C2410C]"
            >
              <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
              Lihat Katalog Publik
            </Link>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={logoutMutation.isPending}
              onClick={() => logoutMutation.mutate()}
              className="w-full gap-1.5 border-rose-300 text-rose-700 hover:bg-rose-50 dark:border-rose-800 dark:text-rose-300 dark:hover:bg-rose-950/50"
            >
              <LogOut className="h-3.5 w-3.5" aria-hidden="true" />
              {logoutMutation.isPending ? "Keluar..." : "Keluar Sesi"}
            </Button>
          </div>
        </div>
      </aside>

      {/* Konten Utama + Header Mobile */}
      <div className="flex min-w-0 flex-1 flex-col print:block print:w-full">
        <header className="sticky top-0 z-20 flex items-center justify-between gap-2 border-b border-[var(--border-warm)] bg-[var(--bg-surface)]/95 px-4 py-3 backdrop-blur-md lg:hidden print:hidden">
          <div className="flex items-center gap-2.5">
            <button
              type="button"
              onClick={() => setMobileMenuOpen((v) => !v)}
              aria-expanded={mobileMenuOpen}
              aria-controls="internal-mobile-drawer"
              aria-label={
                mobileMenuOpen
                  ? "Tutup menu navigasi internal"
                  : "Buka menu navigasi internal"
              }
              className="btn-tactile inline-flex min-h-[44px] items-center gap-1.5 rounded-xl border border-[var(--border-strong)] bg-[var(--bg-surface)] px-3 py-2 text-xs font-bold text-[var(--ink-primary)]"
            >
              {mobileMenuOpen ? (
                <X className="h-4 w-4" aria-hidden="true" />
              ) : (
                <Menu className="h-4 w-4" aria-hidden="true" />
              )}
              <span>Menu</span>
            </button>
            <span className="font-display text-base font-bold text-[var(--ink-primary)]">
              Kalkurasa
            </span>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            {user ? (
              <Badge variant="default" className="text-[11px]">
                {ROLE_LABELS[user.role]}
              </Badge>
            ) : null}
          </div>
        </header>

        {/* Overlay Slide-Over Drawer Navigasi Mobile (Mengambang di Depan Layar) */}
        {mobileMenuOpen ? (
          <div
            className="fixed inset-0 z-50 flex lg:hidden print:hidden"
            role="dialog"
            aria-modal="true"
            aria-label="Menu navigasi internal"
          >
            {/* Backdrop Gelap Blur (Klik di luar untuk menutup) */}
            <div
              className="fixed inset-0 bg-[#12100E]/55 backdrop-blur-xs transition-opacity"
              onClick={() => setMobileMenuOpen(false)}
              aria-hidden="true"
            />

            {/* Panel Drawer di Depan Sebelah Kiri */}
            <aside
              id="internal-mobile-drawer"
              className="relative z-10 flex h-full w-72 max-w-[84vw] flex-col border-r-2 border-[#12100E] bg-[var(--bg-surface)] shadow-[8px_0px_0px_#12100E]"
            >
              {/* Header Drawer */}
              <div className="flex shrink-0 items-center justify-between border-b-2 border-[#12100E] bg-[#F7EFE2] px-4 py-3.5">
                <Link
                  href={user ? getDashboardPathForRole(user.role) : "/app"}
                  onClick={() => setMobileMenuOpen(false)}
                  className="flex min-w-0 items-center gap-2.5 text-[#12100E]"
                >
                  <div className="h-9 w-9 shrink-0">
                    <KalkurasaLogoIcon className="h-9 w-9" />
                  </div>
                  <div className="min-w-0">
                    <span className="font-display block truncate text-base font-extrabold tracking-tight uppercase">
                      Kalku<span className="text-[#EA580C]">rasa</span>
                    </span>
                    <span className="block truncate text-[10px] font-bold text-[#5C5046]">
                      Little Chef · Kendali Dapur
                    </span>
                  </div>
                </Link>

                <button
                  type="button"
                  onClick={() => setMobileMenuOpen(false)}
                  aria-label="Tutup menu navigasi"
                  className="btn-tactile inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border-2 border-[#12100E] bg-[#FFFDF9] text-[#12100E]"
                >
                  <X className="h-4 w-4" aria-hidden="true" />
                </button>
              </div>

              {/* Daftar Menu Scrollable */}
              <nav
                aria-label="Menu internal mobile"
                className="flex-1 space-y-1.5 overflow-y-auto px-3.5 py-4"
              >
                {filteredNav.map((item) => {
                  const Icon = item.icon;
                  const active =
                    pathname === item.href ||
                    pathname.startsWith(`${item.href}/`);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      prefetch={true}
                      onClick={() => setMobileMenuOpen(false)}
                      className={cn(
                        "flex min-h-[44px] items-center gap-3 rounded-xl px-3.5 py-2.5 text-sm font-semibold transition-all",
                        active
                          ? "border-2 border-[#12100E] bg-[#C2410C] text-white shadow-[2px_2px_0px_#12100E]"
                          : "text-[var(--ink-primary)] hover:bg-[var(--bg-subtle)]"
                      )}
                    >
                      <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                      <span className="truncate">{item.label}</span>
                    </Link>
                  );
                })}
              </nav>

              {/* Footer Info Pengguna & Logout */}
              <div className="shrink-0 space-y-3 border-t-2 border-[#12100E] bg-[#F7EFE2]/60 p-4">
                {user ? (
                  <div className="rounded-xl border border-[#12100E] bg-[#FFFDF9] p-3">
                    <div className="flex items-center justify-between gap-2">
                      <p className="truncate text-xs font-extrabold text-[#12100E]">
                        {user.name}
                      </p>
                      <Badge variant="default" className="text-[10px]">
                        {ROLE_LABELS[user.role]}
                      </Badge>
                    </div>
                    <p className="mt-0.5 truncate font-mono text-[11px] text-[#5C5046]">
                      {user.email}
                    </p>
                  </div>
                ) : null}

                <div className="grid grid-cols-2 gap-2">
                  <Link href="/" onClick={() => setMobileMenuOpen(false)}>
                    <Button variant="outline" size="sm" className="w-full gap-1.5 text-xs">
                      <ExternalLink className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                      <span>Katalog</span>
                    </Button>
                  </Link>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={logoutMutation.isPending}
                    onClick={() => logoutMutation.mutate()}
                    className="w-full gap-1.5 border-rose-300 text-xs text-rose-700"
                  >
                    <LogOut className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                    <span>Keluar</span>
                  </Button>
                </div>
              </div>
            </aside>
          </div>
        ) : null}

        <main className="mx-auto w-full max-w-6xl flex-1 p-4 sm:p-6 lg:p-8 print:max-w-none print:w-full print:p-0">
          {meQuery.isLoading || (meQuery.isError && isAuthError) ? (
            <div className="space-y-4">
              <Skeleton className="h-8 w-64" />
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <Skeleton className="h-36 w-full rounded-2xl" />
                <Skeleton className="h-36 w-full rounded-2xl" />
                <Skeleton className="h-36 w-full rounded-2xl" />
              </div>
            </div>
          ) : meQuery.isError ? (
            <div className="space-y-4">
              <ErrorState
                title="Tidak Dapat Terhubung ke Server Backend"
                message={getErrorMessage(
                  meQuery.error,
                  "Pastikan server API Go sedang berjalan dan koneksi database tersedia."
                )}
                onRetry={() => void meQuery.refetch()}
              />
              <div className="text-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    void logout().catch(() => undefined);
                    router.replace(
                      `/login?next=${encodeURIComponent(pathname)}&reauth=1`
                    );
                  }}
                >
                  Masuk Ulang dengan Akun Lain
                </Button>
              </div>
            </div>
          ) : (
            children
          )}
        </main>
      </div>
    </div>
  );
}
