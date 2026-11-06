import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function ManagerDashboardSkeletonPage() {
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
            Dashboard Manajer Operasional
          </h1>
          <p className="text-sm text-stone-500">
            Kalender kapasitas harian, antrian keputusan di bawah ambang pesanan
            besar, jadwal produksi, dan daftar belanja.
          </p>
        </div>
        <Badge variant="amber">Kerangka Tahap 4A</Badge>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Kalender Kapasitas &amp; Antrian Keputusan</CardTitle>
            <CardDescription>
              Pemantauan beban oven dan tenaga kerja beserta persetujuan pesanan
              harian.
            </CardDescription>
          </CardHeader>
          <CardContent className="text-xs text-stone-500">
            Modul interaktif penuh akan diaktifkan pada Tahap 4B.
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Jadwal Produksi, Belanja &amp; Tenggat DP</CardTitle>
            <CardDescription>
              Peringatan bahan di bawah stok minimum dan pesanan yang mendekati
              tenggat pembayaran uang muka.
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

