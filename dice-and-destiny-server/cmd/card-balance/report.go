package main

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

// Fair-price contexts: the mirror and the Warlord by score, the three small
// minions by health left (wounds carry over in an expedition).
var contexts = []struct{ key, title string }{
	{"mirror", "Mirror"},
	{"drowned_regent", "Warlord"},
	{"minions", "Minions (health)"},
}

var smallMinions = []string{"drowned_oracle_brine_mask", "drowned_oracle_bell_diver", "drowned_oracle_ribbon_eel"}

// Pricing is a card's fair XP in each context and the verdict.
type Pricing struct {
	Fair    map[string]float64 `json:"fair_xp"`
	Average float64            `json:"fair_xp_average"`
	Gap     float64            `json:"gap_xp"`
	Verdict string             `json:"verdict"`
	Note    string             `json:"note,omitempty"`
}

// contextValue is a card's measured change in one context.
func contextValue(r Row, context string) (float64, bool) {
	if context != "minions" {
		d, ok := r.Score[context]
		return d.Mean, ok
	}
	total, n := 0.0, 0
	for _, m := range smallMinions {
		if d, ok := r.Health[m]; ok {
			total += d.Mean
			n++
		}
	}
	return total / float64(max(n, 1)), n > 0
}

// price sets every card against the base cards: the average value per XP of
// the character's base deck cards is the fair rate, in each context.
func price(rows []Row, baseCards map[string]bool, blank string) map[string]Pricing {
	rate := map[string]float64{}
	for _, c := range contexts {
		total, xp := 0.0, 0
		for _, r := range rows {
			if v, ok := contextValue(r, c.key); ok && baseCards[r.Card] {
				total += v
				xp += r.Price
			}
		}
		if xp > 0 {
			rate[c.key] = total / float64(xp)
		}
	}
	var blankRow *Row
	for i := range rows {
		if rows[i].Card == blank {
			blankRow = &rows[i]
		}
	}
	out := map[string]Pricing{}
	for _, r := range rows {
		p := Pricing{Fair: map[string]float64{}}
		n := 0
		for _, c := range contexts {
			v, ok := contextValue(r, c.key)
			if !ok || rate[c.key] == 0 {
				continue
			}
			p.Fair[c.key] = v / rate[c.key]
			p.Average += p.Fair[c.key]
			n++
		}
		p.Average /= float64(max(n, 1))
		p.Gap = p.Average - float64(r.Price)
		switch {
		case r.Card == blank:
			p.Verdict = "reference"
			p.Note = "No battle effect: shows what one card of health is worth."
		case p.Gap >= 2:
			p.Verdict = "underpriced"
		case p.Gap <= -2:
			p.Verdict = "overpriced"
		default:
			p.Verdict = "about right"
		}
		if blankRow != nil && r.Card != blank {
			worse := 0
			for _, c := range contexts {
				v, _ := contextValue(r, c.key)
				b, _ := contextValue(*blankRow, c.key)
				if v < b {
					worse++
				}
			}
			if worse >= 2 {
				p.Note = "Worse than a blank card: the AI plays it when holding it would be better, so this measures the AI as much as the card."
			}
		}
		if r.PlayedMirror < 0.1 && r.Card != blank {
			p.Note = strings.TrimSpace(p.Note + " Rarely played: the AI seldom finds it worth its cost, so it is worth little more than its health.")
		}
		out[r.Card] = p
	}
	return out
}

// tree groups a card ID by its card tree.
func tree(id string) string {
	if i := strings.Index(id, "_paths_variant_"); i >= 0 {
		return id[:i]
	}
	return id
}

func renderHTML(hero string, rows []Row, prices map[string]Pricing, baseCards map[string]bool, seeds int) string {
	var sb strings.Builder
	sb.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Card balance</title><style>
body{font:14px/1.45 -apple-system,system-ui,sans-serif;margin:24px auto;max-width:1240px;color:#1d232b;background:#f6f4ef;padding:0 16px}
h1{margin:0 0 4px} h2{margin:28px 0 8px} .muted{color:#6b7480}
table{border-collapse:collapse;width:100%;background:#fff;margin:6px 0 14px} th,td{border-bottom:1px solid #e6e2d9;padding:5px 8px;text-align:left;vertical-align:top}
th{background:#ece8de;font-weight:600} td.num{text-align:right;font-variant-numeric:tabular-nums}
.under{background:#e3f4e1} .over{background:#fbe3dc} .ref{background:#eef1f5} tr.base td{background:#eef1f5;border-top:2px solid #b9c2cf}
.tag{display:inline-block;padding:0 7px;border-radius:9px;font-size:11px;font-weight:700}
.tag.under{background:#2e7d32;color:#fff} .tag.over{background:#c0392b;color:#fff} .tag.right{background:#cfd6df;color:#2b3440} .tag.ref{background:#8a96a8;color:#fff}
.note{font-size:12px;color:#6b5a2a} .effect{font-size:12px;color:#596270}
</style></head><body>`)
	fmt.Fprintf(&sb, `<h1>Card balance · %s</h1><p class="muted">Each card was added as a 13th card to the %s's normal deck, then played in %d seeded battles per opponent by the calibrated heuristic AI: the mirror against the unchanged deck (both seats), the Warlord (Drowned Regent), and the Brine Mask, Bell Diver and Ribbon Eel. Each result is compared with the unchanged deck on the same seeds.</p>`, html.EscapeString(hero), html.EscapeString(hero), seeds)
	sb.WriteString(`<p class="muted"><b>Fair XP</b> is the price at which a card would give as much per XP as the base cards do on average (all base cards cost 10 XP), worked out separately against the mirror (win score), the Warlord (win score) and the small minions (health left after the battle, which matters because wounds carry over). <b>Gap</b> is the average fair XP minus the actual price: positive means the card is worth more than it costs. With these sample sizes a fair price is good to about ±1.5 XP, so gaps inside ±2 count as about right.</p>`)

	sorted := append([]Row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool { return prices[sorted[i].Card].Gap > prices[sorted[j].Card].Gap })
	sb.WriteString(`<h2>All cards, by gap</h2><table><tr><th>Card</th><th>Effect</th><th>XP</th><th>Energy</th><th>Played / battle</th>`)
	for _, c := range contexts {
		fmt.Fprintf(&sb, `<th>Fair XP: %s</th>`, c.title)
	}
	sb.WriteString(`<th>Fair XP (avg)</th><th>Gap</th><th>Verdict</th></tr>`)
	for _, r := range sorted {
		p := prices[r.Card]
		class, tag := "", `<span class="tag right">about right</span>`
		switch p.Verdict {
		case "underpriced":
			class, tag = "under", `<span class="tag under">underpriced</span>`
		case "overpriced":
			class, tag = "over", `<span class="tag over">overpriced</span>`
		case "reference":
			class, tag = "ref", `<span class="tag ref">health only</span>`
		}
		name := html.EscapeString(r.Name)
		if baseCards[r.Card] {
			name += ` <span class="muted">(base)</span>`
		}
		fmt.Fprintf(&sb, `<tr class="%s"><td><b>%s</b></td><td class="effect">%s`, class, name, html.EscapeString(r.Effect))
		if p.Note != "" {
			fmt.Fprintf(&sb, `<div class="note">%s</div>`, html.EscapeString(p.Note))
		}
		fmt.Fprintf(&sb, `</td><td class="num">%d</td><td class="num">%d</td><td class="num">%.2f</td>`, r.Price, r.Energy, r.PlayedMirror)
		for _, c := range contexts {
			fmt.Fprintf(&sb, `<td class="num">%.1f</td>`, p.Fair[c.key])
		}
		fmt.Fprintf(&sb, `<td class="num"><b>%.1f</b></td><td class="num">%+.1f</td><td>%s</td></tr>`, p.Average, p.Gap, tag)
	}
	sb.WriteString(`</table>`)

	// One table per card tree: what each toggle buys over the base card.
	trees := map[string][]Row{}
	var order []string
	for _, r := range rows {
		t := tree(r.Card)
		if trees[t] == nil {
			order = append(order, t)
		}
		trees[t] = append(trees[t], r)
	}
	sb.WriteString(`<h2>Upgrades within each card tree</h2><p class="muted">Each tree starts with its base card, then its variants from cheapest to most expensive. Fair XP and Gap are as in the table above. The last three columns compare each variant with the base: what the upgrade costs in XP, what it is worth (the difference in fair XP), and the difference between the two (green: the upgrade is worth more than it costs; red: less).</p>`)
	for _, t := range order {
		list := trees[t]
		var baseRow *Row
		for i := range list {
			if list[i].Card == t {
				b := list[i]
				baseRow = &b
			}
		}
		if baseRow == nil || len(list) < 2 {
			continue
		}
		bp := prices[baseRow.Card]
		fmt.Fprintf(&sb, `<h3>%s tree</h3><table><tr><th>Card</th><th>Effect</th><th>Energy</th><th>XP price</th><th>Fair XP</th><th>Gap</th><th>Costs over base (XP)</th><th>Worth over base (XP)</th><th>Difference</th></tr>`, html.EscapeString(baseRow.Name))
		fmt.Fprintf(&sb, `<tr class="base"><td><b>%s</b> <span class="muted">(base)</span></td><td class="effect">%s</td><td class="num">%d</td><td class="num">%d</td><td class="num"><b>%.1f</b></td><td class="num">%+.1f</td><td class="num">—</td><td class="num">—</td><td class="num">—</td></tr>`,
			html.EscapeString(baseRow.Name), html.EscapeString(baseRow.Effect), baseRow.Energy, baseRow.Price, bp.Average, bp.Gap)
		sort.SliceStable(list, func(i, j int) bool { return list[i].Price < list[j].Price })
		for _, r := range list {
			if r.Card == baseRow.Card {
				continue
			}
			p := prices[r.Card]
			cost := float64(r.Price - baseRow.Price)
			worth := p.Average - bp.Average
			class := ""
			if worth-cost >= 2 {
				class = "under"
			} else if worth-cost <= -2 {
				class = "over"
			}
			fmt.Fprintf(&sb, `<tr class="%s"><td><b>%s</b></td><td class="effect">%s</td><td class="num">%d</td><td class="num">%d</td><td class="num"><b>%.1f</b></td><td class="num">%+.1f</td><td class="num">%+.0f</td><td class="num">%+.1f</td><td class="num">%+.1f</td></tr>`,
				class, html.EscapeString(r.Name), html.EscapeString(r.Effect), r.Energy, r.Price, p.Average, p.Gap, cost, worth, worth-cost)
		}
		sb.WriteString(`</table>`)
	}
	fmt.Fprintf(&sb, `<p class="muted">Generated %s by cmd/card-balance. Values depend on the AI (a calibrated version of the look-ahead's heuristic), on the %s's deck, and on these opponents. A second or third copy of a card may be worth less than the first.</p></body></html>`, time.Now().Format("2006-01-02 15:04"), html.EscapeString(hero))
	return sb.String()
}
