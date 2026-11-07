"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  RefreshCw,
  Save,
  Settings,
  ShieldCheck,
} from "lucide-react";
import { getSettings, updateSettings, getErrorMessage } from "@/lib/api";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

interface SettingFieldDef {
  key: string;
  label: string;
  helper: string;
  step?: string;
  min?: string;
  max?: string;
}

const SETTING_GROUPS: {
  title: string;
  description: string;
  fields: SettingFieldDef[];
}[] = [
  {
    title: "Gerbang Kapasitas & Biaya Tenaga (G1)",
    description:
      "Batas rasio utilisasi harian sumber daya (0.00 – 1.00) dan tarif tenaga kerja per menit.",
    fields: [
      {
        key: "capacity_yellow",
        label: "Ambang Kuning Kapasitas (Rasio)",
        helper: "Contoh: 0.75 (75% utilisasi mulai ditandai kuning)",
        step: "0.01",
        min: "0.01",
        max: "1",
      },
      {
        key: "capacity_red",
        label: "Ambang Merah Kapasitas (Rasio)",
        helper: "Contoh: 0.90 (90% utilisasi ditandai padat/merah)",
        step: "0.01",
        min: "0.01",
        max: "1",
      },
      {
        key: "capacity_hard_limit",
        label: "Batas Keras Kapasitas G1 (Rasio)",
        helper: "Contoh: 1.00 (100% kapasitas harian maksimum)",
        step: "0.01",
        min: "0.01",
        max: "1.5",
      },
      {
        key: "labor_rate_per_minute",
        label: "Tarif Tenaga Kerja per Menit (Rp)",
        helper: "Contoh: 400 (dipakai oleh engine.UnitCost)",
        step: "1",
        min: "0",
      },
    ],
  },
  {
    title: "Gerbang Margin & Harga (G2)",
    description:
      "Batas margin kotor minimum untuk meloloskan pesanan dan target margin usaha.",
    fields: [
      {
        key: "min_margin",
        label: "Margin Minimum G2 (Rasio)",
        helper: "Contoh: 0.20 (20% margin minimum)",
        step: "0.01",
        min: "0",
        max: "0.99",
      },
      {
        key: "target_margin",
        label: "Target Margin Ideal (Rasio)",
        helper: "Contoh: 0.25 (25% target margin kotor)",
        step: "0.01",
        min: "0",
        max: "0.99",
      },
    ],
  },
  {
    title: "Gerbang Kas & Aturan DP (G3)",
    description:
      "Saldo kas awal, penyangga kas minimum, biaya tetap harian, dan persentase uang muka (DP).",
    fields: [
      {
        key: "opening_cash",
        label: "Saldo Kas Awal (Rp)",
        helper: "Saldo kas dasar pada awal periode proyeksi",
        step: "1",
        min: "0",
      },
      {
        key: "cash_buffer",
        label: "Penyangga Kas Minimum G3 (Rp)",
        helper: "Titik kas terendah tidak boleh turun di bawah angka ini",
        step: "1",
        min: "0",
      },
      {
        key: "fixed_cost_daily",
        label: "Biaya Tetap Harian (Rp/hari)",
        helper: "Beban operasional rutin harian (listrik, gas dasar, dll.)",
        step: "1",
        min: "0",
      },
      {
        key: "dp_pct",
        label: "Persentase DP Standar (Rasio)",
        helper: "Contoh: 0.30 (30% dari nilai pesanan)",
        step: "0.01",
        min: "0",
        max: "1",
      },
      {
        key: "dp_deadline_hours",
        label: "Tenggat Pembayaran DP (Jam)",
        helper: "Contoh: 24 (reservasi kapasitas hangus bila DP lewat tenggat)",
        step: "1",
        min: "1",
      },
    ],
  },
  {
    title: "Ambang Pesanan Besar, Belanja & Pelanggan",
    description:
      "Batas kewenangan persetujuan Manajer vs Owner, buffer belanja bahan, dan batas pesanan belum bayar.",
    fields: [
      {
        key: "big_order_amount",
        label: "Ambang Nominal Pesanan Besar (Rp)",
        helper: "Pesanan >= nilai ini wajib diputuskan oleh Owner",
        step: "1",
        min: "1",
      },
      {
        key: "big_order_units",
        label: "Ambang Jumlah Unit Pesanan Besar (Porsi/Box)",
        helper: "Pesanan >= jumlah unit ini wajib diputuskan oleh Owner",
        step: "1",
        min: "1",
      },
      {
        key: "max_active_unpaid_orders",
        label: "Batas Pesanan Aktif Belum DP per Pelanggan",
        helper: "Contoh: 2 (mencegah blokir kapasitas oleh pesanan fiktif)",
        step: "1",
        min: "1",
      },
      {
        key: "shopping_buffer_pct",
        label: "Buffer Belanja Bahan (Rasio)",
        helper: "Contoh: 0.10 (cadangan stok 10% di daftar belanja)",
        step: "0.01",
        min: "0",
        max: "1",
      },
      {
        key: "shopping_lead_days",
        label: "Lead Time Belanja Bahan (Hari sebelum Acara)",
        helper: "Contoh: 1 (belanja dijadwalkan H-1 sebelum acara)",
        step: "1",
        min: "0",
      },
      {
        key: "counter_horizon_days",
        label: "Horizon Pencarian Tanggal Tawaran Balik (Hari)",
        helper: "Contoh: 7 (mencari tanggal pengganti hingga +7 hari)",
        step: "1",
        min: "1",
      },
    ],
  },
];

export default function PengaturanPage() {
  const toast = useToast();
  const queryClient = useQueryClient();

  const [formValues, setFormValues] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);

  const settingsQuery = useQuery({
    queryKey: ["settings"],
    queryFn: ({ signal }) => getSettings(signal),
  });

  useEffect(() => {
    if (settingsQuery.data) {
      setFormValues(settingsQuery.data);
    }
  }, [settingsQuery.data]);

  const saveMutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: (updated) => {
      setFormValues(updated);
      setFormError(null);
      toast.success(
        "Parameter Usaha Disimpan",
        "Seluruh ambang gerbang (G1–G4) dan parameter kas telah diperbarui."
      );
      queryClient.invalidateQueries({ queryKey: ["settings"] });
      queryClient.invalidateQueries({ queryKey: ["cash-projection"] });
      queryClient.invalidateQueries({ queryKey: ["costing-products"] });
      queryClient.invalidateQueries({ queryKey: ["shopping-list"] });
    },
    onError: (err) => {
      const msg = getErrorMessage(err);
      setFormError(msg);
      toast.error("Gagal Menyimpan Pengaturan", msg);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    const yellow = Number.parseFloat(formValues.capacity_yellow ?? "0.75");
    const red = Number.parseFloat(formValues.capacity_red ?? "0.90");
    const hard = Number.parseFloat(formValues.capacity_hard_limit ?? "1.00");
    if (yellow >= red || red > hard) {
      setFormError(
        "Urutan ambang kapasitas harus memenuhi: Ambang Kuning < Ambang Merah <= Batas Keras."
      );
      return;
    }

    const minMargin = Number.parseFloat(formValues.min_margin ?? "0.20");
    const targetMargin = Number.parseFloat(formValues.target_margin ?? "0.25");
    if (minMargin > targetMargin) {
      setFormError(
        "Margin Minimum (G2) tidak boleh lebih besar dari Target Margin Ideal."
      );
      return;
    }

    saveMutation.mutate(formValues);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-stone-900">
            Parameter Usaha & Ambang Gerbang (G1–G4)
          </h1>
          <p className="text-sm text-stone-600">
            Konfigurasi pusat untuk evaluasi kelayakan pesanan otomatis,
            proyeksi kas, perhitungan HPP, dan batas reservasi pelanggan.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => settingsQuery.refetch()}
          disabled={settingsQuery.isFetching}
          className="gap-1.5 self-start"
        >
          <RefreshCw
            className={`h-4 w-4 ${settingsQuery.isFetching ? "animate-spin" : ""}`}
          />
          Muat Ulang
        </Button>
      </div>

      {settingsQuery.isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-48 w-full rounded-xl" />
          <Skeleton className="h-48 w-full rounded-xl" />
        </div>
      ) : settingsQuery.isError ? (
        <Card className="border-rose-200 bg-rose-50 p-6 text-sm text-rose-800">
          Gagal memuat parameter usaha: {getErrorMessage(settingsQuery.error)}
        </Card>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-6">
          {formError ? (
            <Card className="border-rose-300 bg-rose-50 p-4 text-xs font-medium text-rose-900">
              {formError}
            </Card>
          ) : null}

          {SETTING_GROUPS.map((group) => (
            <Card key={group.title} className="p-5 space-y-4">
              <div className="border-b border-stone-200 pb-3">
                <div className="flex items-center gap-2">
                  <Settings className="h-4 w-4 text-amber-700" />
                  <h2 className="text-base font-bold text-stone-900">
                    {group.title}
                  </h2>
                </div>
                <p className="mt-0.5 text-xs text-stone-500">
                  {group.description}
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {group.fields.map((field) => (
                  <div key={field.key} className="space-y-1">
                    <label
                      htmlFor={`setting-${field.key}`}
                      className="block text-xs font-semibold text-stone-800"
                    >
                      {field.label}
                    </label>
                    <Input
                      id={`setting-${field.key}`}
                      type="number"
                      step={field.step ?? "any"}
                      min={field.min}
                      max={field.max}
                      value={formValues[field.key] ?? ""}
                      onChange={(e) => {
                        const val = e.target.value;
                        setFormValues((prev) => ({
                          ...prev,
                          [field.key]: val,
                        }));
                      }}
                      required
                    />
                    <p className="text-[11px] text-stone-500">{field.helper}</p>
                  </div>
                ))}
              </div>
            </Card>
          ))}

          <div className="flex items-center justify-between rounded-xl border border-stone-200 bg-white p-4 shadow-xs">
            <div className="flex items-center gap-2 text-xs text-stone-600">
              <ShieldCheck className="h-4 w-4 text-emerald-700" />
              <span>
                Setiap perubahan parameter dicatat otomatis di{" "}
                <strong>audit_logs</strong>.
              </span>
            </div>

            <Button
              type="submit"
              disabled={saveMutation.isPending}
              className="gap-1.5"
            >
              {saveMutation.isPending ? (
                "Menyimpan..."
              ) : (
                <>
                  <Save className="h-4 w-4" />
                  Simpan Seluruh Parameter
                </>
              )}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
