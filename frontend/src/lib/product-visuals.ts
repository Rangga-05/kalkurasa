import type { Product } from "./types";

export interface ProductVisualMeta {
  illustration: string;
  tagline: string;
  story: string;
  recipeHighlights: string[];
  ovenHoursPerUnit: number;
  laborMinsPerUnit: number;
  accentColor: string;
  accentBg: string;
  badgeText: string;
  // Slight Twist Color-Block Theme properties
  flavorNumber: string;
  twistWord: string;
  stageBg: string;
  cardHeaderBg: string;
  stickerBg: string;
  stickerRotation: string;
  pairingNote: string;
}

const VISUAL_MAP: Record<string, ProductVisualMeta> = {
  nasi_box: {
    illustration: "/assets/foods/nasi-box-photo.jpg",
    tagline: "Nasi pandan harum di atas daun pisang dengan ayam bakar bumbu rempah.",
    story:
      "Dimasak pagi hari sebelum jam pengiriman: nasi putih pandan hangat, ayam ungkep bakar rempah karamel, telur balado merah segar, tempe orek renyah, dan lalapan kemangi.",
    recipeHighlights: [
      "Beras pandan wangi + taburan bawang goreng renyah",
      "Ayam bakar ungkep 9 rempah & sambal terasi tomat segar",
      "Telur balado utuh, tempe orek manis, dan alas daun pisang",
    ],
    ovenHoursPerUnit: 0.14,
    laborMinsPerUnit: 12,
    accentColor: "#C2410C",
    accentBg: "#FFEDD5",
    badgeText: "Hidangan Utama · Panas Segar",
    flavorNumber: "NO. 01",
    twistWord: "Rempah Karamel",
    stageBg: "#FFEDD5",
    cardHeaderBg: "#FED7AA",
    stickerBg: "#FDE047",
    stickerRotation: "-rotate-2",
    pairingNote: "Ideal untuk rapat kantor, syukuran, & jamuan makan siang (min. 15 box)",
  },
  snack_box: {
    illustration: "/assets/foods/snack-box-photo.jpg",
    tagline: "Kotak kudapan hangat: pastel renyah, risoles ragout, dan bolu pandan.",
    story:
      "Diracik segar setiap subuh menggunakan kulit pastry berlapis renyah, isian ragout ayam wortel susu, serta bolu pandan perasan daun suji asli tanpa perisa sintetis.",
    recipeHighlights: [
      "Pastel tutup kulit renyah isi telur & bihun bumbu lada",
      "Risoles ragout ayam kampung creamy berbalut tepung roti emas",
      "Bolu chiffon pandan suji lembut & cheese roll puff pastry",
    ],
    ovenHoursPerUnit: 0.12,
    laborMinsPerUnit: 10,
    accentColor: "#15803D",
    accentBg: "#ECFCCB",
    badgeText: "Best Seller · Rapat & Acara",
    flavorNumber: "NO. 02",
    twistWord: "Pandan & Ragout",
    stickerBg: "#D9F99D",
    stageBg: "#ECFCCB",
    cardHeaderBg: "#D9F99D",
    stickerRotation: "rotate-2",
    pairingNote: "Pas untuk seminar pagi, pengajian, & coffee break korporat",
  },
  kue_basah: {
    illustration: "/assets/foods/tart-ulang-tahun-photo.jpg",
    tagline: "Sponge cake lembut berlapis dark chocolate ganache & krim vanila.",
    story:
      "Dipanggang dengan teknik water-bath untuk menjaga kelembutan remah kue, disiram lelehan cokelat hitam 70% kakao, dan dihias buah stroberi serta bluberi segar.",
    recipeHighlights: [
      "Dark chocolate couverture 70% & ekstrak vanila alami",
      "Tiga lapis sponge cake lembut dengan buttercream ringan",
      "Topping buah beri segar, serutan cokelat, & papan ucapan",
    ],
    ovenHoursPerUnit: 0.85,
    laborMinsPerUnit: 45,
    accentColor: "#BE123C",
    accentBg: "#FFE4E6",
    badgeText: "Signature Tart · Custom Ucapan",
    flavorNumber: "NO. 03",
    twistWord: "Ganache Beri",
    stageBg: "#FFE4E6",
    cardHeaderBg: "#FECDD3",
    stickerBg: "#FDE047",
    stickerRotation: "-rotate-2",
    pairingNote: "Pusat perayaan ulang tahun, anniversary, & hantaran spesial",
  },
  cupcake: {
    illustration: "/assets/foods/cupcake-box-photo.jpg",
    tagline: "Enam cupcake panggang segar dengan swirl buttercream tiga rasa.",
    story:
      "Setiap kotak memadukan 3 profil rasa favorit dapur kami: Berry Vanilla Bean, Double Belgian Fudge, dan Salted Caramel Pandan dengan tekstur kubah kue yang empuk.",
    recipeHighlights: [
      "Buttercream kocok tangan yang ringan dan tidak meninggalkan rasa getir",
      "3 varian rasa dalam 1 kotak (Vanilla Berry, Choco Fudge, Caramel)",
      "Kemasan kotak jendela bersekat aman untuk perjalanan kurir",
    ],
    ovenHoursPerUnit: 0.35,
    laborMinsPerUnit: 25,
    accentColor: "#B45309",
    accentBg: "#FEF3C7",
    badgeText: "Kotak Isi 6 · Tiga Rasa",
    flavorNumber: "NO. 04",
    twistWord: "Tiga Swirl",
    stageBg: "#FEF3C7",
    cardHeaderBg: "#FDE68A",
    stickerBg: "#D9F99D",
    stickerRotation: "rotate-2",
    pairingNote: "Hadiah manis praktis untuk sahabat, kolega, & pesta anak",
  },
  kue_kering: {
    illustration: "/assets/foods/kue-kering-nastar-photo.jpg",
    tagline: "Nastar klasik berpoles kuning telur emas dengan selai nanas kayu manis.",
    story:
      "Selai nanas Palembang dimasak perlahan selama 4 jam bersama cengkeh dan kayu manis, lalu dibalut adonan mentega Wisjman yang lumer begitu digigit.",
    recipeHighlights: [
      "Selai nanas madu masak lambat 4 jam dengan rempah cengkeh",
      "Kulit nastar mentega lembut berpoles ganda kuning telur omega",
      "Stoples kedap udara menjaga kerenyahan hingga 30 hari",
    ],
    ovenHoursPerUnit: 0.65,
    laborMinsPerUnit: 35,
    accentColor: "#854D0E",
    accentBg: "#FEF9C3",
    badgeText: "Stoples Premium · Tahan 30 Hari",
    flavorNumber: "NO. 05",
    twistWord: "Nanas Rempah",
    stageBg: "#FEF9C3",
    cardHeaderBg: "#FEF08A",
    stickerBg: "#FED7AA",
    stickerRotation: "-rotate-1",
    pairingNote: "Sajian meja tamu, hampers hari raya, & stok camilan toples",
  },
  pastry: {
    illustration: "/assets/foods/brownies-fudgy-photo.jpg",
    tagline: "Fudgy dark chocolate brownies dengan shiny crust & irisan almond panggang.",
    story:
      "Menggunakan lelehan dark chocolate couverture dan mentega tawar yang dikocok hangat bersama gula palem sehingga menghasilkan lapisan atas berkilau renyah dan bagian tengah pekat.",
    recipeHighlights: [
      "Shiny papery crust alami dari kocokan telur & gula hangat",
      "Tekstur fudgy padat dengan lelehan cokelat hitam pekat",
      "Taburan kacang almond panggang & sejumput sea salt",
    ],
    ovenHoursPerUnit: 0.45,
    laborMinsPerUnit: 20,
    accentColor: "#6B21A8",
    accentBg: "#F3E8FF",
    badgeText: "Panggang Loyang · Cokelat Pekat",
    flavorNumber: "NO. 06",
    twistWord: "Shiny Fudge",
    stageBg: "#F3E8FF",
    cardHeaderBg: "#E9D5FF",
    stickerBg: "#D9F99D",
    stickerRotation: "rotate-2",
    pairingNote: "Teman kopi sore, oleh-oleh keluarga, & pencuci mulut rapat",
  },
};

export const SIGNATURE_SHOWCASE_ITEMS: Array<{
  key: string;
  name: string;
  categoryLabel: string;
  price: number;
  unit: string;
  minOrder: number;
  meta: ProductVisualMeta;
}> = [
  {
    key: "nasi_box",
    name: "Nasi Box Ayam Bakar Rempah",
    categoryLabel: "Nasi Kotak Nusantara",
    price: 32000,
    unit: "box",
    minOrder: 15,
    meta: VISUAL_MAP.nasi_box,
  },
  {
    key: "snack_box",
    name: "Snack Box Pasar Pagi (Isi 4)",
    categoryLabel: "Snack Box Artisan",
    price: 22500,
    unit: "box",
    minOrder: 20,
    meta: VISUAL_MAP.snack_box,
  },
  {
    key: "kue_basah",
    name: "Tart Ulang Tahun Dark Ganache",
    categoryLabel: "Celebration Tart",
    price: 245000,
    unit: "loyang 20cm",
    minOrder: 1,
    meta: VISUAL_MAP.kue_basah,
  },
  {
    key: "cupcake",
    name: "Cupcake Box Tiga Swirl (Isi 6)",
    categoryLabel: "Gourmet Cupcakes",
    price: 115000,
    unit: "box isi 6",
    minOrder: 2,
    meta: VISUAL_MAP.cupcake,
  },
  {
    key: "kue_kering",
    name: "Nastar Klasik Wisjman & Cengkeh",
    categoryLabel: "Kue Kering Stoples",
    price: 135000,
    unit: "stoples 500g",
    minOrder: 2,
    meta: VISUAL_MAP.kue_kering,
  },
  {
    key: "pastry",
    name: "Fudgy Brownies Almond Sea Salt",
    categoryLabel: "Artisan Pastry",
    price: 95000,
    unit: "loyang 22x10cm",
    minOrder: 2,
    meta: VISUAL_MAP.pastry,
  },
];

export function getProductVisualMeta(
  productOrName: Pick<Product, "name" | "category"> | string,
  maybeCategory?: string
): ProductVisualMeta {
  const nameLower = (
    typeof productOrName === "string" ? productOrName : productOrName.name || ""
  ).toLowerCase();
  const catLower = (
    typeof productOrName === "string"
      ? maybeCategory || ""
      : productOrName.category || ""
  ).toLowerCase();

  if (nameLower.includes("nasi") || catLower.includes("nasi")) {
    return VISUAL_MAP.nasi_box;
  }
  if (nameLower.includes("cupcake")) {
    return VISUAL_MAP.cupcake;
  }
  if (
    nameLower.includes("tart") ||
    nameLower.includes("ulang tahun") ||
    nameLower.includes("cake")
  ) {
    return VISUAL_MAP.kue_basah;
  }
  if (
    nameLower.includes("nastar") ||
    nameLower.includes("kastengel") ||
    nameLower.includes("putri salju") ||
    catLower.includes("kering")
  ) {
    return VISUAL_MAP.kue_kering;
  }
  if (
    nameLower.includes("brownies") ||
    nameLower.includes("bolen") ||
    nameLower.includes("croissant") ||
    catLower.includes("pastry")
  ) {
    return VISUAL_MAP.pastry;
  }
  if (nameLower.includes("snack") || catLower.includes("snack")) {
    return VISUAL_MAP.snack_box;
  }

  return VISUAL_MAP[catLower] ?? VISUAL_MAP.nasi_box;
}
