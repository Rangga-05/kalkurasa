import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function OwnerDashboardSkeletonPage() {
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
            Dashboard Pemilik Usaha (Owner)
          </h1>
          <p className="text-sm text-stone-500">
            Ringkasan antrian keputusan, proyeksi arus kas 14 hari, margin, dan
            kalender kapasitas.
          </p>
        </div>
        <Badge variant="amber">Kerangka Tahap 4A</Badge>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Antrian Keputusan &amp; Pratinjau Dampak</CardTitle>
            <CardDescription>
              Modul keputusan penuh (termasuk pesanan besar &amp; override) akan
              dihubungkan pada Tahap 4B.
            </CardDescription>
          </CardHeader>
          <CardContent className="text-xs text-stone-500">
            Menampilkan rekomendasi gerbang G1–G4, perubahan utilisasi, dan
            titik terendah kas sebelum/sesudah menerima pesanan.
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Proyeksi Kas 14 Hari &amp; Kalender Kapasitas</CardTitle>
            <CardDescription>
              Grafik proyeksi kas harian terhadap buffer kas minimum dan
              indikator beban produksi.
            </CardDescription>
          </CardHeader>
          <CardContent className="text-xs text-stone-500">
            Akan diaktifkan pada tahap modul internal berikutnya.
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

