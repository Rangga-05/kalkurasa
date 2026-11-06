import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/components/providers";
import { SimulationBanner } from "@/components/simulation-banner";

export const metadata: Metadata = {
  title: "Kalkurasa — Kalkulasi + Rasa",
  description:
    "Sistem manajemen pesanan, kapasitas produksi, dan arus kas untuk UMKM kuliner berbasis pre-order.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id">
      <body className="flex min-h-screen flex-col overflow-x-hidden bg-stone-50 text-stone-900 antialiased">
        <Providers>
          <SimulationBanner />
          <div className="flex flex-1 flex-col">{children}</div>
        </Providers>
      </body>
    </html>
  );
}
