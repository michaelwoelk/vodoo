# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

Terminal-based (TUI) time tracking analysis app for Karlsruhe (Baden-Württemberg). Reads exported time tracking data, absence records, and holiday CSVs — purely read-only, no data modification. Built with Go + Bubble Tea + Lip Gloss.

## Build & Run

```bash
go build -o vodoo .
./vodoo              # uses ./data/
./vodoo --data dataTest   # uses demo data
```

## Architecture

Single-file app (`main.go`, ~2200 lines). All in one file by design — no packages.

**Navigation**: 4 pages cycled with ←→ arrows. No menu. `↑↓` for within-page navigation. `q` to quit.

Pages: Dashboard → Jahr → Monate → Feiertage & Abwesenheiten

**Data pipeline**:
1. `findDataFile()` / `findAbsenceFile()` / `findHolidayFile()` discover files in `dataDir()` (default `./data/`)
2. ODS parsed via `archive/zip` + raw XML string splitting (no XML unmarshalling — ODS namespaces are too complex for `encoding/xml`)
3. XLSX parsed via `archive/zip` + shared strings lookup
4. Absences and holidays read from CSVs via `encoding/csv`
5. All data aggregated into `appData` struct, passed to Bubble Tea model

**Key types**:
- `appData` — all loaded data (timeData map, holidays, absences, hmap)
- `model` — Bubble Tea model with page index, month selection, table state
- `AbsenceDay` — per-day absence info with half-day support

## Color System

Uses brand color tokens defined as constants (`colPrimaryGreen`, `colMint`, `colYellow`, `colOrange`, etc.). All colors reference these — never use raw ANSI codes or lipgloss color numbers.

## Data Files (`data/` or `dataTest/`)

- `Zeiterfassungen.ods` or `.xlsx` — time entries with German date format ("02 Jan. 2026") + hours as float
- `Abwesenheiten (hr.leave).csv` — absence export (Urlaub, Krankheit) with start/end datetime, duration, status
- `feiertage_karlsruhe_2026.csv` — holidays from CSV (not computed)

## Important Patterns

- **No ANSI in Bubble Tea table cells** — the table widget miscounts width with escape codes. Use plain text in table rows, or render tables manually with lipgloss for per-cell colors.
- **Unicode width issues** — characters like `░█▸─` have ambiguous terminal width. Use `lipgloss.NewStyle().Width()` instead of `fmt.Sprintf("%-Ns")` for alignment with styled text.
- **German locale** — all UI text, date parsing, and month names are German. Date format in exports: "DD Mon. YYYY" with German month abbreviations (Jan., Feb., März, Apr., Mai, Juni, Juli, Aug., Sep., Okt., Nov., Dez.).
- **Half-day absences** (0.5 days) reduce Soll by 4h, not 8h.
