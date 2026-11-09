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
  readOnly?: boolean;
  badge?: string;
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
        key: "status_green_below",
        label: "Ambang Hijau Kapasitas (Rasio)",
        helper: "Contoh: 0.70 (utilisasi di bawah 70% ditandai hijau/longgar)",
        step: "0.01",
        min: "0.01",
        max: "1",
      },
      {
        key: "status_yellow_below",
        label: "Ambang Kuning Kapasitas (Rasio)",
        helper: "Contoh: 0.90 (utilisasi 70% s/d 90% ditandai kuning/padat)",
        step: "0.01",
        min: "0.01",
        max: "1",
      },
      {
        key: "capacity_limit",
        label: "Batas Terima Kapasitas G1 (Rasio)",
        helper: "Contoh: 0.90 (90% batas maksimum pesanan diterima otomatis)",
        step: "0.01",
        min: "0.01",
        max: "1.5",
      },
      {
        key: "labor_rate_per_minute",
        label: "Tarif Tenaga Kerja per Menit (Rp)",
        helper: "Contoh: 400 (dipakai oleh kalkulasi HPP engine.UnitCost)",
        step: "1",
        min: "0",
      },
    ],
  },
  {
    title: "Gerbang Margin & Harga (G2)",
    description:
      "Batas margin kotor minimum untuk meloloskan pesanan katering.",
    fields: [
      {
        key: "margin_min",
        label: "Margin Minimum G2 (Rasio)",
        helper: "Contoh: 0.20 (20% margin kotor minimum kelayakan pesanan)",
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
        key: "big_order_total",
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
        key: "lead_time_days",
        label: "Lead Time Minimum Pesanan (Hari sebelum Acara)",
        helper: "Contoh: 2 (pesanan minimal dipesan H-2 sebelum acara)",
        step: "1",
        min: "0",
      },
    ],
  },
  {
    title: "Pajak & Ketentuan Transaksi",
    description:
      "Tarif Pajak Pertambahan Nilai (PPN) dan Biaya Layanan Aplikasi yang dikenakan pada pesanan pelanggan.",
    fields: [
      {
        key: "tax_pct",
        label: "Tarif PPN (Rasio Desimal)",
        helper: "Contoh: 0.11 untuk PPN 11%, atau 0.00 jika katering non-PKP / bebas pajak",
        step: "0.01",
        min: "0",
        max: "1",
      },
      {
        key: "platform_fee",
        label: "Biaya Layanan Aplikasi (Rp)",
        helper:
          "Dikelola oleh Administrator Sistem. Pemilik usaha dapat memantau besaran tarif aktif saat ini.",
        step: "100",
        min: "0",
        readOnly: true,
        badge: "Dikelola Admin Platform",
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

  const parseNum = (val: string | undefined, defaultVal: number): number => {
    if (!val) return defaultVal;
    const cleaned = String(val).replace(",", ".");
    const parsed = Number.parseFloat(cleaned);
    return Number.isNaN(parsed) ? defaultVal : parsed;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);

    const green = parseNum(formValues.status_green_below, 0.70);
    const yellow = parseNum(formValues.status_yellow_below, 0.90);
    if (green >= yellow) {
      setFormError(
        "Ambang Hijau harus lebih kecil dari Ambang Kuning Kapasitas."
      );
      return;
    }

    // Hanya kirim field yang resmi didukung backend
    const allowedKeys = [
      "status_green_below",
      "status_yellow_below",
      "capacity_limit",
      "labor_rate_per_minute",
      "margin_min",
      "opening_cash",
      "cash_buffer",
      "fixed_cost_daily",
      "dp_pct",
      "dp_deadline_hours",
      "big_order_total",
      "big_order_units",
      "max_active_unpaid_orders",
      "shopping_buffer_pct",
      "shopping_lead_days",
      "lead_time_days",
      "tax_pct",
    ];

    const payload: Record<string, string | number> = {};
    for (const key of allowedKeys) {
      const v = formValues[key];
      if (v === undefined || v === null) continue;
      payload[key] = String(v).trim().replace(",", ".");
    }
    if (payload.big_order_units) {
      payload.big_order_units = Math.round(parseNum(String(payload.big_order_units), 50));
    }
    if (payload.lead_time_days) {
      payload.lead_time_days = Math.round(parseNum(String(payload.lead_time_days), 2));
    }
    if (payload.shopping_lead_days) {
      payload.shopping_lead_days = Math.round(parseNum(String(payload.shopping_lead_days), 2));
    }
    if (payload.dp_deadline_hours) {
      payload.dp_deadline_hours = Math.round(parseNum(String(payload.dp_deadline_hours), 24));
    }
    if (payload.max_active_unpaid_orders) {
      payload.max_active_unpaid_orders = Math.round(parseNum(String(payload.max_active_unpaid_orders), 2));
    }

    saveMutation.mutate(payload);
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
                    <div className="flex items-center justify-between gap-1">
                      <label
                        htmlFor={`setting-${field.key}`}
                        className="block text-xs font-semibold text-stone-800"
                      >
                        {field.label}
                      </label>
                      {field.badge && (
                        <span className="rounded-md border border-stone-200 bg-stone-100 px-1.5 py-0.5 text-[10px] font-medium text-stone-600">
                          {field.badge}
                        </span>
                      )}
                    </div>
                    <Input
                      id={`setting-${field.key}`}
                      type="number"
                      step={field.step ?? "any"}
                      min={field.min}
                      max={field.max}
                      value={formValues[field.key] ?? ""}
                      disabled={field.readOnly}
                      className={
                        field.readOnly
                          ? "cursor-not-allowed border-stone-200 bg-stone-50 text-stone-600"
                          : ""
                      }
                      onChange={(e) => {
                        if (field.readOnly) return;
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

          <div className="flex flex-col gap-3 rounded-xl border border-stone-200 bg-white p-4 shadow-xs sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-2 text-xs text-stone-600">
              <ShieldCheck className="h-4 w-4 shrink-0 text-emerald-700" />
              <span>
                Setiap perubahan parameter dicatat otomatis di{" "}
                <strong>audit_logs</strong>.
              </span>
            </div>

            <Button
              type="submit"
              disabled={saveMutation.isPending}
              className="w-full gap-1.5 sm:w-auto"
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
