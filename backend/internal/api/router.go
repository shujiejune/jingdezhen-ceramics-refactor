package api

import (
	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/internal/modules/address"
	"jingdezhen-ceramics-backend/internal/modules/ceramicstory"
	"jingdezhen-ceramics-backend/internal/modules/consent"
	"jingdezhen-ceramics-backend/internal/modules/engage"
	"jingdezhen-ceramics-backend/internal/modules/gallery"
	"jingdezhen-ceramics-backend/internal/modules/notification"
	"jingdezhen-ceramics-backend/internal/modules/twofa"
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
	galleryHandler *gallery.Handler,
	engageHandler *engage.Handler,
	addressHandler *address.Handler,
	consentHandler *consent.Handler,
	twoFAHandler *twofa.Handler,
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

	/* --- Gallery (Public for viewing, Protected for actions) --- */
	gGroup := app.Group("/gallery")
	{
		gGroup.Get("/artworks", galleryHandler.GetArtworks) // Params: ?category=...&artist=...
		gGroup.Get("/artworks/:artwork_id", galleryHandler.GetArtworkByID)
		gGroup.Get("/artists", galleryHandler.GetArtists)
		gGroup.Get("/artists/:artist_id", galleryHandler.GetArtistByID)
		gGroup.Get("/categories", galleryHandler.GetGalleryCategories)

		// Protected actions for gallery
		authGalleryGroup := gGroup.Group("")
		authGalleryGroup.Use(middleware.JWTMAuth(jwtSecretKey))
		{
			authGalleryGroup.Get("/favorites", galleryHandler.GetFavoriteArtworks)
			authGalleryGroup.Post("/artworks/:artwork_id/favorite", galleryHandler.MarkAsFavorite)
			authGalleryGroup.Delete("/artworks/:artwork_id/favorite", galleryHandler.UnmarkAsFavorite)
		}
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
		// Staff account & role management (Super Admin only in v1).
		adminUsers := adminGroup.Group("/users")
		adminUsers.Use(middleware.RequirePermission(models.PermUsersManage))
		{
			adminUsers.Get("", userHandler.AdminListUsers)
			adminUsers.Put("/:user_id/role", userHandler.AdminAssignRole)
		}
		// CMS modules (content, products, orders, itinerary CRM, dashboard)
		// land in later milestones — each with its own RequirePermission.
	}
}
