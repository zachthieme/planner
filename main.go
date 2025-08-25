package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

type Layout struct {
	PageSize        string  // "Letter" or "PaperPro"
	Margin          float64 // pt
	Font            string
	TitleSize       float64
	SubTitleSize    float64
	BodySize        float64
	GridSpacingPt   float64
	ShowWeeks       bool
	ShowDays        bool
	ShowCollections bool
}

// Config passed into builders.
type PlannerConfig struct {
	Year   int
	Output string
	Layout Layout
}

// Centralized hyperlink registry.
type Links struct {
	Year        int            // Year hub anchor
	Months      [12]int        // 1..12
	Weeks       map[int][]int  // month -> per-week anchors
	Days        map[string]int // "YYYY-MM-DD" -> anchor
	Collections map[string]int // ["Writing","Ideas","OOO"] -> anchor
	Hubs        map[string]int // e.g., "Collections" hub
}

func newLinks() *Links {
	return &Links{
		Weeks:       make(map[int][]int),
		Days:        make(map[string]int),
		Collections: make(map[string]int),
		Hubs:        make(map[string]int),
	}
}

/************* Calendar helpers (ISO-ish, Monday-first) *************/
func monthWeeks(year int, month time.Month) [][]time.Time {
	loc := time.UTC
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc)
	offset := (int(first.Weekday()) + 6) % 7 // Monday=0
	start := first.AddDate(0, 0, -offset)

	var weeks [][]time.Time
	cur := start
	for {
		week := make([]time.Time, 7)
		for i := 0; i < 7; i++ {
			week[i] = cur
			cur = cur.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
		// Stop after we step beyond last and land on Monday
		if cur.After(last) && cur.Weekday() == time.Monday {
			break
		}
	}
	return weeks
}

/************************ Drawing helpers ***************************/
func ensurePageSize(pdf *fpdf.Fpdf, layout Layout) {
	if layout.PageSize == "PaperPro" {
		// reMarkable Paper Pro is 2100x2800 px; assuming ~300dpi that’s ~7x9.333in.
		// Points: 72 pt/in → ~504 x ~672. Use a custom size roughly matching the ratio.
		pdf.SetAutoPageBreak(true, layout.Margin)
	}
}

func dotGrid(pdf *fpdf.Fpdf, l Layout) {
	w, h := pdf.GetPageSize()
	left, top, right, bottom := l.Margin, l.Margin, w-l.Margin, h-l.Margin
	r := 0.6 // dot radius
	pdf.SetLineWidth(0.1)
	pdf.SetDrawColor(0, 0, 0)
	for y := top + l.GridSpacingPt; y < bottom; y += l.GridSpacingPt {
		for x := left + l.GridSpacingPt; x < right; x += l.GridSpacingPt {
			pdf.Circle(x, y, r, "F")
		}
	}
}

func setTitle(pdf *fpdf.Fpdf, l Layout, text string) {
	pdf.SetFont(l.Font, "B", l.TitleSize)
	pdf.CellFormat(0, 0, text, "", 1, "CM", false, 0, "")
	pdf.Ln(6)
}

func setSubTitle(pdf *fpdf.Fpdf, l Layout, text string) {
	pdf.SetFont(l.Font, "", l.SubTitleSize)
	pdf.CellFormat(0, 0, text, "", 1, "CM", false, 0, "")
	pdf.Ln(10)
}

func body(pdf *fpdf.Fpdf, l Layout, text string) {
	pdf.SetFont(l.Font, "", l.BodySize)
	pdf.CellFormat(0, 0, text, "", 1, "L", false, 0, "")
}

func addNav(pdf *fpdf.Fpdf, l Layout, homeLink, backLink int) {
	w, _ := pdf.GetPageSize()
	pdf.SetFont(l.Font, "B", l.BodySize)
	// Back (top-left)
	if backLink != 0 {
		pdf.SetXY(l.Margin, l.Margin*0.6)
		pdf.CellFormat(42, 16, "< Back", "", 0, "L", false, 0, "")
		pdf.Link(l.Margin, l.Margin*0.6, 42, 16, backLink)
	}
	// Home (top-right)
	if homeLink != 0 {
		pdf.SetXY(w-l.Margin-60, l.Margin*0.6)
		pdf.CellFormat(60, 16, "Home", "", 0, "R", false, 0, "")
		pdf.Link(w-l.Margin-60, l.Margin*0.6, 60, 16, homeLink)
	}
	pdf.Ln(10)
}

func gridLinks(pdf *fpdf.Fpdf, l Layout, cols int, labels []string, anchors []int) {
	w, _ := pdf.GetPageSize()
	gap := 8.0
	cellW := (w - 2*l.Margin - float64(cols-1)*gap) / float64(cols)
	cellH := 72.0

	x := l.Margin
	y := pdf.GetY() + 6
	pdf.SetFont(l.Font, "", l.BodySize)

	col := 0
	for i, label := range labels {
		pdf.Rect(x, y, cellW, cellH, "D")
		pdf.SetXY(x, y+cellH/2-6)
		pdf.CellFormat(cellW, 12, label, "", 0, "CM", false, 0, "")
		if i < len(anchors) && anchors[i] != 0 {
			pdf.Link(x, y, cellW, cellH, anchors[i])
		}
		col++
		if col == cols {
			col = 0
			x = l.Margin
			y += cellH + gap
		} else {
			x += cellW + gap
		}
	}
	pdf.SetY(y + 8)
}

/************************ Page builders *****************************/
func yearPage(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links) {
	pdf.AddPage()
	ensurePageSize(pdf, cfg.Layout)
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetLink(links.Year, 0, pdf.PageNo())

	setTitle(pdf, cfg.Layout, fmt.Sprintf("%d Bullet Journal", cfg.Year))
	setSubTitle(pdf, cfg.Layout, "Months")

	labels := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	anchors := make([]int, 12)
	for i := 0; i < 12; i++ {
		anchors[i] = links.Months[i]
	}
	gridLinks(pdf, cfg.Layout, 3, labels, anchors)

	if cfg.Layout.ShowCollections {
		setSubTitle(pdf, cfg.Layout, "Collections")
		names := []string{"Writing", "Ideas", "OOO"}
		anchors := []int{
			links.Collections["Writing"],
			links.Collections["Ideas"],
			links.Collections["OOO"],
		}
		gridLinks(pdf, cfg.Layout, 3, names, anchors)
	}
}

func monthPage(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links, month int) {
	pdf.AddPage()
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetLink(links.Months[month-1], 0, pdf.PageNo())

	addNav(pdf, cfg.Layout, links.Year, 0)
	setTitle(pdf, cfg.Layout, fmt.Sprintf("%s %d", time.Month(month), cfg.Year))

	weeks := monthWeeks(cfg.Year, time.Month(month))
	pdf.SetFont(cfg.Layout.Font, "", cfg.Layout.BodySize)
	startY := pdf.GetY()

	weekColWidth := 300.0
	dayColWidth := 60.0

	// List weeks on the left
	for idx, wk := range weeks {
		mon, sun := wk[0], wk[6]
		lbl := fmt.Sprintf("Week %d  (%s – %s)", idx+1, mon.Format("Jan 02"), sun.Format("Jan 02"))
		y := pdf.GetY()
		pdf.CellFormat(weekColWidth, 14, lbl, "", 1, "L", false, 0, "")
		if cfg.Layout.ShowWeeks {
			pdf.Link(cfg.Layout.Margin, y, weekColWidth, 14, links.Weeks[month][idx])
		}
	}

	// Column of days on the right
	daysInMonth := time.Date(cfg.Year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	dayX := cfg.Layout.Margin + weekColWidth + 10
	pdf.SetXY(dayX, startY)
	for d := 1; d <= daysInMonth; d++ {
		y := pdf.GetY()
		label := fmt.Sprintf("%2d", d)
		pdf.CellFormat(dayColWidth, 14, label, "", 0, "L", false, 0, "")
		if cfg.Layout.ShowDays {
			dt := time.Date(cfg.Year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
			key := dt.Format("2006-01-02")
			pdf.Link(dayX, y, dayColWidth, 14, links.Days[key])
		}
		pdf.SetXY(dayX, y+14)
	}
}

func weekPage(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links, month int, weekIdx int, days []time.Time) {
	pdf.AddPage()
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetLink(links.Weeks[month][weekIdx], 0, pdf.PageNo())

	addNav(pdf, cfg.Layout, links.Year, links.Months[month-1])
	setTitle(pdf, cfg.Layout, fmt.Sprintf("%s – Week %d", time.Month(month), weekIdx+1))

	pdf.SetFont(cfg.Layout.Font, "", cfg.Layout.BodySize)
	for _, d := range days {
		txt := d.Format("Mon Jan 02")
		if int(d.Month()) != month {
			pdf.SetTextColor(140, 140, 140)
		} else {
			pdf.SetTextColor(0, 0, 0)
		}
		y := pdf.GetY()
		pdf.CellFormat(0, 14, txt, "", 1, "L", false, 0, "")
		if cfg.Layout.ShowDays && int(d.Month()) == month {
			key := d.Format("2006-01-02")
			pdf.Link(cfg.Layout.Margin, y, 300, 14, links.Days[key])
		}
	}
	pdf.SetTextColor(0, 0, 0)
}

func dayPage(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links, d time.Time, back int) {
	pdf.AddPage()
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	key := d.Format("2006-01-02")
	pdf.SetLink(links.Days[key], 0, pdf.PageNo())

	addNav(pdf, cfg.Layout, links.Year, back)
	setTitle(pdf, cfg.Layout, d.Format("Mon, Jan 02, 2006"))
	dotGrid(pdf, cfg.Layout)
}

func collectionsHub(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links) {
	pdf.AddPage()
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetLink(links.Hubs["Collections"], 0, pdf.PageNo())

	addNav(pdf, cfg.Layout, links.Year, 0)
	setTitle(pdf, cfg.Layout, "Collections")

	names := []string{"Writing", "Ideas", "OOO"}
	anchors := []int{
		links.Collections["Writing"],
		links.Collections["Ideas"],
		links.Collections["OOO"],
	}
	gridLinks(pdf, cfg.Layout, 3, names, anchors)
}

func collectionPage(pdf *fpdf.Fpdf, cfg PlannerConfig, links *Links, name string) {
	pdf.AddPage()
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetLink(links.Collections[name], 0, pdf.PageNo())

	addNav(pdf, cfg.Layout, links.Year, links.Hubs["Collections"])
	setTitle(pdf, cfg.Layout, name)
	dotGrid(pdf, cfg.Layout)
}

/************************ Build *****************************/
func buildPlanner(cfg PlannerConfig) error {
	// Page size selection
	var pdf *fpdf.Fpdf
	switch cfg.Layout.PageSize {
	case "PaperPro":
		// Custom ~7x9.333 in (≈504 x 672 pt). The device scales, but this preserves aspect.
		pdf = fpdf.NewCustom(&fpdf.InitType{
			OrientationStr: "P",
			UnitStr:        "pt",
			Size: fpdf.SizeType{
				Wd: 504, Ht: 672,
			},
		})
	default:
		pdf = fpdf.New("P", "pt", "Letter", "")
	}

	pdf.SetTitle(fmt.Sprintf("Bullet Journal %d", cfg.Year), false)
	pdf.SetAuthor("Planner Generator (Go)", false)
	pdf.SetMargins(cfg.Layout.Margin, cfg.Layout.Margin, cfg.Layout.Margin)
	pdf.SetAutoPageBreak(true, cfg.Layout.Margin)

	links := newLinks()
	links.Year = pdf.AddLink()
	for m := 1; m <= 12; m++ {
		links.Months[m-1] = pdf.AddLink()
		weeks := monthWeeks(cfg.Year, time.Month(m))
		links.Weeks[m] = make([]int, len(weeks))
		for i := range weeks {
			links.Weeks[m][i] = pdf.AddLink()
		}
		for _, wk := range weeks {
			for _, d := range wk {
				if int(d.Month()) == m {
					key := d.Format("2006-01-02")
					if links.Days[key] == 0 {
						links.Days[key] = pdf.AddLink()
					}
				}
			}
		}
	}
	if cfg.Layout.ShowCollections {
		links.Hubs["Collections"] = pdf.AddLink()
		links.Collections["Writing"] = pdf.AddLink()
		links.Collections["Ideas"] = pdf.AddLink()
		links.Collections["OOO"] = pdf.AddLink()
	}

	// Year hub
	yearPage(pdf, cfg, links)

	// Months → Weeks → Days
	for month := 1; month <= 12; month++ {
		monthPage(pdf, cfg, links, month)
		if cfg.Layout.ShowWeeks {
			weeks := monthWeeks(cfg.Year, time.Month(month))
			for idx, wk := range weeks {
				weekPage(pdf, cfg, links, month, idx, wk)
				if cfg.Layout.ShowDays {
					for _, d := range wk {
						if int(d.Month()) != month {
							continue
						}
						dayPage(pdf, cfg, links, d, links.Weeks[month][idx])
					}
				}
			}
		}
	}

	// Collections
	if cfg.Layout.ShowCollections {
		collectionsHub(pdf, cfg, links)
		for _, name := range []string{"Writing", "Ideas", "OOO"} {
			collectionPage(pdf, cfg, links, name)
		}
	}

	return pdf.OutputFileAndClose(cfg.Output)
}

/**************************** CLI ***************************/
func main() {
	var year int
	var out string
	var full bool
	var page string
	var grid float64

	flag.IntVar(&year, "year", time.Now().Year(), "calendar year to generate")
	flag.StringVar(&out, "out", "journal.pdf", "output PDF filename")
	flag.BoolVar(&full, "full", false, "include Weeks and Days (bigger PDF)")
	flag.StringVar(&page, "page", "Letter", "page size: Letter | PaperPro")
	flag.Float64Var(&grid, "grid", 22, "dot-grid spacing in points")
	flag.Parse()

	layout := Layout{
		PageSize:        page,
		Margin:          36, // 0.5"
		Font:            "Helvetica",
		TitleSize:       24,
		SubTitleSize:    14,
		BodySize:        12,
		GridSpacingPt:   grid,
		ShowWeeks:       full,
		ShowDays:        full,
		ShowCollections: true,
	}

	cfg := PlannerConfig{
		Year:   year,
		Output: out,
		Layout: layout,
	}
	if err := buildPlanner(cfg); err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %s for %d\n", out, year)
}
