"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";

/**
 * Memastikan scroll selalu kembali ke puncak (top) setiap kali berpindah rute/halaman.
 */
export function ScrollToTopOnNavigate() {
  const pathname = usePathname();

  useEffect(() => {
    if (typeof window !== "undefined") {
      // Reset scroll window & document
      window.scrollTo({ top: 0, left: 0, behavior: "instant" });
      document.documentElement.scrollTo?.({ top: 0, left: 0, behavior: "instant" });
      document.body.scrollTo?.({ top: 0, left: 0, behavior: "instant" });

      // Reset scroll container utama jika ada overflow lokal
      const mainEl =
        document.querySelector("main") ||
        document.getElementById("main-content");
      if (mainEl) {
        mainEl.scrollTo?.({ top: 0, left: 0, behavior: "instant" });
      }
    }
  }, [pathname]);

  return null;
}

