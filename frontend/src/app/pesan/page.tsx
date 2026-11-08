"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useForm, useFieldArray, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import {
  AlertCircle,
  ArrowRight,
  Calendar as CalendarIcon,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  Copy,
  HelpCircle,
  Info,
  Loader2,
  LogIn,
  Plus,
  ShoppingBag,
  Trash2,
  UserCheck,
  UserPlus,
} from "lucide-react";
import {
  getPublicProducts,
  getPublicConfig,
  checkPublicAvailability,
  createPublicOrder,
  getMe,
  ApiError,
  getErrorMessage,
} from "@/lib/api";
import type { AvailabilityDay, PublicOrderSummary } from "@/lib/types";
import {
  addDaysDateKey,
  formatCategoryName,
  formatDateLong,
  formatDateShort,
  formatDateTimeWIB,
  formatPercent,
  formatRupiah,
  getOrderStatusMeta,
  toDateKeyWIB,
  toNumber,
} from "@/lib/format";
import { PublicNavbar } from "@/components/public-navbar";
import { useToast } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/ui/states";
import { cn } from "@/lib/utils";

const orderItemSchema = z.object({
  product_id: z.string().min(1, "Pilih produk terlebih dahulu."),
  qty: z.coerce
    .number({
      invalid_type_error: "Jumlah harus berupa angka.",
    })
    .int("Jumlah harus bilangan bulat.")
    .min(1, "Jumlah minimal 1 unit.")
    .max(100000, "Jumlah maksimal 100.000 unit."),
});

const createOrderFormSchema = z
  .object({
    items: z
      .array(orderItemSchema)
      .min(1, "Minimal pilih satu produk untuk dipesan.")
      .max(50, "Maksimal 50 baris item dalam satu pesanan."),
    event_date: z
      .string()
      .regex(/^\d{4}-\d{2}-\d{2}$/, "Pilih tanggal acara yang tersedia."),
    customer_name: z
      .string()
      .trim()
      .min(1, "Nama lengkap pelanggan wajib diisi.")
      .max(120, "Nama pelanggan maksimal 120 karakter."),
    customer_contact: z
      .string()
      .trim()
      .min(6, "Nomor WhatsApp / telepon wajib diisi (minimal 6 karakter).")
      .max(80, "Kontak pelanggan maksimal 80 karakter."),
    delivery_type: z.enum(["pickup", "delivery"], {
      required_error: "Pilih metode pengambilan pesanan.",
    }),
    address: z
      .string()
      .max(500, "Alamat maksimal 500 karakter.")
      .optional(),
    notes: z
      .string()
      .max(1000, "Catatan maksimal 1.000 karakter.")
      .optional(),
  })
  .superRefine((val, ctx) => {
    if (val.delivery_type === "delivery") {
      if (!val.address || val.address.trim().length < 5) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["address"],
          message:
            "Alamat pengantaran lengkap wajib diisi untuk pesanan diantar.",
        });
      }
    }
  });

type CreateOrderFormValues = z.infer<typeof createOrderFormSchema>;

const RECOMMENDATION_LABELS: Record<
  string,
  { label: string; description: string; variant: "emerald" | "amber" | "rose" }
> = {
  accept: {
    label: "Rekomendasi Sistem: Layak Diterima",
    description:
      "Kapasitas dapur, margin, arus kas, dan waktu persiapan memenuhi syarat.",
    variant: "emerald",
  },
  accept_with_conditions: {
    label: "Rekomendasi Sistem: Perlu Penyesuaian / Tinjauan",
    description:
      "Pesanan memerlukan konfirmasi Manajer/Pemilik atau penyesuaian tanggal.",
    variant: "amber",
  },
  reject: {
    label: "Rekomendasi Sistem: Tidak Memenuhi Syarat Otomatis",
    description:
      "Pesanan diteruskan ke antrian keputusan untuk ditinjau secara manual oleh pihak usaha.",
    variant: "rose",
  },
};

function OrderFormContent() {
  const searchParams = useSearchParams();
  const preselectedProductId = searchParams.get("product") ?? "";
  const toast = useToast();

  const [windowStart, setWindowStart] = useState<string>(() => {
    // Mulai dari tanggal hari ini di WIB (atau minimal 2026-11-08 agar mencakup demo_today 2026-11-10)
    const nowKey = toDateKeyWIB(new Date());
    return nowKey < "2026-11-08" ? "2026-11-08" : nowKey;
  });
  const windowEnd = useMemo(
    () => addDaysDateKey(windowStart, 20),
    [windowStart]
  );

  const [submittedOrder, setSubmittedOrder] =
    useState<PublicOrderSummary | null>(null);
  const [serverFieldErrors, setServerFieldErrors] = useState<string[]>([]);
  const [copiedCode, setCopiedCode] = useState(false);

  const productsQuery = useQuery({
    queryKey: ["public-products"],
    queryFn: ({ signal }) => getPublicProducts(signal),
  });

  const publicConfigQuery = useQuery({
    queryKey: ["public-config"],
    queryFn: ({ signal }) => getPublicConfig(signal),
    staleTime: 60_000,
  });

  const meQuery = useQuery({
    queryKey: ["auth-me"],
    queryFn: ({ signal }) => getMe(signal),
    retry: false,
  });

  const isCustomerLoggedIn = meQuery.data?.role === "customer";
  const nextUrlParam = preselectedProductId
    ? `/pesan?product=${encodeURIComponent(preselectedProductId)}`
    : "/pesan";

  const form = useForm<CreateOrderFormValues>({
    resolver: zodResolver(createOrderFormSchema),
    defaultValues: {
      items: [{ product_id: preselectedProductId, qty: 10 }],
      event_date: "",
      customer_name: "",
      customer_contact: "",
      delivery_type: "pickup",
      address: "",
      notes: "",
    },
  });

  // Isi otomatis nama dan kontak dari profil pelanggan bila sudah masuk sebagai customer (tetap dapat diedit)
  useEffect(() => {
    if (meQuery.data?.role !== "customer") return;
    const currentName = form.getValues("customer_name");
    const currentContact = form.getValues("customer_contact");
    if (!currentName && meQuery.data.name) {
      form.setValue("customer_name", meQuery.data.name, {
        shouldValidate: true,
      });
    }
    if (!currentContact && meQuery.data.contact) {
      form.setValue("customer_contact", meQuery.data.contact, {
        shouldValidate: true,
      });
    }
  }, [meQuery.data, form]);

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "items",
  });

  // Saat daftar produk selesai dimuat dan belum ada product_id terpilih, isi dengan produk pertama
  useEffect(() => {
    const products = productsQuery.data;
    if (!products || products.length === 0) return;
    const currentItems = form.getValues("items");
    if (currentItems.length > 0 && !currentItems[0].product_id) {
      const match =
        products.find((p) => p.id === preselectedProductId) ?? products[0];
      form.setValue("items.0.product_id", match.id, { shouldValidate: true });
    }
  }, [productsQuery.data, preselectedProductId, form]);

  const watchedItems = useWatch({ control: form.control, name: "items" });
  const watchedDeliveryType = useWatch({
    control: form.control,
    name: "delivery_type",
  });
  const watchedEventDate = useWatch({
    control: form.control,
    name: "event_date",
  });

  // Normalisasi item yang valid untuk pemanggilan /public/availability
  const validAvailabilityItems = useMemo(() => {
    if (!watchedItems) return [];
    const map = new Map<string, number>();
    for (const item of watchedItems) {
      const pid = (item?.product_id ?? "").trim();
      const qty = Number(item?.qty ?? 0);
      if (pid && Number.isInteger(qty) && qty >= 1 && qty <= 100000) {
        map.set(pid, (map.get(pid) ?? 0) + qty);
      }
    }
    return Array.from(map.entries()).map(([product_id, qty]) => ({
      product_id,
      qty,
    }));
  }, [watchedItems]);

  // Debounce 350ms untuk payload pengecekan ketersediaan tanggal
  const [debouncedItemsKey, setDebouncedItemsKey] = useState<string>("");
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedItemsKey(JSON.stringify(validAvailabilityItems));
    }, 350);
    return () => clearTimeout(timer);
  }, [validAvailabilityItems]);

  const debouncedItems = useMemo(() => {
    if (!debouncedItemsKey) return [];
    try {
      return JSON.parse(debouncedItemsKey) as {
        product_id: string;
        qty: number;
      }[];
    } catch {
      return [];
    }
  }, [debouncedItemsKey]);

  const availabilityQuery = useQuery({
    queryKey: [
      "public-availability",
      debouncedItemsKey,
      windowStart,
      windowEnd,
    ],
    queryFn: ({ signal }) =>
      checkPublicAvailability(
        {
          items: debouncedItems,
          from: windowStart,
          to: windowEnd,
        },
        signal
      ),
    enabled: debouncedItems.length > 0,
  });

  // Bila tanggal yang sedang dipilih berubah menjadi tidak tersedia ketika jumlah pesanan diubah, kosongkan pilihan
  useEffect(() => {
    if (!watchedEventDate || !availabilityQuery.data) return;
    const found = availabilityQuery.data.find(
      (d) => d.date === watchedEventDate
    );
    if (found && !found.available) {
      form.setValue("event_date", "", { shouldValidate: true });
    }
  }, [availabilityQuery.data, watchedEventDate, form]);

  // Peta produk untuk menampilkan nama & menghitung estimasi total dari harga katalog
  const productMap = useMemo(() => {
    const map = new Map<
      string,
      { name: string; category: string; price: number }
    >();
    for (const p of productsQuery.data ?? []) {
      map.set(p.id, {
        name: p.name,
        category: p.category,
        price: toNumber(p.price),
      });
    }
    return map;
  }, [productsQuery.data]);

  const taxRate = useMemo(() => {
    return toNumber(publicConfigQuery.data?.tax_pct ?? 0);
  }, [publicConfigQuery.data]);

  const estimatedSubtotal = useMemo(() => {
    let sum = 0;
    for (const it of watchedItems ?? []) {
      const prod = productMap.get(it?.product_id ?? "");
      const qty = Number(it?.qty ?? 0);
      if (prod && qty > 0) {
        sum += prod.price * qty;
      }
    }
    return sum;
  }, [watchedItems, productMap]);

  const estimatedTax = useMemo(() => {
    if (taxRate <= 0) return 0;
    return Math.round(estimatedSubtotal * taxRate);
  }, [estimatedSubtotal, taxRate]);

  const estimatedTotal = useMemo(() => {
    return estimatedSubtotal + estimatedTax;
  }, [estimatedSubtotal, estimatedTax]);

  const totalUnits = useMemo(() => {
    let count = 0;
    for (const it of watchedItems ?? []) {
      const qty = Number(it?.qty ?? 0);
      if (qty > 0) count += qty;
    }
    return count;
  }, [watchedItems]);

  const selectedDayInfo = useMemo(() => {
    if (!watchedEventDate || !availabilityQuery.data) return null;
    return (
      availabilityQuery.data.find((d) => d.date === watchedEventDate) ?? null
    );
  }, [watchedEventDate, availabilityQuery.data]);

  const isLargeOrder = estimatedTotal >= 2_000_000 || totalUnits >= 50;
  const isSelectedDateDense = selectedDayInfo
    ? toNumber(selectedDayInfo.utilization_after) >= 0.7
    : false;

  const createOrderMutation = useMutation({
    mutationFn: createPublicOrder,
    onSuccess: (summary) => {
      setServerFieldErrors([]);
      setSubmittedOrder(summary);
      toast.success(
        "Pesanan Berhasil Dikirim",
        `Kode pesanan Anda: ${summary.code}`
      );
      window.scrollTo({ top: 0, behavior: "smooth" });
    },
    onError: (err) => {
      if (err instanceof ApiError && err.isValidationError) {
        setServerFieldErrors(err.fieldErrors);
      } else {
        setServerFieldErrors([getErrorMessage(err)]);
      }
      toast.error("Gagal Mengirim Pesanan", getErrorMessage(err));
    },
  });

  const onSubmit = (values: CreateOrderFormValues) => {
    setServerFieldErrors([]);
    createOrderMutation.mutate({
      customer_name: values.customer_name.trim(),
      customer_contact: values.customer_contact.trim(),
      delivery_type: values.delivery_type,
      address:
        values.delivery_type === "delivery"
          ? values.address?.trim() ?? ""
          : "",
      notes: values.notes?.trim() ?? "",
      event_date: values.event_date,
      items: values.items.map((it) => ({
        product_id: it.product_id,
        qty: Number(it.qty),
      })),
    });
  };

  const handleCopyCode = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      setCopiedCode(true);
      toast.info("Kode Disalin", `Kode pesanan ${code} disalin ke clipboard.`);
      setTimeout(() => setCopiedCode(false), 2000);
    } catch {
      // Abaikan bila clipboard ditolak browser
    }
  };

  // Tampilan Hasil Pesanan setelah berhasil dikirim
  if (submittedOrder) {
    const statusMeta = getOrderStatusMeta(submittedOrder.status);
    const recMeta = RECOMMENDATION_LABELS[submittedOrder.recommendation] ?? {
      label: `Rekomendasi: ${submittedOrder.recommendation}`,
      description: "",
      variant: "amber" as const,
    };
    const submittedUnits = submittedOrder.items.reduce(
      (sum, it) => sum + it.qty,
      0
    );
    const submittedTotalNum = toNumber(submittedOrder.total);
    const isSubmittedLarge =
      submittedTotalNum >= 2_000_000 || submittedUnits >= 50;

    return (
      <div className="mx-auto max-w-3xl space-y-6">
        <Card className="border-emerald-300 bg-white shadow-sm">
          <CardHeader className="border-b border-stone-100 bg-emerald-50/60 pb-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Badge variant={statusMeta.tone} className="px-3 py-1 text-xs">
                {statusMeta.label}
              </Badge>
              <Badge variant={recMeta.variant} className="px-3 py-1 text-xs">
                {recMeta.label}
              </Badge>
            </div>
            <div className="pt-2 flex items-center gap-2.5 text-emerald-950">
              <CheckCircle2 className="h-6 w-6 shrink-0 text-emerald-600" />
              <CardTitle className="text-xl sm:text-2xl">
                Pesanan Anda Telah Tercatat!
              </CardTitle>
            </div>
            <CardDescription className="text-stone-700">
              Simpan kode pesanan di bawah ini atau buka menu{" "}
              <strong>Pesanan Saya</strong> untuk memantau progres, melihat
              tagihan DP, dan merespons tawaran balik.
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-5 pt-5">
            {/* Kotak Kode Pesanan */}
            <div className="flex flex-col items-center justify-between gap-3 rounded-xl border border-amber-200 bg-amber-50/70 p-4 sm:flex-row">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wider text-amber-800">
                  Kode Lacak Pesanan Anda
                </p>
                <p className="mt-0.5 font-mono text-2xl font-bold tracking-widest text-stone-900">
                  {submittedOrder.code}
                </p>
              </div>
              <div className="flex w-full flex-wrap gap-2 sm:w-auto">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => handleCopyCode(submittedOrder.code)}
                  className="flex-1 sm:flex-initial"
                >
                  <Copy className="h-3.5 w-3.5" />
                  {copiedCode ? "Tersalin" : "Salin Kode"}
                </Button>
                <Link
                  href={`/lacak/${encodeURIComponent(submittedOrder.code)}`}
                  className="flex-1 sm:flex-initial"
                >
                  <Button size="sm" className="w-full gap-1.5">
                    Lacak Pesanan
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            </div>

            {/* Penjelasan Status dengan Bahasa Awam */}
            <div className="rounded-xl border border-stone-200 bg-stone-50 p-4 text-xs leading-relaxed text-stone-700 sm:text-sm space-y-2">
              <p className="font-semibold text-stone-900">{statusMeta.label}</p>
              {submittedOrder.status === "pending_decision" ? (
                <div className="space-y-1.5 text-stone-700">
                  <p>
                    <strong>
                      Mengapa pesanan saya berstatus &ldquo;Menunggu Konfirmasi
                      Usaha&rdquo; dan belum langsung diterima?
                    </strong>
                  </p>
                  <p>
                    {isSubmittedLarge ? (
                      <>
                        Pesanan Anda berjumlah cukup besar (
                        <strong>{submittedUnits} unit</strong> senilai{" "}
                        <strong>{formatRupiah(submittedOrder.total)}</strong>,
                        melewati ambang pesanan besar 50 unit atau Rp 2.000.000).
                        Untuk pesanan skala ini, Pemilik/Manajer perlu memastikan
                        ketersediaan bahan baku segar dan kesiapan tim dapur
                        terlebih dahulu sebelum meminta Anda membayar uang muka
                        (DP).
                      </>
                    ) : (
                      <>
                        Pada tanggal acara yang Anda pilih, jadwal produksi dapur
                        cukup padat atau memerlukan pengecekan kesiapan bahan
                        baku dan waktu persiapan oleh Manajer/Pemilik kami. Oleh
                        karena itu, pesanan Anda tidak ditolak, melainkan
                        diteruskan ke tim kami untuk dikonfirmasi terlebih
                        dahulu.
                      </>
                    )}
                  </p>
                  <p className="text-xs text-stone-600">
                    <strong>Langkah berikutnya:</strong> Anda{" "}
                    <u>belum perlu mentransfer DP sekarang</u>. Begitu tim kami
                    menyetujui jadwalnya, status pesanan akan berubah menjadi{" "}
                    <strong>Disetujui — Menunggu DP</strong> (atau kami akan
                    mengirimkan saran tanggal alternatif jika jadwal penuh).
                  </p>
                </div>
              ) : (
                <div className="space-y-1 text-stone-700">
                  <p>
                    Jadwal dapur dan ketersediaan bahan pada tanggal pilihan Anda
                    mencukupi, sehingga pesanan{" "}
                    <strong>langsung disetujui secara otomatis</strong> dan slot
                    produksi Anda sudah kami amankan sementara.
                  </p>
                  <p className="text-xs text-stone-600">
                    Silakan selesaikan pembayaran uang muka (DP) sebelum tenggat
                    waktu berakhir agar pesanan masuk ke jadwal produksi tetap.
                  </p>
                </div>
              )}
            </div>

            {/* Rincian Tanggal, Total, DP, Tenggat */}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <div className="rounded-xl border border-stone-200 p-3.5">
                <p className="text-xs text-stone-500">Tanggal Acara</p>
                <p className="mt-0.5 text-sm font-semibold text-stone-900">
                  {formatDateLong(submittedOrder.event_date)}
                </p>
              </div>
              <div className="rounded-xl border border-stone-200 p-3.5">
                <p className="text-xs text-stone-500">Total Pesanan</p>
                <p className="mt-0.5 text-sm font-bold text-stone-900">
                  {formatRupiah(submittedOrder.total)}
                </p>
              </div>
              <div className="rounded-xl border border-stone-200 p-3.5">
                <p className="text-xs text-stone-500">Tagihan Uang Muka (DP)</p>
                <p className="mt-0.5 text-sm font-bold text-amber-800">
                  {formatRupiah(submittedOrder.dp_required)}
                </p>
                {submittedOrder.dp_deadline ? (
                  <p className="mt-0.5 text-[11px] text-stone-500">
                    Tenggat: {formatDateTimeWIB(submittedOrder.dp_deadline)}
                  </p>
                ) : null}
              </div>
            </div>

            {/* Pratinjau Counter Offer bila sistem menyarankan tanggal alternatif */}
            {submittedOrder.counter_offer ? (
              <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-4 text-xs sm:text-sm">
                <p className="font-semibold text-amber-950">
                  Usulan Jadwal Alternatif Sistem:
                </p>
                <p className="mt-0.5 text-amber-900">
                  Kapasitas terdekat yang tersedia berada pada{" "}
                  <strong>
                    {formatDateLong(submittedOrder.counter_offer.date)}
                  </strong>{" "}
                  dengan estimasi DP{" "}
                  <strong>
                    {formatRupiah(submittedOrder.counter_offer.dp)}
                  </strong>
                  . Manajer kami akan segera meninjau dan mengirimkan konfirmasi
                  tawaran balik yang dapat Anda setujui di halaman Lacak
                  Pesanan.
                </p>
              </div>
            ) : null}

            {/* Daftar Item */}
            <div className="divide-y divide-stone-200 rounded-xl border border-stone-200">
              {submittedOrder.items.map((it) => (
                <div
                  key={it.product_id}
                  className="flex items-center justify-between gap-3 p-3.5 text-sm"
                >
                  <div>
                    <p className="font-medium text-stone-900">
                      {it.product_name}
                    </p>
                    <p className="text-xs text-stone-500">
                      {it.qty} unit × {formatRupiah(it.unit_price)}
                    </p>
                  </div>
                  <p className="font-semibold text-stone-900">
                    {formatRupiah(it.subtotal)}
                  </p>
                </div>
              ))}
            </div>

            <div className="flex flex-col justify-between gap-3 pt-2 sm:flex-row">
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  setSubmittedOrder(null);
                  form.reset({
                    items: [
                      {
                        product_id: productsQuery.data?.[0]?.id ?? "",
                        qty: 10,
                      },
                    ],
                    event_date: "",
                    customer_name: meQuery.data?.name ?? "",
                    customer_contact: meQuery.data?.contact ?? "",
                    delivery_type: "pickup",
                    address: "",
                    notes: "",
                  });
                }}
              >
                Buat Pesanan Lain
              </Button>
              <div className="flex flex-wrap gap-2">
                <Link href="/pesanan-saya" className="flex-1 sm:flex-initial">
                  <Button variant="outline" className="w-full gap-1.5">
                    <ClipboardList className="h-4 w-4" />
                    Pesanan Saya
                  </Button>
                </Link>
                <Link
                  href={`/lacak/${encodeURIComponent(submittedOrder.code)}`}
                  className="flex-1 sm:flex-initial"
                >
                  <Button className="w-full gap-2">
                    Buka Halaman Lacak Pesanan
                    <ArrowRight className="h-4 w-4" />
                  </Button>
                </Link>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <form
      onSubmit={form.handleSubmit(onSubmit)}
      noValidate
      className="grid grid-cols-1 gap-6 lg:grid-cols-12"
    >
      {/* Banner Status Login Pelanggan (Penuh Lebar di Atas Form) */}
      <div className="lg:col-span-12">
        {!meQuery.isLoading && !isCustomerLoggedIn ? (
          <div className="flex flex-col justify-between gap-3 rounded-2xl border border-amber-300 bg-amber-50/90 p-4 text-xs text-stone-800 shadow-xs sm:flex-row sm:items-center sm:text-sm">
            <div className="space-y-1">
              <p className="font-bold text-amber-950">
                Masuk atau Daftar Akun Pelanggan untuk Mengirim Pesanan
              </p>
              <p className="text-xs text-stone-700">
                Anda tetap dapat memilih produk dan mengecek ketersediaan tanggal
                di bawah ini. Untuk mengirim pesanan dan mengamankan slot dapur,
                silakan masuk atau daftar akun pelanggan terlebih dahulu.
              </p>
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              <Link href={`/login?next=${encodeURIComponent(nextUrlParam)}`}>
                <Button variant="outline" size="sm" className="gap-1.5 bg-white">
                  <LogIn className="h-3.5 w-3.5" />
                  Masuk
                </Button>
              </Link>
              <Link href={`/daftar?next=${encodeURIComponent(nextUrlParam)}`}>
                <Button size="sm" className="gap-1.5">
                  <UserPlus className="h-3.5 w-3.5" />
                  Daftar Akun Pelanggan
                </Button>
              </Link>
            </div>
          </div>
        ) : isCustomerLoggedIn && meQuery.data ? (
          <div className="flex flex-col justify-between gap-2 rounded-2xl border border-emerald-200 bg-emerald-50/70 px-4 py-3 text-xs text-emerald-950 sm:flex-row sm:items-center">
            <div className="flex items-center gap-2">
              <UserCheck className="h-4 w-4 shrink-0 text-emerald-700" />
              <span>
                Anda memesan sebagai{" "}
                <strong>{meQuery.data.name}</strong> ({meQuery.data.email}).
                Nama dan nomor kontak telah diisi otomatis dari profil Anda dan
                tetap dapat diubah bila diperlukan.
              </span>
            </div>
            <Link
              href="/pesanan-saya"
              className="shrink-0 font-semibold text-emerald-800 underline underline-offset-2 hover:text-emerald-950"
            >
              Lihat Pesanan Saya →
            </Link>
          </div>
        ) : null}
      </div>

      {/* Kolom Kiri: Produk & Pemilih Tanggal */}
      <div className="space-y-6 lg:col-span-7">
        {serverFieldErrors.length > 0 ? (
          <div
            role="alert"
            className="rounded-xl border border-rose-200 bg-rose-50 p-4 text-xs text-rose-900 sm:text-sm"
          >
            <div className="flex items-center gap-2 font-semibold text-rose-800">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>Mohon periksa kembali data pesanan Anda:</span>
            </div>
            <ul className="mt-1.5 list-disc space-y-1 pl-5">
              {serverFieldErrors.map((err, i) => (
                <li key={i}>{err}</li>
              ))}
            </ul>
          </div>
        ) : null}

        {/* Langkah 1: Pilih Produk & Jumlah */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between gap-2">
              <div>
                <CardTitle className="text-base sm:text-lg">
                  1. Pilih Produk &amp; Jumlah
                </CardTitle>
                <CardDescription>
                  Ketersediaan tanggal dihitung otomatis dari beban oven dan
                  tenaga kerja item pilihan Anda.
                </CardDescription>
              </div>
              <Badge variant="amber">{totalUnits} unit</Badge>
            </div>
          </CardHeader>

          <CardContent className="space-y-4">
            {productsQuery.isLoading ? (
              <div className="space-y-3">
                <Skeleton className="h-12 w-full" />
                <Skeleton className="h-12 w-full" />
              </div>
            ) : null}

            {productsQuery.isError ? (
              <ErrorState
                message={getErrorMessage(productsQuery.error)}
                onRetry={() => productsQuery.refetch()}
              />
            ) : null}

            {!productsQuery.isLoading && !productsQuery.isError ? (
              <div className="space-y-3">
                {fields.map((field, index) => {
                  const itemErr = form.formState.errors.items?.[index];
                  const selectedPid = watchedItems?.[index]?.product_id ?? "";
                  const selectedQty = Number(watchedItems?.[index]?.qty ?? 0);
                  const prodInfo = productMap.get(selectedPid);
                  const lineSubtotal =
                    prodInfo && selectedQty > 0
                      ? prodInfo.price * selectedQty
                      : 0;

                  return (
                    <div
                      key={field.id}
                      className="rounded-xl border border-stone-200 bg-stone-50/60 p-3.5"
                    >
                      <div className="grid grid-cols-1 gap-3 sm:grid-cols-12 sm:items-end">
                        <div className="space-y-1.5 sm:col-span-7">
                          <Label htmlFor={`item-product-${index}`}>
                            Produk #{index + 1}
                          </Label>
                          <select
                            id={`item-product-${index}`}
                            {...form.register(`items.${index}.product_id`)}
                            className="flex h-10 w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900 focus-visible:border-amber-700 focus-visible:ring-2 focus-visible:ring-amber-600/20 focus-visible:outline-none"
                          >
                            <option value="">— Pilih produk —</option>
                            {(productsQuery.data ?? []).map((p) => (
                              <option key={p.id} value={p.id}>
                                {p.name} ({formatRupiah(p.price)} / unit ·{" "}
                                {formatCategoryName(p.category)})
                              </option>
                            ))}
                          </select>
                        </div>

                        <div className="space-y-1.5 sm:col-span-3">
                          <Label htmlFor={`item-qty-${index}`}>
                            Jumlah (unit)
                          </Label>
                          <Input
                            id={`item-qty-${index}`}
                            type="number"
                            min={1}
                            max={100000}
                            {...form.register(`items.${index}.qty`, {
                              valueAsNumber: true,
                            })}
                          />
                        </div>

                        <div className="flex items-center justify-between gap-2 sm:col-span-2 sm:justify-end">
                          <span className="text-xs font-semibold text-stone-700 sm:hidden">
                            Subtotal: {formatRupiah(lineSubtotal)}
                          </span>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            disabled={fields.length <= 1}
                            onClick={() => remove(index)}
                            aria-label={`Hapus baris produk ${index + 1}`}
                            className="text-stone-500 hover:bg-rose-50 hover:text-rose-700"
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>

                      <div className="mt-2 hidden items-center justify-between text-xs text-stone-500 sm:flex">
                        <span>
                          {prodInfo
                            ? `Harga satuan: ${formatRupiah(prodInfo.price)}`
                            : "Pilih produk untuk melihat harga"}
                        </span>
                        <span className="font-semibold text-stone-800">
                          Subtotal: {formatRupiah(lineSubtotal)}
                        </span>
                      </div>

                      {itemErr?.product_id ? (
                        <p className="mt-1 text-xs font-medium text-rose-600">
                          {itemErr.product_id.message}
                        </p>
                      ) : null}
                      {itemErr?.qty ? (
                        <p className="mt-1 text-xs font-medium text-rose-600">
                          {itemErr.qty.message}
                        </p>
                      ) : null}
                    </div>
                  );
                })}

                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    append({
                      product_id: productsQuery.data?.[0]?.id ?? "",
                      qty: 5,
                    })
                  }
                  className="gap-1.5"
                >
                  <Plus className="h-4 w-4" />
                  Tambah Produk Lain
                </Button>
              </div>
            ) : null}
          </CardContent>
        </Card>

        {/* Langkah 2: Pemilih Tanggal Ketersediaan (POST /public/availability dengan debounce) */}
        <Card>
          <CardHeader className="pb-3">
            <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
              <div>
                <CardTitle className="text-base sm:text-lg">
                  2. Pilih Tanggal Acara
                </CardTitle>
                <CardDescription>
                  Status ketersediaan diperbarui otomatis setiap kali daftar
                  produk atau jumlah berubah.
                </CardDescription>
              </div>

              <div className="flex items-center gap-1.5 self-start sm:self-auto">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    setWindowStart((prev) => addDaysDateKey(prev, -14))
                  }
                  aria-label="14 hari sebelumnya"
                >
                  <ChevronLeft className="h-4 w-4" />
                  Sebelumnya
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    setWindowStart((prev) => addDaysDateKey(prev, 14))
                  }
                  aria-label="14 hari berikutnya"
                >
                  Berikutnya
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </CardHeader>

          <CardContent className="space-y-4">
            {/* Legenda Status Hari & Penjelasan Persentase Beban */}
            <div className="space-y-2 rounded-xl border border-stone-200 bg-stone-50 px-3.5 py-2.5 text-xs text-stone-700">
              <div className="flex flex-wrap items-center gap-3">
                <span className="flex items-center gap-1.5">
                  <span className="h-2.5 w-2.5 rounded-full bg-emerald-500" />
                  Tersedia (&lt; 70%)
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="h-2.5 w-2.5 rounded-full bg-amber-500" />
                  Padat (70% – 90%)
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="h-2.5 w-2.5 rounded-full bg-rose-500" />
                  Penuh (&gt; 90%, Tidak Dapat Dipilih)
                </span>
                <span className="flex items-center gap-1.5">
                  <span className="h-2.5 w-2.5 rounded-full bg-stone-400" />
                  Di Bawah Lead Time
                </span>
              </div>
              <p className="flex items-start gap-1.5 border-t border-stone-200/80 pt-1.5 text-[11px] text-stone-600">
                <HelpCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-700" />
                <span>
                  <strong>Apa arti &ldquo;Beban setelah pesanan ini&rdquo;?</strong>{" "}
                  Angka tersebut menunjukkan persentase kapasitas harian dapur
                  (jam oven &amp; tenaga kerja) yang terpakai jika pesanan Anda
                  masuk pada tanggal itu. Pesanan dapat diterima hingga beban
                  maksimal 90%.
                </span>
              </p>
            </div>

            {availabilityQuery.isLoading || availabilityQuery.isFetching ? (
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
                {Array.from({ length: 12 }).map((_, idx) => (
                  <Skeleton key={idx} className="h-20 w-full rounded-xl" />
                ))}
              </div>
            ) : null}

            {availabilityQuery.isError ? (
              <ErrorState
                title="Gagal Mengecek Ketersediaan Tanggal"
                message={getErrorMessage(availabilityQuery.error)}
                onRetry={() => availabilityQuery.refetch()}
              />
            ) : null}

            {!availabilityQuery.isLoading &&
            !availabilityQuery.isFetching &&
            !availabilityQuery.isError &&
            availabilityQuery.data ? (
              <div
                role="radiogroup"
                aria-label="Pilih tanggal acara"
                className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4"
              >
                {availabilityQuery.data.map((day: AvailabilityDay) => {
                  const isSelected = watchedEventDate === day.date;
                  const utilNum = toNumber(day.utilization_after);
                  const isFullCapacity =
                    !day.available &&
                    (day.status === "full" ||
                      day.status === "red" ||
                      utilNum > 0.9);
                  const isUnderLeadTime = !day.available && !isFullCapacity;
                  const isDense =
                    day.available &&
                    (day.status === "yellow" ||
                      day.status === "red" ||
                      utilNum >= 0.7);
                  const pctRounded = Math.round(utilNum * 100);

                  let statusText = "Tersedia";
                  if (isFullCapacity) statusText = "Penuh";
                  else if (isUnderLeadTime) statusText = "< Lead Time";
                  else if (isDense) statusText = "Padat";

                  const tooltipText = `Beban setelah pesanan ini: ${pctRounded}% (${formatPercent(day.utilization_after)}). Persentase kapasitas harian dapur (oven & tenaga kerja) yang terpakai jika pesanan Anda dijadwalkan pada tanggal ini. Batas terima otomatis adalah 90%.`;

                  return (
                    <button
                      key={day.date}
                      type="button"
                      role="radio"
                      aria-checked={isSelected}
                      disabled={!day.available}
                      title={tooltipText}
                      onClick={() =>
                        form.setValue("event_date", day.date, {
                          shouldValidate: true,
                        })
                      }
                      className={cn(
                        "flex flex-col items-start justify-between rounded-xl border p-2.5 text-left transition-all",
                        day.available
                          ? "cursor-pointer"
                          : "cursor-not-allowed opacity-60",
                        isSelected &&
                          "border-amber-700 bg-amber-50 ring-2 ring-amber-700/30",
                        !isSelected &&
                          day.available &&
                          !isDense &&
                          "border-emerald-200 bg-emerald-50/40 hover:border-emerald-400 hover:bg-emerald-50",
                        !isSelected &&
                          day.available &&
                          isDense &&
                          "border-amber-300 bg-amber-50/50 hover:border-amber-500 hover:bg-amber-50",
                        isFullCapacity &&
                          "border-rose-200 bg-rose-50/50 text-rose-900",
                        isUnderLeadTime &&
                          "border-stone-200 bg-stone-100/80 text-stone-500"
                      )}
                    >
                      <div className="flex w-full items-center justify-between gap-1">
                        <span className="text-xs font-bold text-stone-900">
                          {formatDateShort(day.date)}
                        </span>
                        <span
                          className={cn(
                            "h-2 w-2 rounded-full",
                            day.available &&
                              !isDense &&
                              "bg-emerald-500",
                            isDense && "bg-amber-500",
                            isFullCapacity && "bg-rose-500",
                            isUnderLeadTime && "bg-stone-400"
                          )}
                        />
                      </div>

                      <div className="mt-2 w-full">
                        <p
                          className={cn(
                            "text-[11px] font-semibold",
                            day.available &&
                              !isDense &&
                              "text-emerald-800",
                            isDense && "text-amber-800",
                            isFullCapacity && "text-rose-700",
                            isUnderLeadTime && "text-stone-500"
                          )}
                        >
                          {statusText}
                        </p>
                        <p
                          title={tooltipText}
                          className="text-[10px] leading-snug text-stone-600"
                        >
                          Beban setelah pesanan ini: {pctRounded}%
                        </p>
                      </div>
                    </button>
                  );
                })}
              </div>
            ) : null}

            {watchedEventDate ? (
              <div className="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50/70 px-3.5 py-2.5 text-xs text-emerald-950 sm:text-sm">
                <CalendarIcon className="h-4 w-4 shrink-0 text-emerald-700" />
                <span>
                  Tanggal acara terpilih:{" "}
                  <strong>{formatDateLong(watchedEventDate)}</strong>
                </span>
              </div>
            ) : null}

            {form.formState.errors.event_date ? (
              <p className="text-xs font-medium text-rose-600">
                {form.formState.errors.event_date.message}
              </p>
            ) : null}
          </CardContent>
        </Card>
      </div>

      {/* Kolom Kanan: Data Pemesan & Ringkasan Estimasi Total */}
      <div className="space-y-6 lg:col-span-5">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base sm:text-lg">
              3. Data Pelanggan &amp; Pengambilan
            </CardTitle>
            <CardDescription>
              Lengkapi informasi kontak agar mudah dihubungi untuk konfirmasi
              pesanan.
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="customer_name">Nama Lengkap / Instansi</Label>
              <Input
                id="customer_name"
                placeholder="Contoh: Ibu Ratna / PT Nusantara"
                {...form.register("customer_name")}
              />
              {form.formState.errors.customer_name ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.customer_name.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="customer_contact">
                Nomor WhatsApp / Telepon Aktif
              </Label>
              <Input
                id="customer_contact"
                placeholder="Contoh: 081234567890"
                {...form.register("customer_contact")}
              />
              {form.formState.errors.customer_contact ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.customer_contact.message}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label>Metode Pengambilan</Label>
              <div className="grid grid-cols-2 gap-2.5">
                <label
                  className={cn(
                    "flex cursor-pointer flex-col rounded-xl border p-3 text-xs transition-colors sm:text-sm",
                    watchedDeliveryType === "pickup"
                      ? "border-amber-700 bg-amber-50/70 font-semibold text-amber-950"
                      : "border-stone-200 bg-white text-stone-700 hover:bg-stone-50"
                  )}
                >
                  <input
                    type="radio"
                    value="pickup"
                    {...form.register("delivery_type")}
                    className="sr-only"
                  />
                  <span>Ambil Sendiri</span>
                  <span className="mt-0.5 text-[11px] font-normal text-stone-500">
                    Ambil di Dapur Kalkurasa
                  </span>
                </label>

                <label
                  className={cn(
                    "flex cursor-pointer flex-col rounded-xl border p-3 text-xs transition-colors sm:text-sm",
                    watchedDeliveryType === "delivery"
                      ? "border-amber-700 bg-amber-50/70 font-semibold text-amber-950"
                      : "border-stone-200 bg-white text-stone-700 hover:bg-stone-50"
                  )}
                >
                  <input
                    type="radio"
                    value="delivery"
                    {...form.register("delivery_type")}
                    className="sr-only"
                  />
                  <span>Diantar Kurir</span>
                  <span className="mt-0.5 text-[11px] font-normal text-stone-500">
                    Kirim ke alamat acara
                  </span>
                </label>
              </div>
            </div>

            {watchedDeliveryType === "delivery" ? (
              <div className="space-y-1.5">
                <Label htmlFor="address">Alamat Lengkap Pengantaran</Label>
                <Textarea
                  id="address"
                  rows={3}
                  placeholder="Nama jalan, nomor gedung/rumah, kecamatan, dan patokan lokasi..."
                  {...form.register("address")}
                />
                {form.formState.errors.address ? (
                  <p className="text-xs font-medium text-rose-600">
                    {form.formState.errors.address.message}
                  </p>
                ) : null}
              </div>
            ) : null}

            <div className="space-y-1.5">
              <Label htmlFor="notes">Catatan Tambahan (Opsional)</Label>
              <Textarea
                id="notes"
                rows={2}
                placeholder="Misal: jam pengambilan, ucapan pada kue tart, atau preferensi kemasan..."
                {...form.register("notes")}
              />
              {form.formState.errors.notes ? (
                <p className="text-xs font-medium text-rose-600">
                  {form.formState.errors.notes.message}
                </p>
              ) : null}
            </div>
          </CardContent>
        </Card>

        {/* Ringkasan Estimasi & Tombol Kirim */}
        <Card className="border-amber-200 bg-amber-50/40">
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Ringkasan Estimasi</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2 rounded-xl border border-amber-200/80 bg-white p-4 text-sm">
              <div className="flex justify-between text-stone-600">
                <span>Total Kuantitas</span>
                <span className="font-semibold text-stone-900">
                  {totalUnits} unit
                </span>
              </div>
              <div className="flex justify-between text-stone-600">
                <span>Tanggal Acara</span>
                <span className="font-semibold text-stone-900">
                  {watchedEventDate
                    ? formatDateShort(watchedEventDate)
                    : "Belum dipilih"}
                </span>
              </div>
              {taxRate > 0 ? (
                <>
                  <div className="flex justify-between text-stone-600">
                    <span>Subtotal Produk</span>
                    <span className="font-semibold text-stone-900">
                      {formatRupiah(estimatedSubtotal)}
                    </span>
                  </div>
                  <div className="flex justify-between text-stone-600">
                    <span>PPN ({Math.round(taxRate * 100)}%)</span>
                    <span className="font-semibold text-stone-900">
                      {formatRupiah(estimatedTax)}
                    </span>
                  </div>
                </>
              ) : null}
              <div className="border-t border-stone-200 pt-2.5 flex items-baseline justify-between">
                <span className="font-semibold text-stone-800">
                  Estimasi Total Harga
                </span>
                <span className="text-lg font-bold text-amber-800">
                  {formatRupiah(estimatedTotal)}
                </span>
              </div>
            </div>

            {/* Catatan Pra-Kirim bila pesanan besar atau tanggal padat */}
            {isLargeOrder || isSelectedDateDense ? (
              <div className="rounded-xl border border-amber-300 bg-amber-50 p-3.5 text-xs leading-relaxed text-amber-950 space-y-1">
                <p className="font-semibold">
                  Catatan Sebelum Mengirim Pesanan:
                </p>
                {isLargeOrder ? (
                  <p>
                    • Karena pesanan ini berjumlah{" "}
                    <strong>{totalUnits} unit</strong> dengan nilai{" "}
                    <strong>{formatRupiah(estimatedTotal)}</strong> (mencapai
                    ambang pesanan besar &ge; 50 unit atau &ge; Rp 2.000.000),
                    pesanan akan masuk dengan status{" "}
                    <strong>Menunggu Konfirmasi Usaha</strong> agar Pemilik /
                    Manajer dapat memastikan kesiapan bahan dan jadwal sebelum
                    Anda diminta membayar DP.
                  </p>
                ) : null}
                {isSelectedDateDense && selectedDayInfo ? (
                  <p>
                    • Tanggal yang Anda pilih berada pada kategori{" "}
                    <strong>Padat</strong> (beban setelah pesanan ini:{" "}
                    <strong>
                      {Math.round(toNumber(selectedDayInfo.utilization_after) * 100)}
                      %
                    </strong>
                    , mendekati batas 90%). Pesanan mungkin memerlukan peninjauan
                    jadwal atau konfirmasi ketersediaan bahan oleh tim dapur.
                  </p>
                ) : null}
              </div>
            ) : null}

            <div className="flex items-start gap-2 text-xs text-stone-600">
              <Info className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
              <span>
                Besaran uang muka (DP) dan status penerimaan akhir akan dihitung
                oleh server saat pesanan dikirim.
              </span>
            </div>

            {!isCustomerLoggedIn ? (
              <div className="space-y-2 rounded-xl border border-amber-300 bg-white p-3 text-center">
                <p className="text-xs font-medium text-stone-700">
                  Silakan masuk atau daftar sebagai pelanggan terlebih dahulu
                  untuk mengirim pesanan ini:
                </p>
                <div className="grid grid-cols-2 gap-2">
                  <Link href={`/login?next=${encodeURIComponent(nextUrlParam)}`}>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="w-full gap-1.5"
                    >
                      <LogIn className="h-3.5 w-3.5" />
                      Masuk
                    </Button>
                  </Link>
                  <Link
                    href={`/daftar?next=${encodeURIComponent(nextUrlParam)}`}
                  >
                    <Button type="button" size="sm" className="w-full gap-1.5">
                      <UserPlus className="h-3.5 w-3.5" />
                      Daftar
                    </Button>
                  </Link>
                </div>
              </div>
            ) : (
              <Button
                type="submit"
                size="lg"
                disabled={createOrderMutation.isPending}
                className="w-full gap-2"
              >
                {createOrderMutation.isPending ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Memproses &amp; Mengevaluasi Pesanan...
                  </>
                ) : (
                  <>
                    <ShoppingBag className="h-4 w-4" />
                    Kirim Pesanan Pre-Order
                  </>
                )}
              </Button>
            )}
          </CardContent>
        </Card>
      </div>
    </form>
  );
}

export default function PublicOrderPage() {
  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold tracking-tight text-stone-900 sm:text-3xl">
            Formulir Pemesanan Pre-Order
          </h1>
          <p className="mt-1 text-sm text-stone-600">
            Pilih produk, cek tanggal acara yang tersedia, dan kirim pesanan
            Anda secara instan.
          </p>
        </div>

        <Suspense
          fallback={
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
              <div className="space-y-4 lg:col-span-7">
                <Skeleton className="h-56 w-full rounded-xl" />
                <Skeleton className="h-72 w-full rounded-xl" />
              </div>
              <div className="lg:col-span-5">
                <Skeleton className="h-96 w-full rounded-xl" />
              </div>
            </div>
          }
        >
          <OrderFormContent />
        </Suspense>
      </main>
    </div>
  );
}

