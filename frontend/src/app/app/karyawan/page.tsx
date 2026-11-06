import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function StaffDashboardSkeletonPage() {
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
            Dashboard Karyawan Dapur &amp; Kurir
          </h1>
          <p className="text-sm text-stone-500">
            Daftar tugas produksi, belanja, dan pengantaran hari ini (tanpa
            informasi harga jual, margin, maupun saldo kas).
          </p>
        </div>
        <Badge variant="amber">Kerangka Tahap 4A</Badge>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Tugas Operasional Hari Ini</CardTitle>
          <CardDescription>
            Pembaruan status produksi (Dalam Produksi → Siap → Diantar) dan
            pencatatan durasi aktual produksi.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-xs text-stone-500">
          Modul tugas operasional karyawan akan diaktifkan pada tahap berikutnya.
        </CardContent>
      </Card>
    </div>
  );
}

