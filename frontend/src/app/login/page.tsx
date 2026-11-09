"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  KeyRound,
  Loader2,
  LogIn,
  ShieldCheck,
  UserCheck,
} from "lucide-react";
import { login, getErrorMessage } from "@/lib/api";
import { resolvePostLoginRedirect, ROLE_LABELS } from "@/lib/format";
import type { UserRole } from "@/lib/types";
import { PublicNavbar } from "@/components/public-navbar";
import { KalkurasaLogoIcon } from "@/components/kalkurasa-logo";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardHeader,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const loginSchema = z.object({
  email: z
    .string()
    .trim()
    .min(1, "Alamat email wajib diisi.")
    .email("Format alamat email tidak valid."),
  password: z.string().min(1, "Kata sandi wajib diisi."),
});

type LoginFormValues = z.infer<typeof loginSchema>;

const DEMO_ACCOUNTS: {
  email: string;
  role: UserRole;
  name: string;
  scope: string;
  badge: "default" | "emerald" | "amber" | "sky" | "slate";
}[] = [
  {
    email: "customer@demo.test",
    role: "customer",
    name: "Dewi Lestari",
    scope: "Buat pesanan pre-order, pantau kode unik, & tanggapi tawaran balik",
    badge: "default",
  },
  {
    email: "owner@demo.test",
    role: "owner",
    name: "Bu Sari (Pemilik)",
    scope: "Kendali penuh 7 KPI, antrian keputusan besar, kas 14 hari, & override",
    badge: "amber",
  },
  {
    email: "manager@demo.test",
    role: "manager",
    name: "Rina (Manajer)",
    scope: "Evaluasi 4 gerbang, kalender kapasitas oven, daftar belanja, & stok",
    badge: "emerald",
  },
  {
    email: "staff@demo.test",
    role: "staff",
    name: "Budi (Kru Produksi)",
    scope: "Lembar tugas belanja, produksi, & antar (tanpa akses harga/kas)",
    badge: "sky",
  },
  {
    email: "admin@demo.test",
    role: "admin",
    name: "Admin Sistem",
    scope: "Kelola akun pengguna, master resep, log audit, & reset data simulasi",
    badge: "slate",
  },
];

function LoginFormContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const nextPath = searchParams.get("next");
  const queryClient = useQueryClient();
  const toast = useToast();
  const [serverError, setServerError] = useState<string>("");
  const [selectedDemoEmail, setSelectedDemoEmail] = useState<string>("");

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: "",
      password: "",
    },
  });

  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: (res) => {
      setServerError("");
      queryClient.setQueryData(["auth-me"], res.user);
      toast.success(
        `Selamat datang, ${res.user.name}`,
        `Masuk sebagai ${ROLE_LABELS[res.user.role]}`
      );
      const target = resolvePostLoginRedirect(res.user.role, nextPath);
      router.push(target);
      router.refresh();
    },
    onError: (err) => {
      const msg = getErrorMessage(err, "Email atau kata sandi salah.");
      setServerError(msg);
      toast.error("Gagal Masuk", msg);
    },
  });

  const onSubmit = (values: LoginFormValues) => {
    setServerError("");
    loginMutation.mutate({
      email: values.email.trim().toLowerCase(),
      password: values.password,
    });
  };

  const registerHref = nextPath
    ? `/daftar?next=${encodeURIComponent(nextPath)}`
    : "/daftar";

  return (
    <div className="mx-auto w-full max-w-5xl space-y-6">
      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
        {/* Kolom Kiri: Form Masuk (5 Kolom) */}
        <Card className="border-[var(--border-strong)] shadow-sm lg:col-span-5">
          <CardHeader className="space-y-2 border-b border-[var(--border-warm)] pb-5">
            <div className="flex items-center gap-3">
              <div className="h-12 w-12 shrink-0">
                <KalkurasaLogoIcon className="h-12 w-12" />
              </div>
              <div>
                <h1 className="font-display text-xl font-bold text-[var(--ink-primary)] sm:text-2xl">
                  Masuk ke Kalkurasa
                </h1>
                <p className="text-xs font-medium text-[var(--ink-muted)]">
                  Portal Pelanggan Pre-Order &amp; Tim Operasional Dapur
                </p>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-4 pt-5">
            {serverError ? (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-xl border border-rose-300 bg-rose-50 p-3.5 text-xs font-semibold text-rose-900 dark:border-rose-800 dark:bg-rose-950/60 dark:text-rose-200"
              >
                <AlertCircle
                  className="mt-0.5 h-4 w-4 shrink-0 text-rose-600"
                  aria-hidden="true"
                />
                <span>{serverError}</span>
              </div>
            ) : null}

            <form
              onSubmit={form.handleSubmit(onSubmit)}
              noValidate
              className="space-y-4"
            >
              <div className="space-y-1.5">
                <Label htmlFor="email">Alamat Email</Label>
                <Input
                  id="email"
                  type="email"
                  autoComplete="email"
                  placeholder="nama@demo.test"
                  {...form.register("email")}
                />
                {form.formState.errors.email ? (
                  <p className="text-xs font-semibold text-rose-600">
                    {form.formState.errors.email.message}
                  </p>
                ) : null}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="password">Kata Sandi</Label>
                <Input
                  id="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="Masukkan kata sandi akun"
                  {...form.register("password")}
                />
                {form.formState.errors.password ? (
                  <p className="text-xs font-semibold text-rose-600">
                    {form.formState.errors.password.message}
                  </p>
                ) : null}
              </div>

              <Button
                type="submit"
                size="lg"
                disabled={loginMutation.isPending}
                className="w-full gap-2"
              >
                {loginMutation.isPending ? (
                  <>
                    <Loader2
                      className="h-4 w-4 animate-spin"
                      aria-hidden="true"
                    />
                    Memverifikasi Sesi...
                  </>
                ) : (
                  <>
                    <LogIn className="h-4 w-4" aria-hidden="true" />
                    Masuk Sekarang
                  </>
                )}
              </Button>
            </form>

            <div className="rounded-xl border border-[var(--border-warm)] bg-[var(--bg-subtle)]/70 p-3.5 text-center text-xs text-[var(--ink-secondary)]">
              Pelanggan baru ingin memesan sajian?{" "}
              <Link
                href={registerHref}
                className="font-bold text-[#C2410C] underline underline-offset-4 hover:text-[#9A3412] dark:text-orange-400"
              >
                Daftar Akun Pelanggan
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Kolom Kanan: Kartu Pemilih Peran Simulasi Interaktif (7 Kolom) */}
        <div className="space-y-4 rounded-2xl border border-[var(--border-warm)] bg-[var(--bg-surface)] p-5 shadow-2xs sm:p-6 lg:col-span-7">
          <div className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--border-warm)] pb-3.5">
            <div className="flex items-center gap-2">
              <KeyRound
                className="h-4 w-4 text-[#C2410C]"
                aria-hidden="true"
              />
              <h2 className="font-display text-base font-bold text-[var(--ink-primary)] sm:text-lg">
                Pilih Cepat Akun Peran (Simulasi Pengujian)
              </h2>
            </div>
            <Badge variant="slate">5 Peran RBAC</Badge>
          </div>

          <p className="text-xs leading-relaxed text-[var(--ink-secondary)]">
            Klik salah satu kartu peran di bawah ini untuk mengisi kolom email
            dan kata sandi demo (<code className="font-mono font-bold text-[#12100E]">demo12345</code>) secara otomatis.
          </p>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {DEMO_ACCOUNTS.map((acc) => {
              const isSelected = selectedDemoEmail === acc.email;
              return (
                <button
                  key={acc.email}
                  type="button"
                  onClick={() => {
                    setSelectedDemoEmail(acc.email);
                    form.setValue("email", acc.email, { shouldValidate: true });
                    form.setValue("password", "demo12345", {
                      shouldValidate: true,
                    });
                    setServerError("");
                  }}
                  className={`card-interactive flex flex-col items-start justify-between rounded-xl border p-3.5 text-left transition-all ${
                    isSelected
                      ? "border-[#C2410C] bg-[var(--accent-soft)] ring-2 ring-[#C2410C]/20"
                      : "border-[var(--border-warm)] bg-[var(--bg-canvas)] hover:border-[#C2410C]"
                  }`}
                >
                  <div className="w-full space-y-1.5">
                    <div className="flex items-center justify-between gap-2">
                      <Badge variant={acc.badge}>{ROLE_LABELS[acc.role]}</Badge>
                      {isSelected ? (
                        <CheckCircle2
                          className="h-4 w-4 text-[#C2410C]"
                          aria-hidden="true"
                        />
                      ) : (
                        <UserCheck
                          className="h-3.5 w-3.5 text-[var(--ink-muted)]"
                          aria-hidden="true"
                        />
                      )}
                    </div>
                    <p className="text-sm font-bold text-[var(--ink-primary)]">
                      {acc.name}
                    </p>
                    <p className="text-[11px] leading-relaxed text-[var(--ink-secondary)]">
                      {acc.scope}
                    </p>
                  </div>
                  <span className="mt-2.5 inline-block rounded-md bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[11px] font-semibold text-[#C2410C] dark:text-orange-400">
                    {acc.email}
                  </span>
                </button>
              );
            })}
          </div>

          <div className="flex items-start gap-2.5 rounded-xl border border-[var(--border-warm)] bg-[var(--bg-subtle)]/70 p-3 text-xs text-[var(--ink-secondary)]">
            <ShieldCheck
              className="mt-0.5 h-4 w-4 shrink-0 text-[#15803D]"
              aria-hidden="true"
            />
            <span>
              <strong>Proteksi Hak Akses (RBAC):</strong> Karyawan produksi
              tidak pernah menerima data harga jual, margin G2, maupun saldo kas
              di respons API maupun layar kerja.
            </span>
          </div>
        </div>
      </div>

      <div className="text-center">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-[var(--ink-secondary)] hover:text-[#C2410C]"
        >
          <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
          Kembali ke Halaman Katalog &amp; Simulasi Dapur
        </Link>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />
      <main className="flex flex-1 flex-col justify-center px-4 py-8 sm:px-6">
        <Suspense
          fallback={
            <div className="mx-auto w-full max-w-5xl">
              <Skeleton className="h-96 w-full rounded-2xl" />
            </div>
          }
        >
          <LoginFormContent />
        </Suspense>
      </main>
    </div>
  );
}
