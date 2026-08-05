// Package main is the entrypoint for the Jingdezhen Ceramics Platform API
// (serve mode). It wires config, the Fiber router, the WebSocket hub, and the
// Asynq worker.
//
// API documentation (OpenAPI/Swagger 2.0):
//
//	swag init -g cmd/api/main.go -o internal/docs --parseDependency --parseInternal
//	browse to /admin/swagger/index.html (admin-only, dev-gated)
//
// @title        Jingdezhen Ceramics Platform API
// @version      1.0
// @description  Internationalized culture / e-commerce / custom-travel platform for Jingdezhen ceramic art.
// @description  Backend in Go + Fiber; money as BIGINT minor units (CNY base, USD/EUR/GBP presentment).
// @termsOfService  https://jingdezhen.test/terms
// @contact.name   Jingdezhen Ceramics Platform Team
// @license.name   Apache 2.0
// @license.url    http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath       /
// @securityDefinitions.apikey BearerAuth
// @in   header
// @name Authorization
// @description   "Bearer <access_token>" — JWT access token from POST /auth/login (or OAuth). Required for all /admin/* and customer-authed endpoints; omitted for public /catalog/* reads.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"jingdezhen-ceramics-backend/internal/api"
	"jingdezhen-ceramics-backend/internal/config"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/address"
	"jingdezhen-ceramics-backend/internal/modules/analytics"
	"jingdezhen-ceramics-backend/internal/modules/artist"
	"jingdezhen-ceramics-backend/internal/modules/audit"
	"jingdezhen-ceramics-backend/internal/modules/cart"
	"jingdezhen-ceramics-backend/internal/modules/ceramicstory"
	"jingdezhen-ceramics-backend/internal/modules/certificate"
	"jingdezhen-ceramics-backend/internal/modules/consent"
	"jingdezhen-ceramics-backend/internal/modules/engage"
	"jingdezhen-ceramics-backend/internal/modules/itinerary"
	"jingdezhen-ceramics-backend/internal/modules/media"
	"jingdezhen-ceramics-backend/internal/modules/notification"
	"jingdezhen-ceramics-backend/internal/modules/order"
	"jingdezhen-ceramics-backend/internal/modules/payment"
	"jingdezhen-ceramics-backend/internal/modules/privacy"
	"jingdezhen-ceramics-backend/internal/modules/product"
	"jingdezhen-ceramics-backend/internal/modules/shipping"
	"jingdezhen-ceramics-backend/internal/modules/twofa"
	"jingdezhen-ceramics-backend/internal/modules/user"
	"jingdezhen-ceramics-backend/internal/modules/wishlist"
	"jingdezhen-ceramics-backend/internal/platform/fx"
	"jingdezhen-ceramics-backend/internal/platform/jobs"
	"jingdezhen-ceramics-backend/internal/platform/pdftmpl"
	platformredis "jingdezhen-ceramics-backend/internal/platform/redis"
	"jingdezhen-ceramics-backend/internal/ws"
	"jingdezhen-ceramics-backend/pkg/adapters/certchain"
	"jingdezhen-ceramics-backend/pkg/adapters/geoip"
	"jingdezhen-ceramics-backend/pkg/adapters/payments"
	"jingdezhen-ceramics-backend/pkg/adapters/pdf"
	"jingdezhen-ceramics-backend/pkg/adapters/storage"
	"jingdezhen-ceramics-backend/pkg/adapters/tokenblocklist"
	"jingdezhen-ceramics-backend/pkg/email"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// One Go binary, two run modes (TDD §2.1): `serve` (API + WS) or `worker`
	// (Asynq jobs). Compose runs both from the same image.
	mode := "serve"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	// --- Shared cancellable context for the entire app lifecycle ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch mode {
	case "serve":
		runServe(ctx, cfg)
	case "worker":
		runWorker(ctx, cfg)
	default:
		log.Fatalf("unknown mode %q (want 'serve' or 'worker')", mode)
	}
}

// --- helpers ----------------------------------------------------------------

// shutdownCtx returns a context cancelled by SIGINT/SIGTERM. Used by both
// serve (Fiber) and worker (Asynq) modes for graceful shutdown.
func shutdownCtx() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Shutdown signal received...")
		cancel()
	}()
	return ctx, cancel
}

// redisAddrFromURL extracts the host:port from a redis://... URL, which is
// what Asynq's RedisClientOpt wants (go-redis takes the full URL).
func redisAddrFromURL(redisURL string) (string, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return "", err
	}
	if u.Port() == "" {
		return u.Host + ":6379", nil // default port
	}
	return u.Host, nil
}

// --- Adapters: wire the jobs.Client to the order service's narrow interfaces ---
// The order module defines its own interfaces (EmailEnqueuer, PaymentEnqueuer)
// to stay free of an internal/platform/jobs import; these adapters satisfy
// them with the Asynq client.

type paymentEnqueuer struct{ c *jobs.Client }

func (p paymentEnqueuer) EnqueuePaymentFinalize(ctx context.Context, orderID int64, success bool, gateway, gatewayRef string) error {
	return p.c.EnqueuePaymentFinalize(ctx, jobs.PaymentFinalizePayload{
		OrderID: orderID, Success: success, Gateway: gateway, GatewayRef: gatewayRef,
	})
}

// EnqueueItineraryDepositFinalize enqueues the deposit-finalize variant (the
// webhook/mock-seam drives itinerary quote sent→deposit_paid via the worker).
func (p paymentEnqueuer) EnqueueItineraryDepositFinalize(ctx context.Context, quoteID int64, success bool, gateway, gatewayRef string) error {
	return p.c.EnqueuePaymentFinalize(ctx, jobs.PaymentFinalizePayload{
		ItineraryQuoteID: quoteID, Success: success, Gateway: gateway, GatewayRef: gatewayRef,
	})
}

type orderEmailEnqueuer struct{ c *jobs.Client }

func (e orderEmailEnqueuer) EnqueueEmailSend(ctx context.Context, to, subject, plainText, html string) error {
	return e.c.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To: to, Subject: subject, PlainText: plainText, HTML: html,
	})
}

// orderStockCheckEnqueuer adapts jobs.Client → order.StockCheckEnqueuer
// (serve mode enqueues the job; the worker handles it).
type orderStockCheckEnqueuer struct{ c *jobs.Client }

func (e orderStockCheckEnqueuer) EnqueueStockCheck(ctx context.Context, orderID int64, skuIDs []int64) error {
	return e.c.EnqueueStockCheck(ctx, jobs.StockCheckPayload{OrderID: orderID, SkuIDs: skuIDs})
}

// itinConsentRecorder adapts consent.Service → itinerary.ConsentRecorder.
// Records the Privacy Policy consent at itinerary submit (PRD §3.3.2 step 4)
// for the GDPR audit trail. clientIP is empty (server-side submit; the IP hash
// is optional — the consent is recorded as granted-by-submit, not by-request).
type itinConsentRecorder struct{ s consent.ServiceInterface }

func (r itinConsentRecorder) RecordConsent(ctx context.Context, userID string, kind models.ConsentKind) error {
	uid := userID
	_, err := r.s.RecordConsent(ctx, &uid, "", models.RecordConsentRequest{
		Kind: kind, DocVersion: "v1", Granted: true,
	})
	return err
}

// pdfEnqueuer adapts jobs.Client → the PDFEnqueuer interface (cert +
// itinerary both implement it). Serve mode enqueues pdf:generate at cert
// issue/regenerate + itinerary quote send; the worker renders.
type pdfEnqueuer struct{ c *jobs.Client }

func (e pdfEnqueuer) EnqueuePDFGenerate(ctx context.Context, kind string, entityID int64, locale string) error {
	return e.c.EnqueuePDFGenerate(ctx, jobs.PDFGeneratePayload{Kind: kind, EntityID: entityID, Locale: locale})
}

// --- serve mode (API + WebSocket) --------------------------------------------

func runServe(rootCtx context.Context, cfg config.Config) {
	// Derive a child context so we can stop the WS Hub independently of the
	// root context during graceful shutdown.
	ctx, hubCancel := context.WithCancel(rootCtx)
	defer hubCancel()

	app := fiber.New()
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.ClientOrigin + ", http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowCredentials: true,
	}))

	// --- Database connection ---
	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to parse database configuration: %v\n", err)
	}
	dbPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}
	log.Println("Successfully connected to the database!")

	// --- Redis client (sessions, cache, pub/sub) ---
	redisClient, err := platformredis.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Successfully connected to Redis!")
	_ = redisClient // consumed by cache/session services as they land

	// --- Token blocklist (TDD §5.1 stopgap) ---
	// First auth-path Redis consumer: denylist of deleted users whose
	// outstanding JWT must be invalidated before the token's own expiry. Fail-open
	// on Redis outage (see pkg/adapters/tokenblocklist). The worker has no auth
	// middleware, so it gets the Noop blocklist.
	tokenBlocklist := tokenblocklist.NewRedisBlocklist(redisClient)

	// --- Asynq enqueue client (so handlers can defer flaky/heavy work) ---
	redisAddr, err := redisAddrFromURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}
	jobClient := jobs.NewClient(redisAddr)
	defer jobClient.Close()
	_ = jobClient // injected into services as they need to enqueue jobs

	// --- WebSocket Hub ---
	wsHub := ws.NewHub()
	go wsHub.Run(ctx)
	wsHandler := ws.NewHandler(wsHub)

	// --- Google OAuth config ---
	googleOAuthConfig := &oauth2.Config{
		RedirectURL:  cfg.GoogleOAuthRedirectURL,
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// --- Email: Brevo (replaces AWS SES, TDD §10). Empty key => no-op sender ---
	// In serve mode the emailer is not used directly — emails are enqueued via
	// jobClient and sent by the worker. We still build the template manager here
	// because email rendering happens at enqueue time.
	templateManager, err := email.NewTemplateManager()
	if err != nil {
		log.Fatalf("Failed to parse email templates: %v", err)
	}

	// --- 2FA (TOTP) — TDD §5.3, PRD §4.3 ---
	// The TOTP secret is encrypted at rest with an app key (TWO_FA_ENCRYPTION_KEY).
	// The 2FA service is also passed to the user service as a login-gating checker.
	twoFARepo := twofa.NewRepository(dbPool)
	twoFAService := twofa.NewService(twoFARepo, []byte(cfg.TwoFAEncryptionKey), cfg.JWTSecret, "Jingdezhen Ceramics")
	twoFAHandler := twofa.NewHandler(twoFAService)

	// --- Dependency injection ---
	userRepo := user.NewRepository(dbPool)
	userService := user.NewService(
		userRepo, jobClient, templateManager,
		cfg.JWTSecret, cfg.ClientOrigin, cfg.AdminEmail, googleOAuthConfig,
		twoFAService,
	)
	userHandler := user.NewHandler(userService)

	notifRepo := notification.NewRepository(dbPool)
	notifService := notification.NewService(notifRepo, userRepo, wsHub)
	notifHandler := notification.NewHandler(notifService)

	ceramicStoryRepo := ceramicstory.NewRepository(dbPool)
	ceramicStoryService := ceramicstory.NewService(ceramicStoryRepo)
	ceramicStoryHandler := ceramicstory.NewHandler(ceramicStoryService)

	wishlistRepo := wishlist.NewRepository(dbPool)
	wishlistService := wishlist.NewService(wishlistRepo)
	wishlistHandler := wishlist.NewHandler(wishlistService)

	// --- Cart (PRD §3.2.3, TDD §3.4) ---
	// One server-side cart per signed-in user; guest carts merge on login.
	cartRepo := cart.NewRepository(dbPool)
	cartService := cart.NewService(cartRepo)
	cartHandler := cart.NewHandler(cartService)

	artistRepo := artist.NewRepository(dbPool)
	artistService := artist.NewService(artistRepo)
	artistHandler := artist.NewHandler(artistService)

	productRepo := product.NewRepository(dbPool)
	productService := product.NewService(productRepo)
	productHandler := product.NewHandler(productService)

	// --- FX pipeline (TDD §7) ---
	// Empty ECB_API_URL => fixture rates in dev (no network call). The worker
	// owns Refresh; serve mode owns Convert (read-time display conversion).
	var fxRateSource fx.RateSource
	if cfg.ECB_API_URL != "" {
		fxRateSource = fx.NewECBClient(cfg.ECB_API_URL)
	} else {
		fxRateSource = fx.FixtureRateSource{Rates: fx.DefaultFixtureRates()}
	}
	fxRepo := fx.NewRepository(dbPool)
	fxService := fx.NewService(fxRepo, fxRateSource, cfg.FXMarkupBPS)
	fxHandler := fx.NewHandler(fxService, jobClient)
	productHandler.SetPriceConverter(fxService)
	cartHandler.SetPriceConverter(fxService)

	engageRepo := engage.NewRepository(dbPool)
	engageService := engage.NewService(engageRepo)
	engageHandler := engage.NewHandler(engageService)

	addressRepo := address.NewRepository(dbPool)
	addressService := address.NewService(addressRepo)
	addressHandler := address.NewHandler(addressService)

	// --- Shipping fee tiers (PRD §3.2.3) ---
	shippingRepo := shipping.NewRepository(dbPool)
	shippingService := shipping.NewService(shippingRepo)
	shippingHandler := shipping.NewHandler(shippingService)

	// --- Orders + checkout (PRD §3.2.3, TDD §7/§8) ---
	// Mock payment seam: in PAYMENTS_MODE=mock (dev default), checkout enqueues
	// payment:finalize{success} to drive created→paid. Live mode lands in #6.
	orderRepo := order.NewRepository(dbPool)
	orderService := order.NewService(
		orderRepo,
		cartService,                   // CartFetcher
		cartService,                   // CartClearer (BulkRemove)
		addressService,                // AddressFetcher (GetAddress)
		shippingService,               // ShippingCalcer (TiersForCountry)
		fxService,                     // CheckoutFX (Convert + Rate)
		orderEmailEnqueuer{jobClient}, // EmailEnqueuer
		paymentEnqueuer{jobClient},    // PaymentEnqueuer
		userService,                   // UserPrefFetcher (PreferredCurrency)
		nil,                           // PaymentIntenter (wired after payment.Service below)
		nil,                           // PaymentRefunder (wired after payment.Service below)
		userService,                   // UserFetcher (GetUserProfile for the email)
		cfg.PaymentsMode,
	)

	// --- Payments (TDD §10, PRD §3.2.3) ---
	// Gateway registry: mock → MockGateway for all names; sandbox/live → real
	// Airwallex + PayPal HTTP clients. The webhook handler resolves by name.
	var gatewayRegistry *payment.Registry
	switch cfg.PaymentsMode {
	case "sandbox", "live":
		gatewayRegistry = payment.NewRegistry(
			payments.NewAirwallexGateway(cfg.AirwallexClientID, cfg.AirwallexAPIKey, cfg.AirwallexEnv, cfg.AirwallexWebhookSecret),
			payments.NewPayPalGateway(cfg.PayPalClientID, cfg.PayPalClientSecret, cfg.PayPalEnv, cfg.PayPalWebhookID),
			payments.NewMockGateway(), // mock stays available so dev webhooks still resolve
		)
	default: // "mock"
		// Mock resolves all three names (airwallex/paypal/mock) so dev webhooks
		// + checkout in mock mode both work.
		mock := payments.NewMockGateway()
		gatewayRegistry = payment.NewRegistry()
		gatewayRegistry.Register("mock", mock)
		gatewayRegistry.Register("airwallex", mock)
		gatewayRegistry.Register("paypal", mock)
	}
	paymentRepo := payment.NewRepository(dbPool)
	paymentService := payment.NewService(
		paymentRepo, gatewayRegistry,
		orderService,                        // OrderFinalizer (MarkPaid)
		orderService,                        // OrderLoader (GetAdmin)
		paymentEnqueuer{jobClient},          // PaymentEnqueuer
		cfg.ClientOrigin+"/checkout/return", // gateway redirect URL after payment
	)
	orderService.SetPaymentIntenter(paymentService) // break order↔payment cycle
	orderService.SetPaymentRefunder(paymentService)
	paymentHandler := payment.NewHandler(paymentService)
	orderHandler := order.NewHandler(orderService)

	// --- Certificates (PRD §3.2.1/§5.4, TDD §8) ---
	// certchain.NoopChain for v1 (reserved blockchain adapter, PRD §5.4). The
	// certificate service auto-issues at product creation + appends `sold`
	// provenance at order-paid. NoopChain = no on-chain registration yet.
	certRepo := certificate.NewRepository(dbPool)
	certService := certificate.NewService(certRepo, certchain.NewNoopChain())
	productService.SetCertificateIssuer(certService)                       // auto-issue at create
	orderService.SetProvenanceRecorder(certService)                        // `sold` at MarkPaid
	orderService.SetStockCheckEnqueuer(orderStockCheckEnqueuer{jobClient}) // low-stock alert at MarkPaid
	// Certificate handler is constructed after the media/storage section below
	// (it needs storageStore to resolve pdf_key → public URL in PDFDownload).

	// --- Media assets + product gallery (TDD §3.4 line 127/143, PRD §3.2.1) ---
	// STORAGE_MODE=local (dev): LocalStore writes to a local dir served via a
	// Fiber static mount at /media. "oss": live Alibaba Cloud OSS presign +
	// CDN (NOT live-tested until creds land). The store is injected into the
	// media service for PublicURL resolution + dev uploads + GC deletes.
	var storageStore storage.Store
	switch cfg.StorageMode {
	case "oss":
		storageStore = storage.NewOSSStore(
			cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret, cfg.OSSBucket,
			cfg.OSSEndpoint, cfg.OSSPublicBaseURL,
		)
	default: // "local"
		ls, err := storage.NewLocalStore(cfg.StorageLocalDir, cfg.StoragePublicBaseURL)
		if err != nil {
			log.Fatalf("storage.NewLocalStore: %v", err)
		}
		storageStore = ls
	}
	mediaRepo := media.NewRepository(dbPool)
	mediaService := media.NewService(mediaRepo, storageStore)
	mediaHandler := media.NewHandler(mediaService, storageStore)
	productService.SetGalleryLoader(mediaService) // surface gallery on product detail

	// Certificate handler (needs storageStore for PDFDownload → public URL) +
	// the PDF-enqueue seam (TDD §12: issue/regenerate enqueues pdf:generate).
	certificateHandler := certificate.NewHandler(certService, storageStore)
	certService.SetPDFEnqueuer(pdfEnqueuer{jobClient})

	consentRepo := consent.NewRepository(dbPool)
	consentService := consent.NewService(consentRepo, []byte(cfg.ConsentHMACKey))
	consentHandler := consent.NewHandler(consentService)

	privacyRepo := privacy.NewRepository(dbPool)
	privacyService := privacy.NewService(privacyRepo, jobClient, tokenBlocklist)
	privacyHandler := privacy.NewHandler(privacyService)

	// --- Analytics (in-house, PRD §3.4.2, TDD §3.4/§4.2) ---
	// Public + consent-gated event endpoint. GeoIP flips noop→maxmind via env;
	// noop (dev default) resolves every IP to 'ZZ' so no .mmdb is needed.
	geoipLookup := geoip.MustNew(cfg.GeoIPMode, cfg.GeoLite2DBPath)
	analyticsRepo := analytics.NewRepository(dbPool)
	analyticsService := analytics.NewService(analyticsRepo, consentService, geoipLookup, []byte(cfg.AnalyticsHMACKey))
	analyticsHandler := analytics.NewHandler(analyticsService)
	// Dashboard read endpoints (PRD §3.4.2 Phase B): traffic/sales/funnel live
	// queries + CSV export. Separate repo+service so the ingest path stays
	// focused; same *pgxpool.Pool.
	analyticsDashRepo := analytics.NewDashboardRepo(dbPool)
	analyticsDashService := analytics.NewDashboardService(analyticsDashRepo)
	analyticsDashHandler := analytics.NewDashboardHandler(analyticsDashService)

	// --- Audit log (PRD §3.1.1, TDD §135) ---
	// Records sensitive admin/staff actions (content transitions, deletes, role
	// changes, refunds, erasure, etc.) for GDPR accountability. The Logger is
	// injected into each sensitive service. IP hashed with CONSENT_HMAC_KEY
	// (same short-term audit purpose, one fewer secret).
	auditRepo := audit.NewRepository(dbPool)
	auditLogger := audit.NewService(auditRepo, []byte(cfg.ConsentHMACKey), dbPool)
	auditHandler := audit.NewHandler(auditLogger)

	// --- Itinerary (Custom Travel, PRD §3.3.2) ---
	// Customer-facing wizard: submit + draft + list/get/cancel. The 24h-SLA ack
	// email reuses the order email enqueuer; consent recording adapts consent.Service
	// to a narrow interface (avoids an itinerary→consent import cycle).
	itineraryRepo := itinerary.NewRepository(dbPool)
	itineraryService := itinerary.NewService(
		itineraryRepo,
		orderEmailEnqueuer{jobClient},       // EmailEnqueuer (24h-SLA ack + quote-ready)
		userService,                         // UserDirectory (customer email + ListStaffByRole)
		itinConsentRecorder{consentService}, // ConsentRecorder (GDPR audit)
		fxService,                           // CheckoutFX (quote CNY→presentment)
		cfg.PaymentsMode,                    // mock|sandbox|live
	)
	itineraryHandler := itinerary.NewHandler(itineraryService, storageStore)
	// Quote-payment setters (post-construction, break the itinerary↔payment cycle).
	itineraryService.SetQuotePaymentIntenter(paymentService)
	itineraryService.SetQuoteRefunder(paymentService)
	itineraryService.SetDepositFinalizeEnqueuer(paymentEnqueuer{jobClient}) // mock-mode auto-finalize seam
	itineraryService.SetPDFEnqueuer(pdfEnqueuer{jobClient})                 // quote-send PDF render

	// Wire the audit logger into every sensitive handler (PRD §3.1.1). Each
	// handler's SetAuditLogger accepts the audit.Logger interface; nil-safe.
	for _, h := range []interface{ SetAuditLogger(audit.Logger) }{
		orderHandler, privacyHandler, userHandler, productHandler,
		ceramicStoryHandler, engageHandler, artistHandler,
		itineraryHandler, mediaHandler, shippingHandler,
	} {
		h.SetAuditLogger(auditLogger)
	}

	// --- Static mount for local-dev media (STORAGE_MODE=local) ---
	// Registered BEFORE SetupRoutes: Fiber v2 attaches `Group("").Use()` (the
	// authed checkout group) at the radix root, so it leaks JWTMAuth to every
	// route registered AFTER it. /media is public (product gallery, cert +
	// itinerary PDFs) → must precede the authed empty-prefix group. In OSS mode
	// this is a no-op — the CDN serves media directly.
	if cfg.StorageMode != "oss" && cfg.StorageLocalDir != "" && cfg.StoragePublicBaseURL != "" {
		app.Static(cfg.StoragePublicBaseURL, cfg.StorageLocalDir)
		log.Printf("media: local store at %s -> %s", cfg.StorageLocalDir, cfg.StoragePublicBaseURL)
	}

	api.SetupRoutes(app, cfg.JWTSecret,
		wsHandler, userHandler, notifHandler,
		ceramicStoryHandler, engageHandler, addressHandler,
		consentHandler, artistHandler, productHandler, wishlistHandler, cartHandler, fxHandler,
		shippingHandler, orderHandler, paymentHandler, certificateHandler, mediaHandler, twoFAHandler, privacyHandler,
		itineraryHandler, analyticsHandler, analyticsDashHandler, auditHandler,
		tokenBlocklist,
	)

	// --- Start server (graceful shutdown) ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		if err := app.Listen(":" + cfg.ServerPort); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	<-quit
	log.Println("Shutting down API + WS...")
	hubCancel() // stop the WebSocket Hub
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}

// --- worker mode (Asynq server + scheduler) ---------------------------------

func runWorker(rootCtx context.Context, cfg config.Config) {
	redisAddr, err := redisAddrFromURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Invalid REDIS_URL: %v", err)
	}

	// --- Database connection (worker needs DB for FX refresh + future jobs) ---
	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to parse database configuration: %v\n", err)
	}
	dbPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbPool.Close()

	// --- FX pipeline (TDD §7) ---
	// The worker owns Refresh: ECB fetch → markup → upsert. Serve mode owns
	// Convert (read-time); both share the fx_rates table.
	var fxRateSource fx.RateSource
	if cfg.ECB_API_URL != "" {
		fxRateSource = fx.NewECBClient(cfg.ECB_API_URL)
	} else {
		fxRateSource = fx.FixtureRateSource{Rates: fx.DefaultFixtureRates()}
	}
	fxRepo := fx.NewRepository(dbPool)
	fxService := fx.NewService(fxRepo, fxRateSource, cfg.FXMarkupBPS)

	// --- Orders (worker owns payment:finalize → MarkPaid) ---
	orderRepo := order.NewRepository(dbPool)
	orderService := order.NewService(
		orderRepo, nil, nil, nil, nil, fxService, nil, nil, nil, nil, nil, nil, cfg.PaymentsMode,
	)

	// The worker's MarkPaid appends `sold` provenance — wire the cert service
	// (NoopChain; no DB writes beyond the provenance row).
	wCertRepo := certificate.NewRepository(dbPool)
	wCertService := certificate.NewService(wCertRepo, certchain.NewNoopChain())
	orderService.SetProvenanceRecorder(wCertService)

	// The worker sends emails via Brevo. The serve mode renders templates at
	// enqueue time, so the worker only needs the sender (no template manager).
	emailer := email.NewBrevoSender(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName)

	jobServer := jobs.NewServer(redisAddr)
	// Worker-side itinerary service for deposit-finalize (MarkDepositPaid) + SLA
	// breach scan. Reuses the same fxService + a fresh repo; no email/user deps
	// needed in the worker (MarkDepositPaid only touches the repo; SLA email uses
	// the planner list from the user repo below).
	wItinRepo := itinerary.NewRepository(dbPool)
	wItineraryService := itinerary.NewService(wItinRepo, nil, nil, nil, fxService, cfg.PaymentsMode)
	wJobClient := jobs.NewClient(redisAddr) // worker-side enqueue (SLA email)
	wItineraryService.SetDepositFinalizeEnqueuer(paymentEnqueuer{wJobClient})

	// PDF generator (TDD §12). local=Noop (dev, returns ErrPDFUnavailable →
	// worker skips storage); chromedp=remote-allocator to a headless-shell sidecar.
	var pdfGenerator pdf.Generator = pdf.NewNoopGenerator()
	if cfg.PDFMode == "chromedp" {
		pdfGenerator = pdf.NewChromedpGenerator(cfg.ChromedpURL)
	}

	jobServer.EmailSend = func(ctx context.Context, p jobs.EmailSendPayload) error {
		return emailer.SendEmail(ctx, p.To, p.Subject, p.PlainText, p.HTML)
	}
	jobServer.FXRefresh = fxService.Refresh
	// payment:finalize drives order created→paid. On success → MarkPaid.
	// (mock seam enqueues {success:true} from checkout in dev; the gateway
	// webhook enqueues it in live mode, #6.)
	jobServer.PaymentFinalize = func(ctx context.Context, p jobs.PaymentFinalizePayload) error {
		if !p.Success {
			log.Printf("worker.PaymentFinalize: order=%d quote=%d reported failure (no-op for now)", p.OrderID, p.ItineraryQuoteID)
			return nil // cancel-on-failure lands with #6
		}
		// Dispatch by entity: an order payment → orderService.MarkPaid; an
		// itinerary deposit → itineraryService.MarkDepositPaid. The webhook
		// path sets exactly one of OrderID / ItineraryQuoteID.
		if p.ItineraryQuoteID != 0 {
			// Mock-mode seam: no webhook fires, so advance the pending payment
			// row to 'succeeded' here (the live path's webhook does it via
			// UpsertWebhook). Best-effort: a failure is logged, not fatal.
			if p.GatewayRef != "" {
				wPayRepo := payment.NewRepository(dbPool)
				if err := wPayRepo.MarkSucceededByGatewayRef(ctx, p.GatewayRef); err != nil {
					log.Printf("worker.PaymentFinalize.MarkSucceeded(quote=%d ref=%s): %v", p.ItineraryQuoteID, p.GatewayRef, err)
				}
			}
			return wItineraryService.MarkDepositPaid(ctx, p.ItineraryQuoteID)
		}
		return orderService.MarkPaid(ctx, p.OrderID)
	}

	// stock:check (TDD line 234) — low-stock alert after an order is paid.
	// Loads post-decrement stock for the order's SKUs; for each at/below
	// threshold, creates a dashboard notification + enqueues a Brevo email to
	// every ecommerce_operator. Best-effort: a single notification/email
	// failure is logged + skipped, never blocks the job.
	wProductRepo := product.NewRepository(dbPool)
	wUserRepo := user.NewRepository(dbPool)
	wNotifRepo := notification.NewRepository(dbPool)
	// Worker has no WS hub (only serve mode serves WebSockets); pass nil — the
	// notification service is nil-safe (creates the row, skips the WS push).
	wNotifService := notification.NewService(wNotifRepo, wUserRepo, nil)
	// wJobClient declared above (with wItineraryService). Same client for
	// low-stock emails + SLA emails.
	// The worker's MarkPaid enqueues stock:check to itself (asynq supports
	// enqueue-from-worker). Same client as the emails.
	orderService.SetStockCheckEnqueuer(orderStockCheckEnqueuer{wJobClient})
	jobServer.StockCheck = func(ctx context.Context, p jobs.StockCheckPayload) error {
		lowStock, err := wProductRepo.FindLowStock(ctx, p.SkuIDs)
		if err != nil {
			return fmt.Errorf("worker.StockCheck.FindLowStock: %w", err)
		}
		if len(lowStock) == 0 {
			return nil
		}
		operators, err := wUserRepo.ListByRole(ctx, models.RoleEcommerceOperator)
		if err != nil {
			return fmt.Errorf("worker.StockCheck.ListByRole: %w", err)
		}
		for _, sku := range lowStock {
			msg := fmt.Sprintf("SKU %s is low on stock (%d left, threshold %d)",
				sku.SKUCode, sku.Stock, sku.LowStockThreshold)
			for _, op := range operators {
				// Dashboard notification (creates a row + WS push if online).
				if _, err := wNotifService.CreateNotification(ctx, models.CreateNotificationParams{
					RecipientUserID: op.ID,
					Type:            models.NotificationTypeLowStock,
					ExtraData:       map[string]string{"message": msg},
				}); err != nil {
					log.Printf("worker.StockCheck.Notify(user=%s sku=%s): %v", op.ID, sku.SKUCode, err)
				}
				// Brevo email (plain text; HTML template deferred).
				if err := wJobClient.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
					To: op.Email, Subject: "Low stock: " + sku.SKUCode, PlainText: msg,
				}); err != nil {
					log.Printf("worker.StockCheck.Email(user=%s sku=%s): %v", op.ID, sku.SKUCode, err)
				}
			}
		}
		return nil
	}

	// sla:check (TDD line 235) — flags itinerary requests approaching/past the
	// 24h SLA. Runs every 15min (jobs scheduler). For each breached pending/
	// processing request not yet notified, CAS-set sla_notified_at (exactly-once:
	// only the winner notifies), then push a dashboard notification + email to
	// every travel_planner. Best-effort: a single failure is logged + skipped.
	// wItinRepo declared above (with wItineraryService).
	jobServer.SLACheck = func(ctx context.Context) error {
		breached, err := wItinRepo.ListBreached(ctx)
		if err != nil {
			return fmt.Errorf("worker.SLACheck.ListBreached: %w", err)
		}
		if len(breached) == 0 {
			return nil
		}
		planners, err := wUserRepo.ListByRole(ctx, models.RoleTravelPlanner)
		if err != nil {
			return fmt.Errorf("worker.SLACheck.ListByRole: %w", err)
		}
		for _, req := range breached {
			won, err := wItinRepo.SetSLANotified(ctx, req.ID)
			if err != nil {
				log.Printf("worker.SLACheck.SetSLANotified(req=%d): %v", req.ID, err)
				continue
			}
			if !won {
				continue // a concurrent tick already flagged it
			}
			msg := fmt.Sprintf("Itinerary request #%d has passed its 24-hour response SLA (submitted %s)",
				req.ID, req.SubmittedAt.Format("2006-01-02 15:04"))
			for _, p := range planners {
				if _, err := wNotifService.CreateNotification(ctx, models.CreateNotificationParams{
					RecipientUserID: p.ID,
					Type:            models.NotificationTypeSystem,
					EntityType:      "itinerary_request",
					EntityID:        req.ID,
					ExtraData:       map[string]string{"message": msg},
				}); err != nil {
					log.Printf("worker.SLACheck.Notify(planner=%s req=%d): %v", p.ID, req.ID, err)
				}
				if err := wJobClient.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
					To: p.Email, Subject: "SLA breach: itinerary request #" + strconv.FormatInt(req.ID, 10),
					PlainText: msg,
				}); err != nil {
					log.Printf("worker.SLACheck.Email(planner=%s req=%d): %v", p.ID, req.ID, err)
				}
			}
		}
		return nil
	}

	// analytics:rollup (nightly, TDD §4.2). Aggregates yesterday's events into
	// analytics_daily. Idempotent: re-running for a date SETs the value (does
	// not increment). Worker-side analytics service = fresh repo + noop geoip
	// (rollup reads country from the stored column, never re-resolves) + the
	// same analytics HMAC key (unused in rollup, but the constructor needs it).
	wAnalyticsRepo := analytics.NewRepository(dbPool)
	wAnalyticsService := analytics.NewService(wAnalyticsRepo, nil, geoip.NewNoop(), []byte(cfg.AnalyticsHMACKey))
	jobServer.AnalyticsRollup = func(ctx context.Context) error {
		if err := wAnalyticsService.RollupDaily(ctx, time.Time{}); err != nil {
			log.Printf("worker.AnalyticsRollup: %v", err)
			return fmt.Errorf("analytics:rollup: %w", err)
		}
		return nil
	}

	// pdf:generate (TDD §12 + line 231). Renders a doc via chromedp + stores
	// via the storage adapter, populating the entity's pdf_key. Best-effort:
	// a render failure is logged + the job retries (asynq MaxRetry 5). The
	// NoopGenerator (local mode) returns ErrPDFUnavailable → skip storage.
	jobServer.PDFGenerate = func(ctx context.Context, p jobs.PDFGeneratePayload) error {
		switch p.Kind {
		case "certificate":
			return renderCertificatePDF(ctx, p, wCertRepo, newWorkerStore(cfg), pdfGenerator, cfg.PDFBaseURL)
		case "itinerary_quote":
			return renderItineraryQuotePDF(ctx, p, wItinRepo, newWorkerStore(cfg), pdfGenerator)
		default:
			log.Printf("worker.PDFGenerate: unknown kind %q", p.Kind)
			return nil // don't retry an unknown kind
		}
	}

	jobScheduler := jobs.NewScheduler(redisAddr)

	// Feature modules will assign real handlers here, e.g.:
	//   jobServer.EmailSend = brevoEmailHandler
	//   jobServer.PaymentFinalize = orderFinalizer
	// Until then every job type is a logged no-op so the worker is live today.

	ctx, cancel := shutdownCtx()
	defer cancel()

	serverErr := make(chan error, 1)
	schedulerErr := make(chan error, 1)
	go func() { serverErr <- jobServer.Run(ctx) }()
	go func() { schedulerErr <- jobScheduler.Run(ctx) }()
	// also stop when the app root context is cancelled (not strictly needed for
	// worker, but keeps shutdown consistent with serve mode).
	go func() { <-rootCtx.Done(); cancel() }()

	select {
	case err := <-serverErr:
		log.Printf("Asynq server exited: %v", err)
	case err := <-schedulerErr:
		log.Printf("Asynq scheduler exited: %v", err)
	case <-ctx.Done():
		// signals already trigger shutdown inside Run via shutdownCtx cancel.
	}

	log.Println("Worker exiting")
}

// renderCertificatePDF loads a certificate (locale-joined for the template),
// renders the HTML via pdftmpl, prints to PDF via the generator, stores the
// bytes via the storage adapter, and records the resulting key on the cert row
// (TDD §12). Best-effort: a NoopGenerator (local mode) returns ErrPDFUnavailable
// → the worker skips storage + pdf_key stays NULL (the download endpoint 404s).
func renderCertificatePDF(ctx context.Context, p jobs.PDFGeneratePayload,
	certRepo certificate.RepositoryInterface, store storage.Store,
	gen pdf.Generator, baseURL string) error {
	cert, err := certRepo.GetByID(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.GetByID(cert=%d): %w", p.EntityID, err)
	}
	// Load the locale-joined display fields (product title, artist) for the
	// template. GetByCode joins; GetByID does not — re-read via GetByCode.
	locale := p.Locale
	if locale == "" {
		locale = "en-US"
	}
	display, err := certRepo.GetByCode(ctx, cert.CertCode, locale)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.GetByCode(cert=%d): %w", p.EntityID, err)
	}
	// The QR <img> URL the sidecar fetches at render time (over the compose
	// network → the api container). Falls back to the public base if unset.
	apiBase := baseURL
	if apiBase == "" {
		apiBase = "http://localhost:1323"
	}
	qrURL := apiBase + "/certificates/" + cert.CertCode + "/qr"
	html, err := pdftmpl.RenderCertificate(display, apiBase, qrURL, locale)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.RenderHTML(cert=%d): %w", p.EntityID, err)
	}
	pdfBytes, err := gen.Render(ctx, pdf.RenderRequest{
		HTML: html, PaperFormat: "A4",
	})
	if err != nil {
		if errors.Is(err, pdf.ErrPDFUnavailable) {
			// Local mode: no sidecar. Skip storage; pdf_key stays NULL.
			log.Printf("worker.PDFGenerate: local mode, skipping cert=%d", p.EntityID)
			return nil
		}
		return fmt.Errorf("worker.PDFGenerate.Render(cert=%d): %w", p.EntityID, err)
	}
	// Store the PDF. Key mirrors the media convention but under pdf/.
	key := fmt.Sprintf("pdf/certificates/%s.pdf", cert.CertCode)
	if err := store.Put(ctx, key, bytes.NewReader(pdfBytes), "application/pdf"); err != nil {
		return fmt.Errorf("worker.PDFGenerate.Put(cert=%d): %w", p.EntityID, err)
	}
	if err := certRepo.SetPDFKey(ctx, p.EntityID, key); err != nil {
		return fmt.Errorf("worker.PDFGenerate.SetPDFKey(cert=%d): %w", p.EntityID, err)
	}
	log.Printf("worker.PDFGenerate: rendered cert=%d → %s (%d bytes)", p.EntityID, key, len(pdfBytes))
	return nil
}

// newWorkerStore builds a storage.Store for the worker scope (the serve scope
// builds its own in runServe). The worker only needs Put (render → store) +
// PublicURL (no presign/upload-handler path). Reuses the same env-flip.
func newWorkerStore(cfg config.Config) storage.Store {
	switch cfg.StorageMode {
	case "oss":
		return storage.NewOSSStore(
			cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret, cfg.OSSBucket,
			cfg.OSSEndpoint, cfg.OSSPublicBaseURL,
		)
	default: // "local"
		ls, err := storage.NewLocalStore(cfg.StorageLocalDir, cfg.StoragePublicBaseURL)
		if err != nil {
			log.Printf("worker.newWorkerStore: local store unavailable (%v); PDF Put will fail", err)
		}
		return ls
	}
}

// renderItineraryQuotePDF loads a quote + its request (trip details), renders
// the branded itinerary-quote HTML via pdftmpl, prints to PDF via chromedp,
// stores the bytes via the storage adapter, and records the resulting key on
// the quote row (TDD §12, M3 #4). Best-effort: a NoopGenerator (local mode)
// returns ErrPDFUnavailable → the worker skips storage + pdf_key stays NULL
// (the download endpoints 404 'PDF not yet generated').
func renderItineraryQuotePDF(ctx context.Context, p jobs.PDFGeneratePayload,
	itinRepo itinerary.RepositoryInterface, store storage.Store,
	gen pdf.Generator) error {
	q, err := itinRepo.GetQuoteByID(ctx, p.EntityID)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.GetQuoteByID(quote=%d): %w", p.EntityID, err)
	}
	// Load the request (trip details) for the template. GetByIDAdmin JOINs the
	// customer fields the template doesn't need, but it's the canonical loader
	// + scanAdminRow tolerates a NULL assigned_to.
	req, err := itinRepo.GetByIDAdmin(ctx, q.RequestID)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.GetByIDAdmin(req=%d): %w", q.RequestID, err)
	}
	locale := p.Locale
	if locale == "" {
		locale = req.Locale
	}
	if locale == "" {
		locale = "en-US"
	}
	html, err := pdftmpl.RenderItineraryQuote(req, q, locale)
	if err != nil {
		return fmt.Errorf("worker.PDFGenerate.RenderItineraryQuote(quote=%d): %w", p.EntityID, err)
	}
	pdfBytes, err := gen.Render(ctx, pdf.RenderRequest{HTML: html, PaperFormat: "A4"})
	if err != nil {
		if errors.Is(err, pdf.ErrPDFUnavailable) {
			// Local mode: no sidecar. Skip storage; pdf_key stays NULL.
			log.Printf("worker.PDFGenerate: local mode, skipping itinerary_quote=%d", p.EntityID)
			return nil
		}
		return fmt.Errorf("worker.PDFGenerate.Render(quote=%d): %w", p.EntityID, err)
	}
	// Store the PDF. Key mirrors the cert convention under pdf/itineraries/.
	// Keyed on request_id (one active quote per request → one PDF per request).
	key := fmt.Sprintf("pdf/itineraries/%d.pdf", q.RequestID)
	if err := store.Put(ctx, key, bytes.NewReader(pdfBytes), "application/pdf"); err != nil {
		return fmt.Errorf("worker.PDFGenerate.Put(quote=%d): %w", p.EntityID, err)
	}
	if err := itinRepo.SetQuotePDFKey(ctx, p.EntityID, key); err != nil {
		return fmt.Errorf("worker.PDFGenerate.SetQuotePDFKey(quote=%d): %w", p.EntityID, err)
	}
	log.Printf("worker.PDFGenerate: rendered itinerary_quote=%d → %s (%d bytes)", p.EntityID, key, len(pdfBytes))
	return nil
}
