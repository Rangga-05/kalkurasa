"use client";

import React from "react";
import { ChefHat } from "lucide-react";
import {
  formatDateLong,
  formatDateTimeWIB,
  formatRupiah,
  getOrderStatusMeta,
  toNumber,
} from "@/lib/format";
import type { DecimalValue } from "@/lib/types";

export interface InvoiceItem {
  id?: string;
  product_name: string;
  qty: number;
  unit_price?: DecimalValue | null;
  subtotal?: DecimalValue | null;
}

export interface OrderInvoicePrintProps {
  order: {
    code: string;
    customer_name: string;
    customer_contact?: string;
    delivery_type: "pickup" | "delivery" | string;
    address?: string;
    notes?: string;
    event_date: string;
    created_at: string;
    status: string;
    items: InvoiceItem[];
    subtotal?: DecimalValue | null;
    tax_pct?: DecimalValue | null;
    tax_amount?: DecimalValue | null;
    platform_fee?: DecimalValue | null;
    total?: DecimalValue | null;
    dp_required?: DecimalValue | null;
    paid?: DecimalValue | null;
  };
  /**
   * Bila diset true (misal staf katering mencetak lembar kerja dapur),
   * kolom harga dan rincian keuangan disembunyikan.
   */
  hideFinancials?: boolean;
}

export function OrderInvoicePrint({
  order,
  hideFinancials = false,
}: OrderInvoicePrintProps) {
  const statusMeta = getOrderStatusMeta(order.status);
  const totalNum = toNumber(order.total);
  const paidNum = toNumber(order.paid);
  const dpNum = toNumber(order.dp_required);
  const remainingNum = Math.max(0, totalNum - paidNum);
  const isLunas = totalNum > 0 && remainingNum <= 0;

  const taxAmountNum = toNumber(order.tax_amount);
  const taxPctNum = toNumber(order.tax_pct);
  const platformFeeNum = toNumber(order.platform_fee);

  // Subtotal produk dihitung dari snapshot atau akumulasi item
  let computedSubtotal = toNumber(order.subtotal);
  if (computedSubtotal <= 0 && order.items && order.items.length > 0) {
    computedSubtotal = order.items.reduce(
      (acc, it) => acc + toNumber(it.subtotal),
      0
    );
  }

  const printDateStr = formatDateTimeWIB(new Date().toISOString());

  return (
    <section
      aria-label={`Nota Invoice #${order.code}`}
      className="hidden print:block w-full bg-white text-stone-900 font-sans p-2"
    >
      {/* KOP NOTA / INVOICE */}
      <div className="flex items-start justify-between border-b-2 border-stone-800 pb-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-stone-900 text-white">
              <ChefHat className="h-5 w-5" />
            </div>
            <span className="text-2xl font-black tracking-tight text-stone-950 uppercase">
              Kalkurasa Catering
            </span>
          </div>
          <p className="text-xs text-stone-600 font-medium">
            Layanan Katering &amp; Konsumsi Acara Profesional (Pre-Order)
          </p>
          <p className="text-[11px] text-stone-500">
            Dapur Operasional Kalkurasa · CS WhatsApp: 0812-3456-7890 · Email: halo@kalkurasa.id
          </p>
        </div>

        <div className="text-right space-y-1">
          <h1 className="text-xl font-extrabold tracking-tight text-stone-900 uppercase">
            {hideFinancials ? "SURAT JALAN & LEMBAR KERJA" : "FAKTUR / NOTA PEMESANAN"}
          </h1>
          <p className="font-mono text-base font-bold text-stone-950">
            #{order.code}
          </p>
          <div className="inline-block rounded border border-stone-800 px-2 py-0.5 text-xs font-bold uppercase">
            Status: {statusMeta.label}
          </div>
        </div>
      </div>

      {/* INFORMASI PEMESAN & ACARA */}
      <div className="mt-4 grid grid-cols-2 gap-4 text-xs">
        {/* Kolom Pemesan */}
        <div className="rounded-lg border border-stone-300 p-3 space-y-1.5 bg-stone-50/50">
          <p className="font-bold uppercase tracking-wider text-stone-700 border-b border-stone-200 pb-1">
            Data Pemesan
          </p>
          <div className="grid grid-cols-3 gap-1">
            <span className="text-stone-500">Nama Pelanggan</span>
            <span className="col-span-2 font-bold text-stone-900">
              : {order.customer_name}
            </span>
          </div>
          {order.customer_contact ? (
            <div className="grid grid-cols-3 gap-1">
              <span className="text-stone-500">No. Kontak</span>
              <span className="col-span-2 font-semibold text-stone-900">
                : {order.customer_contact}
              </span>
            </div>
          ) : null}
          <div className="grid grid-cols-3 gap-1">
            <span className="text-stone-500">Metode Penyerahan</span>
            <span className="col-span-2 font-semibold text-stone-900">
              : {order.delivery_type === "delivery"
                  ? "Diantar Kurir (Delivery)"
                  : "Diambil Sendiri di Dapur (Pickup)"}
            </span>
          </div>
          {order.address ? (
            <div className="grid grid-cols-3 gap-1">
              <span className="text-stone-500">Alamat Kirim</span>
              <span className="col-span-2 text-stone-900">
                : {order.address}
              </span>
            </div>
          ) : null}
        </div>

        {/* Kolom Acara & Pesanan */}
        <div className="rounded-lg border border-stone-300 p-3 space-y-1.5 bg-stone-50/50">
          <p className="font-bold uppercase tracking-wider text-stone-700 border-b border-stone-200 pb-1">
            Jadwal Acara &amp; Dokumen
          </p>
          <div className="grid grid-cols-3 gap-1">
            <span className="text-stone-500">Tanggal Acara</span>
            <span className="col-span-2 font-bold text-stone-900">
              : {formatDateLong(order.event_date)}
            </span>
          </div>
          <div className="grid grid-cols-3 gap-1">
            <span className="text-stone-500">Tanggal Pesanan</span>
            <span className="col-span-2 text-stone-800">
              : {formatDateTimeWIB(order.created_at)}
            </span>
          </div>
          <div className="grid grid-cols-3 gap-1">
            <span className="text-stone-500">Waktu Cetak</span>
            <span className="col-span-2 text-stone-800">
              : {printDateStr}
            </span>
          </div>
          {order.notes ? (
            <div className="grid grid-cols-3 gap-1 pt-1 border-t border-stone-200">
              <span className="text-stone-500">Catatan Khusus</span>
              <span className="col-span-2 text-stone-900 italic">
                : {order.notes}
              </span>
            </div>
          ) : null}
        </div>
      </div>

      {/* TABEL ITEM PESANAN */}
      <div className="mt-4">
        <table className="w-full border-collapse border border-stone-400 text-xs">
          <thead>
            <tr className="bg-stone-200/90 text-stone-900">
              <th className="border border-stone-400 px-3 py-2 text-center w-12 font-bold">
                No
              </th>
              <th className="border border-stone-400 px-3 py-2 text-left font-bold">
                Nama Menu / Paket Katering
              </th>
              <th className="border border-stone-400 px-3 py-2 text-center w-24 font-bold">
                Jumlah
              </th>
              {!hideFinancials && (
                <>
                  <th className="border border-stone-400 px-3 py-2 text-right w-36 font-bold">
                    Harga Satuan
                  </th>
                  <th className="border border-stone-400 px-3 py-2 text-right w-40 font-bold">
                    Subtotal
                  </th>
                </>
              )}
            </tr>
          </thead>
          <tbody>
            {order.items.map((item, idx) => (
              <tr
                key={item.id ?? idx}
                className={idx % 2 === 1 ? "bg-stone-50/70" : "bg-white"}
              >
                <td className="border border-stone-300 px-3 py-2 text-center font-medium">
                  {idx + 1}
                </td>
                <td className="border border-stone-300 px-3 py-2 font-semibold text-stone-900">
                  {item.product_name}
                </td>
                <td className="border border-stone-300 px-3 py-2 text-center font-bold">
                  {item.qty} unit
                </td>
                {!hideFinancials && (
                  <>
                    <td className="border border-stone-300 px-3 py-2 text-right">
                      {formatRupiah(item.unit_price ?? 0)}
                    </td>
                    <td className="border border-stone-300 px-3 py-2 text-right font-bold text-stone-900">
                      {formatRupiah(item.subtotal ?? 0)}
                    </td>
                  </>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* RINGKASAN PEMBAYARAN & CATATAN KEBIJAKAN */}
      {!hideFinancials && (
        <div className="mt-4 grid grid-cols-12 gap-4 text-xs">
          {/* Kolom Kebijakan & Stempel Lunas */}
          <div className="col-span-7 flex flex-col justify-between rounded-lg border border-stone-300 p-3 bg-stone-50/40">
            <div>
              <p className="font-bold uppercase tracking-wider text-stone-700">
                Ketentuan &amp; Informasi Transaksi:
              </p>
              <ul className="mt-1 list-disc space-y-1 pl-4 text-[11px] leading-relaxed text-stone-600">
                <li>
                  Nota ini merupakan bukti sah transaksi pemesanan pada sistem resmi Kalkurasa Catering.
                </li>
                <li>
                  Harap menunjukkan bukti nota ini atau kode pesanan saat penyerahan pesanan di lokasi.
                </li>
                <li>
                  Pelunasan sisa tagihan wajib diselesaikan sebelum atau saat pesanan diserahterimakan.
                </li>
              </ul>
            </div>

            {/* Cap Stempel Lunas bila pembayaran sudah tuntas */}
            {isLunas ? (
              <div className="mt-3 flex items-center gap-2 self-start rounded-lg border-2 border-emerald-700 px-3 py-1 font-mono text-sm font-black text-emerald-800 tracking-widest uppercase">
                ✓ LUNAS / FULLY PAID
              </div>
            ) : paidNum > 0 ? (
              <div className="mt-3 flex items-center gap-2 self-start rounded-lg border-2 border-amber-700 px-3 py-1 font-mono text-xs font-black text-amber-800 tracking-wider uppercase">
                DP TERCATAT ({formatRupiah(paidNum)})
              </div>
            ) : null}
          </div>

          {/* Kolom Tabel Keuangan */}
          <div className="col-span-5 rounded-lg border border-stone-300 bg-white p-3 space-y-1.5">
            <div className="flex justify-between text-stone-600">
              <span>Subtotal Menu:</span>
              <span className="font-medium text-stone-900">
                {formatRupiah(computedSubtotal)}
              </span>
            </div>

            {taxAmountNum > 0 ? (
              <div className="flex justify-between text-stone-600">
                <span>PPN ({Math.round(taxPctNum * 100)}%):</span>
                <span className="font-medium text-stone-900">
                  {formatRupiah(taxAmountNum)}
                </span>
              </div>
            ) : null}

            {platformFeeNum > 0 ? (
              <div className="flex justify-between text-stone-600">
                <span>Biaya Layanan Aplikasi:</span>
                <span className="font-medium text-stone-900">
                  {formatRupiah(platformFeeNum)}
                </span>
              </div>
            ) : null}

            <div className="flex justify-between border-t-2 border-stone-800 pt-1.5 text-sm font-extrabold text-stone-950">
              <span>TOTAL TAGIHAN:</span>
              <span>{formatRupiah(totalNum)}</span>
            </div>

            <div className="flex justify-between text-[11px] text-stone-600 pt-1 border-t border-stone-200">
              <span>Minimal Uang Muka (DP):</span>
              <span className="font-semibold text-stone-800">
                {formatRupiah(dpNum)}
              </span>
            </div>

            <div className="flex justify-between text-[11px] text-stone-600">
              <span>Total Pembayaran Diterima:</span>
              <span className="font-semibold text-emerald-700">
                {formatRupiah(paidNum)}
              </span>
            </div>

            <div className="flex justify-between border-t border-stone-300 pt-1.5 font-bold">
              <span className={remainingNum > 0 ? "text-rose-700" : "text-stone-800"}>
                SISA PELUNASAN:
              </span>
              <span className={remainingNum > 0 ? "text-rose-700 font-extrabold" : "text-stone-900"}>
                {formatRupiah(remainingNum)}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* TANDA TANGAN & PENGESAHAN */}
      <div className="mt-8 grid grid-cols-2 gap-8 text-center text-xs">
        <div>
          <p className="font-medium text-stone-600">Penerima / Pemesan,</p>
          <div className="h-16" />
          <p className="font-bold text-stone-900 underline">
            ( {order.customer_name} )
          </p>
          <p className="text-[10px] text-stone-500">Tanda Tangan &amp; Nama Terang</p>
        </div>

        <div>
          <p className="font-medium text-stone-600">
            Hormat Kami, Pengelola Kalkurasa,
          </p>
          <div className="h-16" />
          <p className="font-bold text-stone-900 underline">
            ( Tim Administrasi &amp; Kasir )
          </p>
          <p className="text-[10px] text-stone-500">Stempel Resmi &amp; Tanda Tangan</p>
        </div>
      </div>

      {/* FOOTER NOTA */}
      <div className="mt-6 border-t border-stone-300 pt-2 text-center text-[10px] text-stone-400">
        Dicetak otomatis melalui Sistem Kalkurasa Catering · www.kalkurasa.id · Simpan nota ini sebagai bukti pembayaran resmi
      </div>
    </section>
  );
}

