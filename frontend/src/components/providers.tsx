"use client";

import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CheckCircle2, AlertCircle, Info, X } from "lucide-react";
import { cn } from "@/lib/utils";

export type ToastVariant = "success" | "error" | "info";

export interface ToastItem {
  id: string;
  title: string;
  description?: string;
  variant: ToastVariant;
}

interface ToastContextValue {
  toast: (opts: {
    title: string;
    description?: string;
    variant?: ToastVariant;
  }) => void;
  success: (title: string, description?: string) => void;
  error: (title: string, description?: string) => void;
  info: (title: string, description?: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast harus dipanggil di dalam Providers");
  }
  return ctx;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 120_000,
            gcTime: 600_000,
            retry: 1,
            refetchOnWindowFocus: false,
            refetchOnReconnect: false,
          },
        },
      })
  );

  // Pastikan class dark sisa sesi lama dibersihkan sepenuhnya
  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.classList.remove("dark");
    }
    try {
      window.localStorage.removeItem("kalkurasa-theme");
    } catch {
      // Ignore storage errors
    }
  }, []);

  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const toast = useCallback(
    ({
      title,
      description,
      variant = "info",
    }: {
      title: string;
      description?: string;
      variant?: ToastVariant;
    }) => {
      const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
      setToasts((prev) => [...prev, { id, title, description, variant }]);
      setTimeout(() => {
        dismiss(id);
      }, 4500);
    },
    [dismiss]
  );

  const toastValue = useMemo<ToastContextValue>(
    () => ({
      toast,
      success: (title, description) =>
        toast({ title, description, variant: "success" }),
      error: (title, description) =>
        toast({ title, description, variant: "error" }),
      info: (title, description) =>
        toast({ title, description, variant: "info" }),
    }),
    [toast]
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ToastContext.Provider value={toastValue}>
        {children}
        <div
          aria-live="polite"
          aria-atomic="true"
          className="pointer-events-none fixed right-3 bottom-3 z-50 flex w-full max-w-sm flex-col gap-2.5 px-3 sm:right-5 sm:bottom-5 sm:px-0"
        >
          {toasts.map((t) => (
            <div
              key={t.id}
              role="status"
              className={cn(
                "pointer-events-auto flex items-start gap-3 rounded-2xl border-2 border-[#12100E] p-4 shadow-[4px_4px_0px_#12100E] transition-all",
                t.variant === "success" && "bg-[#ECFCCB] text-[#12100E]",
                t.variant === "error" && "bg-[#FFE4E6] text-[#9F1239]",
                t.variant === "info" && "bg-[#FFFDF9] text-[#12100E]"
              )}
            >
              {t.variant === "success" ? (
                <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-[#15803D]" />
              ) : t.variant === "error" ? (
                <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-[#BE123C]" />
              ) : (
                <Info className="mt-0.5 h-5 w-5 shrink-0 text-[#C2410C]" />
              )}
              <div className="min-w-0 flex-1">
                <p className="text-sm font-extrabold">{t.title}</p>
                {t.description ? (
                  <p className="mt-0.5 text-xs font-medium leading-relaxed opacity-90">
                    {t.description}
                  </p>
                ) : null}
              </div>
              <button
                type="button"
                onClick={() => dismiss(t.id)}
                className="rounded-lg p-1.5 opacity-75 transition-opacity hover:bg-black/5 hover:opacity-100"
                aria-label="Tutup notifikasi"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      </ToastContext.Provider>
    </QueryClientProvider>
  );
}
