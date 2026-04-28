package main

import (
	"archive/zip"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Constants ──────────────────────────────────────────────────────────────

const standardHoursPerDay = 8.0

var weekdaysDE = []string{"Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"}
var monthsDE = []string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}
var germanMonths = map[string]int{
	"Jan.": 1, "Feb.": 2, "März": 3, "Apr.": 4,
	"Mai": 5, "Juni": 6, "Juli": 7, "Aug.": 8,
	"Sep.": 9, "Okt.": 10, "Nov.": 11, "Dez.": 12,
}

// ─── Conuti color tokens ────────────────────────────────────────────────────

const (
	colPrimaryGreen = lipgloss.Color("#A0D22B")
	colPrimaryGrey  = lipgloss.Color("#9B9F9A")
	colDarkTeal     = lipgloss.Color("#1E414E")
	colBgMidGrey    = lipgloss.Color("#5B5B5B")
	colSoftGreen    = lipgloss.Color("#C6CD83")
	colMint         = lipgloss.Color("#B5DEC2")
	colYellow       = lipgloss.Color("#FFE665")
	colOrange       = lipgloss.Color("#FF9F62")
)

// ─── Styles ─────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colMint).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colPrimaryGrey).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDarkTeal).
			Padding(1, 2)

	statLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Width(24)

	positiveStyle = lipgloss.NewStyle().Foreground(colPrimaryGreen)
	negativeStyle = lipgloss.NewStyle().Foreground(colOrange)
	warnStyle     = lipgloss.NewStyle().Foreground(colYellow)
	holidayStyle  = lipgloss.NewStyle().Foreground(colMint)
	dimStyle      = lipgloss.NewStyle().Foreground(colBgMidGrey)
	accentStyle   = lipgloss.NewStyle().Foreground(colMint).Bold(true)
	helpStyle     = lipgloss.NewStyle().Foreground(colBgMidGrey).Padding(1, 1)

	barFull  = lipgloss.NewStyle().Foreground(colPrimaryGreen)
	barEmpty = lipgloss.NewStyle().Foreground(colBgMidGrey)
)


// ─── Helpers ────────────────────────────────────────────────────────────────

func formatHours(h float64) string {
	sign := ""
	if h < 0 {
		sign = "-"
		h = -h
	}
	hours := int(h)
	minutes := int(math.Round((h - float64(hours)) * 60))
	if minutes == 60 {
		hours++
		minutes = 0
	}
	return fmt.Sprintf("%s%d:%02d", sign, hours, minutes)
}

func barChart(value, maxVal float64, width int) string {
	if maxVal == 0 {
		return ""
	}
	filled := int(math.Round(value / maxVal * float64(width)))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return barFull.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", width-filled))
}

func coloredSaldo(h float64) string {
	s := formatHours(h)
	if h >= 0 {
		return positiveStyle.Render(s)
	}
	return negativeStyle.Render(s)
}

// ─── Date helpers ───────────────────────────────────────────────────────────

func dateKey(d time.Time) string { return d.Format("2006-01-02") }

func wdIdx(d time.Time) int {
	w := int(d.Weekday())
	if w == 0 {
		return 6
	}
	return w - 1
}

func isWeekday(d time.Time) bool { return wdIdx(d) < 5 }

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

// ─── Holidays (from CSV) ────────────────────────────────────────────────────

type Holiday struct {
	Date time.Time
	Name string
}

var dataDirPath string

func dataDir() string {
	return dataDirPath
}

func findHolidayFile() string {
	dir := dataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, "feiertage") && strings.HasSuffix(name, ".csv") {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

func readHolidays(path string) ([]Holiday, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, nil
	}

	var holidays []Holiday
	for _, row := range records[1:] { // skip header
		if len(row) < 3 || row[0] == "" {
			continue
		}
		t, err := time.Parse("02.01.2006", strings.TrimSpace(row[0]))
		if err != nil {
			continue
		}
		holidays = append(holidays, Holiday{
			Date: t,
			Name: strings.TrimSpace(row[2]),
		})
	}
	sort.Slice(holidays, func(i, j int) bool { return holidays[i].Date.Before(holidays[j].Date) })
	return holidays, nil
}

func holidayMap(holidays []Holiday) map[string]string {
	m := make(map[string]string)
	for _, h := range holidays {
		m[dateKey(h.Date)] = h.Name
	}
	return m
}

// ─── Working days ───────────────────────────────────────────────────────────

func workingDaysInMonth(year int, month time.Month, hmap map[string]string) []time.Time {
	var days []time.Time
	nd := daysInMonth(year, month)
	for d := 1; d <= nd; d++ {
		t := time.Date(year, month, d, 0, 0, 0, 0, time.Local)
		if isWeekday(t) {
			if _, isH := hmap[dateKey(t)]; !isH {
				days = append(days, t)
			}
		}
	}
	return days
}

func workingDaysInYear(year int, hmap map[string]string) []time.Time {
	var days []time.Time
	for m := time.January; m <= time.December; m++ {
		days = append(days, workingDaysInMonth(year, m, hmap)...)
	}
	return days
}

// ─── ODS reader ─────────────────────────────────────────────────────────────

type timeEntry struct {
	DateStr string
	Hours   float64
}

func readODS(path string) ([]timeEntry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var contentFile *zip.File
	for _, f := range r.File {
		if f.Name == "content.xml" {
			contentFile = f
			break
		}
	}
	if contentFile == nil {
		return nil, fmt.Errorf("content.xml not found in ODS")
	}

	rc, err := contentFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf := make([]byte, 0, 1024*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, readErr := rc.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if readErr != nil {
			break
		}
	}

	content := string(buf)
	var entries []timeEntry

	rows := splitByTag(content, "table:table-row")
	for _, row := range rows {
		cells := splitByTag(row, "table:table-cell")
		if len(cells) < 2 {
			continue
		}
		dateText := strings.TrimSpace(extractTextFromCell(cells[0]))
		valStr := extractAttr(cells[1], "office:value")
		if dateText == "" || valStr == "" || dateText == "Gesamt" {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		entries = append(entries, timeEntry{DateStr: dateText, Hours: val})
	}
	return entries, nil
}

func splitByTag(content, tag string) []string {
	openTag := "<" + tag
	closeTag := "</" + tag + ">"
	var parts []string
	rest := content
	for {
		idx := strings.Index(rest, openTag)
		if idx < 0 {
			break
		}
		rest = rest[idx:]
		end := strings.Index(rest, closeTag)
		if end < 0 {
			break
		}
		parts = append(parts, rest[:end+len(closeTag)])
		rest = rest[end+len(closeTag):]
	}
	return parts
}

func extractTextFromCell(cell string) string {
	var texts []string
	for _, p := range splitByTag(cell, "text:p") {
		texts = append(texts, stripTags(p))
	}
	return strings.TrimSpace(strings.Join(texts, " "))
}

func stripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return result.String()
}

func extractAttr(s, attr string) string {
	search := attr + "=\""
	idx := strings.Index(s, search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(s[start:], "\"")
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

// ─── XLSX reader ────────────────────────────────────────────────────────────

func readXLSX(path string) ([]timeEntry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	sharedStrings, err := readXLSXSharedStrings(r)
	if err != nil {
		return nil, err
	}
	return readXLSXSheet(r, sharedStrings)
}

func readXLSXSharedStrings(r *zip.ReadCloser) ([]string, error) {
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			buf := readAll(rc)
			content := string(buf)
			var ss []string
			for _, si := range splitByTag(content, "si") {
				ss = append(ss, stripTags(si))
			}
			return ss, nil
		}
	}
	return nil, fmt.Errorf("sharedStrings.xml not found")
}

func readXLSXSheet(r *zip.ReadCloser, sharedStrings []string) ([]timeEntry, error) {
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			buf := readAll(rc)
			return parseXLSXRows(string(buf), sharedStrings), nil
		}
	}
	return nil, fmt.Errorf("sheet1.xml not found")
}

func readAll(rc interface{ Read([]byte) (int, error) }) []byte {
	buf := make([]byte, 0, 512*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := rc.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf
}

func parseXLSXRows(content string, sharedStrings []string) []timeEntry {
	var entries []timeEntry
	for _, row := range splitByTag(content, "row") {
		cells := splitByTag(row, "c")
		if len(cells) < 2 {
			continue
		}
		dateText := strings.TrimSpace(xlsxCellValue(cells[0], sharedStrings))
		hoursStr := xlsxCellValue(cells[1], sharedStrings)
		if dateText == "" || hoursStr == "" || dateText == "Gesamt" {
			continue
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(hoursStr), 64)
		if err != nil {
			continue
		}
		entries = append(entries, timeEntry{DateStr: dateText, Hours: val})
	}
	return entries
}

func xlsxCellValue(cell string, sharedStrings []string) string {
	isShared := strings.Contains(cell, ` t="s"`)
	vs := splitByTag(cell, "v")
	if len(vs) == 0 {
		return ""
	}
	val := stripTags(vs[0])
	if isShared {
		idx, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil || idx >= len(sharedStrings) {
			return ""
		}
		return sharedStrings[idx]
	}
	return val
}

// ─── Date parsing ───────────────────────────────────────────────────────────

func parseGermanDate(text string) (time.Time, bool) {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) != 3 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, false
	}
	month, ok := germanMonths[parts[1]]
	if !ok {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local), true
}

// ─── Absence CSV reader ─────────────────────────────────────────────────────

type Absence struct {
	Type        string
	Description string
	Days        float64
	Start       time.Time
	End         time.Time
	Status      string
}

type AbsenceDay struct {
	Category string
	HalfDay  bool
	Desc     string
}

func findAbsenceFile() string {
	dir := dataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(name), "abwesenheiten") && strings.HasSuffix(name, ".csv") {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

func readAbsences(path string) ([]Absence, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, nil
	}

	header := records[0]
	idx := make(map[string]int)
	for i, h := range header {
		idx[h] = i
	}

	var absences []Absence
	for _, row := range records[1:] {
		if len(row) == 0 || (len(row) == 1 && row[0] == "") {
			continue
		}
		a := Absence{}
		if i, ok := idx["Abwesenheitsart"]; ok && i < len(row) {
			a.Type = row[i]
		}
		if i, ok := idx["Beschreibung"]; ok && i < len(row) {
			a.Description = row[i]
		}
		if i, ok := idx["Dauer (Tage)"]; ok && i < len(row) {
			a.Days, _ = strconv.ParseFloat(strings.TrimSpace(row[i]), 64)
		}
		if i, ok := idx["Status"]; ok && i < len(row) {
			a.Status = row[i]
		}
		if i, ok := idx["Startdatum"]; ok && i < len(row) {
			a.Start = parseAbsenceDateTime(row[i])
		}
		if i, ok := idx["Enddatum"]; ok && i < len(row) {
			a.End = parseAbsenceDateTime(row[i])
		}
		if a.Type != "" && a.Status == "Genehmigt" {
			absences = append(absences, a)
		}
	}
	sort.Slice(absences, func(i, j int) bool { return absences[i].Start.Before(absences[j].Start) })
	return absences, nil
}

func parseAbsenceDateTime(s string) time.Time {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		t, _ = time.Parse("2006-01-02", s)
	}
	return t
}

func absenceCategory(absType string) string {
	lower := strings.ToLower(absType)
	if strings.Contains(lower, "urlaub") {
		return "Urlaub"
	}
	if strings.Contains(lower, "krankheit") || strings.Contains(lower, "krank") {
		return "Krank"
	}
	return absType
}

func buildAbsenceMap(absences []Absence, hmap map[string]string) map[string]AbsenceDay {
	m := make(map[string]AbsenceDay)
	for _, a := range absences {
		cat := absenceCategory(a.Type)
		isHalf := a.Days == 0.5
		startDate := time.Date(a.Start.Year(), a.Start.Month(), a.Start.Day(), 0, 0, 0, 0, time.Local)
		endDate := time.Date(a.End.Year(), a.End.Month(), a.End.Day(), 0, 0, 0, 0, time.Local)
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			if isWeekday(d) {
				if _, isH := hmap[dateKey(d)]; !isH {
					m[dateKey(d)] = AbsenceDay{Category: cat, HalfDay: isHalf, Desc: a.Description}
				}
			}
		}
	}
	return m
}

// ─── File discovery & loading ───────────────────────────────────────────────

func findDataFile() string {
	dir := dataDir()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".ods") && !strings.HasPrefix(e.Name(), ".~lock") {
			return filepath.Join(dir, e.Name())
		}
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".xlsx") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func loadTimeData(path string) (map[string]float64, error) {
	var entries []timeEntry
	var err error
	if strings.HasSuffix(path, ".ods") {
		entries, err = readODS(path)
	} else if strings.HasSuffix(path, ".xlsx") {
		entries, err = readXLSX(path)
	} else {
		return nil, fmt.Errorf("unbekanntes Dateiformat: %s", path)
	}
	if err != nil {
		return nil, err
	}
	data := make(map[string]float64)
	for _, e := range entries {
		d, ok := parseGermanDate(e.DateStr)
		if ok {
			data[dateKey(d)] = e.Hours
		}
	}
	return data, nil
}

// ─── App data (shared across views) ────────────────────────────────────────

type appData struct {
	filePath  string
	year      int
	timeData  map[string]float64
	hmap      map[string]string
	absMap    map[string]AbsenceDay
	absences  []Absence
	holidays  []Holiday
	minDate   string
	maxDate   string
}

// ─── Bubble Tea model ───────────────────────────────────────────────────────

// Pages are navigated with ←→. ↑↓ navigates within a page.
const numPages = 4

type model struct {
	data     appData
	page     int // 0=Dashboard, 1=Jahr, 2=Monate, 3=Feiertage & Abwesenheiten
	monthIdx int // selected month on Monate page (0-11)
	width    int
	height   int
}

var pageNames = []string{
	"Dashboard",
	"Jahr",
	"Monate",
	"Feiertage & Abwesenheiten",
}

func initialModel(data appData) model {
	return model{
		data:     data,
		page:     0,
		monthIdx: int(time.Now().Month()) - 1,
		width:    120,
		height:   40,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "right", "l", "tab":
			m.page = (m.page + 1) % numPages
			return m, nil
		case "left", "h", "shift+tab":
			m.page = (m.page - 1 + numPages) % numPages
			return m, nil
		}

		// Page-specific ↑↓ handling
		switch m.page {
		case 2: // Monate: ↑↓ selects month
			switch key {
			case "up", "k":
				m.monthIdx--
				if m.monthIdx < 0 {
					m.monthIdx = 11
				}
				return m, nil
			case "down", "j":
				m.monthIdx++
				if m.monthIdx > 11 {
					m.monthIdx = 0
				}
				return m, nil
			}
		}
	}
	return m, nil
}


// ─── View rendering ─────────────────────────────────────────────────────────

func (m model) pageIndicator() string {
	var parts []string
	for i, name := range pageNames {
		if i == m.page {
			parts = append(parts, accentStyle.Render("● "+name))
		} else {
			parts = append(parts, dimStyle.Render("○ "+name))
		}
	}
	return strings.Join(parts, "  ")
}

func (m model) View() string {
	var body string

	switch m.page {
	case 0:
		body = m.renderDashboard()
	case 1:
		body = m.renderYearPage()
	case 2:
		body = m.renderMonthPage()
	case 3:
		body = m.renderCombinedPage()
	}

	return body
}


func (m model) renderYearPage() string {
	nav := m.pageIndicator()
	help := helpStyle.Render("←→ Seite wechseln  q Beenden")

	stats := m.renderYearly()
	heatmap := m.renderYearHeatmap()

	left := lipgloss.NewStyle().MarginRight(4).Render(stats)
	right := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("Jahreskalender"),
		"",
		heatmap,
	)

	combined := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, "", combined, "", nav, help)
}

func (m model) renderMonthPage() string {
	nav := m.pageIndicator()
	help := helpStyle.Render("←→ Seite wechseln  ↑↓ Monat wählen  q Beenden")
	footer := lipgloss.JoinVertical(lipgloss.Left, nav, help)
	footerH := lipgloss.Height(footer)

	// Left: month list
	var listB strings.Builder
	listB.WriteString(titleStyle.Render(fmt.Sprintf("Monate %d", m.data.year)))
	listB.WriteString("\n\n")

	today := time.Now()
	d := m.data

	for mi := 0; mi < 12; mi++ {
		mo := time.Month(mi + 1)
		wd := workingDaysInMonth(d.year, mo, d.hmap)

		netWd := 0.0
		for _, day := range wd {
			dk := dateKey(day)
			if ad, ok := d.absMap[dk]; ok {
				if ad.HalfDay {
					netWd += 0.5
				}
			} else {
				netWd += 1.0
			}
		}

		monthHours := 0.0
		for dk, h := range d.timeData {
			t, _ := time.Parse("2006-01-02", dk)
			if t.Month() == mo {
				monthHours += h
			}
		}

		firstOfMonth := time.Date(d.year, mo, 1, 0, 0, 0, 0, time.Local)
		lastOfMonth := time.Date(d.year, mo, daysInMonth(d.year, mo), 0, 0, 0, 0, time.Local)

		var soll float64
		if lastOfMonth.Before(today) || dateKey(lastOfMonth) == dateKey(today) {
			soll = netWd * standardHoursPerDay
		} else if firstOfMonth.After(today) {
			soll = netWd * standardHoursPerDay
		} else {
			elapsed := 0.0
			for _, day := range wd {
				if !day.After(today) {
					dk := dateKey(day)
					if ad, ok := d.absMap[dk]; ok {
						if ad.HalfDay {
							elapsed += 0.5
						}
					} else {
						elapsed += 1.0
					}
				}
			}
			soll = elapsed * standardHoursPerDay
		}

		balance := monthHours - soll
		isFuture := monthHours == 0 && firstOfMonth.After(today)

		name := fmt.Sprintf("%-12s", monthsDE[mi])
		istStr := fmt.Sprintf("%8s", formatHours(monthHours))
		sollStr := fmt.Sprintf("%8s", formatHours(soll))
		var line string
		if isFuture {
			line = fmt.Sprintf("%s  %8s  %s", name, "—", sollStr)
			line = dimStyle.Render(line)
		} else {
			bal := coloredSaldo(balance)
			line = fmt.Sprintf("%s  %s  %s  %s", name, istStr, dimStyle.Render(sollStr), bal)
		}

		if mi == m.monthIdx {
			listB.WriteString(accentStyle.Render("▸ ") + lipgloss.NewStyle().Bold(true).Render(line))
		} else {
			listB.WriteString("  " + line)
		}
		listB.WriteString("\n")
	}
	listB.WriteString("\n")
	listB.WriteString(dimStyle.Render(fmt.Sprintf("%-12s  %8s  %8s  %s", "", "Ist", "Soll", "Saldo")))

	leftPanel := listB.String()

	// Right: calendar for selected month
	calTitle := titleStyle.Render(fmt.Sprintf("%s %d", monthsDE[m.monthIdx], m.data.year))
	calBody := m.renderMonthCalendar(m.monthIdx + 1)
	rightPanel := calTitle + "\n\n" + calBody

	// Truncate right panel if it exceeds available height
	leftH := lipgloss.Height(leftPanel)
	availH := m.height - footerH - 2
	if availH < leftH {
		availH = leftH
	}
	rightLines := strings.Split(rightPanel, "\n")
	maxRightH := availH
	if len(rightLines) > maxRightH && maxRightH > 3 {
		rightLines = rightLines[:maxRightH]
		rightPanel = strings.Join(rightLines, "\n")
	}

	combined := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().MarginRight(4).Render(leftPanel),
		rightPanel,
	)

	// Place entire content top-aligned in the terminal
	body := lipgloss.JoinVertical(lipgloss.Left, "", combined)
	placed := lipgloss.Place(m.width, m.height-footerH-1, lipgloss.Left, lipgloss.Top, body)

	return placed + "\n" + footer
}

// ─── Dashboard (start screen) ───────────────────────────────────────────────

var (
	cardWidth    = 34
	cardHeight   = 10
	cardPadding  = lipgloss.NewStyle().Padding(1, 2)
	cardTitle    = lipgloss.NewStyle().Bold(true).MarginBottom(1)
	bigNumber    = lipgloss.NewStyle().Bold(true)
	cardDetail   = lipgloss.NewStyle().Foreground(colPrimaryGrey)
	cardBase = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Width(cardWidth).
			Height(cardHeight).
			MarginRight(2)
	dashTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colMint)
	dashDateStyle = lipgloss.NewStyle().
			Foreground(colPrimaryGrey).
			MarginLeft(1)
)

func saldoCard(title string, balance, ist, soll float64, detail string) string {
	sign := "+"
	color := colPrimaryGreen
	borderColor := colPrimaryGreen
	if balance < 0 {
		sign = ""
		color = colOrange
		borderColor = colOrange
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		cardTitle.Render(title),
		bigNumber.Foreground(color).Render(fmt.Sprintf("%s%s", sign, formatHours(balance))),
		"",
		cardDetail.Render(fmt.Sprintf("Ist  %s", formatHours(ist))),
		cardDetail.Render(fmt.Sprintf("Soll %s", formatHours(soll))),
		"",
		cardDetail.Render(detail),
	)

	return cardBase.BorderForeground(borderColor).Render(cardPadding.Render(content))
}

func todayCard(d appData) string {
	todayKey := dateKey(time.Now())
	today := time.Now()

	var line1, line2 string
	borderColor := colBgMidGrey

	if hours, ok := d.timeData[todayKey]; ok && hours > 0 {
		diff := hours - standardHoursPerDay
		color := colPrimaryGreen
		if diff < 0 {
			color = colYellow
		}
		line1 = bigNumber.Foreground(color).Render(formatHours(hours))
		line2 = cardDetail.Render(fmt.Sprintf("Soll %s  (%+.1fh)", formatHours(standardHoursPerDay), diff))
		if diff >= 0 {
			borderColor = colPrimaryGreen
		}
	} else if name, isH := d.hmap[todayKey]; isH {
		line1 = holidayStyle.Render(name)
	} else if ad, isAbs := d.absMap[todayKey]; isAbs {
		if ad.Category == "Krank" {
			line1 = negativeStyle.Render("Krank")
		} else {
			line1 = warnStyle.Render(ad.Category)
		}
		if ad.Desc != "" {
			line2 = cardDetail.Render(ad.Desc)
		}
	} else if isWeekday(today) {
		line1 = warnStyle.Render("Noch nicht gebucht")
	} else {
		line1 = dimStyle.Render("Wochenende")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		cardTitle.Render("Heute"),
		line1,
	)
	if line2 != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", line2)
	}

	return cardBase.BorderForeground(borderColor).Render(cardPadding.Render(content))
}

func (m model) renderDashboard() string {
	d := m.data
	today := time.Now()
	todayKey := dateKey(today)
	curMonth := today.Month()

	allWorking := workingDaysInYear(d.year, d.hmap)

	// Year
	totalHours := 0.0
	for _, h := range d.timeData {
		totalHours += h
	}
	daysWorked := len(d.timeData)

	elapsedNetYear := 0.0
	for _, day := range allWorking {
		if dateKey(day) <= todayKey {
			dk := dateKey(day)
			if ad, ok := d.absMap[dk]; ok && ad.HalfDay {
				elapsedNetYear += 0.5
			} else if !ok {
				elapsedNetYear += 1.0
			}
		}
	}
	expectedYear := elapsedNetYear * standardHoursPerDay
	balanceYear := totalHours - expectedYear

	// Month
	monthHours := 0.0
	monthDays := 0
	for dk, h := range d.timeData {
		t, _ := time.Parse("2006-01-02", dk)
		if t.Month() == curMonth {
			monthHours += h
			monthDays++
		}
	}

	wd := workingDaysInMonth(d.year, curMonth, d.hmap)
	elapsedNetMonth := 0.0
	for _, day := range wd {
		if !day.After(today) {
			dk := dateKey(day)
			if ad, ok := d.absMap[dk]; ok && ad.HalfDay {
				elapsedNetMonth += 0.5
			} else if !ok {
				elapsedNetMonth += 1.0
			}
		}
	}
	expectedMonth := elapsedNetMonth * standardHoursPerDay
	balanceMonth := monthHours - expectedMonth

	// Absence totals
	urlaubDays := 0.0
	krankDays := 0.0
	for _, ad := range d.absMap {
		inc := 1.0
		if ad.HalfDay {
			inc = 0.5
		}
		if ad.Category == "Urlaub" {
			urlaubDays += inc
		} else if ad.Category == "Krank" {
			krankDays += inc
		}
	}
	netWorkingDays := float64(len(allWorking)) - urlaubDays - krankDays

	holidaysOnWeekdays := 0
	for _, h := range d.holidays {
		if isWeekday(h.Date) {
			holidaysOnWeekdays++
		}
	}

	// Cards
	yCard := saldoCard(
		fmt.Sprintf("Jahr %d", d.year),
		balanceYear, totalHours, expectedYear,
		fmt.Sprintf("%d Tage gearbeitet", daysWorked),
	)
	mCard := saldoCard(
		monthsDE[int(curMonth)-1],
		balanceMonth, monthHours, expectedMonth,
		fmt.Sprintf("%d Tage gearbeitet", monthDays),
	)
	tCard := todayCard(d)

	cards := lipgloss.JoinHorizontal(lipgloss.Top, yCard, mCard, tCard)

	// Progress bar
	progressPct := 0.0
	if netWorkingDays > 0 {
		progressPct = elapsedNetYear / netWorkingDays
	}
	if progressPct > 1 {
		progressPct = 1
	}

	progBarWidth := lipgloss.Width(cards) - lipgloss.Width("Jahresfortschritt  ")
	if progBarWidth < 30 {
		progBarWidth = 30
	}
	prog := progress.New(
		progress.WithScaledGradient("#1E414E", "#A0D22B"),
		progress.WithWidth(progBarWidth),
		progress.WithoutPercentage(),
	)
	progBar := prog.ViewAs(progressPct)

	progLine := lipgloss.JoinHorizontal(lipgloss.Center,
		dimStyle.Render("Jahresfortschritt"),
		"  ",
		progBar,
		"  ",
		accentStyle.Render(fmt.Sprintf("%.0f%%", progressPct*100)),
	)

	statsLine := dimStyle.Render(fmt.Sprintf(
		"%.0f / %.0f Arbeitstage  │  %.1f Urlaub  │  %.1f Krank  │  %d Feiertage",
		elapsedNetYear, netWorkingDays, urlaubDays, krankDays, holidaysOnWeekdays,
	))

	minT, _ := time.Parse("2006-01-02", d.minDate)
	maxT, _ := time.Parse("2006-01-02", d.maxDate)
	fileInfo := dimStyle.Render(fmt.Sprintf(
		"%s  │  %d Einträge (%s — %s)",
		filepath.Base(d.filePath),
		len(d.timeData),
		minT.Format("02.01.2006"),
		maxT.Format("02.01.2006"),
	))

	header := lipgloss.JoinHorizontal(lipgloss.Bottom,
		dashTitleStyle.Render(fmt.Sprintf("Vodoo %d", d.year)),
		dashDateStyle.Render(today.Format("Monday, 02.01.2006")),
	)

	nav := m.pageIndicator()
	help := helpStyle.Render("←→ Seite wechseln  q Beenden")

	// Missing days warning
	missingHint := ""
	if missing := m.countMissingDays(); missing > 0 {
		missingHint = warnStyle.Render(fmt.Sprintf("⚠ %d fehlende Buchung(en)", missing))
	}

	weekView := m.renderWeekView()

	content := lipgloss.JoinVertical(lipgloss.Center,
		header,
		"",
		cards,
		"",
		weekView,
		"",
		progLine,
		statsLine,
		missingHint,
		"",
		fileInfo,
		"",
		nav,
		help,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

// ─── Week view (dashboard widget) ───────────────────────────────────────────

func (m model) renderWeekView() string {
	d := m.data
	today := time.Now()

	monday := today
	for wdIdx(monday) != 0 {
		monday = monday.AddDate(0, 0, -1)
	}
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.Local)

	barWidth := 30
	labelCol := lipgloss.NewStyle().Width(12)
	infoCol := lipgloss.NewStyle().Width(10)

	var rows []string
	weekTotal := 0.0

	for i := 0; i < 5; i++ {
		day := monday.AddDate(0, 0, i)
		dk := dateKey(day)
		isToday := dateKey(day) == dateKey(today)

		labelText := fmt.Sprintf("  %s %s", weekdaysDE[i], day.Format("02.01"))
		if isToday {
			labelText = lipgloss.NewStyle().Foreground(colMint).Bold(true).Render(
				fmt.Sprintf("> %s %s", weekdaysDE[i], day.Format("02.01")))
		}
		label := labelCol.Render(labelText)

		hours, hasEntry := d.timeData[dk]
		_, isHoliday := d.hmap[dk]
		ad, isAbsent := d.absMap[dk]

		var bar, info string

		switch {
		case isHoliday:
			p := progress.New(progress.WithSolidFill("#B5DEC2"), progress.WithWidth(barWidth), progress.WithoutPercentage())
			bar = p.ViewAs(1.0)
			info = holidayStyle.Render("Feiertag")
		case isAbsent && !ad.HalfDay:
			color := "#FFE665"
			infoText := "Urlaub"
			if ad.Category == "Krank" {
				color = "#FF9F62"
				infoText = "Krank"
			}
			p := progress.New(progress.WithSolidFill(color), progress.WithWidth(barWidth), progress.WithoutPercentage())
			bar = p.ViewAs(1.0)
			info = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(infoText)
		case hasEntry && hours > 0:
			weekTotal += hours
			pct := hours / 10.0
			if pct > 1 {
				pct = 1
			}
			color := "#A0D22B"
			if hours < standardHoursPerDay {
				color = "#FFE665"
			}
			p := progress.New(progress.WithSolidFill(color), progress.WithWidth(barWidth), progress.WithoutPercentage())
			bar = p.ViewAs(pct)
			info = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(formatHours(hours))
		default:
			p := progress.New(progress.WithSolidFill("#5B5B5B"), progress.WithWidth(barWidth), progress.WithoutPercentage())
			bar = p.ViewAs(0.0)
			if isWeekday(day) && !day.After(today) {
				info = negativeStyle.Render("--")
			}
		}

		row := lipgloss.JoinHorizontal(lipgloss.Center, label, "  ", bar, "  ", infoCol.Render(info))
		rows = append(rows, row)
	}

	header := dimStyle.Render("  Aktuelle Woche")
	weekRows := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var footer string
	if weekTotal > 0 {
		// Count actual Soll days (exclude holidays, full absences; half-day = 0.5)
		sollDays := 0.0
		for i := 0; i < 5; i++ {
			day := monday.AddDate(0, 0, i)
			dk := dateKey(day)
			if _, isH := d.hmap[dk]; isH {
				continue
			}
			if ad, isA := d.absMap[dk]; isA {
				if ad.HalfDay {
					sollDays += 0.5
				}
				continue
			}
			sollDays += 1.0
		}
		sollWeek := sollDays * standardHoursPerDay
		footer = fmt.Sprintf("  %s %s / %s  %s",
			dimStyle.Render("Woche:"),
			formatHours(weekTotal),
			dimStyle.Render(formatHours(sollWeek)),
			coloredSaldo(weekTotal-sollWeek))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, weekRows, footer)
}

// ─── Yearly overview (stat block, not a table) ──────────────────────────────

func (m model) renderYearly() string {
	d := m.data
	allWorking := workingDaysInYear(d.year, d.hmap)
	today := time.Now()
	todayKey := dateKey(today)

	totalHours := 0.0
	for _, h := range d.timeData {
		totalHours += h
	}
	daysWorked := len(d.timeData)
	avgHours := 0.0
	if daysWorked > 0 {
		avgHours = totalHours / float64(daysWorked)
	}

	totalWorkingDays := len(allWorking)
	urlaubDays := 0.0
	krankDays := 0.0
	for _, ad := range d.absMap {
		inc := 1.0
		if ad.HalfDay {
			inc = 0.5
		}
		if ad.Category == "Urlaub" {
			urlaubDays += inc
		} else if ad.Category == "Krank" {
			krankDays += inc
		}
	}
	netWorkingDays := float64(totalWorkingDays) - urlaubDays - krankDays

	holidaysOnWeekdays := 0
	for _, h := range d.holidays {
		if isWeekday(h.Date) {
			holidaysOnWeekdays++
		}
	}

	elapsedNet := 0.0
	for _, day := range allWorking {
		if dateKey(day) <= todayKey {
			dk := dateKey(day)
			if ad, ok := d.absMap[dk]; ok {
				if ad.HalfDay {
					elapsedNet += 0.5
				}
			} else {
				elapsedNet += 1.0
			}
		}
	}

	expectedHours := elapsedNet * standardHoursPerDay
	balance := totalHours - expectedHours

	stat := func(label string, value string) string {
		return statLabelStyle.Render(label) + value
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("Jahresübersicht %d", d.year)))
	b.WriteString("\n\n")

	section1 := strings.Join([]string{
		stat("Arbeitstage gesamt:", fmt.Sprintf("%d", totalWorkingDays)),
		stat("Urlaubstage:", fmt.Sprintf("%.1f", urlaubDays)),
		stat("Krankheitstage:", fmt.Sprintf("%.1f", krankDays)),
		stat("Feiertage (Mo-Fr):", fmt.Sprintf("%d", holidaysOnWeekdays)),
		stat("Netto-Arbeitstage:", fmt.Sprintf("%.1f", netWorkingDays)),
		stat("Soll-Stunden (Netto):", formatHours(netWorkingDays*standardHoursPerDay)),
	}, "\n")

	section2 := strings.Join([]string{
		stat("Tage gearbeitet:", fmt.Sprintf("%d", daysWorked)),
		stat("Stunden gearbeitet:", fmt.Sprintf("%s  (%.2fh)", formatHours(totalHours), totalHours)),
		stat("Ø Stunden/Tag:", fmt.Sprintf("%s  (%.2fh)", formatHours(avgHours), avgHours)),
	}, "\n")

	balStr := coloredSaldo(balance)
	section3 := strings.Join([]string{
		stat("Soll bis heute:", formatHours(expectedHours)),
		stat("Ist bis heute:", formatHours(totalHours)),
		stat("Saldo:", fmt.Sprintf("%s  (%+.2fh)", balStr, balance)),
	}, "\n")

	content := section1 + "\n\n" + section2 + "\n\n" + section3
	b.WriteString(boxStyle.Render(content))

	return b.String()
}

// ─── Year heatmap calendar ──────────────────────────────────────────────────

var (
	heatNone    = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	heat0       = lipgloss.NewStyle().Foreground(colBgMidGrey)
	heat1       = lipgloss.NewStyle().Foreground(colDarkTeal)
	heat2       = lipgloss.NewStyle().Foreground(colSoftGreen)
	heat3       = lipgloss.NewStyle().Foreground(colSoftGreen)
	heatOver    = lipgloss.NewStyle().Foreground(colPrimaryGreen)
	heatHoliday = lipgloss.NewStyle().Foreground(colMint)
	heatAbsence = lipgloss.NewStyle().Foreground(colYellow)
	heatSick    = lipgloss.NewStyle().Foreground(colOrange)
	heatWeekend = lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))
	calMonthLbl = lipgloss.NewStyle().Bold(true).Width(5)
)

func hoursToBlock(hours float64) (string, lipgloss.Style) {
	if hours <= 0 {
		return "■", heat0
	}
	ratio := hours / standardHoursPerDay
	switch {
	case ratio < 0.5:
		return "■", heat1
	case ratio < 0.75:
		return "■", heat2
	case ratio < 1.0:
		return "■", heat3
	default:
		return "■", heatOver
	}
}

func (m model) renderYearHeatmap() string {
	d := m.data
	today := time.Now()

	// GitHub-style: 7 rows (Mo-So) x ~53 columns (weeks)
	// Build grid: find first Monday on or before Jan 1
	jan1 := time.Date(d.year, 1, 1, 0, 0, 0, 0, time.Local)
	startDate := jan1
	for wdIdx(startDate) != 0 { // find Monday
		startDate = startDate.AddDate(0, 0, -1)
	}

	dec31 := time.Date(d.year, 12, 31, 0, 0, 0, 0, time.Local)
	endDate := dec31
	for wdIdx(endDate) != 6 { // extend to Sunday
		endDate = endDate.AddDate(0, 0, 1)
	}

	// Count weeks
	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1
	numWeeks := totalDays / 7

	// Build grid[weekday][week]
	type cell struct {
		block string
		style lipgloss.Style
	}
	grid := make([][]cell, 7)
	for i := range grid {
		grid[i] = make([]cell, numWeeks)
	}

	// Month labels: track which week each month starts
	monthWeeks := make([]int, 13) // monthWeeks[m] = first week column of month m
	for i := range monthWeeks {
		monthWeeks[i] = -1
	}

	cur := startDate
	for w := 0; w < numWeeks; w++ {
		for wd := 0; wd < 7; wd++ {
			dk := dateKey(cur)

			if cur.Year() == d.year {
				mo := int(cur.Month())
				if monthWeeks[mo] == -1 {
					monthWeeks[mo] = w
				}
			}

			var block string
			var style lipgloss.Style

			if cur.Year() != d.year {
				block = " "
				style = heatNone
			} else if _, isH := d.hmap[dk]; isH {
				block = "■"
				style = heatHoliday
			} else if ad, isAbs := d.absMap[dk]; isAbs {
				block = "■"
				if ad.Category == "Krank" {
					style = heatSick
				} else {
					style = heatAbsence
				}
			} else if wdIdx(cur) >= 5 {
				block = "·"
				style = heatWeekend
			} else if cur.After(today) {
				block = "·"
				style = heatNone
			} else if hours, ok := d.timeData[dk]; ok {
				block, style = hoursToBlock(hours)
			} else {
				block = "■"
				style = negativeStyle
			}

			grid[wd][w] = cell{block, style}
			cur = cur.AddDate(0, 0, 1)
		}
	}

	var b strings.Builder

	// Month labels row
	b.WriteString("      ")
	lastMonth := -1
	for w := 0; w < numWeeks; w++ {
		printed := false
		for mo := 1; mo <= 12; mo++ {
			if monthWeeks[mo] == w && mo != lastMonth {
				label := monthsDE[mo-1]
				if len(label) > 3 {
					label = label[:3]
				}
				b.WriteString(dimStyle.Render(label))
				lastMonth = mo
				printed = true
				break
			}
		}
		if !printed {
			b.WriteString("  ")
		}
	}
	b.WriteString("\n")

	// Rows: one per weekday
	wdLabels := []string{"Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"}
	for wd := 0; wd < 7; wd++ {
		if wd%2 == 0 {
			b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%-3s", wdLabels[wd])) + " ")
		} else {
			b.WriteString("      ")
		}
		for w := 0; w < numWeeks; w++ {
			c := grid[wd][w]
			b.WriteString(c.style.Render(c.block) + " ")
		}
		b.WriteString("\n")
	}

	// Legend
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s  %s  %s  %s  %s",
		dimStyle.Render("Legende:"),
		heat0.Render("■")+" "+dimStyle.Render("0h"),
		heat2.Render("■")+" "+dimStyle.Render("<8h"),
		heatOver.Render("■")+" "+dimStyle.Render("8h"),
		heatOver.Render("■")+" "+dimStyle.Render(">8h"),
		heatHoliday.Render("■")+" "+dimStyle.Render("Feiertag"),
		heatAbsence.Render("■")+" "+dimStyle.Render("Urlaub"),
		heatSick.Render("■")+" "+dimStyle.Render("Krank"),
	))

	return b.String()
}


var (
	calCellHoliday = lipgloss.NewStyle().Foreground(colMint)
	calCellAbsence = lipgloss.NewStyle().Foreground(colYellow)
	calCellSick    = lipgloss.NewStyle().Foreground(colOrange)
	calHeaderCell  = lipgloss.NewStyle().Width(10).Align(lipgloss.Center).Bold(true).Foreground(colMint)
	calSepStyle    = lipgloss.NewStyle().Foreground(colBgMidGrey)
)

func (m model) renderMonthCalendar(month int) string {
	d := m.data
	nd := daysInMonth(d.year, time.Month(month))
	today := time.Now()
	cellW := 10

	// Header
	var headerCells []string
	for _, wd := range weekdaysDE {
		headerCells = append(headerCells, calHeaderCell.Render(wd))
	}
	header := "  " + lipgloss.JoinHorizontal(lipgloss.Top, headerCells...)

	// Build week rows using JoinHorizontal so multiline cells align correctly
	firstDay := time.Date(d.year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	offset := wdIdx(firstDay)

	monthTotal := 0.0
	day := 1
	var weekRows []string

	for day <= nd {
		var cells []string

		for wi := 0; wi < 7; wi++ {
			if (len(weekRows) == 0 && wi < offset) || day > nd {
				// Empty cell
				cells = append(cells, lipgloss.NewStyle().Width(cellW).Render(""))
				continue
			}

			dt := time.Date(d.year, time.Month(month), day, 0, 0, 0, 0, time.Local)
			dk := dateKey(dt)
			hours, hasEntry := d.timeData[dk]
			_, isHoliday := d.hmap[dk]
			ad, isAbsent := d.absMap[dk]
			isToday := dateKey(dt) == dateKey(today)

			dayNum := fmt.Sprintf("%d", day)
			var detail string
			var detailStyle lipgloss.Style
			dayStyle := dimStyle // default: dim day number
			cellBase := lipgloss.NewStyle().Width(cellW).Align(lipgloss.Center)

			switch {
			case isHoliday:
				detail = "Feiert."
				detailStyle = calCellHoliday
			case isAbsent && !ad.HalfDay:
				detail = "Urlaub"
				detailStyle = calCellAbsence
				if ad.Category == "Krank" {
					detail = "Krank"
					detailStyle = calCellSick
				}
			case wi >= 5:
				if hasEntry && hours > 0 {
					detail = formatHours(hours) + "!"
					detailStyle = negativeStyle
					monthTotal += hours
				} else {
					dayStyle = lipgloss.NewStyle().Foreground(colBgMidGrey)
				}
			case hasEntry:
				monthTotal += hours
				detail = formatHours(hours)
				if hours >= standardHoursPerDay {
					detailStyle = positiveStyle
				} else {
					detailStyle = warnStyle
				}
				if isAbsent && ad.HalfDay {
					detail = formatHours(hours) + "½"
				}
			case !dt.After(today) && isWeekday(dt):
				detail = "FEHLT"
				detailStyle = negativeStyle.Bold(true)
			}

			if isToday {
				dayStyle = lipgloss.NewStyle().Bold(true).Background(colDarkTeal).Foreground(lipgloss.Color("#FFFFFF"))
			}

			var content string
			if detail != "" {
				content = dayStyle.Render(dayNum) + "\n" + detailStyle.Render(detail)
			} else {
				content = dayStyle.Render(dayNum)
			}

			cells = append(cells, cellBase.Render(content))
			day++
		}

		weekRow := "  " + lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		weekRows = append(weekRows, weekRow)
	}

	sep := "  " + calSepStyle.Render(strings.Repeat("─", cellW*7))
	var allRows []string
	allRows = append(allRows, header, sep)
	for i, row := range weekRows {
		allRows = append(allRows, row)
		if i < len(weekRows)-1 {
			allRows = append(allRows, "  "+calSepStyle.Render(strings.Repeat("┈", cellW*7)))
		}
	}

	allRows = append(allRows, "",
		fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Bold(true).Render("Summe:"),
			formatHours(monthTotal),
		),
	)

	return strings.Join(allRows, "\n")
}


// ─── Combined Feiertage & Abwesenheiten page ───────────────────────────────

func (m model) renderCombinedPage() string {
	d := m.data
	nav := m.pageIndicator()
	help := helpStyle.Render("←→ Seite wechseln  q Beenden")

	type entry struct {
		date time.Time
		wd   string
		kind string
		kindStyled string
		name string
	}

	var entries []entry

	for _, h := range d.holidays {
		kind := "Feiertag"
		styled := holidayStyle.Render("Feiertag")
		if wdIdx(h.Date) >= 5 {
			styled = dimStyle.Render("Feiertag (WE)")
			kind = "Feiertag (WE)"
		}
		entries = append(entries, entry{h.Date, weekdaysDE[wdIdx(h.Date)], kind, styled, h.Name})
	}

	for _, a := range d.absences {
		cat := absenceCategory(a.Type)
		kind := cat
		var styled string
		if cat == "Krank" {
			styled = negativeStyle.Render(cat)
		} else {
			styled = warnStyle.Render(cat)
		}
		if a.Days == 0.5 {
			kind += " (½)"
			styled += dimStyle.Render(" (½)")
		}
		entries = append(entries, entry{a.Start, weekdaysDE[wdIdx(a.Start)], kind, styled, a.Description})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].date.Before(entries[j].date) })

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Feiertage & Abwesenheiten %d", d.year)))
	b.WriteString("\n\n")

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colMint)
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		headerStyle.Render(fmt.Sprintf("%-12s", "Datum")),
		headerStyle.Render(fmt.Sprintf("%-4s", "Tag")),
		headerStyle.Render(fmt.Sprintf("%-14s", "Art")),
		headerStyle.Render("Bezeichnung")))
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 75)) + "\n")

	for _, e := range entries {
		b.WriteString(fmt.Sprintf("  %-12s  %-4s  %-14s  %s\n",
			e.date.Format("02.01.2006"),
			e.wd,
			e.kindStyled,
			e.name))
	}

	summary := m.renderAbsenceSummary()

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		b.String(),
		summary,
		"",
		nav,
		help,
	)
}

func (m model) renderAbsenceSummary() string {
	totalUrlaub := 0.0
	totalKrank := 0.0
	for _, a := range m.data.absences {
		cat := absenceCategory(a.Type)
		if cat == "Urlaub" {
			totalUrlaub += a.Days
		} else if cat == "Krank" {
			totalKrank += a.Days
		}
	}

	weekdayHolidays := 0
	for _, h := range m.data.holidays {
		if isWeekday(h.Date) {
			weekdayHolidays++
		}
	}

	return fmt.Sprintf("\n  %s %.1f Tage    %s %.1f Tage    %s %d (Mo-Fr)",
		lipgloss.NewStyle().Bold(true).Render("Urlaub:"), totalUrlaub,
		lipgloss.NewStyle().Bold(true).Render("Krank:"), totalKrank,
		lipgloss.NewStyle().Bold(true).Render("Feiertage:"), weekdayHolidays)
}

// ─── Missing days count (for dashboard) ─────────────────────────────────────

func (m model) countMissingDays() int {
	d := m.data
	today := time.Now()
	allWorking := workingDaysInYear(d.year, d.hmap)
	count := 0
	for _, day := range allWorking {
		dk := dateKey(day)
		if !day.After(today) {
			if ad, ok := d.absMap[dk]; ok && !ad.HalfDay {
				continue
			}
			if _, has := d.timeData[dk]; !has {
				count++
			}
		}
	}
	return count
}


// ─── Main ───────────────────────────────────────────────────────────────────

func main() {
	flag.StringVar(&dataDirPath, "data", "", "Pfad zum Datenordner (default: ./data)")
	flag.Parse()

	if dataDirPath == "" {
		dir, _ := os.Getwd()
		dataDirPath = filepath.Join(dir, "data")
	}

	path := findDataFile()
	if path == "" {
		fmt.Printf("Keine ODS/XLSX-Datei in %s gefunden.\n", dataDirPath)
		os.Exit(1)
	}

	timeData, err := loadTimeData(path)
	if err != nil {
		fmt.Printf("Fehler: %s\n", err)
		os.Exit(1)
	}
	if len(timeData) == 0 {
		fmt.Println("Keine Zeitdaten in der Datei gefunden.")
		os.Exit(1)
	}

	year := 0
	var minDate, maxDate string
	for dk := range timeData {
		if minDate == "" || dk < minDate {
			minDate = dk
		}
		if maxDate == "" || dk > maxDate {
			maxDate = dk
		}
		t, _ := time.Parse("2006-01-02", dk)
		if t.Year() > year {
			year = t.Year()
		}
	}

	var holidays []Holiday
	holidayFile := findHolidayFile()
	if holidayFile != "" {
		holidays, err = readHolidays(holidayFile)
		if err != nil {
			fmt.Printf("Fehler beim Lesen der Feiertage (%s): %s\n", holidayFile, err)
			os.Exit(1)
		}
	}
	if len(holidays) == 0 {
		fmt.Printf("Keine Feiertage-CSV gefunden in %s (feiertage_*.csv).\n", dataDirPath)
		os.Exit(1)
	}
	hmap := holidayMap(holidays)

	var absences []Absence
	absMap := make(map[string]AbsenceDay)
	absFile := findAbsenceFile()
	if absFile != "" {
		absences, err = readAbsences(absFile)
		if err != nil {
			fmt.Printf("Fehler beim Lesen der Abwesenheiten (%s): %s\n", absFile, err)
			os.Exit(1)
		}
		absMap = buildAbsenceMap(absences, hmap)
	}

	data := appData{
		filePath: path,
		year:     year,
		timeData: timeData,
		hmap:     hmap,
		absMap:   absMap,
		absences: absences,
		holidays: holidays,
		minDate:  minDate,
		maxDate:  maxDate,
	}

	p := tea.NewProgram(initialModel(data), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Fehler: %v\n", err)
		os.Exit(1)
	}
}
