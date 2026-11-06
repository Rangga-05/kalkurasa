import { NextResponse, type NextRequest } from "next/server";
import { getDashboardPathForRole } from "@/lib/format";
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

const ROUTE_ROLE_RULES: { prefix: string; roles: UserRole[] }[] = [
  { prefix: "/app/owner", roles: ["owner"] },
  { prefix: "/app/manajer", roles: ["manager", "owner"] },
  { prefix: "/app/karyawan", roles: ["staff", "manager", "owner"] },
  { prefix: "/app/admin", roles: ["admin"] },
  { prefix: "/app/keputusan", roles: ["owner", "manager"] },
  { prefix: "/app/kapasitas", roles: ["owner", "manager"] },
  { prefix: "/app/kas", roles: ["owner"] },
  { prefix: "/app/pengaturan", roles: ["owner"] },
  { prefix: "/app/belanja", roles: ["owner", "manager", "staff"] },
  { prefix: "/app/produk", roles: ["owner", "manager", "admin"] },
  { prefix: "/app/bahan", roles: ["owner", "manager", "admin"] },
  { prefix: "/app/pengguna", roles: ["admin"] },
  { prefix: "/app/audit", roles: ["owner", "admin"] },
  { prefix: "/app/pesanan", roles: ["owner", "manager", "staff", "admin"] },
];

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const sessionToken = request.cookies.get("session")?.value;
  const session = decodeSessionCookie(sessionToken);

  // Jika pengguna sudah masuk dan membuka /login, arahkan ke dashboard peran
  if (pathname === "/login") {
    if (session?.role) {
      const target = getDashboardPathForRole(session.role);
      return NextResponse.redirect(new URL(target, request.url));
    }
    return NextResponse.next();
  }

  // Proteksi seluruh area internal /app dan /app/...
  if (pathname === "/app" || pathname.startsWith("/app/")) {
    if (!session?.role) {
      const loginUrl = new URL("/login", request.url);
      loginUrl.searchParams.set("next", pathname);
      return NextResponse.redirect(loginUrl);
    }

    const userDashboard = getDashboardPathForRole(session.role);

    if (pathname === "/app") {
      return NextResponse.redirect(new URL(userDashboard, request.url));
    }

    for (const rule of ROUTE_ROLE_RULES) {
      if (
        pathname === rule.prefix ||
        pathname.startsWith(`${rule.prefix}/`)
      ) {
        if (!rule.roles.includes(session.role)) {
          return NextResponse.redirect(new URL(userDashboard, request.url));
        }
        break;
      }
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/login", "/app", "/app/:path*"],
};

