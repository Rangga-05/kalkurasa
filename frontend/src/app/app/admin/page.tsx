import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";

export default function AdminDashboardSkeletonPage() {
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-stone-900 sm:text-2xl">
            Dashboard Administrator Sistem
          </h1>
          <p className="text-sm text-stone-500">
            Manajemen pengguna &amp; peran, master data katalog, log audit, dan
            kontrol data simulasi.
          </p>
        </div>
        <Badge variant="amber">Kerangka Tahap 4A</Badge>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Pengguna, Peran &amp; Log Audit</CardTitle>
            <CardDescription>
              Pengelolaan akun internal serta penelusuran jejak audit perubahan
              data.
            </CardDescription>
          </CardHeader>
          <CardContent className="text-xs text-stone-500">
            Modul administrasi penuh akan diaktifkan pada tahap berikutnya.
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Master Data &amp; Kontrol Simulasi Demo</CardTitle>
            <CardDescription>
              Pengaturan produk, bahan baku, resep, sumber daya, dan tanggal
              simulasi demo.
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

