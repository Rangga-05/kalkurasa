/**
 * Tipe data kontrak JSON API Kalkurasa (diselaraskan persis dengan
 * backend/internal/orders/handler.go, service.go, lifecycle.go,
 * backend/internal/auth, dan backend/internal/httpx).
 *
 * Catatan: shopspring/decimal di Go men-serialisasi nilai desimal sebagai
 * string JSON (mis. "25000" atau "0.6000"). Gunakan helper toNumber() di
 * src/lib/format.ts saat menampilkan angka.
 */

export type DecimalValue = string | number;

export interface ApiEnvelope<T> {
  data: T;
}

export interface PaginationMeta {
  page: number;
  limit: number;
  total: number;
}

export interface ApiListEnvelope<T> {
  data: T[];
  meta: PaginationMeta;
}

export interface ApiErrorBody {
  code: string;
  message: string;
}

export interface ApiErrorEnvelope {
  error: ApiErrorBody;
}

export type UserRole = "owner" | "manager" | "staff" | "admin" | "customer";

export interface User {
  id: string;
  email: string;
  name: string;
  contact?: string;
  role: UserRole;
  is_active: boolean;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  name: string;
  email: string;
  contact: string;
  password: string;
}

export interface LoginResponse {
  user: User;
  expires_at: string;
  token?: string;
}

export interface LogoutResponse {
  message: string;
}

export interface PublicProduct {
  id: string;
  name: string;
  category: string;
  price: DecimalValue;
}

export type DayCapacityStatus = "green" | "yellow" | "red" | "full";

export interface AvailabilityItemRequest {
  product_id: string;
  qty: number;
}

export interface AvailabilityRequest {
  items: AvailabilityItemRequest[];
  from: string; // YYYY-MM-DD
  to: string; // YYYY-MM-DD
}

export interface AvailabilityDay {
  date: string; // YYYY-MM-DD
  status: DayCapacityStatus;
  utilization_after: DecimalValue;
  available: boolean;
}

export type OrderStatus =
  | "submitted"
  | "pending_decision"
  | "countered"
  | "accepted"
  | "confirmed"
  | "in_production"
  | "ready"
  | "delivering"
  | "completed"
  | "rejected"
  | "cancelled"
  | "expired";

export type RecommendationType =
  | "accept"
  | "accept_with_conditions"
  | "reject";

export type DeliveryType = "pickup" | "delivery";

export interface CounterOfferDTO {
  date: string; // YYYY-MM-DD
  min_price_per_item: Record<string, DecimalValue>;
  dp: DecimalValue;
  utilization_after?: DecimalValue;
}

export interface PublicOrderItem {
  product_id: string;
  product_name: string;
  qty: number;
  unit_price: DecimalValue;
  subtotal: DecimalValue;
}

export interface CreatePublicOrderItemInput {
  product_id: string;
  qty: number;
}

export interface CreatePublicOrderRequest {
  customer_name?: string;
  customer_contact?: string;
  delivery_type: DeliveryType;
  address?: string;
  notes?: string;
  event_date: string; // YYYY-MM-DD
  items: CreatePublicOrderItemInput[];
}

export interface PublicOrderSummary {
  id: string;
  code: string;
  customer_id?: string | null;
  status: OrderStatus;
  recommendation: RecommendationType | string;
  event_date: string; // YYYY-MM-DD
  total: DecimalValue;
  dp_required: DecimalValue;
  dp_deadline: string | null;
  counter_offer?: CounterOfferDTO | null;
  items: PublicOrderItem[];
  created_at: string;
}

export interface PublicTrackResponse {
  code: string;
  customer_name: string;
  delivery_type: DeliveryType;
  event_date: string; // YYYY-MM-DD
  status: OrderStatus;
  items: PublicOrderItem[];
  subtotal?: DecimalValue;
  tax_pct?: DecimalValue;
  tax_amount?: DecimalValue;
  platform_fee?: DecimalValue;
  total: DecimalValue;
  dp_required: DecimalValue;
  dp_deadline: string | null;
  paid: DecimalValue;
  counter_offer: CounterOfferDTO | null;
  created_at: string;
  updated_at: string;
}

export interface PublicConfig {
  tax_pct: DecimalValue;
  platform_fee: DecimalValue;
}

export interface CounterResponseRequest {
  accept: boolean;
}

// ============================================================================
// Tipe Data Internal Alur Inti (Tahap 4B)
// ============================================================================

export type GateCode = "G1" | "G2" | "G3" | "G4" | string;

export interface GateDTO {
  gate: GateCode;
  value: DecimalValue;
  threshold: DecimalValue;
  passed: boolean;
}

export interface ImpactDTO {
  utilization_before: DecimalValue;
  utilization_after: DecimalValue;
  bottleneck: string;
  margin: DecimalValue;
  cash_lowest_before: DecimalValue;
  cash_lowest_after: DecimalValue;
}

export interface EvaluationDTO {
  gates: GateDTO[];
  recommendation: RecommendationType | string;
  reason: string;
  impact: ImpactDTO;
  counter_offer?: CounterOfferDTO | null;
}

export interface OrderItemDetail {
  id: string;
  product_id: string;
  product_name: string;
  qty: number;
  unit_price?: DecimalValue;
  unit_cost?: DecimalValue;
  subtotal?: DecimalValue;
}

export interface PaymentItem {
  id: string;
  order_id: string;
  amount: DecimalValue;
  method: string;
  paid_at: string;
  note?: string;
  recorded_by?: string | null;
}

export type TaskType = "produce" | "shop" | "deliver";
export type TaskStatus = "todo" | "doing" | "done";

export interface TaskOrderItem {
  product_id: string;
  product_name: string;
  qty: number;
}

export interface TaskItem {
  id: string;
  order_id: string;
  order_code?: string;
  customer_name?: string;
  customer_contact?: string;
  delivery_type?: DeliveryType;
  address?: string;
  notes?: string;
  event_date?: string;
  production_date?: string;
  order_status?: OrderStatus;
  type: TaskType;
  title: string;
  due_date: string; // YYYY-MM-DD
  assignee_id?: string | null;
  status: TaskStatus;
  actual_minutes?: number | null;
  items?: TaskOrderItem[];
  created_at: string;
}

export interface OrderAuditItem {
  id: string;
  actor_id?: string | null;
  actor_name?: string;
  action: string;
  detail?: Record<string, unknown>;
  created_at: string;
}

export interface OrderDetail {
  id: string;
  code: string;
  source: "public" | "manual" | string;
  customer_id?: string | null;
  customer_name: string;
  customer_contact: string;
  delivery_type: DeliveryType;
  address?: string;
  notes?: string;
  event_date: string; // YYYY-MM-DD
  production_date: string; // YYYY-MM-DD
  status: OrderStatus;
  subtotal?: DecimalValue;
  tax_pct?: DecimalValue;
  tax_amount?: DecimalValue;
  platform_fee?: DecimalValue;
  total?: DecimalValue;
  total_units: number;
  is_big_order?: boolean;
  can_decide?: boolean;
  dp_required?: DecimalValue;
  dp_deadline?: string | null;
  paid?: DecimalValue;
  recommendation?: RecommendationType | string;
  recommendation_reason?: string;
  counter_offer?: CounterOfferDTO | null;
  decided_by?: string | null;
  decided_at?: string | null;
  override_reason?: string;
  evaluation?: EvaluationDTO;
  items: OrderItemDetail[];
  payments?: PaymentItem[];
  tasks?: TaskItem[];
  audits?: OrderAuditItem[];
  created_at: string;
  updated_at: string;
}

export interface DecisionQueueItem {
  id: string;
  code: string;
  source: string;
  customer_name: string;
  customer_contact: string;
  delivery_type: DeliveryType;
  event_date: string;
  production_date: string;
  status: OrderStatus;
  total: DecimalValue;
  total_units: number;
  dp_required: DecimalValue;
  is_big_order: boolean;
  can_decide: boolean;
  recommendation: RecommendationType | string;
  recommendation_reason: string;
  counter_offer: CounterOfferDTO | null;
  evaluation: EvaluationDTO;
  items: OrderItemDetail[];
  created_at: string;
}

export interface DecideOrderRequest {
  action: "accept" | "counter" | "reject";
  override_reason?: string;
  counter?: {
    date: string;
    min_price_per_item?: Record<string, DecimalValue>;
    dp?: DecimalValue;
  };
}

export interface CreateManualOrderItemInput {
  product_id: string;
  qty: number;
  unit_price?: DecimalValue;
}

export interface CreateManualOrderRequest {
  customer_name: string;
  customer_contact: string;
  delivery_type: DeliveryType;
  address?: string;
  notes?: string;
  event_date: string; // YYYY-MM-DD
  production_date?: string; // YYYY-MM-DD (opsional, khusus manual)
  items: CreateManualOrderItemInput[];
}

export interface RecordPaymentRequest {
  amount: DecimalValue;
  method?: string;
  note?: string;
}

export interface PaymentResult {
  payment: PaymentItem;
  order_id: string;
  code: string;
  status: OrderStatus;
  total: DecimalValue;
  dp_required: DecimalValue;
  paid: DecimalValue;
  remaining: DecimalValue;
  tasks_created?: TaskItem[];
}

export interface TransitionOrderRequest {
  to_status: "in_production" | "ready" | "delivering" | "completed";
  actual_minutes?: number;
}

export interface CancelOrderRequest {
  reason?: string;
  dp_disposition?: "forfeited" | "refunded" | "pending";
  note?: string;
}

export interface CancelOrderResult {
  order_id: string;
  code: string;
  previous_status: OrderStatus;
  status: OrderStatus;
  shopping_cost: DecimalValue;
  dp_received: DecimalValue;
  cancellation_exposure: DecimalValue;
  cash_lowest_after?: DecimalValue;
  dp_disposition?: string;
  reason?: string;
  note?: string;
  cancelled_at: string;
}

export interface CapacityCalendarDay {
  date: string; // YYYY-MM-DD
  status: DayCapacityStatus;
  utilization: DecimalValue;
  bottleneck: string;
  loads: Record<string, DecimalValue>;
  capacities: Record<string, DecimalValue>;
}

export interface CapacityResourceDetail {
  code: string;
  name: string;
  unit: string;
  load: DecimalValue;
  capacity: DecimalValue;
  utilization: DecimalValue;
}

export interface CapacityOrderItem {
  id: string;
  code: string;
  customer_name: string;
  status: OrderStatus;
  event_date: string;
  production_date: string;
  total: DecimalValue;
}

export interface CapacityDayDetail {
  date: string; // YYYY-MM-DD
  status: DayCapacityStatus;
  utilization: DecimalValue;
  bottleneck: string;
  resources: CapacityResourceDetail[];
  orders: CapacityOrderItem[];
}

export interface OrderFilterParams {
  status?: string;
  date?: string;
  event_date?: string;
  from?: string;
  to?: string;
  search?: string;
}

export interface TaskFilterParams {
  mine?: boolean;
  status?: TaskStatus | "";
  type?: TaskType | "";
  date?: string;
  from?: string;
  to?: string;
}

export interface UpdateTaskRequest {
  status?: TaskStatus;
  actual_minutes?: number;
}

// ============================================================================
// Tipe Data Bahan, Belanja, Kas, Biaya Produk, dan Master Data (Tahap 5)
// ============================================================================

export type RoundingRule = "loose" | "pack";

export interface Ingredient {
  id: string;
  name: string;
  use_unit: string;
  buy_unit: string;
  buy_unit_size: DecimalValue;
  buy_price?: DecimalValue;
  stock: DecimalValue;
  min_stock: DecimalValue;
  below_min_stock: boolean;
  rounding_rule: RoundingRule;
  is_active: boolean;
  created_at: string;
}

export interface CreateIngredientRequest {
  name: string;
  use_unit: string;
  buy_unit: string;
  buy_unit_size: DecimalValue;
  buy_price: DecimalValue;
  stock?: DecimalValue;
  min_stock?: DecimalValue;
  rounding?: RoundingRule;
  rounding_rule?: RoundingRule;
  is_active?: boolean;
}

export interface UpdateIngredientRequest {
  name?: string;
  use_unit?: string;
  buy_unit?: string;
  buy_unit_size?: DecimalValue;
  buy_price?: DecimalValue;
  min_stock?: DecimalValue;
  rounding?: RoundingRule;
  rounding_rule?: RoundingRule;
  is_active?: boolean;
}

export interface ShoppingListItem {
  ingredient_id: string;
  ingredient_name: string;
  use_unit: string;
  buy_unit: string;
  buy_unit_size: DecimalValue;
  buy_price?: DecimalValue;
  rounding_rule: RoundingRule;
  required_qty: DecimalValue;
  stock: DecimalValue;
  min_stock: DecimalValue;
  below_min_stock: boolean;
  buffer_pct: DecimalValue;
  need_qty: DecimalValue;
  buy_use_units: DecimalValue;
  qty_buy_units: DecimalValue;
  estimated_cost?: DecimalValue;
  purchase_date: string; // YYYY-MM-DD
  earliest_event: string; // YYYY-MM-DD
  order_codes: string[];
}

export interface LowStockAlertItem {
  ingredient_id: string;
  ingredient_name: string;
  use_unit: string;
  buy_unit: string;
  buy_unit_size: DecimalValue;
  buy_price?: DecimalValue;
  stock: DecimalValue;
  min_stock: DecimalValue;
  deficit: DecimalValue;
}

export interface ShoppingListResult {
  from: string;
  to: string;
  buffer_pct: DecimalValue;
  lead_days: number;
  order_count: number;
  total_estimated?: DecimalValue;
  items: ShoppingListItem[];
  low_stock_items: LowStockAlertItem[];
}

export interface RecordPurchaseRequest {
  ingredient_id: string;
  qty_buy_units: DecimalValue;
  unit_price?: DecimalValue;
  total_cost?: DecimalValue;
  purchased_at?: string; // YYYY-MM-DD
  supplier?: string;
  note?: string;
}

export interface PurchaseResult {
  id?: string;
  purchase_id?: string;
  ingredient_id: string;
  ingredient_name: string;
  use_unit?: string;
  buy_unit?: string;
  buy_unit_size?: DecimalValue;
  qty_buy_units: DecimalValue;
  delta_use_units?: DecimalValue;
  added_use_units?: DecimalValue;
  stock_before?: DecimalValue;
  stock_after?: DecimalValue;
  new_stock?: DecimalValue;
  unit_price?: DecimalValue;
  total_cost?: DecimalValue;
  new_buy_price?: DecimalValue;
  purchased_at?: string;
  supplier?: string;
  cash_entry_id?: string;
}

export interface StockAdjustRequest {
  ingredient_id: string;
  qty?: DecimalValue; // perubahan (+/-) dalam use_unit
  delta?: DecimalValue;
  new_stock?: DecimalValue;
  reason: string;
}

export interface StockAdjustResult {
  movement_id: string;
  ingredient_id: string;
  ingredient_name: string;
  use_unit: string;
  delta?: DecimalValue;
  qty_change?: DecimalValue;
  previous_stock: DecimalValue;
  new_stock: DecimalValue;
  below_min_stock?: boolean;
  reason?: string;
}

export interface CashProjectionDayDTO {
  date: string; // YYYY-MM-DD
  inflow: DecimalValue;
  outflow: DecimalValue;
  fixed_cost: DecimalValue;
  closing: DecimalValue;
  below_buffer: boolean;
}

export interface CashEntryDTO {
  id: string;
  entry_date: string; // YYYY-MM-DD
  amount: DecimalValue;
  category: string;
  note?: string;
  order_id?: string | null;
  created_at: string;
}

export interface CashProjectionResult {
  from: string;
  to: string;
  horizon_days: number;
  opening_cash?: DecimalValue;
  fixed_cost_daily?: DecimalValue;
  cash_buffer: DecimalValue;
  passed: boolean;
  lowest: DecimalValue;
  lowest_date: string;
  days_below_buffer: string[];
  days?: CashProjectionDayDTO[];
  entries?: CashEntryDTO[];
}

export interface CreateCashEntryRequest {
  entry_date: string; // YYYY-MM-DD
  amount: DecimalValue; // positif untuk masuk, negatif untuk keluar
  category: string;
  note?: string;
}

export interface Product {
  id: string;
  name: string;
  category: string;
  price: DecimalValue;
  packaging_cost?: DecimalValue;
  overhead_per_unit?: DecimalValue;
  waste_pct: DecimalValue;
  lead_time_days: number;
  is_active: boolean;
  created_at: string;
}

export interface CreateProductRequest {
  name: string;
  category?: string;
  price: DecimalValue;
  packaging_cost?: DecimalValue;
  overhead_cost?: DecimalValue;
  overhead_per_unit?: DecimalValue;
  waste_pct?: DecimalValue;
  lead_time_days?: number;
  is_active?: boolean;
}

export interface UpdateProductRequest {
  name?: string;
  category?: string;
  price?: DecimalValue;
  packaging_cost?: DecimalValue;
  overhead_cost?: DecimalValue;
  overhead_per_unit?: DecimalValue;
  waste_pct?: DecimalValue;
  lead_time_days?: number;
  is_active?: boolean;
}

export interface RecipeItem {
  product_id: string;
  ingredient_id: string;
  ingredient_name?: string;
  use_unit?: string;
  qty_per_unit: DecimalValue;
}

export interface UpsertProductRecipesRequest {
  replace_all: boolean;
  items?: {
    ingredient_id: string;
    qty_per_unit: DecimalValue;
  }[];
  recipes?: {
    ingredient_id: string;
    qty_per_unit: DecimalValue;
  }[];
}

export interface ProductLoad {
  product_id: string;
  resource_id: string;
  resource_code?: string;
  resource_name?: string;
  resource_unit?: string;
  load?: DecimalValue;
  load_per_unit: DecimalValue;
}

export interface UpsertProductLoadsRequest {
  replace_all: boolean;
  loads?: {
    resource_id: string;
    load?: DecimalValue;
    load_per_unit?: DecimalValue;
  }[];
  items?: {
    resource_id: string;
    load?: DecimalValue;
    load_per_unit?: DecimalValue;
  }[];
}

export interface Resource {
  id: string;
  code: string;
  name: string;
  unit: string;
  daily_capacity: DecimalValue;
  created_at: string;
}

export interface ProductCostingDTO {
  product_id: string;
  name: string;
  category: string;
  is_active: boolean;
  price: DecimalValue;
  ingredient_cost: DecimalValue;
  waste_pct: DecimalValue;
  packaging_cost: DecimalValue;
  labor_minutes: DecimalValue;
  labor_cost: DecimalValue;
  overhead_per_unit: DecimalValue;
  unit_cost: DecimalValue;
  margin: DecimalValue;
  target_margin: DecimalValue;
  min_margin: DecimalValue;
  min_price: DecimalValue;
  max_discount: DecimalValue;
}

export type SettingsMap = Record<string, string>;

// ============================================================================
// Tipe Data Dashboard per Peran, KPI, Pengguna, dan Audit Log (Tahap 6)
// ============================================================================

export interface CounteredOrRejectedStats {
  total_countered: number;
  counter_pending: number;
  counter_accepted: number;
  counter_rejected: number;
  direct_rejected: number;
  total_rejected: number;
  total_countered_or_rejected: number;
}

export interface OrderMarginKPI {
  order_id: string;
  code: string;
  customer_name: string;
  event_date: string;
  status: OrderStatus;
  total: DecimalValue;
  estimated_cost: DecimalValue;
  actual_cost: DecimalValue;
  estimated_margin: DecimalValue;
  realized_margin: DecimalValue;
}

export interface CalibrationSuggestion {
  order_count: number;
  estimated_minutes: DecimalValue;
  avg_actual_minutes: DecimalValue;
  deviation: DecimalValue;
  should_recalibrate: boolean;
}

export interface KPIResult {
  on_time_rate: DecimalValue;
  completed_orders: number;
  on_time_orders: number;

  avg_utilization: DecimalValue;
  peak_utilization: DecimalValue;
  peak_utilization_date?: string;
  active_production_days: number;

  ingredient_cost_variance: DecimalValue;
  estimated_ingredient_cost: DecimalValue;
  actual_ingredient_cost: DecimalValue;

  cancellation_rate: DecimalValue;
  cancelled_orders: number;
  expired_orders: number;
  cancelled_or_expired_orders: number;
  accepted_orders_total: number;

  countered_or_rejected: CounteredOrRejectedStats;

  estimated_margin: DecimalValue;
  realized_margin: DecimalValue;
  total_revenue: DecimalValue;
  total_estimated_cost: DecimalValue;
  total_actual_cost: DecimalValue;
  orders_margin: OrderMarginKPI[];

  vulnerable_cash_days: number;
  vulnerable_cash_dates: string[];
  cash_lowest: DecimalValue;
  cash_lowest_date: string;
  cash_buffer: DecimalValue;
  cash_passed: boolean;

  calibration: CalibrationSuggestion;
}

export interface OwnerDashboardResult {
  today: string;
  decision_queue: DecisionQueueItem[];
  cash_projection: CashProjectionResult;
  kpi: KPIResult;
  capacity_calendar: CapacityCalendarDay[];
}

export interface DPDueAlertItem {
  order_id: string;
  code: string;
  customer_name: string;
  event_date: string;
  total: DecimalValue;
  dp_required: DecimalValue;
  paid: DecimalValue;
  dp_deadline?: string | null;
}

export interface ManagerDashboardResult {
  today: string;
  capacity_calendar: CapacityCalendarDay[];
  decision_queue: DecisionQueueItem[];
  production_schedule: OrderDetail[];
  shopping_list: ShoppingListResult;
  low_stock_alerts: LowStockAlertItem[];
  dp_due_alerts: DPDueAlertItem[];
  cash_status: CashProjectionResult;
  kpi: KPIResult;
}

export interface StaffDeliveryItem {
  order_id: string;
  order_code: string;
  customer_name: string;
  customer_contact: string;
  delivery_type: DeliveryType;
  address?: string;
  notes?: string;
  event_date: string;
  production_date: string;
  order_status: OrderStatus;
  task_id?: string;
  task_status?: TaskStatus;
  items: TaskOrderItem[];
}

export interface StaffDashboardResult {
  today: string;
  today_tasks: TaskItem[];
  active_tasks: TaskItem[];
  deliveries: StaffDeliveryItem[];
  low_stock_alerts: LowStockAlertItem[];
}

export interface AuditLogItem {
  id: string;
  actor_id: string | null;
  actor_name?: string;
  actor_role?: string;
  action: string;
  entity: string;
  entity_id: string | null;
  detail: Record<string, unknown>;
  created_at: string;
}

export interface AuditLogFilterParams {
  action?: string;
  entity?: string;
  actor_id?: string;
  entity_id?: string;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
}

export interface CreateUserRequest {
  email: string;
  name: string;
  role: UserRole;
  password: string;
  is_active?: boolean;
}

export interface UpdateUserRequest {
  email?: string;
  name: string;
  role: UserRole;
  password?: string;
  is_active?: boolean;
}



