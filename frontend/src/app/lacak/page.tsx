"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { PackageSearch, ArrowRight } from "lucide-react";
import { PublicNavbar } from "@/components/public-navbar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";

export default function TrackOrderSearchPage() {
  const router = useRouter();
  const [code, setCode] = useState("");
  const [errMsg, setErrMsg] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const cleaned = code.trim().toUpperCase();
    if (cleaned.length < 6) {
      setErrMsg("Masukkan kode pesanan yang valid (10 karakter).");
      return;
    }
    setErrMsg("");
    router.push(`/lacak/${encodeURIComponent(cleaned)}`);
  };

  return (
    <div className="flex flex-1 flex-col">
      <PublicNavbar />
      <main className="mx-auto flex w-full max-w-xl flex-1 flex-col justify-center px-4 py-10 sm:px-6">
        <Card>
          <CardHeader>
            <div className="mb-1 flex h-10 w-10 items-center justify-center rounded-xl bg-amber-100 text-amber-800">
              <PackageSearch className="h-5 w-5" />
            </div>
            <CardTitle className="text-xl">Lacak Status Pesanan</CardTitle>
            <CardDescription>
              Masukkan kode unik 10 karakter yang Anda terima saat membuat
              pesanan untuk melihat status terkini, tenggat DP, atau tawaran
              balik.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-1.5">
                <label
                  htmlFor="order-code-input"
                  className="text-sm font-medium text-stone-800"
                >
                  Kode Pesanan
                </label>
                <Input
                  id="order-code-input"
                  value={code}
                  onChange={(e) => {
                    setCode(e.target.value.toUpperCase());
                    if (errMsg) setErrMsg("");
                  }}
                  placeholder="Misal: K8M4P2X9TQ"
                  maxLength={12}
                  className="font-mono text-base uppercase tracking-widest"
                  autoFocus
                />
                {errMsg ? (
                  <p className="text-xs font-medium text-rose-600">{errMsg}</p>
                ) : null}
              </div>

              <Button type="submit" className="w-full gap-2">
                Lihat Status Pesanan
                <ArrowRight className="h-4 w-4" />
              </Button>
            </form>
          </CardContent>
        </Card>
      </main>
    </div>
  );
}

