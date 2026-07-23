package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jingdezhen-ceramics-backend/internal/api"
	"jingdezhen-ceramics-backend/internal/config"
	"jingdezhen-ceramics-backend/internal/modules/address"
	"jingdezhen-ceramics-backend/internal/modules/artist"
	"jingdezhen-ceramics-backend/internal/modules/ceramicstory"
	"jingdezhen-ceramics-backend/internal/modules/consent"
	"jingdezhen-ceramics-backend/internal/modules/engage"
	"jingdezhen-ceramics-backend/internal/modules/notification"
	"jingdezhen-ceramics-backend/internal/modules/privacy"
	"jingdezhen-ceramics-backend/internal/modules/product"
	"jingdezhen-ceramics-backend/internal/modules/user"
	"jingdezhen-ceramics-backend/internal/modules/twofa"
	"jingdezhen-ceramics-backend/internal/modules/wishlist"
	"jingdezhen-ceramics-backend/internal/modules/cart"
	"jingdezhen-ceramics-backend/internal/modules/shipping"
	"jingdezhen-ceramics-backend/internal/modules/order"
	"jingdezhen-ceramics-backend/internal/modules/payment"
	"jingdezhen-ceramics-backend/pkg/adapters/payments"
	"jingdezhen-ceramics-backend/internal/platform/fx"
	"jingdezhen-ceramics-backend/internal/platform/jobs"
	platformredis "jingdezhen-ceramics-backend/internal/platform/redis"
	"jingdezhen-ceramics-backend/internal/ws"
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

type orderEmailEnqueuer struct{ c *jobs.Client }

func (e orderEmailEnqueuer) EnqueueEmailSend(ctx context.Context, to, subject, plainText, html string) error {
	return e.c.EnqueueEmailSend(ctx, jobs.EmailSendPayload{
		To: to, Subject: subject, PlainText: plainText, HTML: html,
	})
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
		orderEmailEnqueuer{jobClient},  // EmailEnqueuer
		paymentEnqueuer{jobClient},     // PaymentEnqueuer
		userService,                    // UserPrefFetcher (PreferredCurrency)
		nil,                           // PaymentIntenter (wired after payment.Service below)
		nil,                           // PaymentRefunder (wired after payment.Service below)
		userService,                    // UserFetcher (GetUserProfile for the email)
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
		orderService,                 // OrderFinalizer (MarkPaid)
		orderService,                 // OrderLoader (GetAdmin)
		paymentEnqueuer{jobClient},    // PaymentEnqueuer
		cfg.ClientOrigin+"/checkout/return", // gateway redirect URL after payment
	)
	orderService.SetPaymentIntenter(paymentService) // break order↔payment cycle
	orderService.SetPaymentRefunder(paymentService)
	paymentHandler := payment.NewHandler(paymentService)
	orderHandler := order.NewHandler(orderService)

	consentRepo := consent.NewRepository(dbPool)
	consentService := consent.NewService(consentRepo, []byte(cfg.ConsentHMACKey))
	consentHandler := consent.NewHandler(consentService)

	privacyRepo := privacy.NewRepository(dbPool)
	privacyService := privacy.NewService(privacyRepo, jobClient)
	privacyHandler := privacy.NewHandler(privacyService)

	api.SetupRoutes(app, cfg.JWTSecret,
		wsHandler, userHandler, notifHandler,
		ceramicStoryHandler, engageHandler, addressHandler,
		consentHandler, artistHandler, productHandler, wishlistHandler, cartHandler, fxHandler,
		shippingHandler, orderHandler, paymentHandler, twoFAHandler, privacyHandler,
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

	// The worker sends emails via Brevo. The serve mode renders templates at
	// enqueue time, so the worker only needs the sender (no template manager).
	emailer := email.NewBrevoSender(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName)

	jobServer := jobs.NewServer(redisAddr)
	jobServer.EmailSend = func(ctx context.Context, p jobs.EmailSendPayload) error {
		return emailer.SendEmail(ctx, p.To, p.Subject, p.PlainText, p.HTML)
	}
	jobServer.FXRefresh = fxService.Refresh
	// payment:finalize drives order created→paid. On success → MarkPaid.
	// (mock seam enqueues {success:true} from checkout in dev; the gateway
	// webhook enqueues it in live mode, #6.)
	jobServer.PaymentFinalize = func(ctx context.Context, p jobs.PaymentFinalizePayload) error {
		if !p.Success {
			log.Printf("worker.PaymentFinalize: order=%d reported failure (no-op for now)", p.OrderID)
			return nil // cancel-on-failure lands with #6
		}
		return orderService.MarkPaid(ctx, p.OrderID)
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
