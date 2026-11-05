package engine

import (
	"time"

	"github.com/shopspring/decimal"
)

// counterOfferDays: tawaran balik dicari dalam 14 hari kalender (Bagian 8.3).
const counterOfferDays = 14

// gateEval adalah hasil evaluasi pesanan pada satu tanggal dan satu set harga.
type gateEval struct {
	total        decimal.Decimal
	margin       decimal.Decimal
	utilBefore   decimal.Decimal
	utilAfter    decimal.Decimal
	bottleneck   string
	lowestBefore decimal.Decimal
	lowestWithDP decimal.Decimal
	minDP        decimal.Decimal
	dpCharged    decimal.Decimal
	g1, g2       GateResult
	g3, g4       GateResult
}

// EvaluateOrder menjalankan G1..G4 (BR-10), menentukan rekomendasi, alasan,
// pratinjau dampak, dan tawaran balik (Bagian 8.3).
//
// Rekomendasi:
//   - semua gerbang lolos → accept
//   - G4 gagal → reject
//   - G4 lolos, G1/G2/G3 gagal, dan ada tawaran balik yang terverifikasi
//     (lolos G1, G3, G4 dengan harga minimal) → accept_with_conditions
//   - selain itu → reject
func EvaluateOrder(in EvalInput) EvalResult {
	orig := evaluateAt(in, in.EventDate, in.Items)

	res := EvalResult{
		Gates:      []GateResult{orig.g1, orig.g2, orig.g3, orig.g4},
		Total:      orig.total,
		MinDP:      orig.minDP,
		DPCharged:  orig.dpCharged,
		Bottleneck: orig.bottleneck,
		Impact: Impact{
			UtilizationBefore: orig.utilBefore,
			UtilizationAfter:  orig.utilAfter,
			Margin:            orig.margin,
			CashLowestBefore:  orig.lowestBefore,
			CashLowestAfter:   orig.lowestWithDP,
		},
	}

	if orig.g1.Passed && orig.g2.Passed && orig.g3.Passed && orig.g4.Passed {
		res.Recommendation = RecAccept
		res.Reason = "Semua gerbang lolos: beban " + fmtPct(orig.utilAfter) +
			", margin " + fmtPct(orig.margin) +
			", titik terendah kas " + fmtRp(orig.lowestWithDP) + "."
		return res
	}

	res.CounterOffer = findCounterOffer(in, orig)

	switch {
	case !orig.g4.Passed:
		res.Recommendation = RecReject
	case res.CounterOffer != nil:
		res.Recommendation = RecAcceptWithConditions
	default:
		res.Recommendation = RecReject
	}
	res.Reason = buildReason(in, orig, res.CounterOffer)
	return res
}

// evaluateAt mengevaluasi pesanan bila acara jatuh pada eventDate dengan items
// (harga bisa sudah disesuaikan). Selisih tanggal produksi terhadap tanggal
// acara dipertahankan.
func evaluateAt(in EvalInput, eventDate time.Time, items []EvalItem) gateEval {
	p := in.Params
	today := civil(in.Today)
	event := civil(eventDate)

	offset := 0
	if !in.ProductionDate.IsZero() {
		offset = daysBetween(in.ProductionDate, in.EventDate)
	}
	prodDate := addDays(event, -offset)

	// Total, biaya, dan belanja pesanan.
	total, cost, shopping := decimal.Zero, decimal.Zero, decimal.Zero
	for _, it := range items {
		q := decimal.NewFromInt(int64(it.Qty))
		total = total.Add(q.Mul(it.UnitPrice))
		cost = cost.Add(q.Mul(it.UnitCost))
		shopping = shopping.Add(q.Mul(it.PurchaseCostPerUnit))
	}

	ev := gateEval{total: total, margin: Margin(total, cost)}

	// G1: kapasitas pada tanggal produksi.
	existing := existingLoadItems(in.ExistingLoads[DateKey(prodDate)])
	ev.utilBefore, _ = DayUtilization(DayLoads(existing), in.Capacities)

	all := make([]LoadItem, 0, len(existing)+len(items)*2)
	all = append(all, existing...)
	for _, it := range items {
		q := decimal.NewFromInt(int64(it.Qty))
		for code, load := range it.UnitLoads {
			all = append(all, LoadItem{ResourceCode: code, Qty: q, Load: load})
		}
	}
	ev.utilAfter, ev.bottleneck = DayUtilization(DayLoads(all), in.Capacities)

	// Kas: jadwal sesuai Bagian 7.
	horizon := daysBetween(today, addDays(event, 1)) + 1
	if horizon < 1 {
		horizon = 1
	}
	shopDate := addDays(event, -p.ShoppingLeadDays)
	if shopDate.Before(today) {
		shopDate = today // belanja tidak bisa terjadi di masa lalu
	}
	dpDate := today
	if !in.DPDate.IsZero() {
		dpDate = civil(in.DPDate)
		if dpDate.Before(today) {
			dpDate = today
		}
	}
	settleDate := addDays(event, 1)

	withOrder := func(dp decimal.Decimal) []CashEvent {
		evs := make([]CashEvent, 0, len(in.CashEvents)+3)
		evs = append(evs, in.CashEvents...)
		evs = append(evs,
			CashEvent{Date: shopDate, Amount: shopping.Neg()},
			CashEvent{Date: settleDate, Amount: total.Sub(dp)},
		)
		if dp.IsPositive() {
			evs = append(evs, CashEvent{Date: dpDate, Amount: dp})
		}
		return evs
	}
	lowest := func(evs []CashEvent) decimal.Decimal {
		return ProjectCash(p.OpeningCash, today, horizon, evs, p.FixedCostDaily).Lowest
	}

	ev.lowestBefore = lowest(in.CashEvents)
	lowestNoDP := lowest(withOrder(decimal.Zero))
	ev.minDP = MinDP(p.CashBuffer, lowestNoDP)
	ev.dpCharged = DPCharged(p.DPPct, total, ev.minDP)

	dpApplied := ev.dpCharged
	if dpApplied.GreaterThan(total) {
		dpApplied = total
	}
	ev.lowestWithDP = lowest(withOrder(dpApplied))

	daysAhead := decimal.NewFromInt(int64(daysBetween(today, event)))
	lead := decimal.NewFromInt(int64(p.LeadTimeDays))

	ev.g1 = GateResult{Gate: "G1", Passed: ev.utilAfter.LessThanOrEqual(p.CapacityLimit), Value: ev.utilAfter, Threshold: p.CapacityLimit}
	ev.g2 = GateResult{Gate: "G2", Passed: ev.margin.GreaterThanOrEqual(p.MarginMin), Value: ev.margin, Threshold: p.MarginMin}
	// G3 gagal tanpa perbaikan bila DP minimal melebihi total (BR-09).
	ev.g3 = GateResult{Gate: "G3", Passed: !ev.minDP.GreaterThan(total) && ev.lowestWithDP.GreaterThanOrEqual(p.CashBuffer), Value: ev.lowestWithDP, Threshold: p.CashBuffer}
	ev.g4 = GateResult{Gate: "G4", Passed: daysAhead.GreaterThanOrEqual(lead), Value: daysAhead, Threshold: lead}
	return ev
}

func existingLoadItems(existing map[string]decimal.Decimal) []LoadItem {
	items := make([]LoadItem, 0, len(existing))
	for code, load := range existing {
		items = append(items, LoadItem{ResourceCode: code, Qty: decimal.NewFromInt(1), Load: load})
	}
	return items
}

// findCounterOffer mencari tawaran balik (Bagian 8.3): harga minimal per item
// bila G2 gagal, dan tanggal terdekat yang lolos G1, G3, dan G4. Tanggal acara
// semula dicoba lebih dulu (cukup perbaikan harga); bila tidak lolos, dicari
// maju mulai hari berikutnya (atau tanggal lead time, bila lebih lambat)
// selama 14 hari. Mengembalikan nil bila tidak ada perbaikan.
func findCounterOffer(in EvalInput, orig gateEval) *CounterOffer {
	items := in.Items
	if !orig.g2.Passed {
		items = repriceItems(in.Items, in.Params.MarginMin)
	}

	minPrices := make(map[string]decimal.Decimal, len(in.Items))
	for _, it := range in.Items {
		minPrices[it.ProductID] = MinPrice(it.UnitCost, in.Params.MarginMin)
	}

	try := func(date time.Time) *CounterOffer {
		ev := evaluateAt(in, date, items)
		if !(ev.g1.Passed && ev.g3.Passed && ev.g4.Passed) {
			return nil
		}
		return &CounterOffer{
			Date:             civil(date),
			MinPricePerItem:  minPrices,
			DP:               ev.dpCharged,
			UtilizationAfter: ev.utilAfter,
		}
	}

	if c := try(in.EventDate); c != nil {
		return c
	}

	start := addDays(civil(in.EventDate), 1)
	if leadDate := addDays(civil(in.Today), in.Params.LeadTimeDays); leadDate.After(start) {
		start = leadDate
	}
	for i := 0; i < counterOfferDays; i++ {
		if c := try(addDays(start, i)); c != nil {
			return c
		}
	}
	return nil
}

// repriceItems menaikkan harga item yang di bawah harga minimal ke harga minimal.
func repriceItems(items []EvalItem, marginMin decimal.Decimal) []EvalItem {
	out := make([]EvalItem, len(items))
	for i, it := range items {
		if mp := MinPrice(it.UnitCost, marginMin); it.UnitPrice.LessThan(mp) {
			it.UnitPrice = mp
		}
		out[i] = it
	}
	return out
}

// buildReason menyusun alasan rekomendasi yang terbaca manusia.
func buildReason(in EvalInput, ev gateEval, counter *CounterOffer) string {
	var parts []string
	event := civil(in.EventDate)

	if !ev.g1.Passed {
		parts = append(parts, "Beban "+resourceLabel(ev.bottleneck)+" "+fmtPct(ev.utilAfter)+
			" pada "+fmtDate(addDays(event, -productionOffset(in)))+
			" melebihi batas terima "+fmtPct(in.Params.CapacityLimit)+".")
	}
	if !ev.g2.Passed {
		parts = append(parts, "Margin "+fmtPct(ev.margin)+" di bawah minimum "+
			fmtPct(in.Params.MarginMin)+"; harga minimal per item ada di tawaran balik.")
	}
	if !ev.g3.Passed {
		if ev.minDP.GreaterThan(ev.total) {
			parts = append(parts, "DP minimal "+fmtRp(ev.minDP)+" melebihi total pesanan "+
				fmtRp(ev.total)+"; kas tidak dapat menutup pesanan ini.")
		} else {
			parts = append(parts, "Titik terendah kas "+fmtRp(ev.lowestWithDP)+
				" di bawah buffer "+fmtRp(in.Params.CashBuffer)+".")
		}
	}
	if !ev.g4.Passed {
		parts = append(parts, "Tanggal acara "+ev.g4.Value.String()+" hari lagi, kurang dari lead time minimum "+
			ev.g4.Threshold.String()+" hari.")
	}

	if counter == nil {
		parts = append(parts, "Tidak ada tawaran balik yang memenuhi syarat dalam "+
			itoa(counterOfferDays)+" hari ke depan.")
	} else if !counter.Date.Equal(event) {
		parts = append(parts, "Tersedia "+fmtDate(counter.Date)+" ("+fmtPct(counter.UtilizationAfter)+
			"), DP ditagih "+fmtRp(counter.DP)+".")
	} else {
		parts = append(parts, "Dapat diterima pada tanggal yang sama dengan harga minimal dan DP "+
			fmtRp(counter.DP)+".")
	}

	reason := ""
	for i, s := range parts {
		if i > 0 {
			reason += " "
		}
		reason += s
	}
	return reason
}

func productionOffset(in EvalInput) int {
	if in.ProductionDate.IsZero() {
		return 0
	}
	return daysBetween(in.ProductionDate, in.EventDate)
}

