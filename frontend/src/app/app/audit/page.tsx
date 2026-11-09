"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ChevronLeft,
  ChevronRight,
  FileSpreadsheet,
  Filter,
  RefreshCw,
  RotateCcw,
  ShieldAlert,
} from "lucide-react";
import { getAuditLogs, getErrorMessage } from "@/lib/api";
import {
  AUDIT_ACTION_LABELS,
  AUDIT_ENTITY_LABELS,
  formatDateTimeWIB,
  ROLE_LABELS,
} from "@/lib/format";
import type { AuditLogItem, UserRole } from "@/lib/types";
import { EmptyState, ErrorState, TableSkeleton } from "@/components/ui/states";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

const ACTION_OPTIONS = [
  "create",
  "decision",
  "override",
  "counter_response",
  "payment",
  "status_change",
  "cancel",
  "expire",
  "purchase",
  "stock_adjust",
  "cash_entry",
  "settings_update",
  "master_change",
];

const ENTITY_OPTIONS = [
  "orders",
  "tasks",
  "ingredients",
  "products",
  "recipe_items",
  "product_loads",
  "resources",
  "settings",
  "users",
  "cash_entries",
  "purchases",
];

const ROLE_OPTIONS: { value: string; label: string }[] = [
  { value: "owner", label: "Pemilik Usaha (Owner)" },
  { value: "manager", label: "Manajer Operasional" },
  { value: "staff", label: "Karyawan Dapur & Kurir" },
  { value: "admin", label: "Administrator Sistem" },
  { value: "customer", label: "Pelanggan" },
  { value: "public", label: "Publik / Sistem Otomatis" },
];

export default function AuditLogPage() {
  const [actionFilter, setActionFilter] = useState("");
  const [roleFilter, setRoleFilter] = useState("");
  const [entityFilter, setEntityFilter] = useState("");
  const [fromFilter, setFromFilter] = useState("");
  const [toFilter, setToFilter] = useState("");

  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);

  const [rawItems, setRawItems] = useState<AuditLogItem[]>([]);
  const [serverTotal, setServerTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const loadAudits = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      // Ketika filter peran aktif, ambil hingga 200 baris dari server agar penyaringan peran & paginasi akurat
      const fetchPage = roleFilter ? 1 : page;
      const fetchLimit = roleFilter ? 200 : limit;
      const res = await getAuditLogs({
        action: actionFilter || undefined,
        entity: entityFilter || undefined,
        from: fromFilter || undefined,
        to: toFilter || undefined,
        page: fetchPage,
        limit: fetchLimit,
      });
      setRawItems(res.data);
      setServerTotal(res.meta.total);
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat data Log Audit sistem."));
    } finally {
      setLoading(false);
    }
  }, [actionFilter, entityFilter, fromFilter, toFilter, roleFilter, page, limit]);

  useEffect(() => {
    void loadAudits();
  }, [loadAudits]);

  const { displayedItems, totalFiltered, totalPages } = useMemo(() => {
    if (!roleFilter) {
      const pages = Math.max(1, Math.ceil(serverTotal / limit));
      return {
        displayedItems: rawItems,
        totalFiltered: serverTotal,
        totalPages: pages,
      };
    }
    const filtered = rawItems.filter((item) => {
      if (roleFilter === "public") {
        return !item.actor_role;
      }
      return item.actor_role === roleFilter;
    });
    const total = filtered.length;
    const pages = Math.max(1, Math.ceil(total / limit));
    const start = (page - 1) * limit;
    return {
      displayedItems: filtered.slice(start, start + limit),
      totalFiltered: total,
      totalPages: pages,
    };
  }, [rawItems, roleFilter, serverTotal, page, limit]);

  function handleResetFilters() {
    setActionFilter("");
    setRoleFilter("");
    setEntityFilter("");
    setFromFilter("");
    setToFilter("");
    setPage(1);
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="amber">Jejak Audit &amp; Kepatuhan</Badge>
            <span className="text-xs font-medium text-stone-500">
              Khusus Pemilik Usaha (Owner) &amp; Administrator
            </span>
          </div>
          <h1 className="mt-1.5 text-2xl font-bold tracking-tight text-stone-900">
            Log Audit Sistem
          </h1>
          <p className="mt-0.5 text-sm text-stone-600">
            Rekam jejak seluruh keputusan pesanan, override rekomendasi,
            pembayaran, koreksi stok, dan perubahan parameter/master data.
          </p>
        </div>

        <Button
          variant="secondary"
          size="sm"
          onClick={() => void loadAudits()}
          disabled={loading}
        >
          <RefreshCw
            className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`}
          />
          Segarkan ({totalFiltered})
        </Button>
      </div>

      {/* Panel Filter */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <CardTitle className="flex items-center gap-2 text-sm">
              <Filter className="h-4 w-4 text-amber-700" />
              Filter &amp; Pencarian Log Audit
            </CardTitle>
            <button
              type="button"
              onClick={handleResetFilters}
              className="inline-flex items-center gap-1 text-xs font-semibold text-stone-600 hover:text-stone-900 cursor-pointer"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Reset Filter
            </button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
            {/* Filter Action */}
            <div>
              <label className="block text-xs font-semibold text-stone-600">
                Jenis Aksi (Action)
              </label>
              <select
                value={actionFilter}
                onChange={(e) => {
                  setActionFilter(e.target.value);
                  setPage(1);
                }}
                className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-medium text-stone-800"
              >
                <option value="">Semua Aksi</option>
                {ACTION_OPTIONS.map((act) => (
                  <option key={act} value={act}>
                    {AUDIT_ACTION_LABELS[act] ?? act} ({act})
                  </option>
                ))}
              </select>
            </div>

            {/* Filter Role */}
            <div>
              <label className="block text-xs font-semibold text-stone-600">
                Peran Pelaku (Role)
              </label>
              <select
                value={roleFilter}
                onChange={(e) => {
                  setRoleFilter(e.target.value);
                  setPage(1);
                }}
                className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-medium text-stone-800"
              >
                <option value="">Semua Peran</option>
                {ROLE_OPTIONS.map((opt) => (
                  <option key={opt.value} value={opt.value}>
                    {opt.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Filter Entity */}
            <div>
              <label className="block text-xs font-semibold text-stone-600">
                Entitas Data
              </label>
              <select
                value={entityFilter}
                onChange={(e) => {
                  setEntityFilter(e.target.value);
                  setPage(1);
                }}
                className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-medium text-stone-800"
              >
                <option value="">Semua Entitas</option>
                {ENTITY_OPTIONS.map((ent) => (
                  <option key={ent} value={ent}>
                    {AUDIT_ENTITY_LABELS[ent] ?? ent} ({ent})
                  </option>
                ))}
              </select>
            </div>

            {/* Filter Tanggal Mulai */}
            <div>
              <label className="block text-xs font-semibold text-stone-600">
                Dari Tanggal (WIB)
              </label>
              <input
                type="date"
                value={fromFilter}
                onChange={(e) => {
                  setFromFilter(e.target.value);
                  setPage(1);
                }}
                className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-medium text-stone-800"
              />
            </div>

            {/* Filter Tanggal Akhir */}
            <div>
              <label className="block text-xs font-semibold text-stone-600">
                Sampai Tanggal (WIB)
              </label>
              <input
                type="date"
                value={toFilter}
                onChange={(e) => {
                  setToFilter(e.target.value);
                  setPage(1);
                }}
                className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/80 px-3 py-2 text-xs font-medium text-stone-800"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Tabel Audit Log */}
      {loading ? (
        <TableSkeleton rows={8} />
      ) : errorMsg ? (
        <ErrorState
          title="Gagal Memuat Log Audit"
          message={errorMsg}
          onRetry={() => void loadAudits()}
        />
      ) : displayedItems.length === 0 ? (
        <EmptyState
          title="Tidak Ada Catatan Audit yang Cocok"
          description="Coba ubah filter aksi, peran, entitas, atau rentang tanggal untuk menampilkan riwayat audit."
        />
      ) : (
        <Card>
          <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2 pb-3">
            <div>
              <CardTitle className="flex items-center gap-2 text-base">
                <FileSpreadsheet className="h-4 w-4 text-amber-700" />
                Daftar Catatan Audit ({totalFiltered} Baris)
              </CardTitle>
              <CardDescription>
                Klik tombol &ldquo;Detail JSON&rdquo; pada baris untuk melihat
                payload lengkap sebelum/sesudah perubahan.
              </CardDescription>
            </div>

            <div className="flex items-center gap-2 text-xs">
              <span className="text-stone-500">Baris per halaman:</span>
              <select
                value={limit}
                onChange={(e) => {
                  setLimit(Number(e.target.value));
                  setPage(1);
                }}
                className="rounded-lg border border-stone-200 bg-white px-2 py-1 text-xs font-semibold text-stone-800"
              >
                <option value={10}>10</option>
                <option value={20}>20</option>
                <option value={50}>50</option>
              </select>
            </div>
          </CardHeader>

          <CardContent className="space-y-4">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-stone-200 text-stone-500">
                    <th className="py-2.5 pr-3 font-semibold">Waktu (WIB)</th>
                    <th className="px-3 py-2.5 font-semibold">Pelaku</th>
                    <th className="px-3 py-2.5 font-semibold">Peran</th>
                    <th className="px-3 py-2.5 font-semibold">Aksi</th>
                    <th className="px-3 py-2.5 font-semibold">Entitas</th>
                    <th className="px-3 py-2.5 font-semibold">
                      Ringkasan &amp; Alasan Override
                    </th>
                    <th className="py-2.5 pl-3 text-right font-semibold">
                      Rincian
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-100">
                  {displayedItems.map((log) => {
                    const isExpanded = expandedId === log.id;
                    const overrideReason =
                      typeof log.detail?.override_reason === "string"
                        ? log.detail.override_reason
                        : null;
                    const initialRec =
                      typeof log.detail?.initial_recommendation === "string"
                        ? log.detail.initial_recommendation
                        : null;
                    const orderLink =
                      log.entity === "orders" && log.entity_id
                        ? `/app/pesanan/${log.entity_id}`
                        : null;

                    return (
                      <tr key={log.id} className="align-top hover:bg-stone-50/70">
                        <td className="whitespace-nowrap py-3 pr-3 text-stone-600">
                          {formatDateTimeWIB(log.created_at)}
                        </td>
                        <td className="px-3 py-3 font-semibold text-stone-900">
                          {log.actor_name || "Publik / Sistem"}
                        </td>
                        <td className="px-3 py-3">
                          {log.actor_role ? (
                            <Badge variant="slate">
                              {ROLE_LABELS[log.actor_role as UserRole] ??
                                log.actor_role}
                            </Badge>
                          ) : (
                            <span className="text-stone-400">Publik/Sistem</span>
                          )}
                        </td>
                        <td className="px-3 py-3">
                          <Badge
                            variant={
                              log.action === "override" ||
                              log.action === "cancel"
                                ? "rose"
                                : log.action === "decision" ||
                                    log.action === "payment"
                                  ? "emerald"
                                  : "amber"
                            }
                          >
                            {AUDIT_ACTION_LABELS[log.action] ?? log.action}
                          </Badge>
                        </td>
                        <td className="px-3 py-3">
                          <div className="font-semibold text-stone-800">
                            {AUDIT_ENTITY_LABELS[log.entity] ?? log.entity}
                          </div>
                          {orderLink ? (
                            <Link
                              href={orderLink}
                              className="font-mono text-[11px] font-semibold text-amber-800 hover:underline"
                            >
                              Lihat Pesanan →
                            </Link>
                          ) : log.entity_id ? (
                            <span className="font-mono text-[10px] text-stone-400">
                              {log.entity_id.slice(0, 8)}…
                            </span>
                          ) : null}
                        </td>
                        <td className="px-3 py-3 text-stone-700">
                          {overrideReason ? (
                            <div className="mb-1.5 flex items-start gap-1.5 rounded-lg border border-rose-200 bg-rose-50 px-2.5 py-1.5 text-[11px] text-rose-900">
                              <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-rose-600" />
                              <div>
                                <strong>Alasan Override:</strong>{" "}
                                {overrideReason}
                                {initialRec ? (
                                  <span className="ml-1 text-rose-700">
                                    (Rekomendasi awal: {initialRec})
                                  </span>
                                ) : null}
                              </div>
                            </div>
                          ) : null}

                          <div className="max-w-md font-mono text-[11px] text-stone-600">
                            {Object.entries(log.detail ?? {})
                              .slice(0, 4)
                              .map(([k, v]) => (
                                <span key={k} className="mr-3 inline-block">
                                  <strong className="text-stone-700">
                                    {k}:
                                  </strong>{" "}
                                  {typeof v === "object"
                                    ? JSON.stringify(v)
                                    : String(v)}
                                </span>
                              ))}
                          </div>

                          {isExpanded ? (
                            <pre className="mt-2 max-h-56 overflow-auto rounded-xl border border-stone-200 bg-stone-900 p-3 font-mono text-[11px] text-amber-100">
                              {JSON.stringify(log.detail, null, 2)}
                            </pre>
                          ) : null}
                        </td>
                        <td className="py-3 pl-3 text-right">
                          <button
                            type="button"
                            onClick={() =>
                              setExpandedId(isExpanded ? null : log.id)
                            }
                            className="rounded-lg border border-stone-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-stone-700 hover:bg-stone-100 cursor-pointer"
                          >
                            {isExpanded ? "Tutup" : "Detail JSON"}
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>

            {/* Kontrol Paginasi */}
            <Pagination
              currentPage={page}
              totalPages={totalPages}
              totalItems={totalFiltered}
              pageSize={limit}
              onPageChange={setPage}
              itemName="catatan audit"
            />
          </CardContent>
        </Card>
      )}
    </div>
  );
}
