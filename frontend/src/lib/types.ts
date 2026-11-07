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
  total: DecimalValue;
  dp_required: DecimalValue;
  dp_deadline: string | null;
  paid: DecimalValue;
  counter_offer: CounterOfferDTO | null;
  created_at: string;
  updated_at: string;
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

