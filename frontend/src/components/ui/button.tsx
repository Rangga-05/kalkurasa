import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "twist-btn inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-full text-sm font-extrabold tracking-wide uppercase transition-all focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#C2410C] disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer",
  {
    variants: {
      variant: {
        default:
          "bg-[#C2410C] text-white hover:bg-[#9A3412] active:bg-[#7C2D12]",
        secondary:
          "bg-[#D9F99D] text-[#12100E] hover:bg-[#FDE047]",
        outline:
          "bg-[var(--bg-surface)] text-[var(--ink-primary)] hover:bg-[#FFEDD5] hover:text-[#12100E]",
        destructive:
          "bg-[#BE123C] text-white hover:bg-[#9F1239]",
        success:
          "bg-[#15803D] text-white hover:bg-[#166534]",
        ghost:
          "border-transparent shadow-none text-[var(--ink-secondary)] hover:bg-[var(--bg-subtle)] hover:text-[var(--ink-primary)]",
        link: "border-transparent shadow-none text-[#C2410C] dark:text-orange-400 underline-offset-4 hover:underline",
      },
      size: {
        default: "min-h-[44px] sm:min-h-[42px] px-5 py-2",
        sm: "min-h-[38px] px-3.5 py-1.5 text-xs",
        lg: "min-h-[50px] px-7 py-3 text-sm",
        icon: "h-10 w-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => {
    return (
      <button
        ref={ref}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      />
    );
  }
);
Button.displayName = "Button";

export { buttonVariants };
