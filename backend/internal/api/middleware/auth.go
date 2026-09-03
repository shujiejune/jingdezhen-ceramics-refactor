package middleware

import (
	"jingdezhen-ceramics-backend/internal/models"
	"jingdezhen-ceramics-backend/pkg/adapters/tokenblocklist"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMAuth configures and returns Fiber's JWT middleware.
// It uses the jwtSecretKey from the config file (.env). The blocklist is
// consulted after signature+expiry pass: a revoked user_id → 401 even with a
// valid signature (the stopgap for deleted-user token invalidation, TDD §5.1).
// A nil blocklist skips the check (tests / no-Redis path).
func JWTMAuth(jwtSecretKey string, bl tokenblocklist.Blocklist) fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(jwtSecretKey)},
		ContextKey: "user",
		SuccessHandler: func(c *fiber.Ctx) error {
			token := c.Locals("user").(*jwt.Token)
			claims := token.Claims.(jwt.MapClaims)

			c.Locals("userID", claims["user_id"])
			c.Locals("userEmail", claims["email"])
			c.Locals("userRoles", rolesFromClaims(claims["roles"]))

			// Deleted-user token invalidation (TDD §5.1 stopgap). The signature +
			// expiry already passed; if the user is on the denylist, reject now.
			if bl != nil {
				if uid, ok := claims["user_id"].(string); ok && uid != "" {
					if revoked, _ := bl.IsRevoked(c.UserContext(), uid); revoked {
						return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Token revoked"})
					}
				}
			}

			return c.Next()
		},
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			if err.Error() == "Missing or malformed JWT" {
				return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Missing or malformed JWT"})
			}
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired JWT"})
		},
	})
}

// WsAuth authenticates a WebSocket upgrade request via a `?token=<jwt>` query
// parameter. Browser WebSocket cannot set the Authorization header, so the
// /ws route group uses this middleware instead of JWTMAuth (TDD §5.1). The
// token is parsed and validated the same way (HS256 + expiry + blocklist);
// on success it sets the same c.Locals("userID") that WsUpgradeMiddleware
// and the ws handler rely on.
func WsAuth(jwtSecretKey string, bl tokenblocklist.Blocklist) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Missing or malformed JWT"})
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecretKey), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired JWT"})
		}

		uid, _ := claims["user_id"].(string)
		if uid == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Invalid or expired JWT"})
		}

		// Deleted-user token invalidation (same stopgap as JWTMAuth).
		if bl != nil {
			if revoked, _ := bl.IsRevoked(c.UserContext(), uid); revoked {
				return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse{Message: "Token revoked"})
			}
		}

		c.Locals("userID", uid)
		c.Locals("userEmail", claims["email"])
		c.Locals("userRoles", rolesFromClaims(claims["roles"]))
		return c.Next()
	}
}

// rolesFromClaims normalises the roles claim (which decodes as []interface{}
// from MapClaims) into []string.
func rolesFromClaims(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, r := range arr {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// UserRoles returns the authenticated user's role keys from c.Locals.
func UserRoles(c *fiber.Ctx) []string {
	roles, _ := c.Locals("userRoles").([]string)
	return roles
}

// HasRole reports whether the authenticated user holds the given role.
// Super Administrator implicitly satisfies every role check.
func HasRole(c *fiber.Ctx, role string) bool {
	roles := UserRoles(c)
	for _, r := range roles {
		if r == role || r == models.RoleSuperAdmin {
			return true
		}
	}
	return false
}

// RequirePermission returns a middleware that checks the authenticated user
// holds the given permission (directly via role_permissions, or implicitly as
// Super Administrator). Must be used AFTER JWTMAuth.
//
// Permission checks against role_permissions are enforced in the service /
// repository layer against the DB; for request-gating, Super Administrator
// bypasses and a single staff-role presence is sufficient here. Finer
// permission checks happen in services once the RBAC repo (§4.1) lands.
// Until then, RequireRole is the concrete gate.
func RequirePermission(perm string) fiber.Handler {
	// Map each permission to the staff role(s) allowed to satisfy it at the
	// routing layer. Super Administrator always bypasses.
	allowed := permissionRoleGate[perm]
	return func(c *fiber.Ctx) error {
		roles := UserRoles(c)
		if len(roles) == 0 {
			return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{Message: "Permission denied: staff access required"})
		}
		for _, r := range roles {
			if r == models.RoleSuperAdmin {
				return c.Next()
			}
			for _, a := range allowed {
				if a == r {
					return c.Next()
				}
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{Message: "Permission denied: " + perm})
	}
}

// RequireRole gates a route to a specific staff role (Super Admin bypasses).
func RequireRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if HasRole(c, role) || HasRole(c, models.RoleSuperAdmin) {
			return c.Next()
		}
		return c.Status(fiber.StatusForbidden).JSON(models.ErrorResponse{Message: "Permission denied: " + role + " role required"})
	}
}

// permissionRoleGate maps a permission key to the staff roles that may satisfy
// it at the routing layer. Kept conservative (Super Admin always bypasses).
var permissionRoleGate = map[string][]string{
	models.PermUsersManage:       {models.RoleSuperAdmin},
	models.PermContentWrite:      {models.RoleContentEditor},
	models.PermContentPublish:    {models.RoleSuperAdmin},
	models.PermProductRead:       {models.RoleEcommerceOperator},
	models.PermProductWrite:      {models.RoleEcommerceOperator},
	models.PermProductPublish:   {models.RoleSuperAdmin}, // PRD §3.1.1: only Super Admin can approve/publish products
	models.PermCertificateManage: {models.RoleEcommerceOperator},
	models.PermOrderRead:         {models.RoleEcommerceOperator, models.RoleCustomerService, models.RoleTravelPlanner},
	models.PermOrderWrite:        {models.RoleEcommerceOperator},
	models.PermOrderRefund:       {models.RoleEcommerceOperator},
	models.PermItineraryRead:     {models.RoleTravelPlanner, models.RoleCustomerService},
	models.PermItineraryWrite:    {models.RoleTravelPlanner},
	models.PermItineraryConfirm:  {models.RoleTravelPlanner},
	models.PermChatHandle:        {models.RoleTravelPlanner, models.RoleCustomerService},
	models.PermDashboardView:     {models.RoleEcommerceOperator, models.RoleCustomerService},
	models.PermSettingsManage:    {models.RoleSuperAdmin},
}
