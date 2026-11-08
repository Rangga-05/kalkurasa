import {
  formatDateShort,
  formatDateTimeWIB,
  formatNumber,
  formatRupiah,
  toNumber,
} from "./format";
import type { ShoppingListResult } from "./types";

export interface BuildWhatsAppShoppingTextOptions {
  data: ShoppingListResult;
  fromDate: string;
  toDate: string;
  isStaff?: boolean;
}

/**
 * Menghasilkan teks daftar belanja yang terformat indah dengan standar markdown WhatsApp
 * (*tebal*, _miring_, bullet points, dan emoji profesional).
 */
export function buildWhatsAppShoppingText({
  data,
  fromDate,
  toDate,
  isStaff = false,
}: BuildWhatsAppShoppingTextOptions): string {
  const items = data.items ?? [];
  const needBuy = items.filter((it) => toNumber(it.qty_buy_units) > 0);
  const nowStr = formatDateTimeWIB(new Date().toISOString());

  const lines: string[] = [];

  lines.push(`📋 *DAFTAR BELANJA BAHAN BAKU — KALKURASA CATERING*`);
  lines.push(`📅 *Periode Acara:* ${formatDateShort(fromDate)} s.d. ${formatDateShort(toDate)}`);
  lines.push(`📦 *Pesanan Terkait:* ${data.order_count} pesanan aktif`);
  lines.push(`⏰ *Dicatat Pada:* ${nowStr}`);
  lines.push(`----------------------------------------`);
  lines.push(``);

  if (needBuy.length === 0) {
    lines.push(`✅ *Semua stok bahan saat ini mencukupi.*`);
    lines.push(`Tidak ada bahan baku yang perlu dibeli untuk periode ini.`);
  } else {
    lines.push(`🛒 *BAHAN BAKU YANG HARUS DIBELI (${needBuy.length} Item):*`);
    lines.push(``);

    needBuy.forEach((it, idx) => {
      const buyUnits = formatNumber(it.qty_buy_units);
      const isPack = it.rounding_rule === "pack";
      const buySize = toNumber(it.buy_unit_size);

      lines.push(`${idx + 1}. *${it.ingredient_name}*`);
      if (isPack && buySize > 1) {
        lines.push(`   • *Beli:* ${buyUnits} ${it.buy_unit} (${formatNumber(it.buy_use_units)} ${it.use_unit})`);
      } else {
        lines.push(`   • *Beli:* ${buyUnits} ${it.buy_unit}`);
      }
      lines.push(`   • *Kebutuhan Resep:* ${formatNumber(it.required_qty)} ${it.use_unit}`);
      lines.push(`   • *Stok Gudang:* ${formatNumber(it.stock)} ${it.use_unit}`);

      if (!isStaff && it.estimated_cost !== undefined) {
        lines.push(`   • *Est. Biaya:* ${formatRupiah(it.estimated_cost)}`);
      }
      lines.push(``);
    });

    if (!isStaff && data.total_estimated !== undefined) {
      lines.push(`----------------------------------------`);
      lines.push(`💰 *TOTAL ESTIMASI BELANJA: ${formatRupiah(data.total_estimated)}*`);
      lines.push(`----------------------------------------`);
    }
  }

  lines.push(``);
  lines.push(`*Catatan Petugas Belanja:*`);
  lines.push(`1. Pastikan kemasan utuh, segel tidak rusak, dan tanggal kedaluwarsa aman.`);
  lines.push(`2. Simpan nota/struk belanja asli untuk pencatatan kas keluar & stok.`);
  lines.push(`_Dihasilkan otomatis oleh Sistem Kalkurasa Catering_`);

  return lines.join("\n");
}

/**
 * Membuat URL direct kirim pesan WhatsApp (mendukung web & mobile)
 */
export function getWhatsAppShareUrl(text: string, phone = ""): string {
  const cleanPhone = phone.replace(/\D/g, "");
  const base = cleanPhone
    ? `https://api.whatsapp.com/send?phone=${cleanPhone}`
    : `https://api.whatsapp.com/send`;
  return `${base}?text=${encodeURIComponent(text)}`;
}

