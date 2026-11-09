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
    label: "Pesanan",
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
    label: "Proyeksi Kas",
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

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["auth-me"] });
      toast.info("Berhasil Keluar", "Sesi Anda telah diakhiri.");
      router.replace("/login");
      router.refresh();
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
    <div className="flex flex-1 bg-stone-100/70 print:bg-white print:block">
      {/* Sidebar Desktop */}
      <aside className="sticky top-0 hidden h-screen max-h-screen w-64 shrink-0 flex-col border-r border-stone-200 bg-white lg:flex print:hidden">
        <div className="flex shrink-0 items-center justify-between border-b border-stone-200 px-5 py-4">
          <Link
            href={user ? getDashboardPathForRole(user.role) : "/app"}
            className="flex items-center gap-2.5 text-stone-900"
          >
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-amber-700 text-white">
              <ChefHat className="h-4 w-4" />
            </div>
            <div>
              <span className="block text-base font-bold tracking-tight">
                Kalkurasa
              </span>
              <span className="block text-[11px] text-stone-500">
                Portal Operasional
              </span>
            </div>
          </Link>
        </div>

        <nav
          aria-label="Menu navigasi internal"
          className="flex-1 space-y-1 overflow-y-auto px-3 py-4"
        >
          {meQuery.isLoading ? (
            <div className="space-y-2 px-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-9 w-full" />
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
                    "flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                    active
                      ? "bg-amber-50 text-amber-900 ring-1 ring-amber-200"
                      : "text-stone-600 hover:bg-stone-100 hover:text-stone-900"
                  )}
                >
                  <Icon className="h-4 w-4 shrink-0" />
                  <span className="truncate">{item.label}</span>
                </Link>
              );
            })
          )}
        </nav>

        <div className="shrink-0 border-t border-stone-200 bg-white p-4 space-y-3">
          {user ? (
            <div className="rounded-xl bg-stone-50 p-3 ring-1 ring-stone-200/80">
              <p className="truncate text-sm font-semibold text-stone-900">
                {user.name}
              </p>
              <p className="truncate text-xs text-stone-500">{user.email}</p>
              <Badge variant="amber" className="mt-1.5">
                {ROLE_LABELS[user.role]}
              </Badge>
            </div>
          ) : (
            <Skeleton className="h-16 w-full rounded-xl" />
          )}

          <div className="flex flex-col gap-1.5">
            <Link
              href="/"
              className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-stone-200 px-3 py-1.5 text-xs font-medium text-stone-600 hover:bg-stone-50 hover:text-stone-900"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              Lihat Halaman Publik
            </Link>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={logoutMutation.isPending}
              onClick={() => logoutMutation.mutate()}
              className="w-full gap-1.5 text-rose-700 hover:bg-rose-50 hover:text-rose-800"
            >
              <LogOut className="h-3.5 w-3.5" />
              {logoutMutation.isPending ? "Keluar..." : "Keluar"}
            </Button>
          </div>
        </div>
      </aside>

      {/* Konten Utama + Header Mobile */}
      <div className="flex min-w-0 flex-1 flex-col print:block print:w-full">
        <header className="sticky top-0 z-20 flex items-center justify-between gap-2 border-b border-stone-200 bg-white/95 px-4 py-3 backdrop-blur-sm lg:hidden print:hidden">
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setMobileMenuOpen((v) => !v)}
              className="rounded-lg border border-stone-200 p-2 text-stone-700 hover:bg-stone-100"
              aria-label="Buka menu navigasi"
            >
              {mobileMenuOpen ? (
                <X className="h-5 w-5" />
              ) : (
                <Menu className="h-5 w-5" />
              )}
            </button>
            <span className="text-base font-bold text-stone-900">
              Kalkurasa
            </span>
          </div>

          {user ? (
            <Badge variant="amber" className="text-[11px]">
              {ROLE_LABELS[user.role]}
            </Badge>
          ) : null}
        </header>

        {/* Drawer Navigasi Mobile */}
        {mobileMenuOpen ? (
          <div className="border-b border-stone-200 bg-white px-4 py-4 shadow-md lg:hidden print:hidden">
            <nav className="space-y-1">
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
                    className={cn(
                      "flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium",
                      active
                        ? "bg-amber-50 text-amber-900 ring-1 ring-amber-200"
                        : "text-stone-700 hover:bg-stone-100"
                    )}
                  >
                    <Icon className="h-4 w-4 shrink-0" />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
            </nav>

            <div className="mt-4 flex flex-col gap-2 border-t border-stone-200 pt-3">
              {user ? (
                <p className="text-xs text-stone-600">
                  Masuk sebagai <strong>{user.name}</strong> ({user.email})
                </p>
              ) : null}
              <div className="flex gap-2">
                <Link href="/" className="flex-1">
                  <Button variant="outline" size="sm" className="w-full gap-1">
                    <ExternalLink className="h-3.5 w-3.5" />
                    Halaman Publik
                  </Button>
                </Link>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={logoutMutation.isPending}
                  onClick={() => logoutMutation.mutate()}
                  className="flex-1 gap-1 text-rose-700"
                >
                  <LogOut className="h-3.5 w-3.5" />
                  Keluar
                </Button>
              </div>
            </div>
          </div>
        ) : null}

        <main className="mx-auto w-full max-w-6xl flex-1 p-4 sm:p-6 lg:p-8 print:p-0 print:max-w-none print:w-full">
          {meQuery.isLoading || (meQuery.isError && isAuthError) ? (
            <div className="space-y-4">
              <Skeleton className="h-8 w-64" />
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <Skeleton className="h-36 w-full rounded-xl" />
                <Skeleton className="h-36 w-full rounded-xl" />
                <Skeleton className="h-36 w-full rounded-xl" />
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

