package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config holds every external integration secret + tunable the app needs.
// Sandbox/live is a per-adapter env flip (TDD §4.1): each external service has
// its own `*_ENV` ("sandbox"|"live"); mock impls are selected in code when the
// live service isn't onboarded yet. Never commit real secrets — keep them in
// the gitignored .env (see .env.example for the full key list).
type Config struct {
	// --- Server ---
	ServerPort   string `mapstructure:"SERVER_PORT"`
	ClientOrigin string `mapstructure:"CLIENT_ORIGIN"`
	SiteBaseURL  string `mapstructure:"SITE_BASE_URL"` // canonical public site origin for SEO (sitemap <loc>, robots Sitemap:). The TanStack Start SSR site (TDD §2.2).
	JWTSecret    string `mapstructure:"JWT_SECRET"`
	AdminEmail   string `mapstructure:"ADMIN_EMAIL"` // inbox for admin notifications

	// --- Database (PostgreSQL) ---
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// --- Redis (sessions, cache, Asynq queues, pub/sub) ---
	// Single URL encodes host/port/db/password, e.g. redis://:pass@localhost:6379/0
	RedisURL string `mapstructure:"REDIS_URL"`

	// --- Consent HMAC key (GDPR) — rotates daily in prod (TDD §11); static for MVP ---
	ConsentHMACKey string `mapstructure:"CONSENT_HMAC_KEY"`

	// --- 2FA TOTP-secret encryption key (TDD §5.3) ---
	// The TOTP secret is AES-GCM-encrypted at rest with this app key; it never
	// lives in the DB in plaintext, so a DB dump alone cannot recover secrets.
	TwoFAEncryptionKey string `mapstructure:"TWO_FA_ENCRYPTION_KEY"`

	// --- Google OAuth ---
	GoogleOAuthClientID     string `mapstructure:"GOOGLE_OAUTH_CLIENT_ID"`
	GoogleOAuthClientSecret string `mapstructure:"GOOGLE_OAUTH_CLIENT_SECRET"`
	GoogleOAuthRedirectURL  string `mapstructure:"GOOGLE_OAUTH_REDIRECT_URL"`

	// --- Brevo (transactional email) — replaces AWS SES (TDD §10) ---
	BrevoAPIKey      string `mapstructure:"BREVO_API_KEY"`
	BrevoSenderEmail string `mapstructure:"BREVO_SENDER_EMAIL"` // verified sender
	BrevoSenderName  string `mapstructure:"BREVO_SENDER_NAME"`

	// --- Airwallex (card payments; settles to CNY) — sandbox until onboarding (PRD §7) ---
	AirwallexClientID      string `mapstructure:"AIRWALLEX_CLIENT_ID"`
	AirwallexAPIKey        string `mapstructure:"AIRWALLEX_API_KEY"`
	AirwallexEnv           string `mapstructure:"AIRWALLEX_ENV"` // "sandbox" | "live"
	AirwallexWebhookSecret string `mapstructure:"AIRWALLEX_WEBHOOK_SECRET"`

	// --- PayPal (alternative payment) — sandbox until onboarding (PRD §7) ---
	PayPalClientID     string `mapstructure:"PAYPAL_CLIENT_ID"`
	PayPalClientSecret string `mapstructure:"PAYPAL_CLIENT_SECRET"`
	PayPalEnv          string `mapstructure:"PAYPAL_ENV"` // "sandbox" | "live"
	PayPalWebhookID    string `mapstructure:"PAYPAL_WEBHOOK_ID"`

	// --- Qwen (LLM for the ceramic-art assistant chat) ---
	QwenAPIKey  string `mapstructure:"QWEN_API_KEY"`
	QwenBaseURL string `mapstructure:"QWEN_BASE_URL"` // DashScope endpoint
	QwenModel   string `mapstructure:"QWEN_MODEL"`    // e.g. qwen-plus

	// --- Alibaba Cloud OSS (HK) — object storage + on-the-fly image processing (TDD §2.1) ---
	OSSEndpoint        string `mapstructure:"OSS_ENDPOINT"` // e.g. oss-cn-hongkong.aliyuncs.com
	OSSAccessKeyID     string `mapstructure:"OSS_ACCESS_KEY_ID"`
	OSSAccessKeySecret string `mapstructure:"OSS_ACCESS_KEY_SECRET"`
	OSSBucket          string `mapstructure:"OSS_BUCKET"`
	OSSRegion          string `mapstructure:"OSS_REGION"` // e.g. cn-hongkong

	// --- Object-storage adapter (TDD §4.1/§2.1) ---
	// STORAGE_MODE=local (dev default): LocalStore writes to a local dir,
	// served via a Fiber static mount at STORAGE_PUBLIC_BASE_URL. "oss": live
	// Alibaba Cloud OSS presign + CDN (NOT live-tested until creds land). The
	// media module depends on the Store interface, so swapping is an env flip.
	StorageMode          string `mapstructure:"STORAGE_MODE"`            // local | oss
	StorageLocalDir      string `mapstructure:"STORAGE_LOCAL_DIR"`       // e.g. ./_media
	StoragePublicBaseURL string `mapstructure:"STORAGE_PUBLIC_BASE_URL"` // e.g. /media or https://cdn...com
	OSSPublicBaseURL     string `mapstructure:"OSS_PUBLIC_BASE_URL"`     // CDN domain override (empty = bucket URL)

	// --- PDF adapter (TDD §12: chromedp HTML→PDF) ---
	// PDF_MODE=local (dev default): NoopGenerator returns ErrPDFUnavailable so the
	//   worker skips storage + pdf_key stays NULL (the download endpoint 404s).
	//   Dev + tests never need the sidecar. "chromedp": ChromedpGenerator connects
	//   to a headless-shell sidecar at CHROMEDP_URL (ws://chromedp:9222) over the
	//   DevTools remote protocol. PDF_BASE_URL is the origin the sidecar fetches
	//   <img> assets from (e.g. the QR at GET /certificates/:code/qr), reachable
	//   over the compose network — http://api:<port> inside compose.
	PDFMode     string `mapstructure:"PDF_MODE"`     // local | chromedp
	ChromedpURL string `mapstructure:"CHROMEDP_URL"` // ws://chromedp:9222
	PDFBaseURL  string `mapstructure:"PDF_BASE_URL"` // http://api:1323 (asset origin)

	// --- FX pipeline (TDD §7, PRD §3.2.3) ---
	// ECB_API_URL empty => fixture rates in dev (TDD §4.1). FX_MARKUP_BPS is
	// basis points (200 = 2%); will move to a CMS settings table post-MVP.
	ECB_API_URL string `mapstructure:"ECB_API_URL"`
	FXMarkupBPS int    `mapstructure:"FX_MARKUP_BPS"`

	// --- Payments (TDD §4.1) ---
	// PAYMENTS_MODE=mock (dev default): checkout enqueues payment:finalize
	// {success} to drive created→paid without a real gateway. "live" requires
	// the Airwallex/PayPal adapters (#6) + sandbox creds; until then checkout
	// in live mode returns an error so no order dangles in `created`.
	PaymentsMode string `mapstructure:"PAYMENTS_MODE"`

	// --- In-house analytics (TDD §3.4/§4.2, PRD §3.4.2) ---
	// ANALYTICS_HMAC_KEY seeds the daily-rotating visitor_hash key
	// (HMAC-SHA256(appKey, YYYY-MM-DD)) so the same visitor within a day
	// collides but cannot be tracked across days (GDPR-friendly). Separate
	// from CONSENT_HMAC_KEY so a consent-db leak cannot forge analytics.
	AnalyticsHMACKey string `mapstructure:"ANALYTICS_HMAC_KEY"`
	// GEOIP_MODE=noop (dev default): NoopLookup → country 'ZZ' (no .mmdb
	//   needed). "maxmind": MaxMindLookup reads a local GeoLite2-Country db
	//   at GEOLITE2_DB_PATH. Unknown/private IP → 'ZZ' (TDD §10/§11). The
	//   City db (region-level) is a later schema change; MVP stores CHAR(2).
	GeoIPMode      string `mapstructure:"GEOIP_MODE"`       // noop | maxmind
	GeoLite2DBPath string `mapstructure:"GEOLITE2_DB_PATH"` // path to .mmdb

	// --- Global rate-limit backstop (TDD §333: 100/min/IP in prod) ---
	// RATE_LIMIT_MAX=0 disables the global limiter (for local load testing).
	// Default 100 when unset (see cmd/api/main.go).
	RateLimitMax int `mapstructure:"RATE_LIMIT_MAX"`
}

func LoadConfig(path string) (config Config, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName(".env") // Name of config file (without extension)
	viper.SetConfigType("env")  // Or "dotenv" or "json", "yaml" etc.

	viper.AutomaticEnv() // Read in environment variables that match

	err = viper.ReadInConfig() // Find and read the config file
	if err != nil {
		// Handle errors reading the config file, but allow it if it's just "not found"
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No .env file found.")
		} else {
			return
		}
	}

	err = viper.Unmarshal(&config)
	return
}

// initDefaults sets env-var defaults that are not secrets. Called from init()
// so they apply before Unmarshal (env vars / .env still override).
func init() {
	viper.SetDefault("ECB_API_URL", "")       // empty => fixture rates in dev
	viper.SetDefault("FX_MARKUP_BPS", 200)    // 200 bps = 2% (PRD §3.2.3 default)
	viper.SetDefault("PAYMENTS_MODE", "mock") // dev: mock payment finalize seam

	// Storage adapter defaults (TDD §4.1). Local dev = local-dir store;
	// flip to "oss" when merchant OSS creds land.
	viper.SetDefault("STORAGE_MODE", "local")
	viper.SetDefault("STORAGE_LOCAL_DIR", "./_media")
	viper.SetDefault("STORAGE_PUBLIC_BASE_URL", "/media")

	// PDF adapter (TDD §12). Dev default = local (noop) so the sidecar isn't
	// required to run the app or tests.
	viper.SetDefault("PDF_MODE", "local")
	viper.SetDefault("OSS_PUBLIC_BASE_URL", "") // empty = bucket URL

	// GeoIP adapter (TDD §10/§11). Dev default = noop so no MaxMind download
	// is needed to run the app or tests; flip to "maxmind" with a local
	// GeoLite2-Country .mmdb when ops provisions the MaxMind account.
	viper.SetDefault("GEOIP_MODE", "noop")
	viper.SetDefault("GEOLITE2_DB_PATH", "")

	// Client origin (CORS + OAuth redirects + payment return URL).
	// Defaults to the TanStack Start dev server (port 3000).
	viper.SetDefault("CLIENT_ORIGIN", "http://localhost:3000")
}
