package engine

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kode rekomendasi (BR-10).
const (
	RecAccept               = "accept"
	RecAcceptWithConditions = "accept_with_conditions"
	RecReject               = "reject"
)

// CashEvent adalah satu peristiwa kas pada satu tanggal.
// Amount positif = masuk, negatif = keluar. Hanya tanggal kalender (tahun,
// bulan, hari) pada zona waktu milik Date yang dipakai.
type CashEvent struct {
	Date   time.Time
	Amount decimal.Decimal
}

// DailyBalance adalah saldo akhir satu hari pada proyeksi kas.
type DailyBalance struct {
	Date    time.Time
	Balance decimal.Decimal
}

// CashProjection adalah hasil ProjectCash (BR-08).
type CashProjection struct {
	Days       []DailyBalance
	Lowest     decimal.Decimal
	LowestDate time.Time
}

// EvalItem adalah satu baris pesanan yang dievaluasi.
type EvalItem struct {
	ProductID           string
	Qty                 int
	UnitPrice           decimal.Decimal
	UnitCost            decimal.Decimal            // biaya per unit (BR-05)
	PurchaseCostPerUnit decimal.Decimal            // belanja bahan per unit (bahan + waste + kemasan)
	UnitLoads           map[string]decimal.Decimal // beban per unit per kode sumber daya
}

// EvalParams adalah parameter bisnis (settings) yang dibutuhkan evaluasi.
type EvalParams struct {
	CapacityLimit    decimal.Decimal // batas terima, mis. 0,90
	MarginMin        decimal.Decimal // mis. 0,20
	CashBuffer       decimal.Decimal
	FixedCostDaily   decimal.Decimal
	OpeningCash      decimal.Decimal // saldo kas pada hari Today
	DPPct            decimal.Decimal // mis. 0,30
	LeadTimeDays     int
	ShoppingLeadDays int
}

// EvalInput adalah seluruh masukan EvaluateOrder. Engine tidak membaca jam
// sistem: "hari ini" selalu datang dari Today.
type EvalInput struct {
	Today          time.Time
	EventDate      time.Time
	ProductionDate time.Time // kosong = sama dengan EventDate
	// DPDate adalah tanggal DP pesanan ini masuk ke kas. Kosong = Today.
	DPDate time.Time

	Items  []EvalItem
	Params EvalParams

	// Capacities: kapasitas harian per kode sumber daya (hasil Capacity).
	Capacities map[string]decimal.Decimal
	// ExistingLoads: beban yang sudah terpesan (pesanan accepted/confirmed/
	// in_production) per tanggal produksi (kunci DateKey) per kode sumber daya.
	ExistingLoads map[string]map[string]decimal.Decimal
	// CashEvents: peristiwa kas pesanan aktif lain (tanpa pesanan ini).
	CashEvents []CashEvent
}

// GateResult adalah hasil satu gerbang (disimpan ke order_evaluations).
type GateResult struct {
	Gate      string // G1..G4
	Passed    bool
	Value     decimal.Decimal
	Threshold decimal.Decimal
}

// Impact adalah pratinjau dampak (FR-DEC-06).
type Impact struct {
	UtilizationBefore decimal.Decimal
	UtilizationAfter  decimal.Decimal
	Margin            decimal.Decimal
	CashLowestBefore  decimal.Decimal
	CashLowestAfter   decimal.Decimal
}

// CounterOffer adalah tawaran balik otomatis (Bagian 8.3).
type CounterOffer struct {
	Date             time.Time
	MinPricePerItem  map[string]decimal.Decimal // kunci ProductID
	DP               decimal.Decimal            // DP ditagih pada syarat tawaran balik
	UtilizationAfter decimal.Decimal            // utilisasi hari tawaran bila diterima
}

// EvalResult adalah hasil EvaluateOrder.
type EvalResult struct {
	Recommendation string
	Reason         string
	Gates          []GateResult // selalu 4 elemen: G1, G2, G3, G4
	Impact         Impact
	Total          decimal.Decimal
	MinDP          decimal.Decimal
	DPCharged      decimal.Decimal
	Bottleneck     string // sumber daya penghambat pada tanggal produksi
	CounterOffer   *CounterOffer
}

