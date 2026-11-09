import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center gap-1.5 rounded-full border-2 border-[#12100E] px-3 py-0.5 text-xs font-extrabold tracking-tight shadow-[2px_2px_0px_#12100E] transition-colors",
  {
    variants: {
      variant: {
        default: "bg-[#FFEDD5] text-[#12100E]",
        emerald: "bg-[#D9F99D] text-[#12100E]",
        amber: "bg-[#FDE047] text-[#12100E]",
        sky: "bg-[#E0F2FE] text-[#12100E]",
        indigo: "bg-[#F3E8FF] text-[#12100E]",
        rose: "bg-[#FFE4E6] text-[#9F1239]",
        slate: "bg-[var(--bg-subtle)] text-[var(--ink-primary)]",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ variant }), className)} {...props} />
  );
}
