import type {
  ApiEnvelope,
  ApiErrorEnvelope,
  ApiListEnvelope,
  AuditLogFilterParams,
  AuditLogItem,
  AvailabilityDay,
  AvailabilityRequest,
  CancelOrderRequest,
  CancelOrderResult,
  CapacityCalendarDay,
  CapacityDayDetail,
  CashEntryDTO,
  CashProjectionResult,
  CounterResponseRequest,
  CreateCashEntryRequest,
  CreateIngredientRequest,
  CreateManualOrderRequest,
  CreateProductRequest,
  CreatePublicOrderRequest,
  CreateUserRequest,
  DecideOrderRequest,
  DecisionQueueItem,
  Ingredient,
  KPIResult,
  LoginRequest,
  LoginResponse,
  LogoutResponse,
  ManagerDashboardResult,
  OrderDetail,
  OrderFilterParams,
  OwnerDashboardResult,
  PaymentResult,
  Product,
  ProductCostingDTO,
  ProductLoad,
  PublicOrderSummary,
  PublicProduct,
  PublicConfig,
  PublicTrackResponse,
  PurchaseResult,
  RecipeItem,
  RecordPaymentRequest,
  RecordPurchaseRequest,
  RegisterRequest,
  Resource,
  SettingsMap,
  ShoppingListResult,
  StaffDashboardResult,
  StockAdjustRequest,
  StockAdjustResult,
  TaskFilterParams,
  TaskItem,
  TransitionOrderRequest,
  UpdateIngredientRequest,
  UpdateProductRequest,
  UpdateTaskRequest,
  UpdateUserRequest,
  UpsertProductLoadsRequest,
  UpsertProductRecipesRequest,
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

function normalizeSignal(signal?: unknown): AbortSignal | undefined {
  if (!signal) return undefined;
  if (typeof AbortSignal !== "undefined" && signal instanceof AbortSignal) {
    return signal;
  }
  if (
    typeof signal === "object" &&
    signal !== null &&
    "signal" in signal &&
    typeof AbortSignal !== "undefined" &&
    (signal as { signal?: unknown }).signal instanceof AbortSignal
  ) {
    return (signal as { signal: AbortSignal }).signal;
  }
  return undefined;
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

  const safeSignal = normalizeSignal(init.signal);

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      signal: safeSignal,
      headers,
      credentials: "include",
    });
  } catch (err: unknown) {
    console.error(`[API Network Error] ${path}:`, err);
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

async function requestListJson<T>(
  path: string,
  init: RequestInit = {}
): Promise<ApiListEnvelope<T>> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  headers.set("Accept", "application/json");

  const safeSignal = normalizeSignal(init.signal);

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...init,
      signal: safeSignal,
      headers,
      credentials: "include",
    });
  } catch (err: unknown) {
    console.error(`[API Network Error] ${path}:`, err);
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

  return parsed as ApiListEnvelope<T>;
}

// ============================================================================
// Endpoint Publik (Bagian 9 Brief)
// ============================================================================

/**
 * GET /api/v1/public/config
 * Mengambil parameter publik aman (seperti tarif PPN dinamis).
 */
export async function getPublicConfig(
  signal?: AbortSignal
): Promise<PublicConfig> {
  return requestJson<PublicConfig>("/public/config", {
    method: "GET",
    signal,
  });
}

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

// ============================================================================
// Endpoint Internal Alur Inti (Tahap 4B)
// ============================================================================

/**
 * GET /api/v1/decision-queue
 * Mengambil daftar pesanan berstatus pending_decision beserta pratinjau dampak terkini.
 */
export async function getDecisionQueue(
  signal?: AbortSignal
): Promise<DecisionQueueItem[]> {
  return requestJson<DecisionQueueItem[]>("/decision-queue", {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/orders/:id/decision
 * Mengambil keputusan pesanan (accept, counter, reject) dengan evaluasi ulang.
 */
export async function decideOrder(
  orderId: string,
  payload: DecideOrderRequest
): Promise<OrderDetail> {
  return requestJson<OrderDetail>(
    `/orders/${encodeURIComponent(orderId)}/decision`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * GET /api/v1/orders
 * Mengambil daftar pesanan internal dengan filter status & tanggal (disaring untuk staff).
 */
export async function getOrders(
  params: OrderFilterParams = {},
  signal?: AbortSignal
): Promise<OrderDetail[]> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.date) qs.set("date", params.date);
  if (params.event_date) qs.set("event_date", params.event_date);
  if (params.from) qs.set("from", params.from);
  if (params.to) qs.set("to", params.to);
  if (params.search) qs.set("search", params.search);
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return requestJson<OrderDetail[]>(`/orders${query}`, {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/orders/:id
 * Mengambil detail lengkap pesanan (items, evaluasi gerbang, pembayaran, tugas, audit;
 * field harga/margin/kas dibuang bila pemanggil berperan staff).
 */
export async function getOrderById(
  orderId: string,
  signal?: AbortSignal
): Promise<OrderDetail> {
  return requestJson<OrderDetail>(`/orders/${encodeURIComponent(orderId)}`, {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/orders
 * Membuat pesanan manual oleh manajer atau pemilik (mendukung unit_price & production_date khusus).
 */
export async function createManualOrder(
  payload: CreateManualOrderRequest
): Promise<OrderDetail> {
  return requestJson<OrderDetail>("/orders", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * POST /api/v1/orders/:id/payments
 * Mencatat pembayaran manual pesanan dan mengubah status ke confirmed saat DP terpenuhi.
 */
export async function recordOrderPayment(
  orderId: string,
  payload: RecordPaymentRequest
): Promise<PaymentResult> {
  return requestJson<PaymentResult>(
    `/orders/${encodeURIComponent(orderId)}/payments`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * POST /api/v1/orders/:id/transition
 * Menjalankan transisi status pesanan (in_production, ready, delivering, completed) sesuai peran.
 */
export async function transitionOrder(
  orderId: string,
  payload: TransitionOrderRequest
): Promise<OrderDetail> {
  return requestJson<OrderDetail>(
    `/orders/${encodeURIComponent(orderId)}/transition`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * POST /api/v1/orders/:id/cancel
 * Membatalkan pesanan, melepas kapasitas, membatalkan tugas aktif, dan menghitung eksposur pembatalan.
 */
export async function cancelOrder(
  orderId: string,
  payload: CancelOrderRequest = {}
): Promise<CancelOrderResult> {
  return requestJson<CancelOrderResult>(
    `/orders/${encodeURIComponent(orderId)}/cancel`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * GET /api/v1/capacity/calendar?month=YYYY-MM
 * Mengambil kalender kapasitas bulanan berwarna (hijau, kuning, merah, penuh).
 */
export async function getCapacityCalendar(
  month?: string,
  signal?: AbortSignal
): Promise<CapacityCalendarDay[]> {
  const qs = month ? `?month=${encodeURIComponent(month)}` : "";
  return requestJson<CapacityCalendarDay[]>(`/capacity/calendar${qs}`, {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/capacity/day/:date
 * Mengambil rincian beban & kapasitas per sumber daya serta daftar pesanan pada tanggal produksi.
 */
export async function getCapacityDay(
  date: string,
  signal?: AbortSignal
): Promise<CapacityDayDetail> {
  return requestJson<CapacityDayDetail>(
    `/capacity/day/${encodeURIComponent(date)}`,
    {
      method: "GET",
      signal,
    }
  );
}

/**
 * GET /api/v1/tasks
 * Mengambil daftar tugas operasional (produce, shop, deliver) tanpa field harga/margin/kas.
 */
export async function getTasks(
  params: TaskFilterParams = {},
  signal?: AbortSignal
): Promise<TaskItem[]> {
  const qs = new URLSearchParams();
  if (params.mine) qs.set("mine", "true");
  if (params.status) qs.set("status", params.status);
  if (params.type) qs.set("type", params.type);
  if (params.date) qs.set("date", params.date);
  if (params.from) qs.set("from", params.from);
  if (params.to) qs.set("to", params.to);
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return requestJson<TaskItem[]>(`/tasks${query}`, {
    method: "GET",
    signal,
  });
}

/**
 * PATCH /api/v1/tasks/:id
 * Mengubah status tugas (todo|doing|done) dan/atau actual_minutes.
 */
export async function updateTask(
  taskId: string,
  payload: UpdateTaskRequest
): Promise<TaskItem> {
  return requestJson<TaskItem>(`/tasks/${encodeURIComponent(taskId)}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

// ============================================================================
// Endpoint Bahan, Belanja, Kas, Biaya Produk & Master Data (Tahap 5)
// ============================================================================

/**
 * GET /api/v1/shopping-list?from=YYYY-MM-DD&to=YYYY-MM-DD
 * Mengambil daftar belanja bahan dari pesanan aktif (confirmed & in_production)
 * menggunakan engine.ShoppingNeed, beserta daftar bahan yang stoknya di bawah minimum.
 */
export async function getShoppingList(
  params: { from?: string; to?: string } = {},
  signal?: AbortSignal
): Promise<ShoppingListResult> {
  const qs = new URLSearchParams();
  if (params.from) qs.set("from", params.from);
  if (params.to) qs.set("to", params.to);
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return requestJson<ShoppingListResult>(`/shopping-list${query}`, {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/purchases
 * Mencatat pembelian bahan: menambah stok, memperbarui buy_price aktual,
 * mencatat stock_movements dan cash_entries keluar.
 */
export async function recordPurchase(
  payload: RecordPurchaseRequest
): Promise<PurchaseResult> {
  return requestJson<PurchaseResult>("/purchases", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * POST /api/v1/stock/adjust
 * Mencatat koreksi stok bahan dengan alasan wajib.
 */
export async function adjustStock(
  payload: StockAdjustRequest
): Promise<StockAdjustResult> {
  return requestJson<StockAdjustResult>("/stock/adjust", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * GET /api/v1/cash/projection?days=14
 * Mengambil proyeksi arus kas harian dengan engine.ProjectCash.
 * Owner melihat rincian penuh; Manager hanya melihat passed, lowest, lowest_date, days_below_buffer.
 */
export async function getCashProjection(
  days = 14,
  signal?: AbortSignal
): Promise<CashProjectionResult> {
  return requestJson<CashProjectionResult>(
    `/cash/projection?days=${encodeURIComponent(String(days))}`,
    {
      method: "GET",
      signal,
    }
  );
}

/**
 * POST /api/v1/cash/entries
 * Mencatat entri kas manual (khusus Owner).
 */
export async function createCashEntry(
  payload: CreateCashEntryRequest
): Promise<CashEntryDTO> {
  return requestJson<CashEntryDTO>("/cash/entries", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * GET /api/v1/costing/products
 * Mengambil HPP (UnitCost), margin, harga minimum, dan diskon maksimum seluruh produk.
 */
export async function getProductsCosting(
  signal?: AbortSignal
): Promise<ProductCostingDTO[]> {
  return requestJson<ProductCostingDTO[]>("/costing/products", {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/ingredients
 * Mengambil daftar bahan (dilengkapi below_min_stock; buy_price disembunyikan untuk staff).
 */
export async function getIngredients(
  signal?: AbortSignal
): Promise<Ingredient[]> {
  return requestJson<Ingredient[]>("/ingredients", {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/ingredients
 * Membuat bahan baru.
 */
export async function createIngredient(
  payload: CreateIngredientRequest
): Promise<Ingredient> {
  return requestJson<Ingredient>("/ingredients", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * PUT /api/v1/ingredients/:id
 * Memperbarui data master bahan.
 */
export async function updateIngredient(
  id: string,
  payload: UpdateIngredientRequest
): Promise<Ingredient> {
  return requestJson<Ingredient>(`/ingredients/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

/**
 * DELETE /api/v1/ingredients/:id
 * Menghapus atau menonaktifkan bahan.
 */
export async function deleteIngredient(
  id: string
): Promise<{ id: string; deleted?: boolean; deactivated?: boolean }> {
  return requestJson<{ id: string; deleted?: boolean; deactivated?: boolean }>(
    `/ingredients/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
    }
  );
}

/**
 * GET /api/v1/products
 * Mengambil daftar semua produk internal.
 */
export async function getProducts(signal?: AbortSignal): Promise<Product[]> {
  return requestJson<Product[]>("/products", {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/products
 * Membuat produk baru.
 */
export async function createProduct(
  payload: CreateProductRequest
): Promise<Product> {
  return requestJson<Product>("/products", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * PUT /api/v1/products/:id
 * Memperbarui data produk.
 */
export async function updateProduct(
  id: string,
  payload: UpdateProductRequest
): Promise<Product> {
  return requestJson<Product>(`/products/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

/**
 * DELETE /api/v1/products/:id
 * Menghapus atau menonaktifkan produk.
 */
export async function deleteProduct(
  id: string
): Promise<{ id: string; deleted?: boolean; deactivated?: boolean }> {
  return requestJson<{ id: string; deleted?: boolean; deactivated?: boolean }>(
    `/products/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
    }
  );
}

/**
 * GET /api/v1/products/:id/recipes
 * Mengambil resep bahan untuk satu produk.
 */
export async function getProductRecipes(
  productId: string,
  signal?: AbortSignal
): Promise<RecipeItem[]> {
  return requestJson<RecipeItem[]>(
    `/products/${encodeURIComponent(productId)}/recipes`,
    {
      method: "GET",
      signal,
    }
  );
}

/**
 * PUT /api/v1/products/:id/recipes
 * Menyimpan/mengganti daftar resep bahan untuk satu produk.
 */
export async function upsertProductRecipes(
  productId: string,
  payload: UpsertProductRecipesRequest
): Promise<RecipeItem[]> {
  return requestJson<RecipeItem[]>(
    `/products/${encodeURIComponent(productId)}/recipes`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * GET /api/v1/products/:id/loads
 * Mengambil beban produksi per sumber daya untuk satu produk.
 */
export async function getProductLoads(
  productId: string,
  signal?: AbortSignal
): Promise<ProductLoad[]> {
  return requestJson<ProductLoad[]>(
    `/products/${encodeURIComponent(productId)}/loads`,
    {
      method: "GET",
      signal,
    }
  );
}

/**
 * PUT /api/v1/products/:id/loads
 * Menyimpan/mengganti beban produksi per sumber daya untuk satu produk.
 */
export async function upsertProductLoads(
  productId: string,
  payload: UpsertProductLoadsRequest
): Promise<ProductLoad[]> {
  return requestJson<ProductLoad[]>(
    `/products/${encodeURIComponent(productId)}/loads`,
    {
      method: "PUT",
      body: JSON.stringify(payload),
    }
  );
}

/**
 * GET /api/v1/resources
 * Mengambil daftar sumber daya kapasitas (oven, kompor, tenaga kerja, pengemasan).
 */
export async function getResources(signal?: AbortSignal): Promise<Resource[]> {
  return requestJson<Resource[]>("/resources", {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/settings
 * Mengambil parameter bisnis (owner menerima semua field; manager menerima field non-kas).
 */
export async function getSettings(
  signal?: AbortSignal
): Promise<SettingsMap> {
  return requestJson<SettingsMap>("/settings", {
    method: "GET",
    signal,
  });
}

/**
 * PUT /api/v1/settings
 * Memperbarui parameter bisnis (khusus Owner).
 */
export async function updateSettings(
  payload: Record<string, string | number>
): Promise<SettingsMap> {
  return requestJson<SettingsMap>("/settings", {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

// ============================================================================
// Endpoint Dashboard per Peran, KPI, Pengguna, dan Audit Log (Tahap 6)
// ============================================================================

/**
 * GET /api/v1/dashboard/owner
 * Mengambil ringkasan Dashboard Pemilik Usaha (khusus Owner).
 */
export async function getOwnerDashboard(
  signal?: AbortSignal | { signal?: AbortSignal }
): Promise<OwnerDashboardResult> {
  return requestJson<OwnerDashboardResult>("/dashboard/owner", {
    method: "GET",
    signal: normalizeSignal(signal),
  });
}

/**
 * GET /api/v1/dashboard/manager
 * Mengambil ringkasan Dashboard Manajer Operasional (Manager & Owner).
 */
export async function getManagerDashboard(
  signal?: AbortSignal | { signal?: AbortSignal }
): Promise<ManagerDashboardResult> {
  return requestJson<ManagerDashboardResult>("/dashboard/manager", {
    method: "GET",
    signal: normalizeSignal(signal),
  });
}

/**
 * GET /api/v1/dashboard/staff
 * Mengambil ringkasan Dashboard Karyawan tanpa harga/margin/kas (Staff, Manager, Owner).
 */
export async function getStaffDashboard(
  signal?: AbortSignal | { signal?: AbortSignal }
): Promise<StaffDashboardResult> {
  return requestJson<StaffDashboardResult>("/dashboard/staff", {
    method: "GET",
    signal: normalizeSignal(signal),
  });
}

/**
 * GET /api/v1/kpi
 * Mengambil 7 indikator kinerja utama (Owner, Manager, Admin).
 */
export async function getKPI(signal?: AbortSignal): Promise<KPIResult> {
  return requestJson<KPIResult>("/kpi", {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/audit-logs
 * Mengambil daftar log audit dengan filter dan paginasi (khusus Owner & Admin).
 */
export async function getAuditLogs(
  params: AuditLogFilterParams = {},
  signal?: AbortSignal
): Promise<ApiListEnvelope<AuditLogItem>> {
  const qs = new URLSearchParams();
  if (params.action) qs.set("action", params.action);
  if (params.entity) qs.set("entity", params.entity);
  if (params.actor_id) qs.set("actor_id", params.actor_id);
  if (params.entity_id) qs.set("entity_id", params.entity_id);
  if (params.from) qs.set("from", params.from);
  if (params.to) qs.set("to", params.to);
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return requestListJson<AuditLogItem>(`/audit-logs${query}`, {
    method: "GET",
    signal,
  });
}

/**
 * GET /api/v1/users
 * Mengambil daftar pengguna dengan paginasi (khusus Admin).
 */
export async function getUsers(
  params: { page?: number; limit?: number } = {},
  signal?: AbortSignal
): Promise<ApiListEnvelope<User>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  const query = qs.toString() ? `?${qs.toString()}` : "";
  return requestListJson<User>(`/users${query}`, {
    method: "GET",
    signal,
  });
}

/**
 * POST /api/v1/users
 * Membuat akun pengguna baru (khusus Admin).
 */
export async function createUser(payload: CreateUserRequest): Promise<User> {
  return requestJson<User>("/users", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

/**
 * PUT /api/v1/users/:id
 * Memperbarui data pengguna (khusus Admin).
 */
export async function updateUser(
  userId: string,
  payload: UpdateUserRequest
): Promise<User> {
  return requestJson<User>(`/users/${encodeURIComponent(userId)}`, {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

/**
 * DELETE /api/v1/users/:id
 * Menonaktifkan (mengarsipkan) akun pengguna (khusus Admin).
 */
export async function deleteUser(
  userId: string
): Promise<{ id: string; archived: boolean; is_active: boolean }> {
  return requestJson<{ id: string; archived: boolean; is_active: boolean }>(
    `/users/${encodeURIComponent(userId)}`,
    {
      method: "DELETE",
    }
  );
}

/**
 * POST /api/v1/admin/reset-demo
 * Mereset dan mengisi ulang 40 pesanan demo deterministik dan kata sandi pengguna (khusus Admin).
 */
export async function resetDemoData(): Promise<Record<string, unknown>> {
  return requestJson<Record<string, unknown>>("/admin/reset-demo", {
    method: "POST",
  });
}

/**
 * POST /api/v1/admin/clean-transactions
 * Membersihkan seluruh transaksi pesanan, tugas, kas, dan audit log untuk mode produksi riil (khusus Admin).
 */
export async function cleanTransactions(): Promise<Record<string, unknown>> {
  return requestJson<Record<string, unknown>>("/admin/clean-transactions", {
    method: "POST",
  });
}

export interface RecalibrateWorkloadResult {
  message: string;
  product_id: string;
  product_name: string;
  old_minutes: string | number;
  new_minutes: string | number;
}

/**
 * POST /api/v1/capacity/recalibrate
 * Memperbarui standar durasi pengerjaan produk berdasarkan rekomendasi deviasi aktual (Khusus Manajer & Owner).
 */
export async function recalibrateWorkload(payload: {
  product_id?: string;
  new_minutes: string | number;
}): Promise<RecalibrateWorkloadResult> {
  return requestJson<RecalibrateWorkloadResult>("/capacity/recalibrate", {
    method: "POST",
    body: JSON.stringify({
      product_id: payload.product_id || undefined,
      new_minutes: String(payload.new_minutes),
    }),
  });
}




