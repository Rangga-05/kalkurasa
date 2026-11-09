"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowDownRight,
  ArrowUpRight,
  CheckCircle2,
  PlusCircle,
  RefreshCw,
  ShieldAlert,
  TrendingDown,
  Wallet,
} from "lucide-react";
import {
  getCashProjection,
  createCashEntry,
  getErrorMessage,
} from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  formatRupiah,
  toDateKeyWIB,
  toNumber,
} from "@/lib/format";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { useToast } from "@/components/providers";
import { CashProjectionChart } from "@/components/internal/cash-projection-chart";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

export default function KasPage() {
  const { user } = useInternalSession();
  const isOwner = user?.role === "owner";
  const toast = useToast();
  const queryClient = useQueryClient();

  const todayStr = toDateKeyWIB(new Date());
  const [horizonDays, setHorizonDays] = useState(14);

  // Form Entri Kas Manual (Owner)
  const [entryDate, setEntryDate] = useState(todayStr);
  const [entryDirection, setEntryDirection] = useState<"in" | "out">("in");
  const [entryAmount, setEntryAmount] = useState("");
  const [entryCategory, setEntryCategory] = useState("modal_tambahan");
  const [entryNote, setEntryNote] = useState("");
  const [formError, setFormError] = useState<string | null>(null);

  const cashQuery = useQuery({
    queryKey: ["cash-projection", horizonDays],
    queryFn: ({ signal }) => getCashProjection(horizonDays, signal),
  });

  const entryMutation = useMutation({
    mutationFn: createCashEntry,
    onSuccess: (res) => {
      const amt = toNumber(res.amount);
      toast.success(
        "Entri Kas Tercatat",
        `${amt >= 0 ? "Kas masuk" : "Kas keluar"} sebesar ${formatRupiah(Math.abs(amt))} pada ${formatDateShort(res.entry_date)} berhasil disimpan.`
      );
      setEntryAmount("");
      setEntryNote("");
      setFormError(null);
      queryClient.invalidateQueries({ queryKey: ["cash-projection"] });
    },
    onError: (err) => {
      const msg = getErrorMessage(err);
      setFormError(msg);
      toast.error("Gagal Mencatat Entri Kas", msg);
    },
  });

  const handleEntrySubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    const rawNum = Number.parseFloat(entryAmount);
    if (!Number.isFinite(rawNum) || rawNum <= 0) {
      setFormError("Nominal kas harus lebih dari Rp 0.");
      return;
    }
    const signedAmount = entryDirection === "out" ? -rawNum : rawNum;
    entryMutation.mutate({
      entry_date: entryDate || todayStr,
      amount: String(signedAmount),
      category: entryCategory.trim() || "manual",
      note: entryNote.trim() || undefined,
    });
  };

  const data = cashQuery.data;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-stone-900">
            Proyeksi Arus Kas & Gerbang Kas (G3)
          </h1>
          <p className="text-sm text-stone-600">
            Simulasi arus kas harian otomatis dari DP masuk, belanja bahan (H-lead),
            pelunasan (H+1), biaya tetap harian, dan kas manual.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-lg border border-stone-200 bg-white p-1">
            {[7, 14, 30].map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setHorizonDays(d)}
                className={`rounded-md px-2.5 py-1 text-xs font-semibold transition-colors ${
                  horizonDays === d
                    ? "bg-amber-700 text-white"
                    : "text-stone-600 hover:bg-stone-100"
                }`}
              >
                {d} Hari
              </button>
            ))}
          </div>

          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => cashQuery.refetch()}
            disabled={cashQuery.isFetching}
            className="gap-1.5"
          >
            <RefreshCw
              className={`h-4 w-4 ${cashQuery.isFetching ? "animate-spin" : ""}`}
            />
            Segarkan
          </Button>
        </div>
      </div>

      {cashQuery.isLoading ? (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-28 w-full rounded-xl" />
          </div>
          <Skeleton className="h-80 w-full rounded-xl" />
        </div>
      ) : cashQuery.isError || !data ? (
        <Card className="border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          Gagal memuat proyeksi kas: {getErrorMessage(cashQuery.error)}
        </Card>
      ) : (
        <>
          {/* Ringkasan KPI Kas */}
          <div
            className={`grid grid-cols-1 gap-4 ${
              isOwner ? "sm:grid-cols-2 lg:grid-cols-4" : "sm:grid-cols-3"
            }`}
          >
            {/* Status G3 */}
            <Card
              className={`p-4 ${
                data.passed
                  ? "border-emerald-200 bg-emerald-50/50"
                  : "border-rose-200 bg-rose-50/60"
              }`}
            >
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-stone-600">
                  Status Gerbang Kas (G3)
                </span>
                {data.passed ? (
                  <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                ) : (
                  <ShieldAlert className="h-5 w-5 text-rose-600" />
                )}
              </div>
              <div className="mt-2">
                <Badge variant={data.passed ? "emerald" : "rose"}>
                  {data.passed
                    ? "Lolos : Di Atas Penyangga"
                    : "Rawan : Di Bawah Penyangga"}
                </Badge>
              </div>
              <p className="mt-2 text-xs text-stone-600">
                Rentang {formatDateShort(data.from)} s.d. {formatDateShort(data.to)}{" "}
                ({data.horizon_days} hari)
              </p>
            </Card>

            {/* Titik Terendah Kas */}
            <Card className="p-4">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-stone-600">
                  Titik Terendah Kas
                </span>
                <TrendingDown className="h-4 w-4 text-amber-700" />
              </div>
              <p
                className={`mt-2 text-xl font-bold ${
                  data.passed ? "text-stone-900" : "text-rose-700"
                }`}
              >
                {formatRupiah(data.lowest)}
              </p>
              <p className="mt-1 text-xs text-stone-500">
                Terjadi pada{" "}
                <strong className="text-stone-700">
                  {formatDateShort(data.lowest_date)}
                </strong>
              </p>
            </Card>

            {/* Penyangga Kas Minimum */}
            <Card className="p-4">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-stone-600">
                  Penyangga Kas Minimum
                </span>
                <Wallet className="h-4 w-4 text-stone-500" />
              </div>
              <p className="mt-2 text-xl font-bold text-stone-900">
                {formatRupiah(data.cash_buffer)}
              </p>
              <p className="mt-1 text-xs text-stone-500">
                Hari di bawah penyangga:{" "}
                <strong
                  className={
                    data.days_below_buffer.length > 0
                      ? "text-rose-700"
                      : "text-emerald-700"
                  }
                >
                  {data.days_below_buffer.length} hari
                </strong>
              </p>
            </Card>

            {/* Saldo Awal & Biaya Tetap (Hanya Owner) */}
            {isOwner && data.opening_cash !== undefined ? (
              <Card className="p-4">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold uppercase tracking-wider text-stone-600">
                    Saldo Kas Awal
                  </span>
                  <Wallet className="h-4 w-4 text-emerald-700" />
                </div>
                <p className="mt-2 text-xl font-bold text-stone-900">
                  {formatRupiah(data.opening_cash)}
                </p>
                <p className="mt-1 text-xs text-stone-500">
                  Biaya tetap:{" "}
                  <strong className="text-stone-700">
                    {formatRupiah(data.fixed_cost_daily)}/hari
                  </strong>
                </p>
              </Card>
            ) : null}
          </div>

          {/* Peringatan Hari di Bawah Buffer */}
          {data.days_below_buffer.length > 0 ? (
            <Card className="border-rose-300 bg-rose-50/70 p-4">
              <div className="flex items-start gap-3">
                <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-rose-700" />
                <div className="space-y-1.5">
                  <h2 className="text-sm font-bold text-rose-950">
                    Peringatan Hari Rawan Kas ({data.days_below_buffer.length}{" "}
                    Hari di Bawah Penyangga {formatRupiah(data.cash_buffer)})
                  </h2>
                  <p className="text-xs text-rose-800">
                    Pertimbangkan menaikkan DP pesanan besar, menjadwal ulang
                    belanja bahan, atau menambah modal kerja sebelum tanggal
                    berikut:
                  </p>
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {data.days_below_buffer.map((dt) => (
                      <Badge key={dt} variant="rose">
                        {formatDateShort(dt)} ({dt})
                      </Badge>
                    ))}
                  </div>
                </div>
              </div>
            </Card>
          ) : null}

          {/* Tampilan Manajer (Terbatas sesuai RBAC Bagian 7 & 10 Brief) */}
          {!isOwner ? (
            <Card className="border-amber-200 bg-amber-50/40 p-5 text-sm text-stone-700">
              <p className="font-semibold text-stone-900">
                Mode Tampilan Manajer Operasional
              </p>
              <p className="mt-1 text-xs text-stone-600">
                Sesuai kebijakan akses (RBAC), Manajer hanya melihat status
                kelayakan Gerbang Kas (G3), titik kas terendah (
                <strong>{formatRupiah(data.lowest)}</strong> pada{" "}
                <strong>{formatDateLong(data.lowest_date)}</strong>), serta
                tanggal hari rawan tanpa rincian saldo kas awal maupun mutasi
                rekening pemilik.
              </p>
            </Card>
          ) : null}

          {/* Grafik Recharts & Rincian Harian (Khusus Owner) */}
          {isOwner && data.days ? (
            <>
              <Card className="p-5 space-y-4">
                <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <h2 className="text-base font-bold text-stone-900">
                      Kurva Proyeksi Saldo Kas Harian ({data.horizon_days} Hari)
                    </h2>
                    <p className="text-xs text-stone-500">
                      Garis putus merah menandai Penyangga Kas (
                      {formatRupiah(data.cash_buffer)}); titik bulat menandai
                      Titik Terendah ({formatRupiah(data.lowest)} pada{" "}
                      {formatDateShort(data.lowest_date)}).
                    </p>
                  </div>
                </div>

                <CashProjectionChart
                  days={data.days}
                  cashBuffer={data.cash_buffer}
                  lowestDate={data.lowest_date}
                  lowestAmount={data.lowest}
                />
              </Card>

              <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
                {/* Form Entri Kas Manual */}
                <Card className="p-5 lg:col-span-1">
                  <div className="flex items-center gap-2 border-b border-stone-200 pb-3">
                    <PlusCircle className="h-4 w-4 text-amber-700" />
                    <h2 className="text-sm font-bold text-stone-900">
                      Catat Entri Kas Manual
                    </h2>
                  </div>

                  <form
                    onSubmit={handleEntrySubmit}
                    className="mt-4 space-y-3.5"
                  >
                    {formError ? (
                      <div className="rounded-lg border border-rose-200 bg-rose-50 p-2.5 text-xs font-medium text-rose-800">
                        {formError}
                      </div>
                    ) : null}

                    <div>
                      <label className="mb-1 block text-xs font-semibold text-stone-700">
                        Jenis Arus Kas
                      </label>
                      <div className="grid grid-cols-2 gap-2">
                        <button
                          type="button"
                          onClick={() => {
                            setEntryDirection("in");
                            setEntryCategory("modal_tambahan");
                          }}
                          className={`flex items-center justify-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-semibold transition-colors ${
                            entryDirection === "in"
                              ? "border-emerald-600 bg-emerald-50 text-emerald-900"
                              : "border-stone-200 bg-white text-stone-600 hover:bg-stone-50"
                          }`}
                        >
                          <ArrowUpRight className="h-4 w-4 text-emerald-600" />
                          Kas Masuk (+)
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setEntryDirection("out");
                            setEntryCategory("operasional_lain");
                          }}
                          className={`flex items-center justify-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-semibold transition-colors ${
                            entryDirection === "out"
                              ? "border-rose-600 bg-rose-50 text-rose-900"
                              : "border-stone-200 bg-white text-stone-600 hover:bg-stone-50"
                          }`}
                        >
                          <ArrowDownRight className="h-4 w-4 text-rose-600" />
                          Kas Keluar (-)
                        </button>
                      </div>
                    </div>

                    <div>
                      <label
                        htmlFor="cash-entry-date"
                        className="mb-1 block text-xs font-semibold text-stone-700"
                      >
                        Tanggal Transaksi
                      </label>
                      <Input
                        id="cash-entry-date"
                        type="date"
                        value={entryDate}
                        onChange={(e) => setEntryDate(e.target.value)}
                        required
                      />
                    </div>

                    <div>
                      <label
                        htmlFor="cash-entry-amount"
                        className="mb-1 block text-xs font-semibold text-stone-700"
                      >
                        Nominal (Rp)
                      </label>
                      <Input
                        id="cash-entry-amount"
                        type="number"
                        step="1"
                        min="1"
                        value={entryAmount}
                        onChange={(e) => setEntryAmount(e.target.value)}
                        placeholder="Contoh: 500000"
                        required
                      />
                    </div>

                    <div>
                      <label
                        htmlFor="cash-entry-category"
                        className="mb-1 block text-xs font-semibold text-stone-700"
                      >
                        Kategori
                      </label>
                      <select
                        id="cash-entry-category"
                        value={entryCategory}
                        onChange={(e) => setEntryCategory(e.target.value)}
                        className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 focus:border-amber-600 focus:outline-none"
                      >
                        {entryDirection === "in" ? (
                          <>
                            <option value="modal_tambahan">
                              Suntikan Modal Kerja
                            </option>
                            <option value="pendapatan_lain">
                              Pendapatan Lain-lain
                            </option>
                          </>
                        ) : (
                          <>
                            <option value="operasional_lain">
                              Biaya Operasional Tambahan
                            </option>
                            <option value="peralatan">
                              Pembelian Peralatan / Kemasan
                            </option>
                            <option value="prive">Penarikan Pemilik</option>
                          </>
                        )}
                      </select>
                    </div>

                    <div>
                      <label
                        htmlFor="cash-entry-note"
                        className="mb-1 block text-xs font-semibold text-stone-700"
                      >
                        Catatan (Opsional)
                      </label>
                      <Input
                        id="cash-entry-note"
                        type="text"
                        value={entryNote}
                        onChange={(e) => setEntryNote(e.target.value)}
                        placeholder="Mis. Tambahan kas untuk belanja akhir pekan"
                      />
                    </div>

                    <Button
                      type="submit"
                      disabled={entryMutation.isPending}
                      className="w-full"
                    >
                      {entryMutation.isPending
                        ? "Menyimpan..."
                        : "Simpan Entri Kas"}
                    </Button>
                  </form>

                  {/* Riwayat Entri Kas Manual & Belanja */}
                  {data.entries && data.entries.length > 0 ? (
                    <div className="mt-6 border-t border-stone-200 pt-4">
                      <h3 className="text-xs font-bold uppercase tracking-wider text-stone-500">
                        Entri Kas Tercatat pada Rentang Ini (
                        {data.entries.length})
                      </h3>
                      <div className="mt-2 max-h-56 space-y-2 overflow-y-auto pr-1">
                        {data.entries.map((ent) => {
                          const amt = toNumber(ent.amount);
                          return (
                            <div
                              key={ent.id}
                              className="flex items-center justify-between gap-2 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-xs"
                            >
                              <div className="min-w-0">
                                <p className="truncate font-semibold text-stone-900">
                                  {ent.note || ent.category}
                                </p>
                                <p className="text-[11px] text-stone-500">
                                  {formatDateShort(ent.entry_date)} ·{" "}
                                  {ent.category}
                                </p>
                              </div>
                              <span
                                className={`shrink-0 font-bold ${
                                  amt >= 0
                                    ? "text-emerald-700"
                                    : "text-rose-700"
                                }`}
                              >
                                {amt >= 0 ? "+" : ""}
                                {formatRupiah(amt)}
                              </span>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  ) : null}
                </Card>

                {/* Tabel Rincian Saldo Harian */}
                <Card className="overflow-hidden lg:col-span-2">
                  <div className="border-b border-stone-200 bg-stone-50 px-4 py-3">
                    <h2 className="text-sm font-bold text-stone-900">
                      Tabel Rincian Proyeksi Arus Kas Harian
                    </h2>
                  </div>
                  <div className="max-h-[540px] overflow-auto">
                    <table className="w-full text-left text-xs">
                      <thead className="sticky top-0 border-b border-stone-200 bg-stone-100 text-[11px] font-semibold uppercase tracking-wider text-stone-600">
                        <tr>
                          <th className="px-4 py-2.5">Tanggal</th>
                          <th className="px-3 py-2.5 text-right">Kas Masuk</th>
                          <th className="px-3 py-2.5 text-right">
                            Belanja & Keluar
                          </th>
                          <th className="px-3 py-2.5 text-right">
                            Biaya Tetap
                          </th>
                          <th className="px-4 py-2.5 text-right">
                            Saldo Akhir
                          </th>
                          <th className="px-3 py-2.5 text-center">Status</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-stone-200 bg-white">
                        {data.days.map((d) => {
                          const isLowest = d.date === data.lowest_date;
                          return (
                            <tr
                              key={d.date}
                              className={
                                d.below_buffer
                                  ? "bg-rose-50/40"
                                  : isLowest
                                    ? "bg-amber-50/40"
                                    : "hover:bg-stone-50"
                              }
                            >
                              <td className="px-4 py-2.5 font-medium text-stone-900 whitespace-nowrap">
                                {formatDateShort(d.date)}
                                {isLowest ? (
                                  <span className="ml-1.5 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-bold text-amber-900">
                                    Terendah
                                  </span>
                                ) : null}
                              </td>
                              <td className="px-3 py-2.5 text-right font-medium text-emerald-700 whitespace-nowrap">
                                {toNumber(d.inflow) > 0
                                  ? `+${formatRupiah(d.inflow)}`
                                  : "-"}
                              </td>
                              <td className="px-3 py-2.5 text-right font-medium text-rose-700 whitespace-nowrap">
                                {toNumber(d.outflow) > 0
                                  ? `-${formatRupiah(d.outflow)}`
                                  : "-"}
                              </td>
                              <td className="px-3 py-2.5 text-right text-stone-600 whitespace-nowrap">
                                -{formatRupiah(d.fixed_cost)}
                              </td>
                              <td
                                className={`px-4 py-2.5 text-right font-bold whitespace-nowrap ${
                                  d.below_buffer
                                    ? "text-rose-700"
                                    : "text-stone-900"
                                }`}
                              >
                                {formatRupiah(d.closing)}
                              </td>
                              <td className="px-3 py-2.5 text-center whitespace-nowrap">
                                {d.below_buffer ? (
                                  <Badge variant="rose" className="text-[10px]">
                                    Rawan
                                  </Badge>
                                ) : (
                                  <Badge
                                    variant="emerald"
                                    className="text-[10px]"
                                  >
                                    Aman
                                  </Badge>
                                )}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                </Card>
              </div>
            </>
          ) : null}
        </>
      )}
    </div>
  );
}
