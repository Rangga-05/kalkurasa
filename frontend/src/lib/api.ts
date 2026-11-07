import type {
  ApiEnvelope,
  ApiErrorEnvelope,
  AvailabilityDay,
  AvailabilityRequest,
  CounterResponseRequest,
  CreatePublicOrderRequest,
  LoginRequest,
  LoginResponse,
  LogoutResponse,
  PublicOrderSummary,
  PublicProduct,
  PublicTrackResponse,
  RegisterRequest,
  User,
} from "./types";

const API_BASE = "/api/v1";

/**
 * ApiError merepresentasikan respons error JSON seragam dari backend:
 * { "error": { "code": "...", "message": "..." } }
 * termasuk status 422 Unprocessable Entity untuk kesalahan validasi.
 */
export class ApiError extends Error {
  public readonly status: number;
  public readonly code: string;
  public readonly fieldErrors: string[];

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    // Backend menggabungkan pesan validasi 422 dengan pemisah "; "
    this.fieldErrors =
      status === 422
        ? message
            .split("; ")
            .map((s) => s.trim())
            .filter(Boolean)
        : [message];
  }

  public get isValidationError(): boolean {
    return this.status === 422 || this.code === "validation_error";
  }

  public get isUnauthorized(): boolean {
    return this.status === 401 || this.code === "unauthorized";
  }

  public get isForbidden(): boolean {
    return this.status === 403 || this.code === "forbidden";
  }

  public get isNotFound(): boolean {
    return this.status === 404 || this.code === "not_found";
  }
}

export function getErrorMessage(
  err: unknown,
  fallback = "Terjadi kesalahan saat menghubungi server."
): string {
  if (err instanceof ApiError) {
    return err.message;
  }
  if (err instanceof Error && err.message) {
    return err.message;
  }
  return fallback;
}

async function requestJson<T>(
  path: string,
  init: RequestInit = {}
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  headers.set("Accept", "application/json");

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      headers,
      credentials: "include",
    });
  } catch {
    throw new ApiError(
      0,
      "network_error",
      "Tidak dapat terhubung ke server. Periksa koneksi jaringan Anda."
    );
  }

  const rawText = await response.text();
  let parsed: unknown = null;
  if (rawText) {
    try {
      parsed = JSON.parse(rawText);
    } catch {
      parsed = null;
    }
  }

  if (!response.ok) {
    const errEnv = parsed as Partial<ApiErrorEnvelope> | null;
    const code = errEnv?.error?.code || `http_${response.status}`;
    const message =
      errEnv?.error?.message ||
      (response.status === 404
        ? "Data yang diminta tidak ditemukan."
        : response.status === 422
          ? "Data masukan belum sesuai ketentuan."
          : response.status === 429
            ? "Terlalu banyak permintaan. Silakan tunggu sebentar."
            : "Terjadi kesalahan pada server.");
    throw new ApiError(response.status, code, message);
  }

  const envelope = parsed as ApiEnvelope<T>;
  return envelope.data;
}

// ============================================================================
// Endpoint Publik (Bagian 9 Brief)
// ============================================================================

/**
 * GET /api/v1/public/products
 * Mengambil daftar produk aktif untuk katalog pelanggan publik.
 */
export async function getPublicProducts(
  signal?: AbortSignal
): Promise<PublicProduct[]> {
  return requestJson<PublicProduct[]>("/public/products", {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/public/availability
 * Mengecek ketersediaan tanggal berdasarkan G1 (kapasitas) dan G4 (lead time).
 */
export async function checkPublicAvailability(
  payload: AvailabilityRequest,
  signal?: AbortSignal
): Promise<AvailabilityDay[]> {
  return requestJson<AvailabilityDay[]>("/public/availability", {
    method: "POST",
    body: JSON.stringify(payload),
    signal,
  });
}

/**
 * POST /api/v1/public/orders
 * Membuat pesanan publik baru dengan evaluasi G1..G4 dan penguncian day_locks.
 */
export async function createPublicOrder(
  payload: CreatePublicOrderRequest
): Promise<PublicOrderSummary> {
  return requestJson<PublicOrderSummary>("/public/orders", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * GET /api/v1/public/orders/:code
 * Melacak status dan rincian aman pesanan berdasarkan kode publik 10 karakter.
 */
export async function trackPublicOrder(
  code: string,
  signal?: AbortSignal
): Promise<PublicTrackResponse> {
  const cleanCode = encodeURIComponent(code.trim().toUpperCase());
  return requestJson<PublicTrackResponse>(`/public/orders/${cleanCode}`, {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/public/orders/:code/counter-response
 * Menerima atau menolak tawaran balik dari pihak usaha ({ accept: boolean }).
 */
export async function respondCounterOffer(
  code: string,
  payload: CounterResponseRequest
): Promise<PublicTrackResponse> {
  const cleanCode = encodeURIComponent(code.trim().toUpperCase());
  return requestJson<PublicTrackResponse>(
    `/public/orders/${cleanCode}/counter-response`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  );
}

// ============================================================================
// Endpoint Autentikasi & Akun Pelanggan (/api/v1/auth/*, /api/v1/me/*)
// ============================================================================

/**
 * POST /api/v1/auth/register
 * Mendaftarkan akun pelanggan baru (role customer) dan langsung masuk (cookie sesi httpOnly).
 */
export async function register(
  payload: RegisterRequest
): Promise<LoginResponse> {
  return requestJson<LoginResponse>("/auth/register", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * POST /api/v1/auth/login
 * Masuk dengan email & kata sandi; mengatur cookie sesi httpOnly.
 */
export async function login(payload: LoginRequest): Promise<LoginResponse> {
  return requestJson<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * POST /api/v1/auth/logout
 * Menghapus cookie sesi httpOnly.
 */
export async function logout(): Promise<LogoutResponse> {
  return requestJson<LogoutResponse>("/auth/logout", {
    method: "POST",
  });
}

/**
 * GET /api/v1/auth/me
 * Mengambil profil pengguna yang sedang masuk dari sesi aktif.
 */
export async function getMe(signal?: AbortSignal): Promise<User> {
  return requestJson<User>("/auth/me", {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/me/orders
 * Mengambil daftar pesanan milik pelanggan yang sedang masuk (field aman).
 */
export async function getMyOrders(
  signal?: AbortSignal
): Promise<PublicTrackResponse[]> {
  return requestJson<PublicTrackResponse[]>("/me/orders", {
    method: "GET",
    signal,
  });
}

