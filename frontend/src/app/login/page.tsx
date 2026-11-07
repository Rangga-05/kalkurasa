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
  ChefHat,
  KeyRound,
  Loader2,
  LogIn,
  UserCheck,
} from "lucide-react";
import { login, getErrorMessage } from "@/lib/api";
import { getDashboardPathForRole, ROLE_LABELS } from "@/lib/format";
import type { UserRole } from "@/lib/types";
import { PublicNavbar } from "@/components/public-navbar";
import { useToast } from "@/components/providers";
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

const DEMO_ACCOUNTS: { email: string; role: UserRole; name: string }[] = [
  {
    email: "customer@demo.test",
    role: "customer",
    name: "Dewi Lestari (Pelanggan)",
  },
  { email: "owner@demo.test", role: "owner", name: "Bu Sari (Pemilik)" },
  { email: "manager@demo.test", role: "manager", name: "Rina (Manajer)" },
  { email: "staff@demo.test", role: "staff", name: "Budi (Karyawan)" },
  { email: "admin@demo.test", role: "admin", name: "Admin Sistem" },
];

function LoginFormContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const nextPath = searchParams.get("next");
  const queryClient = useQueryClient();
  const toast = useToast();
  const [serverError, setServerError] = useState<string>("");

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
      if (res.user.role === "customer") {
        const customerTarget =
          nextPath &&
          (nextPath.startsWith("/pesan") ||
            nextPath.startsWith("/pesanan-saya") ||
            nextPath.startsWith("/lacak"))
            ? nextPath
            : "/pesanan-saya";
        router.push(customerTarget);
        router.refresh();
        return;
      }
      const defaultTarget = getDashboardPathForRole(res.user.role);
      const target =
        nextPath && nextPath.startsWith("/app") ? nextPath : defaultTarget;
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
    <div className="mx-auto w-full max-w-md space-y-5">
      <Card className="border-stone-200/90 shadow-sm">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-700 text-white shadow-xs">
            <ChefHat className="h-6 w-6" />
          </div>
          <CardTitle className="text-xl sm:text-2xl">
            Masuk ke Akun Kalkurasa
          </CardTitle>
          <CardDescription>
            Halaman masuk terpadu untuk Pelanggan Pre-Order maupun Tim Internal
            (Pemilik, Manajer, Karyawan, dan Admin).
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {serverError ? (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs font-medium text-rose-800"
            >
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />
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
                <p className="text-xs font-medium text-rose-600">
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
                placeholder="Masukkan kata sandi"
                {...form.register("password")}
              />
              {form.formState.errors.password ? (
                <p className="text-xs font-medium text-rose-600">
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
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Memverifikasi Akun...
                </>
              ) : (
                <>
                  <LogIn className="h-4 w-4" />
                  Masuk Sekarang
                </>
              )}
            </Button>
          </form>

          <div className="rounded-xl border border-amber-200/80 bg-amber-50/60 p-3 text-center text-xs text-stone-700">
            Belum punya akun pelanggan untuk memesan?{" "}
            <Link
              href={registerHref}
              className="font-semibold text-amber-800 underline underline-offset-2 hover:text-amber-950"
            >
              Daftar Akun Pelanggan
            </Link>
          </div>

          {/* Pilihan Cepat Akun Demo */}
          <div className="border-t border-stone-200 pt-4">
            <div className="mb-2.5 flex items-center gap-1.5 text-xs font-semibold text-stone-600">
              <KeyRound className="h-3.5 w-3.5 text-amber-700" />
              <span>Isi Cepat Email Akun Demo (Simulasi):</span>
            </div>
            <div className="grid grid-cols-2 gap-2">
              {DEMO_ACCOUNTS.map((acc) => (
                <button
                  key={acc.email}
                  type="button"
                  onClick={() => {
                    form.setValue("email", acc.email, { shouldValidate: true });
                    setServerError("");
                  }}
                  className="flex flex-col items-start rounded-lg border border-stone-200 bg-stone-50 px-2.5 py-2 text-left text-xs transition-colors hover:border-amber-300 hover:bg-amber-50/60 cursor-pointer"
                >
                  <span className="flex items-center gap-1 font-semibold text-stone-800">
                    <UserCheck className="h-3 w-3 text-amber-700" />
                    {acc.name}
                  </span>
                  <span className="mt-0.5 font-mono text-[11px] text-stone-500">
                    {acc.email}
                  </span>
                </button>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="text-center">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-stone-600 hover:text-stone-900"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Kembali ke Halaman Katalog Pelanggan
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
            <div className="mx-auto w-full max-w-md">
              <Skeleton className="h-96 w-full rounded-xl" />
            </div>
          }
        >
          <LoginFormContent />
        </Suspense>
      </main>
    </div>
  );
}

