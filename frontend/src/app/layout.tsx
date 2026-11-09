import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/components/providers";

export const metadata: Metadata = {
  title: "Kalkurasa : Dapur Pre-Order & Kalkulasi Rasa Presisi",
  description:
    "Platform kuliner pre-order dengan evaluasi kapasitas oven, margin resep, dan ketahanan kas 4-Gerbang.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id" suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          rel="stylesheet"
          href="https://fonts.googleapis.com/css2?family=Bricolage+Grotesque:opsz,wght@12..96,600;12..96,700;12..96,800&family=Fraunces:ital,opsz,wght@0,9..144,600;0,9..144,700;1,9..144,600;1,9..144,700&family=JetBrains+Mono:wght@500;600;700&family=Plus+Jakarta+Sans:wght@400;500;600;700;800&display=swap"
        />
      </head>
      <body className="flex min-h-screen flex-col overflow-x-hidden bg-[var(--bg-canvas)] font-sans text-[var(--ink-primary)] antialiased transition-colors duration-200">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-full focus:border-2 focus:border-[#12100E] focus:bg-[#D9F99D] focus:px-5 focus:py-2.5 focus:text-sm focus:font-bold focus:text-[#12100E] focus:shadow-[4px_4px_0px_#12100E]"
        >
          Lewati ke konten utama
        </a>
        <Providers>
          <div id="main-content" className="flex flex-1 flex-col">
            {children}
          </div>
        </Providers>
      </body>
    </html>
  );
}
