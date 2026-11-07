import Link from "next/link";
import { ArrowLeft, Clock } from "lucide-react";

export function ComingSoonPage({
  title,
  stageLabel,
  description,
}: {
  title: string;
  stageLabel: string;
  description: string;
}) {
  return (
    <div className="mx-auto max-w-2xl py-10">
      <div className="rounded-3xl border border-stone-200 bg-white p-8 shadow-xs">
        <div className="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
          <Clock className="h-3.5 w-3.5" />
          Segera Hadir • {stageLabel}
        </div>
        <h1 className="mt-4 text-2xl font-bold text-stone-900">{title}</h1>
        <p className="mt-2 text-sm leading-relaxed text-stone-600">
          {description}
        </p>
        <div className="mt-6 flex flex-wrap items-center gap-3 border-t border-stone-100 pt-5">
          <Link
            href="/app/pesanan"
            className="inline-flex items-center gap-2 rounded-xl bg-amber-600 px-4 py-2 text-xs font-semibold text-white transition hover:bg-amber-700"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Buka Daftar Pesanan
          </Link>
          <Link
            href="/app/kapasitas"
            className="inline-flex items-center gap-2 rounded-xl border border-stone-200 px-4 py-2 text-xs font-semibold text-stone-700 transition hover:bg-stone-50"
          >
            Lihat Kalender Kapasitas
          </Link>
        </div>
      </div>
    </div>
  );
}

