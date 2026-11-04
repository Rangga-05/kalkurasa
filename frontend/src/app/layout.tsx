import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Kalkurasa — Kalkulasi + Rasa",
  description:
    "Sistem manajemen pesanan untuk UMKM kuliner berbasis pre-order.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id">
      <body className="min-h-screen bg-gray-50 text-gray-900 antialiased">
        {children}
        <footer className="py-4 text-center text-xs text-gray-400">
          Data simulasi — proyek GTNIC 2026
        </footer>
      </body>
    </html>
  );
}

