"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ChefHat, Lock, PackageSearch, ShoppingBag, Store } from "lucide-react";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  { href: "/", label: "Katalog", icon: Store },
  { href: "/pesan", label: "Pesan", icon: ShoppingBag },
  { href: "/lacak", label: "Lacak Pesanan", icon: PackageSearch },
];

export function PublicNavbar() {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-30 border-b border-stone-200 bg-white/95 backdrop-blur-sm">
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
          className="flex items-center gap-1 sm:gap-2"
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
          <Link
            href="/login"
            className="ml-0.5 inline-flex items-center gap-1.5 rounded-lg border border-stone-300 bg-stone-50 px-2.5 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:bg-stone-100 hover:text-stone-900 sm:ml-2 sm:px-3 sm:text-sm"
          >
            <Lock className="h-3.5 w-3.5 shrink-0" />
            <span className="hidden sm:inline">Portal Internal</span>
            <span className="sm:hidden">Staf</span>
          </Link>
        </nav>
      </div>
    </header>
  );
}

