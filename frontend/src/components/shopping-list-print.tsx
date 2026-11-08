"use client";

import React from "react";
import { ChefHat } from "lucide-react";
import {
  formatDateLong,
  formatDateTimeWIB,
  formatNumber,
  formatPercent,
  formatRupiah,
  toNumber,
} from "@/lib/format";
import type { ShoppingListResult } from "@/lib/types";

export interface ShoppingListPrintProps {
  data: ShoppingListResult;
  fromDate: string;
  toDate: string;
  isStaff?: boolean;
}

export function ShoppingListPrint({
  data,
  fromDate,
  toDate,
  isStaff = false,
}: ShoppingListPrintProps) {
  const items = data.items ?? [];
  const needBuyItems = items.filter((it) => toNumber(it.qty_buy_units) > 0);
  const printDateStr = formatDateTimeWIB(new Date().toISOString());

  return (
    <section
      aria-label="Lembar Belanja Bahan Baku"
      className="hidden print:block w-full bg-white text-stone-900 font-sans p-2"
    >
      {/* KOP DOKUMEN BELANJA */}
      <div className="flex items-start justify-between border-b-2 border-stone-800 pb-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-stone-900 text-white">
              <ChefHat className="h-5 w-5" />
            </div>
            <span className="text-xl font-black tracking-tight text-stone-950 uppercase">
              Kalkurasa Catering
            </span>
          </div>
          <p className="text-xs font-semibold text-stone-700 uppercase tracking-wide">
            Lembar Kerja &amp; Checklist Belanja Bahan Baku Dapur
          </p>
          <p className="text-[11px] text-stone-500">
            Dapur Operasional Kalkurasa · Divisi Pengadaan &amp; Logistik Bahan
          </p>
        </div>

        <div className="text-right space-y-1 text-xs">
          <div className="inline-block rounded border border-stone-800 bg-stone-100 px-2.5 py-1 font-bold uppercase tracking-wider text-stone-900">
            DOKUMEN PENGADAAN PASAR
          </div>
          <p className="text-[11px] text-stone-600">
            Waktu Cetak: <span className="font-semibold">{printDateStr}</span>
          </p>
          <p className="text-[11px] text-stone-600">
            Pesanan Terkait: <span className="font-semibold">{data.order_count} Pesanan Aktif</span>
          </p>
        </div>
      </div>

      {/* PARAMETER & RENTANG WAKTU BELANJA */}
      <div className="mt-3 grid grid-cols-3 gap-3 text-xs">
        <div className="rounded border border-stone-300 bg-stone-50/70 p-2 space-y-0.5">
          <span className="text-[11px] text-stone-500 font-medium">Rentang Tanggal Acara:</span>
          <p className="font-bold text-stone-900">
            {formatDateLong(fromDate)} s.d. {formatDateLong(toDate)}
          </p>
        </div>

        <div className="rounded border border-stone-300 bg-stone-50/70 p-2 space-y-0.5">
          <span className="text-[11px] text-stone-500 font-medium">Kebijakan Buffer Belanja:</span>
          <p className="font-bold text-stone-900">
            +{formatPercent(data.buffer_pct)} Kebutuhan Resep (Lead H-{data.lead_days} hari)
          </p>
        </div>

        <div className="rounded border border-stone-300 bg-stone-50/70 p-2 space-y-0.5">
          <span className="text-[11px] text-stone-500 font-medium">Status Pengadaan:</span>
          <p className="font-bold text-stone-900">
            {needBuyItems.length} Bahan Perlu Dibeli (dari {items.length} total bahan)
          </p>
        </div>
      </div>

      {/* TABEL CHECKLIST BELANJA BAHAN */}
      <div className="mt-4">
        <h2 className="text-xs font-bold uppercase tracking-wider text-stone-800 mb-1.5">
          A. Bahan Baku yang Harus Dibeli ({needBuyItems.length} Item)
        </h2>

        {needBuyItems.length === 0 ? (
          <div className="rounded border border-stone-300 p-4 text-center text-xs text-stone-600">
            Seluruh stok bahan baku mencukupi untuk kebutuhan pesanan saat ini. Tidak ada bahan yang perlu dibeli.
          </div>
        ) : (
          <table className="w-full border-collapse border border-stone-400 text-xs">
            <thead>
              <tr className="bg-stone-200/90 text-stone-900">
                <th className="border border-stone-400 px-2 py-2 text-center w-10 font-bold">
                  Cek
                </th>
                <th className="border border-stone-400 px-2 py-2 text-center w-8 font-bold">
                  No
                </th>
                <th className="border border-stone-400 px-3 py-2 text-left font-bold">
                  Nama Bahan Baku
                </th>
                <th className="border border-stone-400 px-2 py-2 text-right w-24 font-bold">
                  Butuh Resep
                </th>
                <th className="border border-stone-400 px-2 py-2 text-right w-20 font-bold">
                  Stok Gudang
                </th>
                <th className="border border-stone-400 px-3 py-2 text-center w-40 font-bold bg-amber-100/70 text-stone-950">
                  Rekomendasi Beli
                </th>
                {!isStaff && (
                  <>
                    <th className="border border-stone-400 px-2 py-2 text-right w-28 font-bold">
                      Est. Harga
                    </th>
                    <th className="border border-stone-400 px-2 py-2 text-right w-32 font-bold">
                      Est. Subtotal
                    </th>
                  </>
                )}
                <th className="border border-stone-400 px-3 py-2 text-center w-28 font-bold">
                  Paraf / Catatan Toko
                </th>
              </tr>
            </thead>
            <tbody>
              {needBuyItems.map((item, idx) => {
                const buyUnits = formatNumber(item.qty_buy_units);
                const isPack = item.rounding_rule === "pack";
                return (
                  <tr
                    key={item.ingredient_id}
                    className={idx % 2 === 1 ? "bg-stone-50/80" : "bg-white"}
                  >
                    {/* Kotak Centang Fisik saat Belanja di Pasar */}
                    <td className="border border-stone-300 px-2 py-2 text-center">
                      <div className="mx-auto h-4 w-4 rounded border-2 border-stone-700" />
                    </td>
                    <td className="border border-stone-300 px-2 py-2 text-center font-medium">
                      {idx + 1}
                    </td>
                    <td className="border border-stone-300 px-3 py-2 font-bold text-stone-950">
                      <div>{item.ingredient_name}</div>
                      <div className="text-[10px] text-stone-500 font-normal">
                        {isPack
                          ? `Aturan: Kemasan Utuh (1 ${item.buy_unit} = ${formatNumber(item.buy_unit_size)} ${item.use_unit})`
                          : `Aturan: Curah (${item.use_unit})`}
                      </div>
                    </td>
                    <td className="border border-stone-300 px-2 py-2 text-right font-medium">
                      {formatNumber(item.required_qty)} {item.use_unit}
                    </td>
                    <td className="border border-stone-300 px-2 py-2 text-right font-medium">
                      {formatNumber(item.stock)} {item.use_unit}
                    </td>
                    <td className="border border-stone-300 px-3 py-2 text-center font-black bg-amber-50/50 text-stone-900">
                      <span className="text-sm">
                        {buyUnits} {item.buy_unit}
                      </span>
                      {isPack && toNumber(item.buy_unit_size) > 1 && (
                        <div className="text-[10px] font-semibold text-stone-600">
                          (Setara {formatNumber(item.buy_use_units)} {item.use_unit})
                        </div>
                      )}
                    </td>
                    {!isStaff && (
                      <>
                        <td className="border border-stone-300 px-2 py-2 text-right text-stone-700">
                          {formatRupiah(item.buy_price ?? 0)}
                        </td>
                        <td className="border border-stone-300 px-2 py-2 text-right font-bold text-stone-950">
                          {formatRupiah(item.estimated_cost ?? 0)}
                        </td>
                      </>
                    )}
                    <td className="border border-stone-300 px-2 py-2 text-center text-[10px] text-stone-400">
                      [ &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; ]
                    </td>
                  </tr>
                );
              })}
            </tbody>
            {!isStaff && data.total_estimated !== undefined && (
              <tfoot>
                <tr className="bg-stone-100 text-stone-950 font-bold border-t-2 border-stone-800">
                  <td
                    colSpan={7}
                    className="border border-stone-400 px-3 py-2 text-right text-xs uppercase tracking-wide"
                  >
                    Total Estimasi Anggaran Belanja Bahan:
                  </td>
                  <td className="border border-stone-400 px-2 py-2 text-right text-sm font-black">
                    {formatRupiah(data.total_estimated)}
                  </td>
                  <td className="border border-stone-400 px-2 py-2" />
                </tr>
              </tfoot>
            )}
          </table>
        )}
      </div>

      {/* PANDUAN QUALITY CONTROL & PENGESAHAN */}
      <div className="mt-4 grid grid-cols-12 gap-4 text-xs">
        {/* Kolom Panduan QC */}
        <div className="col-span-7 rounded border border-stone-300 bg-stone-50/50 p-2.5 space-y-1">
          <p className="font-bold uppercase tracking-wider text-stone-800">
            Panduan Mutu &amp; Verifikasi Belanja (QC):
          </p>
          <ul className="list-decimal pl-4 space-y-0.5 text-[11px] text-stone-700 leading-relaxed">
            <li>
              Periksa masa kedaluwarsa bahan baku minimal 6 bulan kedepan.
            </li>
            <li>
              Pastikan kemasan tidak robek, tidak lembab, dan berstempel halal/izin edar resmi.
            </li>
            <li>
              Bahan segar (sayur, telur, ayam, daging) wajib dalam kondisi dingin dan bebas bau tidak sedap.
            </li>
            <li>
              Wajib meminta faktur/nota toko asli bermaterai/stempel untuk bukti pencatatan kas keluar.
            </li>
          </ul>
        </div>

        {/* Kolom Catatan Total Ringkasan */}
        <div className="col-span-5 rounded border border-stone-300 p-2.5 space-y-1 bg-white">
          <p className="font-bold uppercase tracking-wider text-stone-800">
            Ringkasan Anggaran Kas:
          </p>
          <div className="flex justify-between text-[11px] text-stone-600">
            <span>Jumlah Bahan Perlu Dibeli:</span>
            <span className="font-bold text-stone-900">{needBuyItems.length} jenis</span>
          </div>
          {!isStaff && data.total_estimated !== undefined && (
            <div className="flex justify-between text-xs font-bold text-stone-950 border-t border-stone-200 pt-1">
              <span>Estimasi Kebutuhan Dana:</span>
              <span>{formatRupiah(data.total_estimated)}</span>
            </div>
          )}
          <div className="flex justify-between text-[11px] text-stone-600 border-t border-stone-200 pt-1">
            <span>Realisasi Kas Keluar:</span>
            <span className="font-mono text-stone-500">Rp ............................</span>
          </div>
          <div className="flex justify-between text-[11px] text-stone-600">
            <span>Sisa Kas / Lebih Bayar:</span>
            <span className="font-mono text-stone-500">Rp ............................</span>
          </div>
        </div>
      </div>

      {/* TANDA TANGAN SERAH TERIMA & BELANJA */}
      <div className="mt-8 grid grid-cols-2 gap-8 text-center text-xs">
        <div>
          <p className="font-medium text-stone-600">Petugas Belanja / Pengadaan,</p>
          <div className="h-14" />
          <p className="font-bold text-stone-900 underline">
            ( &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; )
          </p>
          <p className="text-[10px] text-stone-500">Nama Terang &amp; Tanda Tangan</p>
        </div>

        <div>
          <p className="font-medium text-stone-600">Kasir / Manajer Operasional,</p>
          <div className="h-14" />
          <p className="font-bold text-stone-900 underline">
            ( &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp; )
          </p>
          <p className="text-[10px] text-stone-500">Pemberi Dana Kas &amp; Verifikasi</p>
        </div>
      </div>

      {/* FOOTER */}
      <div className="mt-6 border-t border-stone-300 pt-2 text-center text-[10px] text-stone-400">
        Dicetak otomatis dari Sistem Kalkurasa Catering · Modul Pengadaan &amp; Belanja Bahan Baku (Bagian 8.4)
      </div>
    </section>
  );
}

