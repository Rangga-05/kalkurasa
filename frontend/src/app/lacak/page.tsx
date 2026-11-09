"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  ArrowRight,
  Clock,
  FileCheck2,
  PackageSearch,
  ShoppingBag,
} from "lucide-react";
import { PublicNavbar } from "@/components/public-navbar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function TrackOrderSearchPage() {
  const router = useRouter();
  const [code, setCode] = useState("");
  const [errMsg, setErrMsg] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const cleaned = code.trim().toUpperCase();
    if (cleaned.length < 6) {
      setErrMsg("Masukkan kode pesanan yang valid (minimal 6 karakter).");
      return;
    }
    setErrMsg("");
    router.push(`/lacak/${encodeURIComponent(cleaned)}`);
  };

  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />
      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col justify-center px-4 py-10 sm:px-6">
        <Card className="overflow-hidden border-[var(--border-strong)] shadow-sm">
          <div className="border-b border-[#382C26] bg-[#18120F] px-6 py-5 text-[#FAF5EE]">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-[#C2410C] text-white shadow-xs">
                <PackageSearch className="h-5 w-5" aria-hidden="true" />
              </div>
              <div>
                <p className="text-[11px] font-bold tracking-wider text-orange-300 uppercase">
                  Tiket Pemantauan Dapur Pre-Order
                </p>
                <h1 className="font-display text-xl font-bold text-white sm:text-2xl">
                  Lacak Status Pesanan &amp; Tawaran Balik
                </h1>
              </div>
            </div>
          </div>

          <CardHeader className="pb-3">
            <CardTitle className="text-base sm:text-lg">
              Masukkan Kode Unik Pesanan Anda
            </CardTitle>
            <CardDescription>
              Setiap pesanan memiliki 10 karakter kode unik untuk memantau
              jadwal panggang, sisa tagihan DP, hingga merespons tawaran balik
              dari dapur.
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-6">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="order-code-input">
                  Kode Pesanan (10 Karakter)
                </Label>
                <div className="flex flex-col gap-3 sm:flex-row">
                  <Input
                    id="order-code-input"
                    value={code}
                    onChange={(e) => {
                      setCode(e.target.value.toUpperCase());
                      if (errMsg) setErrMsg("");
                    }}
                    placeholder="Contoh: K8M4P2X9TQ"
                    maxLength={12}
                    className="font-mono text-base uppercase tracking-widest"
                    autoFocus
                  />
                  <Button type="submit" size="lg" className="shrink-0 gap-2">
                    Lihat Status Pesanan
                    <ArrowRight className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </div>
                {errMsg ? (
                  <p role="alert" className="text-xs font-semibold text-rose-600">
                    {errMsg}
                  </p>
                ) : null}
              </div>
            </form>

            <div className="ticket-divider pt-5">
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="flex items-start gap-3 rounded-xl border border-[var(--border-warm)] bg-[var(--bg-subtle)]/70 p-3.5 text-xs">
                  <Clock
                    className="mt-0.5 h-4 w-4 shrink-0 text-[#C2410C]"
                    aria-hidden="true"
                  />
                  <div>
                    <p className="font-bold text-[var(--ink-primary)]">
                      Pantau Tahap Produksi
                    </p>
                    <p className="mt-0.5 text-[var(--ink-secondary)]">
                      Lihat apakah pesanan sedang menunggu DP, dipanggang di
                      oven, siap diambil, atau sedang diantar.
                    </p>
                  </div>
                </div>

                <div className="flex items-start gap-3 rounded-xl border border-[var(--border-warm)] bg-[var(--bg-subtle)]/70 p-3.5 text-xs">
                  <FileCheck2
                    className="mt-0.5 h-4 w-4 shrink-0 text-[#15803D]"
                    aria-hidden="true"
                  />
                  <div>
                    <p className="font-bold text-[var(--ink-primary)]">
                      Punya Akun Pelanggan?
                    </p>
                    <p className="mt-0.5 text-[var(--ink-secondary)]">
                      Lihat seluruh daftar riwayat kode pesanan Anda di halaman{" "}
                      <Link
                        href="/pesanan-saya"
                        className="font-bold text-[#C2410C] underline underline-offset-2"
                      >
                        Pesanan Saya
                      </Link>
                      .
                    </p>
                  </div>
                </div>
              </div>
            </div>

            <div className="flex items-center justify-between border-t border-[var(--border-warm)] pt-4 text-xs text-[var(--ink-muted)]">
              <span>Belum memiliki pesanan aktif?</span>
              <Link
                href="/pesan"
                className="inline-flex items-center gap-1.5 font-bold text-[#C2410C] hover:underline dark:text-orange-400"
              >
                <ShoppingBag className="h-3.5 w-3.5" aria-hidden="true" />
                Buat Pesanan Baru
              </Link>
            </div>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}
