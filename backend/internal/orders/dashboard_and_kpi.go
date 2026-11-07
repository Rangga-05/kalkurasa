package orders

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// CounteredOrRejectedStats memuat jumlah pesanan yang ditawar balik atau ditolak beserta hasil akhirnya (Bagian 10 Spesifikasi).
type CounteredOrRejectedStats struct {
	TotalCountered           int `json:"total_countered"`
	CounterPending           int `json:"counter_pending"`
	CounterAccepted          int `json:"counter_accepted"`
	CounterRejected          int `json:"counter_rejected"`
	DirectRejected           int `json:"direct_rejected"`
	TotalRejected            int `json:"total_rejected"`
	TotalCounteredOrRejected int `json:"total_countered_or_rejected"`
}

// OrderMarginKPI memuat perbandingan biaya & margin estimasi vs realisasi per pesanan.
type OrderMarginKPI struct {
	OrderID         string          `json:"order_id"`
	Code            string          `json:"code"`
	CustomerName    string          `json:"customer_name"`
	EventDate       string          `json:"event_date"`
	Status          string          `json:"status"`
	Total           decimal.Decimal `json:"total"`
	EstimatedCost   decimal.Decimal `json:"estimated_cost"`
	ActualCost      decimal.Decimal `json:"actual_cost"`
	EstimatedMargin decimal.Decimal `json:"estimated_margin"`
	RealizedMargin  decimal.Decimal `json:"realized_margin"`
}

// CalibrationSuggestion memuat saran kalibrasi durasi tenaga kerja produk berdasarkan BR-12 (TC-07).
type CalibrationSuggestion struct {
	OrderCount          int             `json:"order_count"`
	EstimatedMinutes    decimal.Decimal `json:"estimated_minutes"`
	AvgActualMinutes    decimal.Decimal `json:"avg_actual_minutes"`
	Deviation           decimal.Decimal `json:"deviation"`
	ShouldRecalibrate   bool            `json:"should_recalibrate"`
}

// KPIResult adalah struktur respons GET /api/v1/kpi sesuai Bagian 10 Spesifikasi.
type KPIResult struct {
	// 1. Ketepatan waktu: Pesanan selesai paling lambat tanggal acara ÷ pesanan selesai
	OnTimeRate      decimal.Decimal `json:"on_time_rate"`
	CompletedOrders int             `json:"completed_orders"`
	OnTimeOrders    int             `json:"on_time_orders"`

	// 2. Utilisasi rata-rata: Rata-rata utilisasi hari produksi
	AvgUtilization       decimal.Decimal `json:"avg_utilization"`
	PeakUtilization      decimal.Decimal `json:"peak_utilization"`
	PeakUtilizationDate  string          `json:"peak_utilization_date,omitempty"`
	ActiveProductionDays int             `json:"active_production_days"`

	// 3. Selisih biaya bahan: (biaya aktual − estimasi) ÷ estimasi
	IngredientCostVariance  decimal.Decimal `json:"ingredient_cost_variance"`
	EstimatedIngredientCost decimal.Decimal `json:"estimated_ingredient_cost"`
	ActualIngredientCost    decimal.Decimal `json:"actual_ingredient_cost"`

	// 4. Tingkat pembatalan: Pesanan Dibatalkan atau Kedaluwarsa ÷ pesanan diterima
	CancellationRate         decimal.Decimal `json:"cancellation_rate"`
	CancelledOrders          int             `json:"cancelled_orders"`
	ExpiredOrders            int             `json:"expired_orders"`
	CancelledOrExpiredOrders int             `json:"cancelled_or_expired_orders"`
	AcceptedOrdersTotal      int             `json:"accepted_orders_total"`

	// 5. Pesanan ditawar balik atau ditolak: Jumlah dan hasil akhirnya
	CounteredOrRejected CounteredOrRejectedStats `json:"countered_or_rejected"`

	// 6. Margin realisasi: Margin per pesanan berdasarkan biaya aktual
	EstimatedMargin    decimal.Decimal  `json:"estimated_margin"`
	RealizedMargin     decimal.Decimal  `json:"realized_margin"`
	TotalRevenue       decimal.Decimal  `json:"total_revenue"`
	TotalEstimatedCost decimal.Decimal  `json:"total_estimated_cost"`
	TotalActualCost    decimal.Decimal  `json:"total_actual_cost"`
	OrdersMargin       []OrderMarginKPI `json:"orders_margin"`

	// 7. Hari kas rawan: Jumlah hari proyeksi saldo di bawah buffer
	VulnerableCashDays  int             `json:"vulnerable_cash_days"`
	VulnerableCashDates []string        `json:"vulnerable_cash_dates"`
	CashLowest          decimal.Decimal `json:"cash_lowest"`
	CashLowestDate      string          `json:"cash_lowest_date"`
	CashBuffer          decimal.Decimal `json:"cash_buffer"`
	CashPassed          bool            `json:"cash_passed"`

	// Ringkasan kalibrasi durasi aktual produksi (BR-12)
	Calibration CalibrationSuggestion `json:"calibration"`
}

// GetKPI menghitung ketujuh indikator kinerja utama (Bagian 10 Spesifikasi) menggunakan internal/engine.
func GetKPI(ctx context.Context, conn db.Conn, actorRole string) (*KPIResult, error) {
	if actorRole != auth.RoleOwner && actorRole != auth.RoleManager && actorRole != auth.RoleAdmin {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk melihat KPI.")
	}

	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, conn)
	if err != nil {
		return nil, err
	}

	// Muat seluruh pesanan dengan visibilitas internal penuh untuk perhitungan agregat
	allOrders, err := ListOrders(ctx, conn, OrderListFilter{ActorRole: auth.RoleOwner})
	if err != nil {
		return nil, err
	}

	// Muat biaya aktual produk terkini (memakai harga beli bahan terkini di ingredients)
	costingList, err := GetProductsCosting(ctx, conn)
	if err != nil {
		return nil, err
	}
	costByProd := make(map[string]ProductCostingDTO, len(costingList))
	for _, c := range costingList {
		costByProd[c.ProductID] = c
	}

	// Muat kapasitas sumber daya harian
	caps, err := LoadCapacities(ctx, conn)
	if err != nil {
		return nil, err
	}

	var (
		completedCount int
		onTimeCount    int

		cancelledCount     int
		expiredCount       int
		acceptedTotalCount int

		counterStats CounteredOrRejectedStats

		totalEstIngCost = decimal.Zero
		totalActIngCost = decimal.Zero

		totalRevenue = decimal.Zero
		totalEstCost = decimal.Zero
		totalActCost = decimal.Zero
		ordersMargin = make([]OrderMarginKPI, 0)

		prodDayLoads = make(map[string][]engine.LoadItem)

		actualMinSum = decimal.Zero
		estMinSum    = decimal.Zero
		sampleCount  int
	)

	onePlusWaste := decimal.NewFromInt(1).Add(params.WastePct)

	// Urutkan kronologis menaik untuk daftar margin per pesanan
	sortedOrders := make([]OrderDetail, len(allOrders))
	copy(sortedOrders, allOrders)
	sort.Slice(sortedOrders, func(i, j int) bool {
		if sortedOrders[i].EventDate == sortedOrders[j].EventDate {
			return sortedOrders[i].CreatedAt.Before(sortedOrders[j].CreatedAt)
		}
		return sortedOrders[i].EventDate < sortedOrders[j].EventDate
	})

	for _, o := range sortedOrders {
		// 1. Ketepatan waktu (pesanan completed yang selesai <= event_date)
		if o.Status == StatusCompleted {
			completedCount++
			completedDayKey := engine.DateKey(o.UpdatedAt)
			if completedDayKey <= o.EventDate {
				onTimeCount++
			}
		}

		// 4. Tingkat pembatalan (cancelled + expired dibagi seluruh pesanan yang pernah diterima)
		switch o.Status {
		case StatusAccepted, StatusConfirmed, StatusInProduction, StatusReady, StatusDelivering, StatusCompleted:
			acceptedTotalCount++
		case StatusCancelled:
			acceptedTotalCount++
			cancelledCount++
		case StatusExpired:
			acceptedTotalCount++
			expiredCount++
		}

		// 5. Pesanan ditawar balik atau ditolak beserta hasil akhirnya
		hadCounter := o.CounterOffer != nil || o.Status == StatusCountered || o.Recommendation == "accept_with_conditions"
		if o.Status == StatusCountered {
			counterStats.TotalCountered++
			counterStats.CounterPending++
		} else if hadCounter && o.Status != StatusPendingDecision && o.Status != StatusSubmitted {
			counterStats.TotalCountered++
			if o.Status == StatusRejected {
				counterStats.CounterRejected++
				counterStats.TotalRejected++
			} else {
				counterStats.CounterAccepted++
			}
		} else if o.Status == StatusRejected {
			counterStats.DirectRejected++
			counterStats.TotalRejected++
		}

		// 2. Kumpulkan beban hari produksi untuk pesanan yang mereservasi/menyelesaikan produksi
		isProdOrder := o.Status == StatusAccepted ||
			o.Status == StatusConfirmed ||
			o.Status == StatusInProduction ||
			o.Status == StatusReady ||
			o.Status == StatusDelivering ||
			o.Status == StatusCompleted

		if isProdOrder {
			for _, it := range o.Items {
				pc, ok := costByProd[it.ProductID]
				if !ok {
					continue
				}
				qtyDec := decimal.NewFromInt(int64(it.Qty))
				for resCode, ld := range pc.Loads {
					prodDayLoads[o.ProductionDate] = append(prodDayLoads[o.ProductionDate], engine.LoadItem{
						ResourceCode: resCode,
						Qty:          qtyDec,
						Load:         ld,
					})
				}
			}
		}

		// 3 & 6. Selisih biaya bahan & Margin realisasi (untuk pesanan terkonfirmasi s.d. selesai)
		isConfirmedOrBeyond := o.Status == StatusConfirmed ||
			o.Status == StatusInProduction ||
			o.Status == StatusReady ||
			o.Status == StatusDelivering ||
			o.Status == StatusCompleted

		if isConfirmedOrBeyond {
			ordEstCost := decimal.Zero
			ordActCost := decimal.Zero
			ordEstLaborMin := decimal.Zero

			for _, it := range o.Items {
				qtyDec := decimal.NewFromInt(int64(it.Qty))
				ordEstCost = ordEstCost.Add(qtyDec.Mul(it.UnitCost))

				pc, ok := costByProd[it.ProductID]
				if ok {
					// Biaya bahan aktual per unit (termasuk waste)
					actIngUnit := pc.IngredientCost.Mul(onePlusWaste)
					// Biaya bahan estimasi per unit saat snapshot pesanan dibuat
					nonIngUnit := pc.PackagingCost.Add(pc.LaborCost).Add(pc.OverheadCost)
					estIngUnit := it.UnitCost.Sub(nonIngUnit)
					if estIngUnit.IsNegative() {
						estIngUnit = decimal.Zero
					}

					totalEstIngCost = totalEstIngCost.Add(qtyDec.Mul(estIngUnit))
					totalActIngCost = totalActIngCost.Add(qtyDec.Mul(actIngUnit))

					ordActCost = ordActCost.Add(qtyDec.Mul(pc.UnitCost))
					ordEstLaborMin = ordEstLaborMin.Add(qtyDec.Mul(pc.LaborMinutes))
				} else {
					ordActCost = ordActCost.Add(qtyDec.Mul(it.UnitCost))
				}
			}

			// Jika tugas produce pada pesanan ini memiliki actual_minutes, gunakan durasi aktual tersebut
			// untuk menghitung biaya tenaga aktual dan sampel kalibrasi (BR-12)
			for _, tk := range o.Tasks {
				if tk.Type == "produce" && tk.ActualMinutes != nil && *tk.ActualMinutes > 0 {
					actMinDec := decimal.NewFromInt(int64(*tk.ActualMinutes))
					estLaborCost := ordEstLaborMin.Mul(params.LaborRatePerMin)
					actLaborCost := actMinDec.Mul(params.LaborRatePerMin)
					ordActCost = ordActCost.Sub(estLaborCost).Add(actLaborCost)
					if ordActCost.IsNegative() {
						ordActCost = decimal.Zero
					}
					if ordEstLaborMin.IsPositive() {
						actualMinSum = actualMinSum.Add(actMinDec)
						estMinSum = estMinSum.Add(ordEstLaborMin)
						sampleCount++
					}
					break
				}
			}

			estMargin := engine.Margin(o.Total, ordEstCost)
			realMargin := engine.Margin(o.Total, ordActCost)

			totalRevenue = totalRevenue.Add(o.Total)
			totalEstCost = totalEstCost.Add(ordEstCost)
			totalActCost = totalActCost.Add(ordActCost)

			ordersMargin = append(ordersMargin, OrderMarginKPI{
				OrderID:         o.ID,
				Code:            o.Code,
				CustomerName:    o.CustomerName,
				EventDate:       o.EventDate,
				Status:          o.Status,
				Total:           o.Total.Round(2),
				EstimatedCost:   ordEstCost.Round(2),
				ActualCost:      ordActCost.Round(2),
				EstimatedMargin: estMargin.Round(4),
				RealizedMargin:  realMargin.Round(4),
			})
		}
	}

	counterStats.TotalCounteredOrRejected = counterStats.TotalCountered + counterStats.DirectRejected

	// Hitung rasio ketepatan waktu
	onTimeRate := decimal.Zero
	if completedCount > 0 {
		onTimeRate = decimal.NewFromInt(int64(onTimeCount)).
			DivRound(decimal.NewFromInt(int64(completedCount)), 4)
	}

	// Hitung utilisasi rata-rata hari produksi
	prodDates := make([]string, 0, len(prodDayLoads))
	for d := range prodDayLoads {
		prodDates = append(prodDates, d)
	}
	sort.Strings(prodDates)

	utilSum := decimal.Zero
	peakUtil := decimal.Zero
	peakDate := ""
	for _, d := range prodDates {
		dayMap := engine.DayLoads(prodDayLoads[d])
		u, _ := engine.DayUtilization(dayMap, caps)
		utilSum = utilSum.Add(u)
		if u.GreaterThan(peakUtil) || peakDate == "" {
			peakUtil = u
			peakDate = d
		}
	}
	avgUtil := decimal.Zero
	if len(prodDates) > 0 {
		avgUtil = utilSum.DivRound(decimal.NewFromInt(int64(len(prodDates))), 4)
	}

	// Hitung selisih biaya bahan via engine.Deviation
	ingVariance := decimal.Zero
	if totalEstIngCost.IsPositive() {
		ingVariance = engine.Deviation(totalActIngCost, totalEstIngCost).Round(4)
	}

	// Hitung tingkat pembatalan
	cancelRate := decimal.Zero
	cancelledOrExpired := cancelledCount + expiredCount
	if acceptedTotalCount > 0 {
		cancelRate = decimal.NewFromInt(int64(cancelledOrExpired)).
			DivRound(decimal.NewFromInt(int64(acceptedTotalCount)), 4)
	}

	// Hitung margin estimasi & margin realisasi agregat via engine.Margin
	aggEstMargin := engine.Margin(totalRevenue, totalEstCost).Round(4)
	aggRealMargin := engine.Margin(totalRevenue, totalActCost).Round(4)

	// 7. Proyeksi kas 14 hari untuk menghitung hari kas rawan
	cashProj, err := GetCashProjection(ctx, conn, 14, auth.RoleOwner)
	if err != nil {
		return nil, err
	}
	vulnDates := cashProj.DaysBelowBuffer
	if vulnDates == nil {
		vulnDates = []string{}
	}

	// Kalibrasi durasi aktual produksi (BR-12)
	var cal CalibrationSuggestion
	if sampleCount > 0 {
		nDec := decimal.NewFromInt(int64(sampleCount))
		avgAct := actualMinSum.DivRound(nDec, 2)
		avgEst := estMinSum.DivRound(nDec, 2)
		dev := engine.Deviation(avgAct, avgEst)
		cal = CalibrationSuggestion{
			OrderCount:        sampleCount,
			EstimatedMinutes:  avgEst,
			AvgActualMinutes:  avgAct,
			Deviation:         dev.Round(4),
			ShouldRecalibrate: engine.ShouldRecalibrate(dev, sampleCount),
		}
	}

	return &KPIResult{
		OnTimeRate:               onTimeRate,
		CompletedOrders:          completedCount,
		OnTimeOrders:             onTimeCount,
		AvgUtilization:           avgUtil,
		PeakUtilization:          peakUtil.Round(4),
		PeakUtilizationDate:      peakDate,
		ActiveProductionDays:     len(prodDates),
		IngredientCostVariance:   ingVariance,
		EstimatedIngredientCost:  totalEstIngCost.Round(2),
		ActualIngredientCost:     totalActIngCost.Round(2),
		CancellationRate:         cancelRate,
		CancelledOrders:          cancelledCount,
		ExpiredOrders:            expiredCount,
		CancelledOrExpiredOrders: cancelledOrExpired,
		AcceptedOrdersTotal:      acceptedTotalCount,
		CounteredOrRejected:      counterStats,
		EstimatedMargin:          aggEstMargin,
		RealizedMargin:           aggRealMargin,
		TotalRevenue:             totalRevenue.Round(2),
		TotalEstimatedCost:       totalEstCost.Round(2),
		TotalActualCost:          totalActCost.Round(2),
		OrdersMargin:             ordersMargin,
		VulnerableCashDays:       len(vulnDates),
		VulnerableCashDates:      vulnDates,
		CashLowest:               cashProj.Lowest,
		CashLowestDate:           cashProj.LowestDate,
		CashBuffer:               params.EvalParams.CashBuffer.Round(2),
		CashPassed:               cashProj.Passed,
		Calibration:              cal,
	}, nil
}

// OwnerDashboardResult memuat keempat elemen utama Dashboard Pemilik (Bagian 10 Brief):
// 1. Antrian keputusan dengan pratinjau dampak
// 2. Proyeksi kas 14 hari dengan hari rawan
// 3. Margin dan KPI ringkas
// 4. Kalender kapasitas ringkas
type OwnerDashboardResult struct {
	Today            string                `json:"today"`
	DecisionQueue    []DecisionQueueItem   `json:"decision_queue"`
	CashProjection   *CashProjectionResult `json:"cash_projection"`
	KPI              *KPIResult            `json:"kpi"`
	CapacityCalendar []CapacityCalendarDay `json:"capacity_calendar"`
}

// GetOwnerDashboard mengembalikan data lengkap untuk GET /api/v1/dashboard/owner (khusus owner).
func GetOwnerDashboard(ctx context.Context, conn db.Conn, actorRole string) (*OwnerDashboardResult, error) {
	if actorRole != auth.RoleOwner {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Hanya pemilik usaha (owner) yang berhak mengakses dashboard pemilik.")
	}

	today := db.TodayQ(ctx, conn)
	todayKey := engine.DateKey(today)
	monthKey := today.Format("2006-01")

	queue, err := GetDecisionQueue(ctx, conn, auth.RoleOwner)
	if err != nil {
		return nil, err
	}
	cashProj, err := GetCashProjection(ctx, conn, 14, auth.RoleOwner)
	if err != nil {
		return nil, err
	}
	kpiRes, err := GetKPI(ctx, conn, auth.RoleOwner)
	if err != nil {
		return nil, err
	}
	cal, err := GetCapacityCalendar(ctx, conn, monthKey)
	if err != nil {
		return nil, err
	}

	return &OwnerDashboardResult{
		Today:            todayKey,
		DecisionQueue:    queue,
		CashProjection:   cashProj,
		KPI:              kpiRes,
		CapacityCalendar: cal,
	}, nil
}

// DPDueAlertItem merepresentasikan peringatan pesanan yang sedang menunggu pembayaran DP.
type DPDueAlertItem struct {
	OrderID      string          `json:"order_id"`
	Code         string          `json:"code"`
	CustomerName string          `json:"customer_name"`
	EventDate    string          `json:"event_date"`
	Total        decimal.Decimal `json:"total"`
	DPRequired   decimal.Decimal `json:"dp_required"`
	Paid         decimal.Decimal `json:"paid"`
	DPDeadline   *time.Time      `json:"dp_deadline"`
}

// ManagerDashboardResult memuat elemen Dashboard Manajer (Bagian 10 Brief):
// 1. Kalender kapasitas
// 2. Antrian keputusan di bawah ambang (beserta penanda pesanan besar)
// 3. Jadwal produksi & daftar belanja
// 4. Peringatan stok di bawah minimum & tenggat DP
// 5. Status kelayakan kas (tanpa saldo awal) & KPI
type ManagerDashboardResult struct {
	Today              string                `json:"today"`
	CapacityCalendar   []CapacityCalendarDay `json:"capacity_calendar"`
	DecisionQueue      []DecisionQueueItem   `json:"decision_queue"`
	ProductionSchedule []OrderDetail         `json:"production_schedule"`
	ShoppingList       *ShoppingListResult   `json:"shopping_list"`
	LowStockAlerts     []LowStockAlertItem   `json:"low_stock_alerts"`
	DPDueAlerts        []DPDueAlertItem      `json:"dp_due_alerts"`
	CashStatus         *CashProjectionResult `json:"cash_status"`
	KPI                *KPIResult            `json:"kpi"`
}

// GetManagerDashboard mengembalikan data untuk GET /api/v1/dashboard/manager (manager dan owner).
func GetManagerDashboard(ctx context.Context, conn db.Conn, actorRole string) (*ManagerDashboardResult, error) {
	if actorRole != auth.RoleManager && actorRole != auth.RoleOwner {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk mengakses dashboard manajer.")
	}

	today := db.TodayQ(ctx, conn)
	todayKey := engine.DateKey(today)
	monthKey := today.Format("2006-01")

	cal, err := GetCapacityCalendar(ctx, conn, monthKey)
	if err != nil {
		return nil, err
	}

	// Gunakan RoleManager untuk CanDecide pada antrian keputusan agar terlihat jelas mana yang di bawah ambang
	queue, err := GetDecisionQueue(ctx, conn, auth.RoleManager)
	if err != nil {
		return nil, err
	}

	allOrders, err := ListOrders(ctx, conn, OrderListFilter{ActorRole: auth.RoleManager})
	if err != nil {
		return nil, err
	}

	horizonEndKey := engine.DateKey(today.AddDate(0, 0, 13))
	prodSchedule := make([]OrderDetail, 0)
	dpDueAlerts := make([]DPDueAlertItem, 0)

	for _, o := range allOrders {
		if (o.Status == StatusConfirmed || o.Status == StatusInProduction || o.Status == StatusReady) &&
			o.ProductionDate >= todayKey && o.ProductionDate <= horizonEndKey {
			prodSchedule = append(prodSchedule, o)
		}
		if o.Status == StatusAccepted && o.Paid.LessThan(o.DPRequired) {
			dpDueAlerts = append(dpDueAlerts, DPDueAlertItem{
				OrderID:      o.ID,
				Code:         o.Code,
				CustomerName: o.CustomerName,
				EventDate:    o.EventDate,
				Total:        o.Total,
				DPRequired:   o.DPRequired,
				Paid:         o.Paid,
				DPDeadline:   o.DPDeadline,
			})
		}
	}

	sort.Slice(prodSchedule, func(i, j int) bool {
		if prodSchedule[i].ProductionDate == prodSchedule[j].ProductionDate {
			return prodSchedule[i].Code < prodSchedule[j].Code
		}
		return prodSchedule[i].ProductionDate < prodSchedule[j].ProductionDate
	})

	sort.Slice(dpDueAlerts, func(i, j int) bool {
		if dpDueAlerts[i].DPDeadline == nil {
			return false
		}
		if dpDueAlerts[j].DPDeadline == nil {
			return true
		}
		return dpDueAlerts[i].DPDeadline.Before(*dpDueAlerts[j].DPDeadline)
	})

	shopList, err := GetShoppingList(ctx, conn, today, today.AddDate(0, 0, 13), auth.RoleManager)
	if err != nil {
		return nil, err
	}

	// Selalu panggil dengan RoleManager agar saldo awal (opening_cash) dan rincian harian disembunyikan
	cashStatus, err := GetCashProjection(ctx, conn, 14, auth.RoleManager)
	if err != nil {
		return nil, err
	}

	kpiRes, err := GetKPI(ctx, conn, auth.RoleManager)
	if err != nil {
		return nil, err
	}

	return &ManagerDashboardResult{
		Today:              todayKey,
		CapacityCalendar:   cal,
		DecisionQueue:      queue,
		ProductionSchedule: prodSchedule,
		ShoppingList:       shopList,
		LowStockAlerts:     shopList.LowStockItems,
		DPDueAlerts:        dpDueAlerts,
		CashStatus:         cashStatus,
		KPI:                kpiRes,
	}, nil
}

// StaffDeliveryItem merepresentasikan jadwal pengantaran/penyerahan untuk karyawan
// TANPA memuat informasi harga jual, biaya, margin, DP, maupun kas.
type StaffDeliveryItem struct {
	OrderID         string          `json:"order_id"`
	OrderCode       string          `json:"order_code"`
	CustomerName    string          `json:"customer_name"`
	CustomerContact string          `json:"customer_contact"`
	DeliveryType    string          `json:"delivery_type"`
	Address         string          `json:"address,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	EventDate       string          `json:"event_date"`
	ProductionDate  string          `json:"production_date"`
	OrderStatus     string          `json:"order_status"`
	TaskID          string          `json:"task_id,omitempty"`
	TaskStatus      string          `json:"task_status,omitempty"`
	Items           []TaskOrderItem `json:"items"`
}

// StaffDashboardResult memuat elemen Dashboard Karyawan (Bagian 10 Brief):
// 1. Tugas hari ini
// 2. Seluruh tugas aktif karyawan (untuk ubah status & input durasi aktual)
// 3. Daftar pengantaran/penyerahan terurut menurut tanggal acara
// 4. Peringatan stok bahan menipis (tanpa harga beli)
// Dijamin bebas dari field harga, margin, atau kas.
type StaffDashboardResult struct {
	Today          string              `json:"today"`
	TodayTasks     []TaskItem          `json:"today_tasks"`
	ActiveTasks    []TaskItem          `json:"active_tasks"`
	Deliveries     []StaffDeliveryItem `json:"deliveries"`
	LowStockAlerts []LowStockAlertItem `json:"low_stock_alerts"`
}

// GetStaffDashboard mengembalikan data untuk GET /api/v1/dashboard/staff.
// Staff tidak pernah menerima field harga jual, biaya, margin, atau kas.
func GetStaffDashboard(ctx context.Context, conn db.Conn, actorID, actorRole string) (*StaffDashboardResult, error) {
	if actorRole != auth.RoleStaff && actorRole != auth.RoleManager && actorRole != auth.RoleOwner {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk mengakses dashboard karyawan.")
	}

	today := db.TodayQ(ctx, conn)
	todayKey := engine.DateKey(today)

	// Gunakan filter RoleStaff supaya aturan visibilitas karyawan selalu ditegakkan
	tasks, err := ListTasks(ctx, conn, TaskFilter{
		ActorID:   actorID,
		ActorRole: auth.RoleStaff,
	})
	if err != nil {
		return nil, err
	}

	todayTasks := make([]TaskItem, 0)
	activeTasks := make([]TaskItem, 0)
	deliveries := make([]StaffDeliveryItem, 0)

	for _, tk := range tasks {
		if tk.DueDate == todayKey || tk.Status == "doing" {
			todayTasks = append(todayTasks, tk)
		}
		if tk.Status != "done" {
			activeTasks = append(activeTasks, tk)
		}
		if tk.Type == "deliver" && tk.Status != "done" {
			deliveries = append(deliveries, StaffDeliveryItem{
				OrderID:         tk.OrderID,
				OrderCode:       tk.OrderCode,
				CustomerName:    tk.CustomerName,
				CustomerContact: tk.CustomerContact,
				DeliveryType:    tk.DeliveryType,
				Address:         tk.Address,
				Notes:           tk.Notes,
				EventDate:       tk.EventDate,
				ProductionDate:  tk.ProductionDate,
				OrderStatus:     tk.OrderStatus,
				TaskID:          tk.ID,
				TaskStatus:      tk.Status,
				Items:           tk.Items,
			})
		}
	}

	sort.Slice(deliveries, func(i, j int) bool {
		if deliveries[i].EventDate == deliveries[j].EventDate {
			return deliveries[i].OrderCode < deliveries[j].OrderCode
		}
		return deliveries[i].EventDate < deliveries[j].EventDate
	})

	// Panggil GetShoppingList dengan auth.RoleStaff sehingga BuyPrice dan EstimatedCost dijamin nil
	shopList, err := GetShoppingList(ctx, conn, today, today.AddDate(0, 0, 7), auth.RoleStaff)
	if err != nil {
		return nil, err
	}

	return &StaffDashboardResult{
		Today:          todayKey,
		TodayTasks:     todayTasks,
		ActiveTasks:    activeTasks,
		Deliveries:     deliveries,
		LowStockAlerts: shopList.LowStockItems,
	}, nil
}

// AuditLogItem adalah satu baris respons GET /api/v1/audit-logs.
type AuditLogItem struct {
	ID        string         `json:"id"`
	ActorID   *string        `json:"actor_id"`
	ActorName string         `json:"actor_name,omitempty"`
	ActorRole string         `json:"actor_role,omitempty"`
	Action    string         `json:"action"`
	Entity    string         `json:"entity"`
	EntityID  *string        `json:"entity_id"`
	Detail    map[string]any `json:"detail"`
	CreatedAt time.Time      `json:"created_at"`
}

// AuditLogFilter adalah parameter filter dan paginasi untuk GET /api/v1/audit-logs.
type AuditLogFilter struct {
	Action   string
	Entity   string
	ActorID  string
	EntityID string
	From     string // YYYY-MM-DD
	To       string // YYYY-MM-DD
	Page     int
	Limit    int
}

// ListAuditLogs mengambil daftar catatan audit dengan filter dan paginasi (khusus owner dan admin).
func ListAuditLogs(ctx context.Context, conn db.Conn, filter AuditLogFilter) ([]AuditLogItem, int, error) {
	rows, err := conn.Query(ctx, `
		SELECT a.id::text, a.actor_id::text, COALESCE(u.name, ''), COALESCE(u.role, ''),
		       a.action, a.entity, a.entity_id::text, COALESCE(a.detail::text, '{}'), a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.actor_id
		ORDER BY a.created_at DESC, a.id DESC
	`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	matched := make([]AuditLogItem, 0)
	for rows.Next() {
		var (
			item      AuditLogItem
			detailRaw string
		)
		if err := rows.Scan(
			&item.ID, &item.ActorID, &item.ActorName, &item.ActorRole,
			&item.Action, &item.Entity, &item.EntityID, &detailRaw, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}

		if filter.Action != "" && !strings.EqualFold(item.Action, filter.Action) {
			continue
		}
		if filter.Entity != "" && !strings.EqualFold(item.Entity, filter.Entity) {
			continue
		}
		if filter.ActorID != "" {
			if item.ActorID == nil || !strings.EqualFold(*item.ActorID, filter.ActorID) {
				continue
			}
		}
		if filter.EntityID != "" {
			if item.EntityID == nil || !strings.EqualFold(*item.EntityID, filter.EntityID) {
				continue
			}
		}
		createdDateKey := engine.DateKey(item.CreatedAt)
		if filter.From != "" && createdDateKey < filter.From {
			continue
		}
		if filter.To != "" && createdDateKey > filter.To {
			continue
		}

		det := make(map[string]any)
		if detailRaw != "" {
			_ = json.Unmarshal([]byte(detailRaw), &det)
		}
		item.Detail = det
		matched = append(matched, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	total := len(matched)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 50
	}
	offset := (page - 1) * limit
	if offset >= total {
		return []AuditLogItem{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return matched[offset:end], total, nil
}

// --- HTTP Handlers untuk Tahap 6 ---

// KPI menangani GET /api/v1/kpi (owner, manager, admin).
func (h *Handler) KPI(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}
	res, err := GetKPI(c.Request.Context(), h.DB, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// DashboardOwner menangani GET /api/v1/dashboard/owner (khusus owner).
func (h *Handler) DashboardOwner(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}
	res, err := GetOwnerDashboard(c.Request.Context(), h.DB, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// DashboardManager menangani GET /api/v1/dashboard/manager (manager, owner).
func (h *Handler) DashboardManager(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}
	res, err := GetManagerDashboard(c.Request.Context(), h.DB, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// DashboardStaff menangani GET /api/v1/dashboard/staff (staff, manager, owner; tanpa harga/margin/kas).
func (h *Handler) DashboardStaff(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}
	res, err := GetStaffDashboard(c.Request.Context(), h.DB, u.ID, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// AuditLogs menangani GET /api/v1/audit-logs (owner, admin) dengan filter dan paginasi.
func (h *Handler) AuditLogs(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}
	if u.Role != auth.RoleOwner && u.Role != auth.RoleAdmin {
		httpx.Forbidden(c, "Hanya pemilik usaha atau admin yang berhak mengakses log audit.")
		return
	}

	fromStr := strings.TrimSpace(c.Query("from"))
	if fromStr != "" {
		if _, err := db.ParseDate(fromStr); err != nil {
			httpx.UnprocessableEntity(c, "validation_error", "Parameter from harus berformat YYYY-MM-DD.")
			return
		}
	}
	toStr := strings.TrimSpace(c.Query("to"))
	if toStr != "" {
		if _, err := db.ParseDate(toStr); err != nil {
			httpx.UnprocessableEntity(c, "validation_error", "Parameter to harus berformat YYYY-MM-DD.")
			return
		}
	}
	actorID := strings.TrimSpace(c.Query("actor_id"))
	if actorID != "" && !httpx.ValidUUID(strings.ToLower(actorID)) {
		httpx.UnprocessableEntity(c, "validation_error", "Parameter actor_id harus berformat UUID yang valid.")
		return
	}
	entityID := strings.TrimSpace(c.Query("entity_id"))
	if entityID != "" && !httpx.ValidUUID(strings.ToLower(entityID)) {
		httpx.UnprocessableEntity(c, "validation_error", "Parameter entity_id harus berformat UUID yang valid.")
		return
	}

	page, limit, _ := httpx.Paginate(c)
	items, total, err := ListAuditLogs(c.Request.Context(), h.DB, AuditLogFilter{
		Action:   strings.TrimSpace(c.Query("action")),
		Entity:   strings.TrimSpace(c.Query("entity")),
		ActorID:  actorID,
		EntityID: entityID,
		From:     fromStr,
		To:       toStr,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.List(c, items, httpx.Meta{
		Page:  page,
		Limit: limit,
		Total: total,
	})
}
