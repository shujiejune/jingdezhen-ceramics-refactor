package sitemap

import (
	"log"
	"strings"

	"jingdezhen-ceramics-backend/internal/config"

	"github.com/gofiber/fiber/v2"
)

// Handler serves the public SEO endpoints: GET /sitemap.xml (always fresh,
// rebuild-on-read) and GET /robots.txt (static body with the Sitemap: line).
// Both are PUBLIC — no auth — so crawlers can fetch them (PRD §4.4).
type Handler struct {
	builder     *Builder
	siteBaseURL string
}

// NewHandler constructs a Handler. cfg.SiteBaseURL is used for robots.txt's
// Sitemap: line (the builder has its own copy for <loc>).
func NewHandler(builder *Builder, siteBaseURL string) *Handler {
	return &Handler{builder: builder, siteBaseURL: siteBaseURL}
}

// SitemapXML returns the sitemap built on-read (always current). 4 small
// SELECTs at MVP volume — a cache can be added if a crawler hammers this.
//
//	@Summary		Sitemap (multi-locale, hreflang)
//	@Description	Returns the sitemap.xml with one <url> per published (entity,
//	@Description	locale, slug) and xhtml:link hreflang alternates. Rebuilt on
//	@Description	read; also refreshed by the sitemap:rebuild job on publish.
//	@Tags			seo
//	@Produce		xml
//	@Success		200	{string}	string	"application/xml"
//	@Router			/sitemap.xml [get]
func (h *Handler) SitemapXML(c *fiber.Ctx) error {
	xmlBytes, err := h.builder.BuildXML(c.Context())
	if err != nil {
		log.Printf("Handler.SitemapXML.BuildXML: %v", err)
		// Empty/invalid SITE_BASE_URL is a config error — return 503 so a
		// monitor catches it; the sitemap is absent until fixed.
		return c.Status(fiber.StatusServiceUnavailable).SendString("sitemap unavailable: " + err.Error())
	}
	c.Set("Content-Type", "application/xml; charset=utf-8")
	return c.Send(xmlBytes)
}

// RobotsTXT returns a static robots.txt: allow all public content, disallow
// /admin (CMS) + /api (non-SEO API surface) + the auth paths, and point at
// the sitemap. PRD §4.4.
//
//	@Summary		robots.txt
//	@Description	Allow public content; disallow /admin + /api + /auth; Sitemap:
//	@Description	line points at /sitemap.xml.
//	@Tags			seo
//	@Produce		plain
//	@Success		200	{string}	string	"text/plain"
//	@Router			/robots.txt [get]
func (h *Handler) RobotsTXT(c *fiber.Ctx) error {
	base := strings.TrimRight(h.siteBaseURL, "/")
	body := "User-agent: *\n" +
		"Allow: /\n" +
		"Disallow: /admin\n" +
		"Disallow: /api\n" +
		"Disallow: /auth\n" +
		"Disallow: /profile\n" +
		"\n" +
		"Sitemap: " + base + "/sitemap.xml\n"
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(body)
}

// NewHandlerFromConfig is a convenience for wiring; returns nil if the
// builder is nil (e.g. a worker that doesn't serve routes).
func NewHandlerFromConfig(builder *Builder, cfg config.Config) *Handler {
	if builder == nil {
		return nil
	}
	return NewHandler(builder, cfg.SiteBaseURL)
}
