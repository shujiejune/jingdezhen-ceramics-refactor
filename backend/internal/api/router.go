package api

import (
	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/address"
	"jingdezhen-ceramics-backend/internal/modules/artist"
	"jingdezhen-ceramics-backend/internal/modules/ceramicstory"
	"jingdezhen-ceramics-backend/internal/modules/consent"
	"jingdezhen-ceramics-backend/internal/modules/engage"
	"jingdezhen-ceramics-backend/internal/modules/notification"
	"jingdezhen-ceramics-backend/internal/modules/product"
	"jingdezhen-ceramics-backend/internal/modules/privacy"
	"jingdezhen-ceramics-backend/internal/modules/twofa"
	"jingdezhen-ceramics-backend/internal/modules/wishlist"
	"jingdezhen-ceramics-backend/internal/modules/cart"
	"jingdezhen-ceramics-backend/internal/modules/shipping"
	"jingdezhen-ceramics-backend/internal/modules/order"
	"jingdezhen-ceramics-backend/internal/modules/payment"
	"jingdezhen-ceramics-backend/internal/modules/certificate"
	"jingdezhen-ceramics-backend/internal/modules/media"
	"jingdezhen-ceramics-backend/internal/platform/fx"
	"jingdezhen-ceramics-backend/internal/modules/user"
	"jingdezhen-ceramics-backend/internal/ws"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures the API routes.
func SetupRoutes(
	app *fiber.App, jwtSecretKey string,
	wsHandler *ws.Handler,
	userHandler *user.Handler,
	notifHandler *notification.Handler,
	csHandler *ceramicstory.Handler,
	engageHandler *engage.Handler,
	addressHandler *address.Handler,
	consentHandler *consent.Handler,
	artistHandler *artist.Handler,
	productHandler *product.Handler,
	wishlistHandler *wishlist.Handler,
	cartHandler *cart.Handler,
	fxHandler *fx.Handler,
	shippingHandler *shipping.Handler,
	orderHandler *order.Handler,
	paymentHandler *payment.Handler,
	certificateHandler *certificate.Handler,
	mediaHandler *media.Handler,
	twoFAHandler *twofa.Handler,
	privacyHandler *privacy.Handler,
) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Welcome to the Jingdezhen Ceramics Platform!"})
	})

	// --- WebSocket Route ---
	// This route group ensures the JWT middleware runs first to authenticate the user.
	wsGroup := app.Group("/ws")
	wsGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	wsGroup.Use(ws.WsUpgradeMiddleware()) // This middleware checks if it's a valid WebSocket request
	// Pass the handler's method, not the handler struct itself.
	// The websocket.New() middleware expects a function argument with signature func(*websocket.Conn)
	// that it can call whenever a new WebSocket connection is established.
	wsGroup.Get("/", websocket.New(wsHandler.UpgradeConnection))

	/* --- Contact (send feedback) --- */
	app.Post("/contact", userHandler.SubmitContactForm)

	/* --- Consent (GDPR) — POST is public so anonymous visitors can record
	cookie consent before signup; GET state/history is protected. --- */
	app.Post("/consent", consentHandler.RecordConsent)

	/* --- Auth (Public) --- */
	authGroup := app.Group("/auth")
	{
		authGroup.Post("/signup", userHandler.Signup)
		authGroup.Post("/login", userHandler.Login)
		authGroup.Post("/activate", userHandler.ActivateAccount)
		authGroup.Post("resend-activation", userHandler.ResendActivation)
		authGroup.Post("request-password-reset", userHandler.RequestPasswordReset)
		authGroup.Post("reset-password", userHandler.ResetPassword)
		authGroup.Get("/google/login", userHandler.GoogleLogin)
		authGroup.Get("/google/callback", userHandler.GoogleCallback)

		// 2FA login completion — PUBLIC (the pending token is the credential;
		// a JWT is not yet available because login hasn't finished). TDD §5.3.
		// Routes to the user handler, which owns the JWT and delegates verification
		// to the 2FA service.
		authGroup.Post("/2fa/verify", userHandler.Verify2FALogin)

		// 2FA must-enroll flow for super_admin (PRD §4.3 mandate). PUBLIC — the
		// pending token (from the blocked login) is the credential. Enroll returns
		// the QR/secret; confirm verifies the code, enables 2FA, and mints the
		// real access token (login completes).
		authGroup.Post("/2fa/pending-enroll", userHandler.Pending2FAEnroll)
		authGroup.Post("/2fa/pending-confirm", userHandler.Pending2FAConfirm)
	}

	/* --- User Profile (Protected) --- */
	// If need backend routes for auth (e.g., refresh token, logout initiated by backend), define here.
	profileGroup := app.Group("/profile")
	profileGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		profileGroup.Get("", userHandler.GetProfile)
		profileGroup.Put("", userHandler.UpdateProfile)

		// Shipping address book (PRD §3.5) — scoped to the authenticated user.
		profileGroup.Get("/addresses", addressHandler.ListAddresses)
		profileGroup.Post("/addresses", addressHandler.CreateAddress)
		profileGroup.Get("/addresses/:id", addressHandler.GetAddress)
		profileGroup.Put("/addresses/:id", addressHandler.UpdateAddress)
		profileGroup.Delete("/addresses/:id", addressHandler.DeleteAddress)
		profileGroup.Post("/addresses/:id/default", addressHandler.SetDefaultAddress)

		// Consent history (GDPR data export) + latest consent state per kind.
		profileGroup.Get("/consent", consentHandler.ListConsentHistory)
		profileGroup.Get("/consent/:kind", consentHandler.GetConsentState)

		// TOTP 2FA management (TDD §5.3, PRD §4.3). Enroll returns the QR/secret,
		// confirm verifies the first code and enables 2FA; disable turns it off.
		profileGroup.Post("/2fa/enroll", twoFAHandler.Enroll)
		profileGroup.Post("/2fa/confirm", twoFAHandler.Confirm)
		profileGroup.Delete("/2fa", twoFAHandler.Disable)

		// One-time backup codes (recovery). Regenerate returns the new set ONCE;
		// the GET returns only the remaining count (never the codes themselves).
		profileGroup.Post("/2fa/backup-codes/regenerate", twoFAHandler.RegenerateBackupCodes)
		profileGroup.Get("/2fa/backup-codes", twoFAHandler.BackupCodesRemaining)

		// GDPR self-service (PRD §4.3): machine-readable data export.
		profileGroup.Get("/export", privacyHandler.ExportUserData)
		// ... other user-specific routes like badges, subscriptions
	}

	/* --- Notification Module (Protected) --- */
	notifGroup := app.Group("/notifications")
	notifGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		notifGroup.Get("", notifHandler.GetNotifications)
		notifGroup.Get("/unread-count", notifHandler.GetUnreadNotificationCount)
		notifGroup.Post("/mark-all-read", notifHandler.MarkAllAsRead)
		notifGroup.Post("/:notification_id/mark-read", notifHandler.MarkAsRead)
	}

	/* --- Ceramic Story (History & Heritage, public) --- */
	csGroup := app.Group("/ceramicstory")
	{
		csGroup.Get("", csHandler.GetAllDynasties)
		csGroup.Get("/:slug", csHandler.GetDynastyDetail)
	}

	/* --- Artist profiles (Public) — PRD §3.1.3 / §3.2.1 --- */
	/* Artist reads are locale-aware (?locale= / Accept-Language) and return only
	   published translations. Artist profiles are cross-linked from gallery
	   products (PRD §3.2.1). */
	app.Get("/artists", artistHandler.GetArtists)
	app.Get("/artists/:slug", artistHandler.GetArtistBySlug)

	/* --- Product Catalog (Public) — PRD §3.2.1 --- */
	/* Locale-aware (?locale= / Accept-Language), published-only. The detail view
	   includes the product's SKUs (purchasable units). */
	app.Get("/catalog/products", productHandler.GetProducts)
	app.Get("/catalog/products/:slug", productHandler.GetProductBySlug)
	app.Get("/catalog/products/:id/media", mediaHandler.PublicListProductMedia)
	app.Get("/catalog/categories", productHandler.GetCategories)

	/* --- FX rates (dev-debug, public read) — TDD §7 --- */
	app.Get("/fx/rates", fxHandler.ListRates)

	/* --- Shipping quote (public preview) — PRD §3.2.3, TDD §5.2 --- */
	app.Get("/shipping/quote", shippingHandler.Quote)

	/* --- Payment webhooks (public, signature-verified) — PRD §3.2.3, TDD §10 --- */
	/* Enqueue-and-ack: verify the gateway signature, record the event
	   idempotently (idempotency_key UNIQUE), enqueue payment:finalize, ack 200.
	   The worker drives the order created→paid. TDD §2.2. */
	app.Post("/webhooks/airwallex", paymentHandler.AirwallexWebhook)
	app.Post("/webhooks/paypal", paymentHandler.PayPalWebhook)

	/* --- Certificates (public QR target, no auth) — PRD §3.2.1 --- */
	/* Each product gets a digital certificate with a unique cert_code + QR.
	   GET /certificates/:code is the QR target page (product + artist +
	   provenance chain). GET /certificates/:code/qr renders the QR PNG
	   on-demand (no OSS storage needed). */
	app.Get("/certificates/:code", certificateHandler.GetByCode)
	app.Get("/certificates/:code/qr", certificateHandler.QRCode)

	/* --- Wishlist (Protected) — PRD §3.5 --- */
	/* Favorites are keyed on SKU (the purchasable unit). A customer favorites a
	   specific variant, not a product. Locale-aware read path. */
	wishlistGroup := app.Group("/wishlist")
	wishlistGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		wishlistGroup.Get("", wishlistHandler.GetWishlist)
		wishlistGroup.Post("", wishlistHandler.AddToWishlist)
		wishlistGroup.Delete("/:sku_id", wishlistHandler.RemoveFromWishlist)
	}

	/* --- Cart (Protected) — PRD §3.2.3, TDD §3.4 --- */
	/* One server-side cart per signed-in user. Guests use a localStorage cart
	   and merge it on login via POST /cart/merge. Items keyed on SKU (the
	   purchasable unit). POST add is additive; PATCH sets absolute qty. */
	cartGroup := app.Group("/cart")
	cartGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		cartGroup.Get("", cartHandler.GetCart)
		cartGroup.Post("/items", cartHandler.AddItem)
		cartGroup.Patch("/items/:sku_id", cartHandler.UpdateItemQty)
		cartGroup.Delete("/items/:sku_id", cartHandler.RemoveItem)
		cartGroup.Delete("/items", cartHandler.BulkRemove)
		cartGroup.Post("/merge", cartHandler.MergeCart)
	}

	/* --- Checkout + Orders (signed-in customers) — PRD §3.2.3, TDD §8 --- */
	/* Checkout is signed-in-only (PRD §3.2.3). Customer cancels only an unpaid
	   (created) order. Lifecycle: created→paid→shipped→completed; cancelled;
	   refunded (full refunds only). */
	checkoutGroup := app.Group("")
	checkoutGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		checkoutGroup.Post("/checkout", orderHandler.Checkout)
		checkoutGroup.Get("/orders", orderHandler.ListMine)
		checkoutGroup.Get("/orders/:id", orderHandler.GetMine)
		checkoutGroup.Post("/orders/:id/cancel", orderHandler.CancelMine)
	}

	/* --- Engage (Destinations & Local Lifestyle, public) --- */
	engageGroup := app.Group("/engage")
	{
		engageGroup.Get("", engageHandler.GetActivities)           // ?locale=&type=&page=&limit=
		engageGroup.Get("/:slug", engageHandler.GetActivityArticle) // ?locale=
	}

	/* --- Admin Routes (PRD §3.4.1 RBAC) --- */
	// All /admin routes require auth + the route-specific permission.
	// Super Administrator bypasses every permission check.
	adminGroup := app.Group("/admin")
	adminGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		// FX rate refresh — settings.manage (operator-triggered; the daily cron
		// also enqueues fx:refresh from the worker). PRD §3.2.3.
		adminFX := adminGroup.Group("/fx")
		adminFX.Use(middleware.RequirePermission(models.PermSettingsManage))
		adminFX.Post("/refresh", fxHandler.RefreshFX)

		// Shipping fee tiers CRUD — settings.manage (E-commerce Operators
		// maintain the per-country weight-tiered fee table, PRD §3.2.3).
		adminShipping := adminGroup.Group("/shipping/tiers")
		adminShipping.Use(middleware.RequirePermission(models.PermSettingsManage))
		adminShipping.Get("", shippingHandler.ListTiers)
		adminShipping.Post("", shippingHandler.CreateTier)
		adminShipping.Put("/:id", shippingHandler.UpdateTier)
		adminShipping.Delete("/:id", shippingHandler.DeleteTier)

		// Orders — list/read (order.read), ship+complete (order.write),
		// refund (order.refund, full refunds only — PRD §3.2.3).
		adminOrdersRead := adminGroup.Group("/orders")
		adminOrdersRead.Use(middleware.RequirePermission(models.PermOrderRead))
		adminOrdersRead.Get("", orderHandler.ListAdmin)
		adminOrdersRead.Get("/:id", orderHandler.GetAdmin)

		adminOrdersWrite := adminGroup.Group("/orders")
		adminOrdersWrite.Use(middleware.RequirePermission(models.PermOrderWrite))
		adminOrdersWrite.Post("/:id/ship", orderHandler.Ship)
		adminOrdersWrite.Post("/:id/complete", orderHandler.Complete)

		adminOrdersRefund := adminGroup.Group("/orders")
		adminOrdersRefund.Use(middleware.RequirePermission(models.PermOrderRefund))
		adminOrdersRefund.Post("/:id/refund", orderHandler.Refund)

		// Certificates — list/view (certificate.manage), regenerate.
		// Media assets (central registry). content.write (content editors + super
		// admin) — taxonomy + media are content-editor tools. Upload via presign
		// (OSS) or direct POST (local dev); then register the asset.
		adminMedia := adminGroup.Group("/media")
		adminMedia.Use(middleware.RequirePermission(models.PermContentWrite))
		adminMedia.Post("/presign", mediaHandler.PresignUpload)
		adminMedia.Post("/upload", mediaHandler.UploadLocal)
		adminMedia.Get("/assets", mediaHandler.ListAssets)
		adminMedia.Post("/assets", mediaHandler.RegisterAsset)
		adminMedia.Delete("/assets/:id", mediaHandler.DeleteAsset)

		// Certificates — list/view (certificate.manage), regenerate.
		adminCerts := adminGroup.Group("/certificates")
		adminCerts.Use(middleware.RequirePermission(models.PermCertificateManage))
		adminCerts.Get("", certificateHandler.ListCertificates)
		adminCerts.Get("/:id", certificateHandler.GetCertificate)
		adminCerts.Post("/:id/regenerate", certificateHandler.Regenerate)
		// Staff account & role management (Super Admin only in v1).
		adminUsers := adminGroup.Group("/users")
		adminUsers.Use(middleware.RequirePermission(models.PermUsersManage))
		{
			adminUsers.Get("", userHandler.AdminListUsers)
			adminUsers.Put("/:user_id/role", userHandler.AdminAssignRole)
		}

		// --- CMS: History & Heritage (ceramic stories) ---
		// PermContentWrite (content_editor + super admin): create, edit, delete, submit.
		// PermContentPublish (super admin only): approve, reject, unpublish.
		adminStories := adminGroup.Group("/ceramicstory")
		adminStories.Use(middleware.RequirePermission(models.PermContentWrite))
		{
			adminStories.Get("", csHandler.AdminListStories)
			adminStories.Get("/:slug", csHandler.AdminGetStory)
			adminStories.Post("", csHandler.AdminCreateStory)
			adminStories.Put("/:id", csHandler.AdminUpdateStory)
			adminStories.Delete("/:id", csHandler.AdminDeleteStory)
			adminStories.Post("/:id/submit", csHandler.AdminSubmitStory)
		}
		// Publish-gated transitions: separate sub-group with stricter permission.
		adminStoriesPublish := adminGroup.Group("/ceramicstory")
		adminStoriesPublish.Use(middleware.RequirePermission(models.PermContentPublish))
		{
			adminStoriesPublish.Post("/:id/approve", csHandler.AdminApproveStory)
			adminStoriesPublish.Post("/:id/reject", csHandler.AdminRejectStory)
			adminStoriesPublish.Post("/:id/unpublish", csHandler.AdminUnpublishStory)
		}

		// --- CMS: Destinations & Local Lifestyle (engage) ---
		adminEngage := adminGroup.Group("/engage")
		adminEngage.Use(middleware.RequirePermission(models.PermContentWrite))
		{
			adminEngage.Get("", engageHandler.AdminListActivities)
			adminEngage.Get("/:slug", engageHandler.AdminGetActivity)
			adminEngage.Post("", engageHandler.AdminCreateActivity)
			adminEngage.Put("/:id", engageHandler.AdminUpdateActivity)
			adminEngage.Delete("/:id", engageHandler.AdminDeleteActivity)
			adminEngage.Post("/:id/submit", engageHandler.AdminSubmitActivity)
		}
		adminEngagePublish := adminGroup.Group("/engage")
		adminEngagePublish.Use(middleware.RequirePermission(models.PermContentPublish))
		{
			adminEngagePublish.Post("/:id/approve", engageHandler.AdminApproveActivity)
			adminEngagePublish.Post("/:id/reject", engageHandler.AdminRejectActivity)
			adminEngagePublish.Post("/:id/unpublish", engageHandler.AdminUnpublishActivity)
		}

		// --- CMS: Artist profiles (PRD §3.1.3 / §3.2.1) ---
		adminArtists := adminGroup.Group("/artists")
		adminArtists.Use(middleware.RequirePermission(models.PermContentWrite))
		{
			adminArtists.Get("", artistHandler.AdminListArtists)
			adminArtists.Get("/:slug", artistHandler.AdminGetArtist)
			adminArtists.Post("", artistHandler.AdminCreateArtist)
			adminArtists.Put("/:id", artistHandler.AdminUpdateArtist)
			adminArtists.Delete("/:id", artistHandler.AdminDeleteArtist)
			adminArtists.Post("/:id/submit", artistHandler.AdminSubmitArtist)
		}
		adminArtistsPublish := adminGroup.Group("/artists")
		adminArtistsPublish.Use(middleware.RequirePermission(models.PermContentPublish))
		{
			adminArtistsPublish.Post("/:id/approve", artistHandler.AdminApproveArtist)
			adminArtistsPublish.Post("/:id/reject", artistHandler.AdminRejectArtist)
			adminArtistsPublish.Post("/:id/unpublish", artistHandler.AdminUnpublishArtist)
		}

		// --- CMS: Product Catalog (PRD §3.2.1) ---
		// PermProductWrite (ecommerce_operator + super admin): create, edit, delete,
		// submit, manage SKUs. PermProductPublish (super admin only): approve/reject/unpublish.
		adminProducts := adminGroup.Group("/products")
		adminProducts.Use(middleware.RequirePermission(models.PermProductWrite))
		{
			adminProducts.Get("", productHandler.AdminListProducts)
			adminProducts.Get("/:slug", productHandler.AdminGetProduct)
			adminProducts.Post("", productHandler.AdminCreateProduct)
			adminProducts.Put("/:id", productHandler.AdminUpdateProduct)
			adminProducts.Delete("/:id", productHandler.AdminDeleteProduct)
			adminProducts.Post("/:id/submit", productHandler.AdminSubmitProduct)
			// SKU management (nested under the product for creation).
			adminProducts.Post("/:id/skus", productHandler.AdminCreateSKU)
			// Ordered media gallery (attach/detach/reorder/list).
			adminProducts.Get("/:id/media", mediaHandler.ListProductMedia)
			adminProducts.Post("/:id/media", mediaHandler.AttachToProduct)
			adminProducts.Delete("/:id/media/:media_id", mediaHandler.DetachFromProduct)
			adminProducts.Patch("/:id/media/order", mediaHandler.ReorderProductMedia)
		}
		adminProductsPublish := adminGroup.Group("/products")
		adminProductsPublish.Use(middleware.RequirePermission(models.PermProductPublish))
		{
			adminProductsPublish.Post("/:id/approve", productHandler.AdminApproveProduct)
			adminProductsPublish.Post("/:id/reject", productHandler.AdminRejectProduct)
			adminProductsPublish.Post("/:id/unpublish", productHandler.AdminUnpublishProduct)
		}
		// SKU update/delete live under /admin/skus (flat, not nested).
		adminSKUs := adminGroup.Group("/skus")
		adminSKUs.Use(middleware.RequirePermission(models.PermProductWrite))
		{
			adminSKUs.Put("/:id", productHandler.AdminUpdateSKU)
			adminSKUs.Delete("/:id", productHandler.AdminDeleteSKU)
		}
	}

	/* --- GDPR self-service: account erasure (PRD §4.3) --- */
	// Separate from /profile so the route reads as the privacy action it is.
	// Requires a signed-in user (JWT) + a deliberate {"confirm":"DELETE"} body.
	privacyGroup := app.Group("/privacy")
	privacyGroup.Use(middleware.JWTMAuth(jwtSecretKey))
	{
		privacyGroup.Post("/delete-account", privacyHandler.DeleteAccount)
	}
}
