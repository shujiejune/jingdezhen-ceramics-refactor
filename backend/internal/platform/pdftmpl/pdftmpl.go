// Package pdftmpl renders the HTML documents the chromedp adapter prints to
// PDF (TDD §12). Templates live here, not in the pdf adapter, so the adapter
// stays generic across certificate/itinerary/quote (one engine, many docs).
// Each template is self-contained (inline CSS; any <img> fetched over the
// network at render time, so the chromedp sidecar must reach the asset URL).
package pdftmpl

import (
	"bytes"
	"embed"
	"fmt"
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