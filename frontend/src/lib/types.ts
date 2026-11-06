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

export type UserRole = "owner" | "manager" | "staff" | "admin";

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  is_active: boolean;
  created_at: string;
}

export interface LoginRequest {
  email: string;
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
  customer_name: string;
  customer_contact: string;
  delivery_type: DeliveryType;
  address?: string;
  notes?: string;
  event_date: string; // YYYY-MM-DD
  items: CreatePublicOrderItemInput[];
}

export interface PublicOrderSummary {
  id: string;
  code: string;
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

