"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  Boxes,
  ChefHat,
  Coins,
  Cpu,
  Database,
  FileSpreadsheet,
  RefreshCw,
  RotateCcw,
  Save,
  ShieldCheck,
  Users,
} from "lucide-react";
import {
  getAuditLogs,
  getErrorMessage,
  getIngredients,
  getProducts,
  getResources,
  getSettings,
  getUsers,
  resetDemoData,
  updateSettings,
} from "@/lib/api";
import {
  AUDIT_ACTION_LABELS,
  AUDIT_ENTITY_LABELS,
  formatDateTimeWIB,
  formatNumber,
  formatRupiah,
  ROLE_LABELS,
} from "@/lib/format";
import type {
  AuditLogItem,
  Ingredient,
  Product,
  Resource,
  User,
  UserRole,
} from "@/lib/types";
import { ErrorState, TableSkeleton } from "@/components/ui/states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useToast } from "@/components/providers";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

function summarizeAuditDetail(detail: Record<string, unknown>): string {
  if (!detail || Object.keys(detail).length === 0) return "-";
  const entries = Object.entries(detail).slice(0, 4);
  return entries
    .map(([k, v]) => {
      if (v === null || v === undefined) return `${k}: -`;
      if (typeof v === "object") return `${k}: ${JSON.stringify(v)}`;
      return `${k}: ${String(v)}`;
    })
    .join(" · ");
}

export default function AdminDashboardPage() {
  const toast = useToast();
  const [users, setUsers] = useState<User[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [ingredients, setIngredients] = useState<Ingredient[]>([]);
  const [resources, setResources] = useState<Resource[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLogItem[]>([]);
  const [totalAudits, setTotalAudits] = useState(0);

  const [platformFee, setPlatformFee] = useState<string>("2000");
  const [savingFee, setSavingFee] = useState(false);

  const [resettingDemo, setResettingDemo] = useState(false);
  const [showResetConfirm, setShowResetConfirm] = useState(false);

  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleResetDemo = async () => {
    setResettingDemo(true);
    try {
      await resetDemoData();
      toast.success(
        "Data Demo Berhasil Dimuat Ulang",
        "40 pesanan katering (lunas & belum lunas) serta kata sandi akun demo telah disetel ulang sesuai konfigurasi backend."
      );
      setShowResetConfirm(false);
      await loadAdminDashboard();
    } catch (err) {
      toast.error("Gagal Reset Demo", getErrorMessage(err));
    } finally {
      setResettingDemo(false);
    }
  };

  const loadAdminDashboard = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const [usersRes, prodsRes, ingsRes, resList, auditsRes, settingsRes] =
        await Promise.all([
          getUsers({ page: 1, limit: 100 }),
          getProducts(),
          getIngredients(),
          getResources(),
          getAuditLogs({ page: 1, limit: 10 }),
          getSettings(),
        ]);
      setUsers(usersRes.data);
      setProducts(prodsRes);
      setIngredients(ingsRes);
      setResources(resList);
      setAuditLogs(auditsRes.data);
      setTotalAudits(auditsRes.meta.total);
      if (settingsRes?.platform_fee !== undefined) {
        setPlatformFee(String(settingsRes.platform_fee));
      }
    } catch (err) {
      setErrorMsg(
        getErrorMessage(err, "Gagal memuat data Dashboard Administrator.")
      );
    } finally {
      setLoading(false);
    }
  }, []);

  const handleSaveFee = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingFee(true);
    try {
      const num = Number.parseFloat(platformFee);
      if (isNaN(num) || num < 0) {
        toast.error(
          "Format Tidak Valid",
          "Tarif biaya aplikasi harus berupa angka non-negatif."
        );
        return;
      }
      await updateSettings({ platform_fee: num });
      toast.success(
        "Tarif Aplikasi Diperbarui",
        `Biaya layanan aplikasi berhasil diatur menjadi ${formatRupiah(num)} per transaksi.`
      );
    } catch (err) {
      toast.error(
        "Gagal Menyimpan",
        getErrorMessage(err, "Gagal memperbarui tarif aplikasi.")
      );
    } finally {
      setSavingFee(false);
    }
  };

  useEffect(() => {
    void loadAdminDashboard();
  }, [loadAdminDashboard]);

  if (loading && users.length === 0 && products.length === 0) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-stone-900">
            Dashboard Administrator Sistem
          </h1>
          <p className="text-sm text-stone-500">
            Memuat ringkasan akun pengguna, metrik master data, dan 10 log audit
            terakhir...
          </p>
        </div>
        <TableSkeleton rows={6} />
      </div>
    );
  }

  if (errorMsg && users.length === 0) {
    return (
      <ErrorState
        title="Gagal Memuat Dashboard Admin"
        message={errorMsg}
        onRetry={() => void loadAdminDashboard()}
      />
    );
  }

  const activeUsers = users.filter((u) => u.is_active);
  const roleCounts: Record<UserRole, number> = {
    owner: 0,
    manager: 0,
    staff: 0,
    admin: 0,
    customer: 0,
  };
  for (const u of activeUsers) {
    if (roleCounts[u.role] !== undefined) {
      roleCounts[u.role] += 1;
    }
  }

  const activeProducts = products.filter((p) => p.is_active);
  const activeIngredients = ingredients.filter((i) => i.is_active);
  const lowStockIngredients = activeIngredients.filter(
    (i) => i.below_min_stock
  );

  return (
    <div className="space-y-6">
      {/* Header Dashboard */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="amber">Administrator Sistem</Badge>
            <span className="text-xs font-medium text-stone-500">
              Kendali Akun RBAC, Master Data &amp; Audit
            </span>
          </div>
          <h1 className="mt-1.5 text-2xl font-bold tracking-tight text-stone-900">
            Dashboard Administrator Sistem
          </h1>
          <p className="mt-0.5 text-sm text-stone-600">
            Ringkasan akun pengguna aktif per peran, metrik master data katalog,
            dan 10 catatan audit terakhir.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Link href="/app/pengguna">
            <Button size="sm">
              <Users className="h-3.5 w-3.5" />
              Kelola Pengguna
            </Button>
          </Link>
          <Link href="/app/audit">
            <Button variant="outline" size="sm">
              <FileSpreadsheet className="h-3.5 w-3.5" />
              Semua Audit Log ({totalAudits})
            </Button>
          </Link>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void loadAdminDashboard()}
            disabled={loading}
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`}
            />
            Segarkan
          </Button>
        </div>
      </div>

      {/* KONTROL ADMIN: Konfigurasi Tarif Biaya Layanan Aplikasi */}
      <Card className="border-amber-200/90 bg-linear-to-r from-amber-50/70 via-stone-50/40 to-white p-5 shadow-xs">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Coins className="h-5 w-5 text-amber-700" />
              <h2 className="text-base font-bold text-stone-900">
                Tarif Biaya Layanan Aplikasi (Platform Fee)
              </h2>
              <Badge variant="amber">Wewenang Khusus Admin</Badge>
            </div>
            <p className="max-w-2xl text-xs leading-relaxed text-stone-600">
              Biaya tetap per transaksi yang dikenakan kepada pelanggan pada kalkulasi pesanan.
              Pemilik Usaha (Owner) dapat memantau besaran tarif ini secara transparan di menu Parameter Usaha tanpa wewenang mengubahnya secara sepihak.
            </p>
          </div>

          <form onSubmit={handleSaveFee} className="flex shrink-0 items-center gap-2.5">
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-xs font-bold text-stone-500">
                Rp
              </span>
              <Input
                type="number"
                min="0"
                step="100"
                value={platformFee}
                onChange={(e) => setPlatformFee(e.target.value)}
                className="w-36 pl-9 text-sm font-bold text-stone-900"
                required
              />
            </div>
            <Button type="submit" size="sm" disabled={savingFee}>
              <Save className="h-3.5 w-3.5" />
              {savingFee ? "Menyimpan..." : "Simpan Tarif"}
            </Button>
          </form>
        </div>
      </Card>

      {/* KONTROL ADMIN 2: Reset & Muat Ulang Data Demo Katering */}
      <Card className="border-stone-200 bg-stone-50/60 p-5 shadow-xs">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <Database className="h-5 w-5 text-amber-700" />
              <h2 className="text-base font-bold text-stone-900">
                Data Demo Katering &amp; Skenario Transaksi (40 Pesanan)
              </h2>
              <Badge variant="slate">Lingkungan Pengujian</Badge>
            </div>
            <p className="max-w-2xl text-xs leading-relaxed text-stone-600">
              Muat ulang seluruh 40 pesanan deterministik (termasuk pesanan Lunas yang siap dicetak nota, pesanan Belum Lunas / DP saja, pesanan dalam produksi, hingga pembatalan) dan setel ulang kata sandi semua akun demo (<code className="font-mono text-stone-800">@demo.test</code>) sesuai nilai <code className="font-mono text-stone-800">SEED_DEMO_PASSWORD</code> pada berkas backend <code className="font-mono">.env</code>.
            </p>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            {showResetConfirm ? (
              <div className="flex items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 p-2 text-xs">
                <span className="font-medium text-rose-900">Yakin reset database?</span>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={resettingDemo}
                  onClick={() => void handleResetDemo()}
                  className="gap-1"
                >
                  <RotateCcw className={`h-3 w-3 ${resettingDemo ? "animate-spin" : ""}`} />
                  {resettingDemo ? "Mereset..." : "Ya, Reset Sekarang"}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={resettingDemo}
                  onClick={() => setShowResetConfirm(false)}
                >
                  Batal
                </Button>
              </div>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowResetConfirm(true)}
                className="gap-1.5 border-stone-300 text-stone-700 hover:bg-stone-100"
              >
                <RotateCcw className="h-3.5 w-3.5 text-stone-500" />
                Reset &amp; Muat Ulang Data Demo
              </Button>
            )}
          </div>
        </div>
      </Card>

      {/* ELEMEN 1: Ringkasan Akun Pengguna Aktif */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <Users className="h-4 w-4 text-amber-700" />
              1. Ringkasan Akun Pengguna Aktif ({activeUsers.length} Aktif dari{" "}
              {users.length} Akun)
            </CardTitle>
            <CardDescription>
              Distribusi akun aktif berdasarkan peran akses (RBAC) dan daftar
              pengguna terdaftar.
            </CardDescription>
          </div>
          <Link
            href="/app/pengguna"
            className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
          >
            Buka Manajemen Pengguna
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            {(
              ["owner", "manager", "staff", "admin", "customer"] as UserRole[]
            ).map((role) => (
              <div
                key={role}
                className="rounded-xl border border-stone-200 bg-stone-50/70 p-3"
              >
                <p className="text-[11px] font-semibold text-stone-500">
                  {ROLE_LABELS[role]}
                </p>
                <p className="mt-1 text-xl font-bold text-stone-900">
                  {roleCounts[role]} Akun
                </p>
              </div>
            ))}
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-stone-200 text-stone-500">
                  <th className="py-2 pr-3 font-semibold">Nama Pengguna</th>
                  <th className="px-3 py-2 font-semibold">Email</th>
                  <th className="px-3 py-2 font-semibold">Peran</th>
                  <th className="py-2 pl-3 text-right font-semibold">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100">
                {users.slice(0, 8).map((u) => (
                  <tr key={u.id} className="hover:bg-stone-50/80">
                    <td className="py-2 pr-3 font-semibold text-stone-900">
                      {u.name}
                    </td>
                    <td className="px-3 py-2 font-mono text-stone-600">
                      {u.email}
                    </td>
                    <td className="px-3 py-2">
                      <Badge variant="amber">
                        {ROLE_LABELS[u.role] ?? u.role}
                      </Badge>
                    </td>
                    <td className="py-2 pl-3 text-right">
                      <Badge variant={u.is_active ? "emerald" : "slate"}>
                        {u.is_active ? "Aktif" : "Nonaktif"}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* ELEMEN 2: Master Data Metrics */}
      <section aria-label="Metrik Master Data" className="space-y-3">
        <h2 className="flex items-center gap-2 text-base font-bold text-stone-900">
          <ShieldCheck className="h-4 w-4 text-amber-700" />
          2. Master Data Metrics (Produk, Bahan Baku &amp; Sumber Daya)
        </h2>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <Card>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm">Katalog Produk &amp; Resep</CardTitle>
                <ChefHat className="h-4 w-4 text-amber-700" />
              </div>
              <CardDescription>
                Produk pre-order beserta komposisi resep dan beban produksi
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold text-stone-900">
                  {activeProducts.length} Aktif
                </span>
                <span className="text-xs text-stone-500">
                  Total {products.length} produk terdaftar
                </span>
              </div>
              <Link
                href="/app/produk"
                className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:underline"
              >
                Kelola Produk, Resep &amp; Beban →
              </Link>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm">Master Bahan Baku</CardTitle>
                <Boxes className="h-4 w-4 text-emerald-700" />
              </div>
              <CardDescription>
                Persediaan bahan dapur, satuan beli, dan aturan pembulatan
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-baseline justify-between">
                <span className="text-2xl font-bold text-stone-900">
                  {activeIngredients.length} Bahan
                </span>
                <Badge
                  variant={lowStockIngredients.length > 0 ? "rose" : "emerald"}
                >
                  {lowStockIngredients.length} di bawah stok min
                </Badge>
              </div>
              <Link
                href="/app/bahan"
                className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:underline"
              >
                Kelola Master Bahan Baku →
              </Link>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm">Sumber Daya Kapasitas</CardTitle>
                <Cpu className="h-4 w-4 text-sky-700" />
              </div>
              <CardDescription>
                Kapasitas efektif harian peralatan dan tenaga kerja dapur
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <div className="text-2xl font-bold text-stone-900">
                {resources.length} Sumber Daya
              </div>
              <div className="space-y-1 text-xs text-stone-600">
                {resources.map((r) => (
                  <div key={r.id} className="flex justify-between">
                    <span className="font-medium text-stone-800">{r.name}:</span>
                    <span className="font-semibold">
                      {formatNumber(r.daily_capacity)} {r.unit}/hari
                    </span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
      </section>

      {/* ELEMEN 3: Audit Log 10 Baris Terakhir */}
      <Card>
        <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
          <div>
            <CardTitle className="flex items-center gap-2">
              <FileSpreadsheet className="h-4 w-4 text-amber-700" />
              3. Audit Log 10 Baris Terakhir
            </CardTitle>
            <CardDescription>
              Jejak perubahan terbaru lintas entitas (keputusan, override,
              pembayaran, stok, dan master data).
            </CardDescription>
          </div>
          <Link
            href="/app/audit"
            className="inline-flex items-center gap-1 text-xs font-semibold text-amber-800 hover:text-amber-950"
          >
            Buka Tabel Audit Log Lengkap
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </CardHeader>
        <CardContent>
          {auditLogs.length === 0 ? (
            <p className="py-8 text-center text-xs text-stone-500">
              Belum ada catatan audit log yang tersimpan.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-stone-200 text-stone-500">
                    <th className="py-2.5 pr-3 font-semibold">Waktu (WIB)</th>
                    <th className="px-3 py-2.5 font-semibold">Pelaku &amp; Peran</th>
                    <th className="px-3 py-2.5 font-semibold">Aksi</th>
                    <th className="px-3 py-2.5 font-semibold">Entitas</th>
                    <th className="py-2.5 pl-3 font-semibold">
                      Ringkasan Detail
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {auditLogs.map((log) => (
                    <tr key={log.id} className="hover:bg-stone-50/80">
                      <td className="whitespace-nowrap py-2.5 pr-3 text-stone-600">
                        {formatDateTimeWIB(log.created_at)}
                      </td>
                      <td className="px-3 py-2.5">
                        <div className="font-semibold text-stone-900">
                          {log.actor_name || "Publik / Sistem"}
                        </div>
                        {log.actor_role ? (
                          <span className="text-[11px] text-stone-500">
                            {ROLE_LABELS[log.actor_role as UserRole] ??
                              log.actor_role}
                          </span>
                        ) : null}
                      </td>
                      <td className="px-3 py-2.5">
                        <Badge
                          variant={
                            log.action === "override" || log.action === "cancel"
                              ? "rose"
                              : log.action === "decision" ||
                                  log.action === "payment"
                                ? "emerald"
                                : "amber"
                          }
                        >
                          {AUDIT_ACTION_LABELS[log.action] ?? log.action}
                        </Badge>
                      </td>
                      <td className="px-3 py-2.5 font-medium text-stone-700">
                        {AUDIT_ENTITY_LABELS[log.entity] ?? log.entity}
                      </td>
                      <td className="max-w-md truncate py-2.5 pl-3 font-mono text-[11px] text-stone-600">
                        {summarizeAuditDetail(log.detail)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
