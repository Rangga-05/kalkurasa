"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  AlertTriangle,
  Calendar,
  CheckCircle2,
  ClipboardList,
  Clock,
  ExternalLink,
  Loader2,
  MapPin,
  PackageCheck,
  Play,
  RefreshCw,
  ShoppingBasket,
  Truck,
  UtensilsCrossed,
} from "lucide-react";
import { useInternalSession } from "@/components/internal/use-internal-session";
import { OrderStatusBadge } from "@/components/internal/order-timeline";
import {
  getErrorMessage,
  getTasks,
  transitionOrder,
  updateTask,
} from "@/lib/api";
import {
  formatDateLong,
  formatDateShort,
  TASK_STATUS_LABELS,
  TASK_TYPE_LABELS,
  toDateKeyWIB,
} from "@/lib/format";
import type { TaskItem, TaskStatus, TaskType } from "@/lib/types";

export default function StaffTasksPage() {
  const { user } = useInternalSession();
  const canManage = user?.role === "owner" || user?.role === "manager";

  const [tasks, setTasks] = useState<TaskItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [actionBanner, setActionBanner] = useState<string | null>(null);

  const [dateFilter, setDateFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<TaskStatus | "">("");
  const [typeFilter, setTypeFilter] = useState<TaskType | "">("");

  // State input actual_minutes per task ID
  const [minutesMap, setMinutesMap] = useState<Record<string, string>>({});
  const [busyTaskId, setBusyTaskId] = useState<string | null>(null);

  const loadTasks = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const data = await getTasks({
        mine: true,
        status: statusFilter || undefined,
        type: typeFilter || undefined,
        date: dateFilter || undefined,
      });
      setTasks(data);
      setMinutesMap((prev) => {
        const next = { ...prev };
        for (const tk of data) {
          if (
            tk.actual_minutes !== undefined &&
            tk.actual_minutes !== null &&
            next[tk.id] === undefined
          ) {
            next[tk.id] = String(tk.actual_minutes);
          }
        }
        return next;
      });
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memuat daftar tugas operasional."));
    } finally {
      setLoading(false);
    }
  }, [dateFilter, statusFilter, typeFilter]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  async function handleUpdateTask(
    task: TaskItem,
    newStatus: TaskStatus,
    includeMinutes = true
  ) {
    setErrorMsg(null);
    setActionBanner(null);
    setBusyTaskId(task.id);

    let mins: number | undefined;
    const rawMins = minutesMap[task.id]?.trim();
    if (includeMinutes && rawMins) {
      const parsed = Number.parseInt(rawMins, 10);
      if (!Number.isFinite(parsed) || parsed <= 0) {
        setErrorMsg("Durasi aktual (menit) harus berupa angka lebih dari 0.");
        setBusyTaskId(null);
        return;
      }
      mins = parsed;
    }

    try {
      const updated = await updateTask(task.id, {
        status: newStatus,
        actual_minutes: mins,
      });
      setTasks((prev) =>
        prev.map((item) => (item.id === updated.id ? updated : item))
      );
      setActionBanner(
        `Tugas "${updated.title}" diperbarui menjadi ${TASK_STATUS_LABELS[updated.status]?.label ?? updated.status}.`
      );
    } catch (err) {
      setErrorMsg(getErrorMessage(err, "Gagal memperbarui status tugas."));
    } finally {
      setBusyTaskId(null);
    }
  }

  async function handleOrderTransition(
    task: TaskItem,
    toStatus: "in_production" | "ready" | "delivering"
  ) {
    setErrorMsg(null);
    setActionBanner(null);
    setBusyTaskId(task.id);

    let mins: number | undefined;
    const rawMins = minutesMap[task.id]?.trim();
    if (toStatus === "ready" && rawMins) {
      const parsed = Number.parseInt(rawMins, 10);
      if (!Number.isFinite(parsed) || parsed <= 0) {
        setErrorMsg("Durasi aktual produksi harus lebih dari 0 menit.");
        setBusyTaskId(null);
        return;
      }
      mins = parsed;
    }

    try {
      await transitionOrder(task.order_id, {
        to_status: toStatus,
        actual_minutes: mins,
      });
      setActionBanner(
        `Status pesanan #${task.order_code ?? ""} berhasil diubah ke "${toStatus}".`
      );
      await loadTasks();
    } catch (err) {
      setErrorMsg(
        getErrorMessage(err, "Gagal mengubah status tahapan pesanan.")
      );
    } finally {
      setBusyTaskId(null);
    }
  }

  function getTaskIcon(type: TaskType) {
    switch (type) {
      case "shop":
        return ShoppingBasket;
      case "produce":
        return UtensilsCrossed;
      case "deliver":
        return Truck;
      default:
        return ClipboardList;
    }
  }

  const todayKey = toDateKeyWIB(new Date());

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-800">
            <ClipboardList className="h-3.5 w-3.5" />
            Ruang Kerja Operasional Dapur & Kurir
          </span>
          <h1 className="mt-2 text-2xl font-bold text-stone-900">
            Daftar Tugas Harian (Belanja, Produksi, Pengantaran)
          </h1>
          <p className="mt-1 text-sm text-stone-600">
            Perbarui progres pekerjaan, catat durasi menit aktual produksi, dan
            majukan status pesanan sesuai tahapan operasional.
          </p>
        </div>

        <button
          type="button"
          onClick={() => void loadTasks()}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-xl border border-stone-200 bg-white px-4 py-2 text-xs font-semibold text-stone-700 shadow-2xs hover:bg-stone-50"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          Segarkan ({tasks.length})
        </button>
      </div>

      {/* Filter Bar */}
      <div className="flex flex-wrap items-end gap-3 rounded-2xl border border-stone-200 bg-white p-4 shadow-2xs">
        <div>
          <label className="block text-xs font-semibold text-stone-600">
            Filter Tanggal Tenggat
          </label>
          <div className="mt-1 flex items-center gap-2">
            <input
              type="date"
              value={dateFilter}
              onChange={(e) => setDateFilter(e.target.value)}
              className="rounded-xl border border-stone-200 bg-stone-50/70 px-3 py-2 text-xs font-medium text-stone-800"
            />
            <button
              type="button"
              onClick={() => setDateFilter(todayKey)}
              className={`rounded-xl border px-3 py-2 text-xs font-semibold transition ${
                dateFilter === todayKey
                  ? "border-amber-500 bg-amber-50 text-amber-900"
                  : "border-stone-200 bg-white text-stone-700 hover:bg-stone-50"
              }`}
            >
              Hari Ini
            </button>
            <button
              type="button"
              onClick={() => setDateFilter("")}
              className={`rounded-xl border px-3 py-2 text-xs font-semibold transition ${
                dateFilter === ""
                  ? "border-amber-500 bg-amber-50 text-amber-900"
                  : "border-stone-200 bg-white text-stone-700 hover:bg-stone-50"
              }`}
            >
              Semua Tanggal
            </button>
          </div>
        </div>

        <div className="min-w-40 flex-1">
          <label className="block text-xs font-semibold text-stone-600">
            Jenis Tugas
          </label>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as TaskType | "")}
            className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/70 px-3 py-2 text-xs font-medium text-stone-800"
          >
            <option value="">Semua Jenis (Belanja, Produksi, Antar)</option>
            <option value="shop">Belanja Bahan (Shop)</option>
            <option value="produce">Produksi Dapur (Produce)</option>
            <option value="deliver">Pengantaran / Penyerahan (Deliver)</option>
          </select>
        </div>

        <div className="min-w-40 flex-1">
          <label className="block text-xs font-semibold text-stone-600">
            Status Tugas
          </label>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as TaskStatus | "")}
            className="mt-1 w-full rounded-xl border border-stone-200 bg-stone-50/70 px-3 py-2 text-xs font-medium text-stone-800"
          >
            <option value="">Semua Status</option>
            <option value="todo">Belum Mulai (Todo)</option>
            <option value="doing">Sedang Dikerjakan (Doing)</option>
            <option value="done">Selesai (Done)</option>
          </select>
        </div>
      </div>

      {actionBanner && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900">
          <CheckCircle2 className="h-5 w-5 shrink-0 text-emerald-600" />
          <span>{actionBanner}</span>
        </div>
      )}

      {errorMsg && (
        <div className="flex items-center gap-2.5 rounded-2xl border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          <AlertTriangle className="h-5 w-5 shrink-0 text-rose-600" />
          <span>{errorMsg}</span>
        </div>
      )}

      {/* Daftar Kartu Tugas */}
      {loading ? (
        <div className="flex items-center justify-center rounded-2xl border border-stone-200 bg-white py-16 text-sm text-stone-500">
          <Loader2 className="mr-2 h-5 w-5 animate-spin text-amber-600" />
          Memuat daftar tugas operasional...
        </div>
      ) : tasks.length === 0 ? (
        <div className="rounded-2xl border border-stone-200 bg-white p-12 text-center shadow-xs">
          <CheckCircle2 className="mx-auto h-10 w-10 text-emerald-500" />
          <h2 className="mt-3 text-base font-bold text-stone-900">
            Tidak Ada Tugas untuk Filter Ini
          </h2>
          <p className="mx-auto mt-1 max-w-md text-xs text-stone-500">
            Tugas belanja, produksi, dan pengantaran baru akan muncul secara
            otomatis ketika pesanan telah terkonfirmasi (DP lunas).
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {tasks.map((tk) => {
            const Icon = getTaskIcon(tk.type);
            const stMeta = TASK_STATUS_LABELS[tk.status] ?? {
              label: tk.status,
              tone: "slate",
            };
            const isBusy = busyTaskId === tk.id;

            return (
              <article
                key={tk.id}
                className="rounded-2xl border border-stone-200 bg-white p-5 shadow-xs"
              >
                <div className="flex flex-wrap items-start justify-between gap-4 border-b border-stone-100 pb-4">
                  <div className="flex items-start gap-3">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-100 text-amber-800">
                      <Icon className="h-5 w-5" />
                    </div>
                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-full bg-stone-100 px-2.5 py-0.5 text-xs font-bold text-stone-800">
                          {TASK_TYPE_LABELS[tk.type] ?? tk.type}
                        </span>
                        <span
                          className={`rounded-full px-2.5 py-0.5 text-xs font-bold ${
                            stMeta.tone === "emerald"
                              ? "bg-emerald-100 text-emerald-800"
                              : stMeta.tone === "amber"
                                ? "bg-amber-100 text-amber-800"
                                : "bg-stone-200 text-stone-700"
                          }`}
                        >
                          {stMeta.label}
                        </span>
                        {tk.order_code && (
                          <span className="font-mono text-xs font-bold text-stone-700">
                            Pesanan #{tk.order_code}
                          </span>
                        )}
                        {tk.order_status && (
                          <OrderStatusBadge status={tk.order_status} />
                        )}
                      </div>
                      <h3 className="mt-1.5 text-base font-bold text-stone-900">
                        {tk.title}
                      </h3>
                      <div className="mt-1 flex flex-wrap items-center gap-3 text-xs text-stone-600">
                        <span className="inline-flex items-center gap-1">
                          <Calendar className="h-3.5 w-3.5 text-stone-400" />
                          Tenggat:{" "}
                          <strong>{formatDateLong(tk.due_date)}</strong>
                        </span>
                        {tk.event_date && (
                          <span>
                            • Acara:{" "}
                            <strong>{formatDateShort(tk.event_date)}</strong>
                          </span>
                        )}
                        {tk.delivery_type && (
                          <span>
                            •{" "}
                            {tk.delivery_type === "delivery"
                              ? "Diantar Kurir"
                              : "Ambil Sendiri"}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  <Link
                    href={`/app/pesanan/${tk.order_id}`}
                    className="inline-flex items-center gap-1 rounded-xl border border-stone-200 px-3 py-1.5 text-xs font-semibold text-stone-700 hover:bg-stone-50"
                  >
                    Detail Pesanan
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                </div>

                {/* Konteks Item Produksi & Alamat Pengantaran (Tanpa Harga/Margin) */}
                <div className="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
                  <div>
                    <p className="text-xs font-bold tracking-wider text-stone-500 uppercase">
                      Item & Kuantitas Pesanan
                    </p>
                    {!tk.items || tk.items.length === 0 ? (
                      <p className="mt-1 text-xs text-stone-500">-</p>
                    ) : (
                      <ul className="mt-1.5 space-y-1 text-xs text-stone-800">
                        {tk.items.map((it, idx) => (
                          <li
                            key={`${it.product_id}-${idx}`}
                            className="flex items-center justify-between rounded-lg bg-stone-50 px-3 py-1.5 font-medium"
                          >
                            <span>{it.product_name}</span>
                            <span className="font-bold text-stone-900">
                              {it.qty} unit
                            </span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>

                  <div className="space-y-2 text-xs text-stone-700">
                    {tk.customer_name && (
                      <div>
                        <span className="font-semibold text-stone-500">
                          Pelanggan:
                        </span>{" "}
                        <strong>{tk.customer_name}</strong>{" "}
                        {tk.customer_contact ? `(${tk.customer_contact})` : ""}
                      </div>
                    )}
                    {tk.address && (
                      <div className="flex items-start gap-1.5">
                        <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-stone-400" />
                        <span>
                          <strong>Alamat Antar:</strong> {tk.address}
                        </span>
                      </div>
                    )}
                    {tk.notes && (
                      <div className="rounded-lg border border-stone-200/80 bg-stone-50 p-2.5 text-stone-700">
                        <strong>Catatan:</strong> {tk.notes}
                      </div>
                    )}
                  </div>
                </div>

                {/* Kontrol Update Status Tugas, Actual Minutes, & Transisi Status Pesanan */}
                <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-stone-100 pt-4">
                  {/* Input Durasi Aktual (Menit) */}
                  <div className="flex flex-wrap items-center gap-2">
                    <label className="inline-flex items-center gap-1 text-xs font-semibold text-stone-700">
                      <Clock className="h-3.5 w-3.5 text-stone-400" />
                      Durasi Aktual (menit):
                    </label>
                    <input
                      type="number"
                      min={1}
                      placeholder="Mis. 180"
                      value={minutesMap[tk.id] ?? ""}
                      onChange={(e) =>
                        setMinutesMap((prev) => ({
                          ...prev,
                          [tk.id]: e.target.value,
                        }))
                      }
                      className="w-28 rounded-xl border border-stone-300 bg-white px-2.5 py-1.5 text-xs text-stone-900"
                    />
                    <button
                      type="button"
                      disabled={isBusy}
                      onClick={() => handleUpdateTask(tk, tk.status, true)}
                      className="rounded-xl border border-stone-200 bg-stone-50 px-3 py-1.5 text-xs font-semibold text-stone-700 hover:bg-stone-100 disabled:opacity-50"
                    >
                      Simpan Menit
                    </button>
                  </div>

                  {/* Tombol Status Tugas & Transisi Pesanan */}
                  <div className="flex flex-wrap items-center gap-2">
                    <div className="inline-flex rounded-xl border border-stone-200 bg-stone-50 p-1">
                      {(["todo", "doing", "done"] as TaskStatus[]).map((st) => (
                        <button
                          key={st}
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleUpdateTask(tk, st, true)}
                          className={`rounded-lg px-2.5 py-1 text-xs font-bold transition ${
                            tk.status === st
                              ? "bg-amber-600 text-white shadow-2xs"
                              : "text-stone-600 hover:text-stone-900"
                          }`}
                        >
                          {TASK_STATUS_LABELS[st]?.label ?? st}
                        </button>
                      ))}
                    </div>

                    {/* Tombol Transisi Status Pesanan sesuai Bagian 8.5 */}
                    {tk.order_status === "confirmed" && canManage && (
                      <button
                        type="button"
                        disabled={isBusy}
                        onClick={() =>
                          handleOrderTransition(tk, "in_production")
                        }
                        className="inline-flex items-center gap-1.5 rounded-xl bg-indigo-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-indigo-700 disabled:opacity-50"
                      >
                        <Play className="h-3.5 w-3.5" />
                        Mulai Produksi Pesanan
                      </button>
                    )}

                    {tk.order_status === "in_production" &&
                      tk.type === "produce" && (
                        <button
                          type="button"
                          disabled={isBusy}
                          onClick={() => handleOrderTransition(tk, "ready")}
                          className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-emerald-700 disabled:opacity-50"
                        >
                          <PackageCheck className="h-3.5 w-3.5" />
                          Tandai Pesanan Siap (Ready)
                        </button>
                      )}

                    {tk.order_status === "ready" && tk.type === "deliver" && (
                      <button
                        type="button"
                        disabled={isBusy}
                        onClick={() => handleOrderTransition(tk, "delivering")}
                        className="inline-flex items-center gap-1.5 rounded-xl bg-sky-600 px-3.5 py-2 text-xs font-bold text-white hover:bg-sky-700 disabled:opacity-50"
                      >
                        <Truck className="h-3.5 w-3.5" />
                        Mulai Pengantaran (Delivering)
                      </button>
                    )}
                  </div>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
