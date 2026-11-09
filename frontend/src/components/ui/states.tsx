import * as React from "react";
import { AlertCircle, ChefHat, RefreshCw } from "lucide-react";
import { Button } from "./button";
import { Skeleton } from "./skeleton";
import { cn } from "@/lib/utils";

export interface EmptyStateProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export function EmptyState({
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-[var(--border-strong)] bg-[var(--bg-subtle)]/60 p-8 text-center sm:p-10",
        className
      )}
    >
      <div className="mb-3.5 flex h-12 w-12 items-center justify-center rounded-2xl border border-[var(--border-warm)] bg-[var(--bg-surface)] text-[#C2410C] shadow-xs">
        <ChefHat className="h-6 w-6" aria-hidden="true" />
      </div>
      <h3 className="font-display text-base font-bold text-[var(--ink-primary)]">
        {title}
      </h3>
      {description ? (
        <p className="mt-1.5 max-w-md text-xs leading-relaxed text-[var(--ink-secondary)] sm:text-sm">
          {description}
        </p>
      ) : null}
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}

export interface ErrorStateProps {
  title?: string;
  message: string;
  onRetry?: () => void;
  className?: string;
}

export function ErrorState({
  title = "Koneksi Dapur Sedang Terkendala",
  message,
  onRetry,
  className,
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center rounded-2xl border border-rose-300 bg-rose-50/90 p-6 text-center text-rose-950 dark:border-rose-800 dark:bg-rose-950/50 dark:text-rose-100 sm:p-8",
        className
      )}
    >
      <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-2xl bg-rose-100 text-rose-700 dark:bg-rose-900/60 dark:text-rose-300">
        <AlertCircle className="h-5 w-5" aria-hidden="true" />
      </div>
      <h3 className="font-display text-base font-bold">{title}</h3>
      <p className="mt-1.5 max-w-md text-xs leading-relaxed text-rose-900 dark:text-rose-200 sm:text-sm">
        {message}
      </p>
      {onRetry ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onRetry}
          className="mt-4 border-rose-300 bg-white text-rose-900 hover:bg-rose-50 dark:border-rose-700 dark:bg-rose-950 dark:text-rose-100"
        >
          <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
          Muat Ulang Data
        </Button>
      ) : null}
    </div>
  );
}

export function TableSkeleton({
  rows = 4,
  className,
}: {
  rows?: number;
  className?: string;
}) {
  return (
    <div
      role="status"
      aria-label="Sedang memuat daftar data..."
      className={cn("space-y-3", className)}
    >
      {Array.from({ length: rows }).map((_, idx) => (
        <div
          key={idx}
          className="flex items-center justify-between gap-4 rounded-2xl border border-[var(--border-warm)] bg-[var(--bg-surface)] p-4"
        >
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-2/5" />
            <Skeleton className="h-3 w-3/5" />
          </div>
          <Skeleton className="h-8 w-24" />
        </div>
      ))}
      <span className="sr-only">Memuat data dapur dan pesanan...</span>
    </div>
  );
}
