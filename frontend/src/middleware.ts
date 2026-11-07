import { NextResponse, type NextRequest } from "next/server";
import {
  canRoleAccessInternalPath,
  getDashboardPathForRole,
  resolvePostLoginRedirect,
} from "@/lib/format";
import type { UserRole } from "@/lib/types";

interface DecodedJwtPayload {
  sub?: string;
  role?: UserRole;
  exp?: number;
}

function decodeSessionCookie(token: string | undefined): DecodedJwtPayload | null {
  if (!token) return null;
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  try {
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = base64.padEnd(
      base64.length + ((4 - (base64.length % 4)) % 4),
      "="
    );
    const jsonStr = atob(padded);
    const payload = JSON.parse(jsonStr) as DecodedJwtPayload;
    if (!payload.sub || !payload.role || !payload.exp) return null;
    if (payload.exp * 1000 <= Date.now()) return null;
    return payload;
  } catch {
    return null;
  }
}

export function middleware(request: NextRequest) {
  const { pathname, searchParams } = request.nextUrl;
  const sessionToken = request.cookies.get("session")?.value;
  const session = decodeSessionCookie(sessionToken);

  // Jika pengguna sudah masuk dan membuka /login atau /daftar, arahkan sesuai peran.
  // Pengecualian: bila reauth=1 dikirim (karena GET /api/v1/auth/me ditolak backend),
  // hapus cookie session yang basi dan biarkan halaman /login terbuka agar tidak terjadi redirect loop.
  if (pathname === "/login" || pathname === "/daftar") {
    if (searchParams.get("reauth") === "1") {
      const response = NextResponse.next();
      response.cookies.delete("session");
      return response;
    }
    if (session?.role) {
      const nextParam = searchParams.get("next");
      const target = resolvePostLoginRedirect(session.role, nextParam);
      return NextResponse.redirect(new URL(target, request.url));
    }
    return NextResponse.next();
  }

  // Proteksi halaman pelanggan /pesanan-saya
  if (pathname === "/pesanan-saya") {
    if (!session?.role) {
      const loginUrl = new URL("/login", request.url);
      loginUrl.searchParams.set("next", "/pesanan-saya");
      return NextResponse.redirect(loginUrl);
    }
    if (session.role !== "customer") {
      return NextResponse.redirect(
        new URL(getDashboardPathForRole(session.role), request.url)
      );
    }
    return NextResponse.next();
  }

  // Proteksi seluruh area internal /app dan /app/...
  // Peran customer TIDAK PERNAH boleh diarahkan atau masuk ke /app
  if (pathname === "/app" || pathname.startsWith("/app/")) {
    if (!session?.role) {
      const loginUrl = new URL("/login", request.url);
      loginUrl.searchParams.set("next", pathname);
      return NextResponse.redirect(loginUrl);
    }

    if (session.role === "customer") {
      return NextResponse.redirect(new URL("/pesanan-saya", request.url));
    }

    const userDashboard = getDashboardPathForRole(session.role);

    if (pathname === "/app") {
      return NextResponse.redirect(new URL(userDashboard, request.url));
    }

    if (!canRoleAccessInternalPath(session.role, pathname)) {
      return NextResponse.redirect(new URL(userDashboard, request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/login", "/daftar", "/pesanan-saya", "/app", "/app/:path*"],
};


