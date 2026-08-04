// Package pdftmpl renders the HTML documents the chromedp adapter prints to
// PDF (TDD §12). Templates live here, not in the pdf adapter, so the adapter
// stays generic across certificate/itinerary/quote (one engine, many docs).
// Each template is self-contained (inline CSS; any <img> fetched over the
// network at render time, so the chromedp sidecar must reach the asset URL).
package pdftmpl

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"jingdezhen-ceramics-backend/internal/models"
)

//go:embed *.html.tmpl
var tmplFS embed.FS

// CertificateData is the template input for the certificate PDF.
type CertificateData struct {
	Code         string // JDZ-<6-base32>
	ProductTitle string
	ArtistName   string // "" if none
	IssuedAt     time.Time
	PublicURL    string // public cert page URL (the QR target + footer link)
	QRURL        string // GET /certificates/:code/qr (rendered as <img>)
	Locale       string // BCP 47 for date formatting
}

// RenderCertificate renders the certificate PDF HTML from a Certificate + the
// public/QR URLs. The caller (worker) passes the cert (locale-joined) + the
// base URL the chromedp sidecar can reach (PDF_BASE_URL, e.g. http://api:1323).
func RenderCertificate(cert *models.Certificate, baseURL, qrURL, locale string) (string, error) {
	if cert == nil {
		return "", fmt.Errorf("pdftmpl.RenderCertificate: nil cert")
	}
	tmpl, err := template.ParseFS(tmplFS, "certificate.html.tmpl")
	if err != nil {
		return "", fmt.Errorf("pdftmpl.RenderCertificate.Parse: %w", err)
	}
	artist := ""
	if cert.ArtistName != nil {
		artist = *cert.ArtistName
	}
	data := CertificateData{
		Code:         cert.CertCode,
		ProductTitle: cert.ProductTitle,
		ArtistName:   artist,
		IssuedAt:     cert.IssuedAt,
		PublicURL:    baseURL + "/certificates/" + cert.CertCode,
		QRURL:        qrURL,
		Locale:       locale,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("pdftmpl.RenderCertificate.Execute: %w", err)
	}
	return buf.String(), nil
}
// ItineraryQuoteData is the template input for the itinerary quote PDF
// (PRD §3.3.2 line 269: "day-by-day plan, included services, pricing summary;
// branded template, in the customer's locale"). The worker loads the request
// (trip details) + the quote (priced line items + FX-snapshotted totals) and
// passes both here.
type ItineraryQuoteData struct {
	RequestID        string
	Locale           string
	SentAt           string // pre-formatted (locale-aware date)
	QuoteStatus      string
	// Trip details (from the request)
	ArrivalDate      string
	DurationDays     int
	Flexible         bool
	Adults           int
	Children         int
	Pace             string
	Interests        string
	ServicesSummary  string
	Notes            string
	// Priced line items (from the quote JSONB)
	LineItems        []quoteLineDisplay
	// Totals
	TotalCNYMajor    string // ¥ fen → yuan
	Currency         string
	CurrencySymbol   string
	TotalMinorMajor  string
	DepositMinorMajor string
	BalanceMinorMajor string
	FxRateUsed       string
}

// quoteLineDisplay is one row of the priced line-items table.
type quoteLineDisplay struct {
	Label       string
	Qty         int
	Unit        string
	RateCNYMajor string // fen → yuan
	LineCNYMajor string
}

// currencySymbol returns the common glyph for the 3 presentment currencies
// (USD/EUR/GBP). Falls back to the code for anything else.
func currencySymbol(code string) string {
	switch code {
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	default:
		return code
	}
}

// minorToMajor renders a minor-units int64 as a major-unit decimal string
// (e.g. 11974 → "119.74"). CNY values are suffixed with ¥ by the caller via
// the template; this returns the bare number.
func minorToMajor(minor int64) string {
	abs := minor
	neg := ""
	if abs < 0 {
		neg = "-"
		abs = -abs
	}
	whole := abs / 100
	cents := abs % 100
	return fmt.Sprintf("%s%d.%02d", neg, whole, cents)
}

// RenderItineraryQuote renders the itinerary quote PDF HTML from the request
// (trip details) + the quote (priced line items + totals). The caller (worker)
// loads both via the itinerary repository.
func RenderItineraryQuote(req *models.ItineraryAdminRow, quote *models.ItineraryQuote, locale string) (string, error) {
	if req == nil || quote == nil {
		return "", fmt.Errorf("pdftmpl.RenderItineraryQuote: nil request or quote")
	}
	tmpl, err := template.ParseFS(tmplFS, "itinerary_quote.html.tmpl")
	if err != nil {
		return "", fmt.Errorf("pdftmpl.RenderItineraryQuote.Parse: %w", err)
	}

	// Parse the line_items JSONB → []QuoteLineItem for the table.
	var lines []models.QuoteLineItem
	if len(quote.LineItems) > 0 {
		if err := json.Unmarshal(quote.LineItems, &lines); err != nil {
			return "", fmt.Errorf("pdftmpl.RenderItineraryQuote.Unmarshal: %w", err)
		}
	}
	lineDisplay := make([]quoteLineDisplay, len(lines))
	for i, l := range lines {
		lineDisplay[i] = quoteLineDisplay{
			Label: l.Label, Qty: l.Qty, Unit: humanUnit(l.Unit),
			RateCNYMajor: minorToMajor(l.RateCNY),
			LineCNYMajor: minorToMajor(l.LineCNY),
		}
	}

	// Arrival date (YYYY-MM-DD or "Flexible").
	arrival := "Flexible"
	if req.ArrivalDate != nil {
		arrival = req.ArrivalDate.Format("2 January 2006")
	}

	// Interests (JSON array → comma-joined).
	interests := joinJSONStrings(req.Interests)

	// Services summary (guide/hotel/pickup/experience from JSON).
	services := summarizeServices(req.Services)

	// Notes: prefer the top-level Notes, fall back to contact notes.
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}

	fx := ""
	if quote.FxRateUsed != nil {
		fx = *quote.FxRateUsed
	}

	loc := locale
	if loc == "" {
		loc = "en-US"
	}

	balance := quote.TotalMinor - quote.DepositMinor

	data := ItineraryQuoteData{
		RequestID:         strconvI64(req.ID),
		Locale:            loc,
		SentAt:            quote.SentAt.Format("2 January 2006"),
		QuoteStatus:       string(quote.Status),
		ArrivalDate:       arrival,
		DurationDays:      req.DurationDays,
		Flexible:          req.Flexible,
		Adults:            req.Adults,
		Children:          req.Children,
		Pace:              req.Pace,
		Interests:         interests,
		ServicesSummary:   services,
		Notes:             notes,
		LineItems:         lineDisplay,
		TotalCNYMajor:     minorToMajor(quote.TotalCNY),
		Currency:          quote.Currency,
		CurrencySymbol:    currencySymbol(quote.Currency),
		TotalMinorMajor:   minorToMajor(quote.TotalMinor),
		DepositMinorMajor: minorToMajor(quote.DepositMinor),
		BalanceMinorMajor: minorToMajor(balance),
		FxRateUsed:        fx,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("pdftmpl.RenderItineraryQuote.Execute: %w", err)
	}
	return buf.String(), nil
}

// humanUnit renders a per_person/per_day/flat code for the template.
func humanUnit(u string) string {
	switch u {
	case "per_person":
		return "per person"
	case "per_day":
		return "per day"
	case "flat":
		return "flat"
	default:
		return u
	}
}

// joinJSONStrings unmarshals a JSON array of strings → comma-joined display.
func joinJSONStrings(raw json.RawMessage) string {
	var ss []string
	if len(raw) == 0 {
		return ""
	}
	if err := json.Unmarshal(raw, &ss); err != nil {
		return ""
	}
	return strings.Join(ss, ", ")
}

// summarizeServices renders the services JSONB (guide/hotel/pickup/experience)
// as a short human-readable summary.
func summarizeServices(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var svc models.ServicesInput
	if err := json.Unmarshal(raw, &svc); err != nil {
		return ""
	}
	var parts []string
	if svc.Guide != "" && svc.Guide != "none" {
		parts = append(parts, "Guide ("+svc.Guide+")")
	}
	if svc.Hotel {
		level := svc.HotelLevel
		if level == "" {
			level = "standard"
		}
		parts = append(parts, "Hotel ("+level+")")
	}
	if svc.Pickup {
		parts = append(parts, "Airport pickup")
	}
	if svc.Experience {
		parts = append(parts, "Studio experience")
	}
	if svc.DietaryAccessibility != "" {
		parts = append(parts, svc.DietaryAccessibility)
	}
	if len(parts) == 0 {
		return "None specified"
	}
	return strings.Join(parts, " · ")
}

// strconvI64 formats an int64 without importing strconv at the top (kept local
// to avoid widening the import block for one call).
func strconvI64(n int64) string {
	return fmt.Sprintf("%d", n)
}
