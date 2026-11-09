"use client";

import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "./button";

export interface PaginationProps {
  currentPage: number;
  totalPages: number;
  totalItems: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  itemName?: string;
  className?: string;
}

export function Pagination({
  currentPage,
  totalPages,
  totalItems,
  pageSize,
  onPageChange,
  itemName = "data",
  className = "",
}: PaginationProps) {
  if (totalItems <= 0 || totalPages <= 1) {
    return null;
  }

  const startItem = (currentPage - 1) * pageSize + 1;
  const endItem = Math.min(currentPage * pageSize, totalItems);

  // Generate page numbers to show
  const getPageNumbers = () => {
    const pages: (number | string)[] = [];
    if (totalPages <= 5) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      pages.push(1);
      if (currentPage > 3) {
        pages.push("...");
      }

      const start = Math.max(2, currentPage - 1);
      const end = Math.min(totalPages - 1, currentPage + 1);

      for (let i = start; i <= end; i++) {
        if (!pages.includes(i)) {
          pages.push(i);
        }
      }

      if (currentPage < totalPages - 2) {
        pages.push("...");
      }
      if (!pages.includes(totalPages)) {
        pages.push(totalPages);
      }
    }
    return pages;
  };

  const pages = getPageNumbers();

  return (
    <div
      className={`flex flex-col sm:flex-row items-center justify-between gap-3 border-t border-stone-200/80 pt-4 text-xs text-stone-600 ${className}`}
    >
      <div className="text-center sm:text-left">
        Menampilkan{" "}
        <strong className="font-semibold text-stone-900">{startItem}</strong> -{" "}
        <strong className="font-semibold text-stone-900">{endItem}</strong> dari{" "}
        <strong className="font-semibold text-stone-900">{totalItems}</strong>{" "}
        {itemName}
      </div>

      <div className="flex items-center gap-1.5">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onPageChange(currentPage - 1)}
          disabled={currentPage <= 1}
          className="h-8 gap-1 px-2.5 text-xs font-semibold cursor-pointer disabled:cursor-not-allowed"
          aria-label="Halaman sebelumnya"
        >
          <ChevronLeft className="h-3.5 w-3.5" />
          <span className="hidden xs:inline">Sebelumnya</span>
        </Button>

        {/* Desktop Page Numbers */}
        <div className="hidden sm:flex items-center gap-1">
          {pages.map((p, idx) => {
            if (p === "...") {
              return (
                <span
                  key={`ellipsis-${idx}`}
                  className="px-2 py-1 text-stone-400 select-none font-bold"
                >
                  …
                </span>
              );
            }
            const isCurrent = p === currentPage;
            return (
              <button
                type="button"
                key={p}
                onClick={() => onPageChange(Number(p))}
                className={`h-8 min-w-[32px] rounded-lg px-2 text-xs font-bold transition-all cursor-pointer ${
                  isCurrent
                    ? "bg-[#E05A47] text-white shadow-2xs"
                    : "border border-stone-200 bg-white text-stone-700 hover:bg-stone-100"
                }`}
                aria-current={isCurrent ? "page" : undefined}
              >
                {p}
              </button>
            );
          })}
        </div>

        {/* Mobile Page Indicator */}
        <div className="sm:hidden px-2 text-xs font-bold text-stone-700">
          {currentPage} / {totalPages}
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onPageChange(currentPage + 1)}
          disabled={currentPage >= totalPages}
          className="h-8 gap-1 px-2.5 text-xs font-semibold cursor-pointer disabled:cursor-not-allowed"
          aria-label="Halaman selanjutnya"
        >
          <span className="hidden xs:inline">Berikutnya</span>
          <ChevronRight className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

