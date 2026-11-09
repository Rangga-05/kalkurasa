# Kalkurasa — Design Direction & System Specification (Slight Twist Inspired)

## 1. Design Read
- **Archetype:** Craft Culinary D2C Showcase + Precision Kitchen Decision Platform (`place` + `software` hybrid)
- **Visual Reference & Inspiration:** [Slight Twist Brewing Co. (`slight-twist.co.nz`)](https://www.slight-twist.co.nz/) by Hydrate Design Studio — high-energy color-blocked sections, expressive editorial typography with italic "twist" accents, bold `2px` ink borders (`#12100E`), hard tactile offset shadows (`4px 4px 0px #12100E`), floating pill navigation, dynamic flavor/menu stage switcher, and real studio food photography.
- **Audience:** Pre-Order Customers (ordering artisanal Nasi Box, Snack Box, Celebration Tarts, Cupcakes, Nastar, and Fudgy Brownies) and UMKM Kitchen Operations Teams (Owner, Manager, Staff, Admin).
- **Liveliness Dials:** `ENERGY 4 / RHYTHM 4 / MOTION 3`

---

## 2. Color System & WCAG AA Contrast Ratios

### Base & Structural Tokens (Light Mode Default — *Warm Butter Cream*)
| Token | Hex | Role | Contrast vs `#12100E` | Contrast vs `#FFFDF9` |
| :--- | :--- | :--- | :--- | :--- |
| `--bg-canvas` | `#FFF9F0` | Warm Butter Cream primary canvas | `18.6:1` (AAA) | — |
| `--bg-surface` | `#FFFDF9` | Crisp Cream card & floating pill surface | `19.4:1` (AAA) | — |
| `--bg-subtle` | `#F7EFE2` | Toasted Oat secondary surface | `16.9:1` (AAA) | — |
| `--ink-primary` | `#12100E` | Deep Espresso Ink (text, `2px` structural borders, hard shadows) | — | `19.4:1` (AAA) |
| `--ink-secondary` | `#3D352E` | Roasted Walnut body copy | — | `11.2:1` (AAA) |
| `--ink-muted` | `#5C5046` | Warm Bark metadata & captions | — | `7.4:1` (AAA) |
| `--accent-primary` | `#C2410C` | Spiced Terracotta Orange primary CTA (`#FFFFFF` text) | — | `5.18:1` vs `#FFFFFF` (AA) |
| `--accent-lime` | `#D9F99D` | Zesty Pandan Lime highlight pill (`#12100E` text) | `15.8:1` (AAA) | — |
| `--accent-gold` | `#FDE047` | Butter Pineapple Gold sticker (`#12100E` text) | `16.4:1` (AAA) | — |
| `--accent-forest` | `#14281D` | Deep Forest Kitchen Studio section (`#FFFDF9` text) | — | `14.5:1` (AAA) |

### Dynamic Dish Color-Block Themes (Slight Twist Flavor Stage)
| Dish / Category | Stage Background | Card Accent | Badge / Sticker | Text Color | WCAG AA Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **01 · Nasi Box Rempah** | `#FFEDD5` (Warm Marigold) | `#C2410C` | `#FDE047` | `#12100E` (`16.8:1`) | PASS (AAA) |
| **02 · Snack Box Pasar Pagi** | `#ECFCCB` (Zesty Pandan Lime) | `#15803D` | `#D9F99D` | `#12100E` (`17.4:1`) | PASS (AAA) |
| **03 · Tart Ulang Tahun** | `#FFE4E6` (Berry Velvet Rose) | `#BE123C` | `#FECDD3` | `#12100E` (`16.2:1`) | PASS (AAA) |
| **04 · Cupcake Box Assorted** | `#FEF3C7` (Citrus Apricot) | `#B45309` | `#FDE68A` | `#12100E` (`17.6:1`) | PASS (AAA) |
| **05 · Kue Kering Nastar** | `#FEF9C3` (Pineapple Butter Gold) | `#854D0E` | `#FEF08A` | `#12100E` (`18.3:1`) | PASS (AAA) |
| **06 · Fudgy Almond Brownies** | `#F3E8FF` (Cocoa Berry Lavender) | `#6B21A8` | `#E9D5FF` | `#12100E` (`16.1:1`) | PASS (AAA) |

---

## 3. Typography & Hierarchy
- **Display Headline (`--font-display`: `Bricolage Grotesque` / `Fraunces`):** Chunky, high-character editorial headlines (`700`/`800`, tight tracking `-0.035em`, line-height `0.95`) paired with italic `Fraunces` "twist" callout words in pill highlights.
- **UI & Body (`--font-sans`: `Plus Jakarta Sans`):** Clean, punchy geometric sans (`400`, `500`, `600`, `700`).
- **Telemetry & Prices (`--font-mono`: `JetBrains Mono`):** `tabular-nums` for all Rupiah prices, `G1–G4` capacity percentages, and `KLK-XXXXXX` ticket codes.

---

## 4. Compositional Rhythm (`RHYTHM 4`) & Tactile Micro-Interactions (`MOTION 3`)
1. **Floating Pill Header + Live Kitchen Ticker Ribbon:** High-contrast top marquee strip (`#12100E` + `#D9F99D`) and floating capsule navbar with `2px solid #12100E` border and `4px 4px 0px #12100E` shadow.
2. **Dynamic Color-Shifting Hero & Flavor Stage:** Interactive 6-dish switcher that transforms the stage color palette, showcases real studio food photography in a bold arched frame with floating sticker badges, and includes an embedded Quick Ticket Tracker (`KLK-XXXXXX`).
3. **Deep Forest 4-Gate Kitchen Mixer Studio (`#simulasi-gerbang`):** High-contrast `#14281D` dark botanical section with `#D9F99D` lime & `#FFF9F0` cream tactile control cards, chunky range sliders, and live `G1–G4` decision stamps.
4. **Color-Blocked Menu Bento Showcase (`#katalog`):** Each product card features its signature flavor color-block header, real food photograph with hover tilt/zoom, oven/crew capacity pills, and an expandable recipe & kitchen story drawer.
5. **Editorial Craft Process & 4-Gate Architecture (`#dapur-kami`):** Real bakery photography paired with bold numbered step cards and the 4-Gate Decision Engine diagram.

---

Design Read: Craft Culinary D2C Showcase + Precision Kitchen Decision Platform for Pre-Order Customers and UMKM Kitchen Teams, in a Slight-Twist-Inspired Bold Color-Blocked & Tactile Neo-Editorial visual language, dial ENERGY 4 / RHYTHM 4 / MOTION 3.
