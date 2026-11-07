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
  Loader2,
  ShieldCheck,
  UserPlus,
} from "lucide-react";
import { register as registerApi, getErrorMessage } from "@/lib/api";
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

const registerSchema = z.object({
  name: z
    .string()
    .trim()
    .min(2, "Nama lengkap minimal 2 karakter.")
    .max(100, "Nama lengkap maksimal 100 karakter."),
  email: z
    .string()
    .trim()
    .min(1, "Alamat email wajib diisi.")
    .email("Format alamat email tidak valid."),
  contact: z
    .string()
    .trim()
    .min(6, "Nomor WhatsApp/telepon minimal 6 karakter.")
    .max(40, "Nomor kontak maksimal 40 karakter."),
  password: z
    .string()
    .min(8, "Kata sandi minimal 8 karakter.")
    .max(72, "Kata sandi maksimal 72 karakter."),
});

type RegisterFormValues = z.infer<typeof registerSchema>;

function RegisterFormContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const nextPath = searchParams.get("next");
  const queryClient = useQueryClient();
  const toast = useToast();
  const [serverError, setServerError] = useState<string>("");

  const form = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      name: "",
      email: "",
      contact: "",
      password: "",
    },
  });

  const registerMutation = useMutation({
    mutationFn: registerApi,
    onSuccess: (res) => {
      setServerError("");
      queryClient.setQueryData(["auth-me"], res.user);
      toast.success(
        `Selamat bergabung, ${res.user.name}!`,
        "Akun pelanggan Anda berhasil dibuat dan langsung masuk."
      );
      const safeTarget =
        nextPath &&
        (nextPath.startsWith("/pesan") ||
          nextPath.startsWith("/pesanan-saya") ||
          nextPath.startsWith("/lacak"))
          ? nextPath
          : "/pesanan-saya";
      router.push(safeTarget);
      router.refresh();
    },
    onError: (err) => {
      const msg = getErrorMessage(
        err,
        "Gagal mendaftarkan akun. Periksa kembali data Anda."
      );
      setServerError(msg);
      toast.error("Pendaftaran Gagal", msg);
    },
  });

  const onSubmit = (values: RegisterFormValues) => {
    setServerError("");
    registerMutation.mutate({
      name: values.name.trim(),
      email: values.email.trim().toLowerCase(),
      contact: values.contact.trim(),
      password: values.password,
    });
  };

  const loginHref = nextPath
    ? `/login?next=${encodeURIComponent(nextPath)}`
    : "/login";

  return (
    <div className="mx-auto w-full max-w-md space-y-5">
      <Card className="border-stone-200/90 shadow-sm">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-amber-700 text-white shadow-xs">
            <ChefHat className="h-6 w-6" />
          </div>
          <CardTitle className="text-xl sm:text-2xl">
            Daftar Akun Pelanggan
          </CardTitle>
          <CardDescription>
            Buat akun untuk memesan produk pre-order Kalkurasa, menyimpan data
            kontak otomatis, dan memantau seluruh pesanan Anda di satu tempat.
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          <div className="flex items-start gap-2 rounded-xl border border-stone-200 bg-stone-50 p-3 text-xs text-stone-600">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
            <span>
              Untuk menjaga kepastian slot kapasitas dapur bagi semua pelanggan,
              pembuatan pesanan pre-order memerlukan akun yang terverifikasi.
            </span>
          </div>

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
            className="space-y-3.5"
          >
            <div className="space-y-1.5">
              <Label htmlFor="name">Nama Lengkap / Instansi</Label>
              <Input
                id="name"
                type="text"
                autoComplete="name"
                placeholder="Mis. Dewi Lestari"
                {...form.register("name")}
              />
              {form.formState.errors.name ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.name.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="email">Alamat Email</Label>
              <Input
                id="email"
                type="email"
                autoComplete="email"
                placeholder="nama@email.com"
                {...form.register("email")}
              />
              {form.formState.errors.email ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.email.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="contact">Nomor WhatsApp / Telepon Aktif</Label>
              <Input
                id="contact"
                type="tel"
                autoComplete="tel"
                placeholder="Mis. 081234567890"
                {...form.register("contact")}
              />
              {form.formState.errors.contact ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.contact.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="password">Kata Sandi (Minimal 8 Karakter)</Label>
              <Input
                id="password"
                type="password"
                autoComplete="new-password"
                placeholder="Minimal 8 karakter"
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
              disabled={registerMutation.isPending}
              className="w-full gap-2"
            >
              {registerMutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  Mendaftarkan Akun...
                </>
              ) : (
                <>
                  <UserPlus className="h-4 w-4" />
                  Daftar & Masuk Sekarang
                </>
              )}
            </Button>
          </form>

          <div className="border-t border-stone-200 pt-3 text-center text-xs text-stone-600">
            Sudah memiliki akun pelanggan?{" "}
            <Link
              href={loginHref}
              className="font-semibold text-amber-800 underline underline-offset-2 hover:text-amber-950"
            >
              Masuk di sini
            </Link>
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

export default function RegisterPage() {
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
          <RegisterFormContent />
        </Suspense>
      </main>
    </div>
  );
}

