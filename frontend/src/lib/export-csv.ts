import { getOrderStatusMeta, toNumber } from "./format";
import type { OrderDetail } from "./types";

/**
 * Mengubah string menjadi nilai sel CSV yang aman sesuai standar RFC 4180.
 * Karakter tanda kutip digandakan (""), dan jika terdapat koma, tanda kutip,
 * atau baris baru maka seluruh sel dibungkus dengan tanda kutip ganda.
 */
function escapeCSVCell(val: unknown): string {
  if (val === null || val === undefined) return "";
  const str = String(val);
  if (/[",\n\r]/.test(str)) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  return str;
}

/**
 * Mengekspor daftar pesanan ke dalam format berkas CSV (Excel-Compatible UTF-8 BOM).
 * Dapat dibuka langsung oleh Microsoft Excel, Google Sheets, LibreOffice Calc, dan Numbers.
 *
 * @param orders Daftar OrderDetail yang akan diekspor
 * @param filenamePrefix Awalan nama berkas (default: "kalkurasa-laporan-pesanan")
 */
export function exportOrdersToCSV(
  orders: OrderDetail[],
  filenamePrefix = "kalkurasa-laporan-pesanan"
): { success: boolean; count: number; filename: string } {
  if (!orders || orders.length === 0) {
    return { success: false, count: 0, filename: "" };
  }

  const headers = [
    "No",
    "Kode Pesanan",
    "Sumber",
    "Nama Pelanggan",
    "Kontak Pelanggan",
    "Metode Penyerahan",
    "Alamat",
    "Tanggal Acara",
    "Tanggal Produksi",
    "Status Pesanan",
    "Rincian Menu",
    "Total Unit",
    "Subtotal (Rp)",
    "Tarif PPN",
    "Nilai PPN (Rp)",
    "Biaya Layanan Aplikasi (Rp)",
    "Total Tagihan (Rp)",
    "Minimal DP (Rp)",
    "Total Terbayar (Rp)",
    "Sisa Tagihan (Rp)",
    "Status Pelunasan",
    "Catatan Khusus",
  ];

  const rows: string[][] = [headers];

  orders.forEach((ord, index) => {
    const statusMeta = getOrderStatusMeta(ord.status);
    const subtotalNum = ord.subtotal ? toNumber(ord.subtotal) : 0;
    const taxPctNum = ord.tax_pct ? toNumber(ord.tax_pct) * 100 : 0;
    const taxAmountNum = ord.tax_amount ? toNumber(ord.tax_amount) : 0;
    const platformFeeNum = ord.platform_fee ? toNumber(ord.platform_fee) : 0;
    const totalNum = toNumber(ord.total);
    const dpRequiredNum = toNumber(ord.dp_required);
    const paidNum = toNumber(ord.paid);
    const remainingNum = Math.max(0, totalNum - paidNum);
    const isLunas = remainingNum <= 0;

    const itemsSummary = (ord.items || [])
      .map((it) => `${it.product_name} (${it.qty} unit)`)
      .join("; ");

    rows.push([
      String(index + 1),
      `#${ord.code}`,
      ord.source === "manual" ? "Manual / Internal" : "Formulir Publik",
      ord.customer_name,
      ord.customer_contact,
      ord.delivery_type === "delivery" ? "Diantar ke Alamat" : "Ambil Sendiri di Dapur",
      ord.address || "-",
      ord.event_date,
      ord.production_date,
      statusMeta.label,
      itemsSummary,
      String(ord.total_units || 0),
      String(subtotalNum),
      `${taxPctNum}%`,
      String(taxAmountNum),
      String(platformFeeNum),
      String(totalNum),
      String(dpRequiredNum),
      String(paidNum),
      String(remainingNum),
      isLunas ? "LUNAS" : "BELUM LUNAS",
      ord.notes || "-",
    ]);
  });

  // Konversi baris menjadi teks CSV
  const csvContent = rows
    .map((row) => row.map(escapeCSVCell).join(","))
    .join("\r\n");

  // Sisipkan UTF-8 BOM (\uFEFF) agar Microsoft Excel langsung membaca aksen & karakter Indonesia secara tepat
  const blob = new Blob(["\uFEFF" + csvContent], {
    type: "text/csv;charset=utf-8;",
  });

  const now = new Date();
  const dateStr = now.toISOString().slice(0, 10);
  const timeStr = `${String(now.getHours()).padStart(2, "0")}${String(now.getMinutes()).padStart(2, "0")}`;
  const filename = `${filenamePrefix}-${dateStr}-${timeStr}.csv`;

  // Picu pengunduhan berkas di browser
  const link = document.createElement("a");
  const url = URL.createObjectURL(blob);
  link.setAttribute("href", url);
  link.setAttribute("download", filename);
  link.style.visibility = "hidden";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);

  return { success: true, count: orders.length, filename };
}

