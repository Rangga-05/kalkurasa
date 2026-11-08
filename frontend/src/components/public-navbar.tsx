"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ChefHat,
  ClipboardList,
  LayoutDashboard,
  LogIn,
  LogOut,
  PackageSearch,
  ShoppingBag,
  Store,
  UserPlus,
} from "lucide-react";
import { getErrorMessage, getMe, logout } from "@/lib/api";
import { getDashboardPathForRole, ROLE_LABELS } from "@/lib/format";
import { useToast } from "@/components/providers";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  { href: "/", label: "Katalog", icon: Store },
  { href: "/pesan", label: "Pesan", icon: ShoppingBag },
  { href: "/lacak", label: "Lacak Pesanan", icon: PackageSearch },
];

export function PublicNavbar() {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const toast = useToast();

  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    retry: false,
    staleTime: 60_000,
  });

  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: ["auth-me"] });
      queryClient.removeQueries({ queryKey: ["my-orders"] });
      toast.info("Berhasil Keluar", "Sesi akun Anda telah diakhiri.");
      if (pathname === "/pesanan-saya") {
        router.replace("/login");
      } else {
        router.refresh();
      }
    },
    onError: (err) => {
      toast.error("Gagal Keluar", getErrorMessage(err));
    },
  });

  const user = meQuery.data;

  return (
    <header className="sticky top-0 z-30 border-b border-stone-200 bg-white/95 backdrop-blur-sm print:hidden">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-2 px-3 py-3 sm:px-6">
        <Link
          href="/"
          className="flex min-w-0 items-center gap-2.5 text-stone-900 hover:opacity-90"
        >
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-amber-700 text-white shadow-xs">
            <ChefHat className="h-5 w-5" />
          </div>
          <div className="min-w-0">
            <span className="block truncate text-base font-bold tracking-tight sm:text-lg">
              Kalkurasa
            </span>
            <span className="hidden text-[11px] font-medium text-stone-500 sm:block">
              Kalkulasi + Rasa · Dapur Pre-Order
            </span>
          </div>
        </Link>

        <nav
          aria-label="Navigasi publik"
          className="flex flex-wrap items-center justify-end gap-1 sm:gap-2"
        >
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const active =
              item.href === "/"
                ? pathname === "/"
                : pathname === item.href ||
                  pathname.startsWith(`${item.href}/`);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors sm:px-3 sm:text-sm",
                  active
                    ? "bg-amber-50 text-amber-900 ring-1 ring-amber-200"
                    : "text-stone-600 hover:bg-stone-100 hover:text-stone-900"
                )}
              >
                <Icon className="h-3.5 w-3.5 shrink-0 sm:h-4 sm:w-4" />
                <span>{item.label}</span>
              </Link>
            );
          })}

          {user?.role === "customer" ? (
            <>
              <div className="hidden items-center gap-2 rounded-xl border border-amber-200/80 bg-amber-50/80 px-2.5 py-1 text-xs text-stone-800 sm:flex">
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-amber-700 text-[11px] font-bold text-white shadow-2xs">
                  {user.name ? user.name.charAt(0).toUpperCase() : "P"}
                </div>
                <div className="flex flex-col text-left">
                  <span className="max-w-[130px] truncate font-semibold leading-tight text-stone-900">
                    {user.name}
                  </span>
                  <span className="text-[10px] text-amber-800 leading-none">
                    Pelanggan
                  </span>
                </div>
              </div>

              <Link
                href="/pesanan-saya"
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors sm:px-3 sm:text-sm",
                  pathname === "/pesanan-saya"
                    ? "bg-amber-50 text-amber-900 ring-1 ring-amber-200"
                    : "text-stone-600 hover:bg-stone-100 hover:text-stone-900"
                )}
              >
                <ClipboardList className="h-3.5 w-3.5 shrink-0 sm:h-4 sm:w-4" />
                <span>Pesanan Saya</span>
              </Link>
              <button
                type="button"
                disabled={logoutMutation.isPending}
                onClick={() => logoutMutation.mutate()}
                className="ml-0.5 inline-flex items-center gap-1 rounded-lg border border-stone-300 bg-stone-50 px-2.5 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-100 hover:text-stone-900 sm:ml-1 sm:px-3 sm:text-sm cursor-pointer"
                title={`Keluar (${user.name})`}
              >
                <LogOut className="h-3.5 w-3.5 shrink-0 text-rose-700" />
                <span className="hidden sm:inline">Keluar</span>
              </button>
            </>
          ) : user ? (
            <>
              <div className="hidden items-center gap-2 rounded-xl border border-stone-200 bg-stone-50 px-2.5 py-1 text-xs text-stone-800 sm:flex">
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-stone-700 text-[11px] font-bold text-white shadow-2xs">
                  {user.name ? user.name.charAt(0).toUpperCase() : "U"}
                </div>
                <div className="flex flex-col text-left">
                  <span className="max-w-[130px] truncate font-semibold leading-tight text-stone-900">
                    {user.name}
                  </span>
                  <span className="text-[10px] text-stone-500 leading-none">
                    {ROLE_LABELS[user.role]}
                  </span>
                </div>
              </div>

              <Link
                href={getDashboardPathForRole(user.role)}
                className="ml-0.5 inline-flex items-center gap-1.5 rounded-lg border border-amber-300 bg-amber-50 px-2.5 py-1.5 text-xs font-medium text-amber-900 transition-colors hover:bg-amber-100 sm:ml-1 sm:px-3 sm:text-sm"
              >
                <LayoutDashboard className="h-3.5 w-3.5 shrink-0" />
                <span className="hidden sm:inline">Dashboard Internal</span>
                <span className="sm:hidden">Panel</span>
              </Link>
              <button
                type="button"
                disabled={logoutMutation.isPending}
                onClick={() => logoutMutation.mutate()}
                className="inline-flex items-center gap-1 rounded-lg border border-stone-300 bg-stone-50 px-2.5 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-100 hover:text-stone-900 sm:px-3 sm:text-sm cursor-pointer"
                title={`Keluar (${user.name})`}
              >
                <LogOut className="h-3.5 w-3.5 shrink-0 text-rose-700" />
                <span className="hidden sm:inline">Keluar</span>
              </button>
            </>
          ) : (
            <>
              <Link
                href="/login"
                className={cn(
                  "ml-0.5 inline-flex items-center gap-1.5 rounded-lg border border-stone-300 bg-stone-50 px-2.5 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-100 hover:text-stone-900 sm:ml-1 sm:px-3 sm:text-sm",
                  pathname === "/login" && "border-amber-300 bg-amber-50 text-amber-900"
                )}
              >
                <LogIn className="h-3.5 w-3.5 shrink-0" />
                <span>Masuk</span>
              </Link>
              <Link
                href="/daftar"
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-lg bg-amber-700 px-2.5 py-1.5 text-xs font-medium text-white shadow-xs transition-colors hover:bg-amber-800 sm:px-3 sm:text-sm",
                  pathname === "/daftar" && "ring-2 ring-amber-400"
                )}
              >
                <UserPlus className="h-3.5 w-3.5 shrink-0" />
                <span>Daftar</span>
              </Link>
            </>
          )}
        </nav>
      </div>
    </header>
  );
}

