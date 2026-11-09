import * as React from "react";
import { cn } from "@/lib/utils";

export type InputProps = React.InputHTMLAttributes<HTMLInputElement>;

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = "text", ...props }, ref) => {
    return (
      <input
        type={type}
        ref={ref}
        className={cn(
          "flex min-h-[44px] sm:min-h-[42px] w-full rounded-2xl border-2 border-[var(--ink-border)] bg-[var(--bg-surface)] px-4 py-2 text-sm font-semibold text-[var(--ink-primary)] placeholder:font-normal placeholder:text-[var(--ink-muted)] shadow-[3px_3px_0px_var(--ink-shadow)] transition-all focus-visible:border-[#C2410C] focus-visible:ring-2 focus-visible:ring-[#C2410C]/25 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-[var(--bg-subtle)] disabled:opacity-60",
          className
        )}
        {...props}
      />
    );
  }
);
Input.displayName = "Input";
