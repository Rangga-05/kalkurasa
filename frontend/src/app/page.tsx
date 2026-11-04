"use client";

import { useEffect, useState } from "react";

interface HealthStatus {
  status: string;
  time: string;
  db: string;
  db_error?: string;
}

export default function Home() {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch("/api/healthz")
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: HealthStatus) => {
        setHealth(data);
        setError(null);
      })
      .catch((err: Error) => {
        setError(err.message);
      })
      .finally(() => setLoading(false));
  }, []);

  return (
    <main className="flex min-h-screen flex-col items-center justify-center p-8">
      <h1 className="mb-2 text-4xl font-bold tracking-tight">Kalkurasa</h1>
      <p className="mb-8 text-gray-500">
        Kalkulasi + Rasa — sistem manajemen pesanan UMKM kuliner
      </p>

      <div className="w-full max-w-md rounded-lg border bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold">Status Sistem</h2>

        {loading && (
          <p className="text-sm text-gray-400">Memeriksa koneksi...</p>
        )}

        {error && (
          <div className="rounded-md bg-red-50 p-3 text-sm text-red-700">
            <span className="font-medium">Gagal terhubung:</span> {error}
          </div>
        )}

        {health && (
          <div className="space-y-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-500">API</span>
              <span
                className={
                  health.status === "ok" ? "text-green-600" : "text-red-600"
                }
              >
                {health.status === "ok" ? "✓ Terhubung" : "✗ Gagal"}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">Database</span>
              <span
                className={
                  health.db === "ok"
                    ? "text-green-600"
                    : health.db === "not_configured"
                      ? "text-yellow-600"
                      : "text-red-600"
                }
              >
                {health.db === "ok"
                  ? "✓ Terhubung"
                  : health.db === "not_configured"
                    ? "— Belum diatur"
                    : `✗ ${health.db_error || "Gagal"}`}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">Waktu Server</span>
              <span>{health.time}</span>
            </div>
          </div>
        )}
      </div>
    </main>
  );
}

