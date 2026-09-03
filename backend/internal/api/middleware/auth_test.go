package middleware_test

// Unit tests for the RBAC permission gate (TDD §11 priority: RBAC middleware
// matrix). The middleware reads the authenticated user's roles from
// c.Locals("userRoles") (set by JWTMAuth's SuccessHandler from the JWT claims)
// and gates against the static `permissionRoleGate` map. Super Admin bypasses
// every gate; each permission maps to one or more staff roles.
//
// These are unit tests (no DB, no testcontainers): the gate is pure control
// flow over a static map + the roles in Locals. A DB-backed permission check
// (role_permissions join) would be a separate integration test, but the
// routing-layer gate is the security boundary that must not regress silently
// — a widened gate here is a permission escalation.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jingdezhen-ceramics-backend/internal/api/middleware"
	"jingdezhen-ceramics-backend/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

// rbacCase is one row of the RBAC matrix: given these roles, does this
// permission gate let the request through?
type rbacCase struct {
	name       string
	perm       string
	roles      []string
	wantStatus int // 200 = allowed, 403 = denied
}

// runGate builds a throwaway Fiber app: a "pre" handler seeds the roles into
// Locals (mimicking what JWTMAuth's SuccessHandler does for a real request),
// then RequirePermission gates, then a sentinel returns 200. The response
// status tells us whether the gate passed.
func runGate(t *testing.T, perm string, roles []string) int {
	t.Helper()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userRoles", roles)
		return c.Next()
	}, middleware.RequirePermission(perm))
	app.Use(func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := mustRequest()
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp.StatusCode
}

func mustRequest() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) }

// TestRequirePermission_RBACMatrix is the TDD §11 RBAC matrix: for every
// permission in the gate, assert the exact set of roles that satisfy it and
// that super_admin bypasses all. A regression here is a privilege escalation.
func TestRequirePermission_RBACMatrix(t *testing.T) {
	// One row per (permission, role-combo) the gate must rule on. 200 = allow.
	// Order groups by permission for readability; the matrix is exhaustive for
	// the staff roles + super_admin bypass + the "no roles → 403" case.
	cases := []rbacCase{
		// --- users.manage / settings.manage / content.publish: super_admin only ---
		{perm: models.PermUsersManage, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermUsersManage, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},
		{perm: models.PermSettingsManage, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermSettingsManage, roles: []string{models.RoleContentEditor}, wantStatus: 403},
		{perm: models.PermContentPublish, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermContentPublish, roles: []string{models.RoleContentEditor}, wantStatus: 403},

		// --- content.write: content_editor only (plus super_admin bypass) ---
		{perm: models.PermContentWrite, roles: []string{models.RoleContentEditor}, wantStatus: 200},
		{perm: models.PermContentWrite, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},

		// --- product.read / product.write / certificate.manage: ecommerce_operator ---
		{perm: models.PermProductRead, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermProductRead, roles: []string{models.RoleCustomerService}, wantStatus: 403},
		{perm: models.PermProductWrite, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermProductWrite, roles: []string{models.RoleTravelPlanner}, wantStatus: 403},
		{perm: models.PermCertificateManage, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermCertificateManage, roles: []string{models.RoleContentEditor}, wantStatus: 403},

		// --- order.read: ecommerce_operator + customer_service + travel_planner ---
		{perm: models.PermOrderRead, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermOrderRead, roles: []string{models.RoleCustomerService}, wantStatus: 200},
		{perm: models.PermOrderRead, roles: []string{models.RoleTravelPlanner}, wantStatus: 200},
		{perm: models.PermOrderRead, roles: []string{models.RoleContentEditor}, wantStatus: 403},
		// --- order.write / order.refund: ecommerce_operator only ---
		{perm: models.PermOrderWrite, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermOrderWrite, roles: []string{models.RoleCustomerService}, wantStatus: 403},
		{perm: models.PermOrderRefund, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermOrderRefund, roles: []string{models.RoleCustomerService}, wantStatus: 403},

		// --- itinerary.*: travel_planner (read also customer_service) ---
		{perm: models.PermItineraryRead, roles: []string{models.RoleTravelPlanner}, wantStatus: 200},
		{perm: models.PermItineraryRead, roles: []string{models.RoleCustomerService}, wantStatus: 200},
		{perm: models.PermItineraryRead, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},
		{perm: models.PermItineraryWrite, roles: []string{models.RoleTravelPlanner}, wantStatus: 200},
		{perm: models.PermItineraryWrite, roles: []string{models.RoleCustomerService}, wantStatus: 403},
		{perm: models.PermItineraryConfirm, roles: []string{models.RoleTravelPlanner}, wantStatus: 200},
		{perm: models.PermItineraryConfirm, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},

		// --- chat.handle: travel_planner + customer_service ---
		{perm: models.PermChatHandle, roles: []string{models.RoleTravelPlanner}, wantStatus: 200},
		{perm: models.PermChatHandle, roles: []string{models.RoleCustomerService}, wantStatus: 200},
		{perm: models.PermChatHandle, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},

		// --- dashboard.view: ecommerce_operator + customer_service ---
		{perm: models.PermDashboardView, roles: []string{models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermDashboardView, roles: []string{models.RoleCustomerService}, wantStatus: 200},
		{perm: models.PermDashboardView, roles: []string{models.RoleTravelPlanner}, wantStatus: 403},

		// --- product.publish: super_admin only (PRD §3.1.1) ---
		// Explicitly in permissionRoleGate so the gate is symmetric with
		// PermContentPublish. The DB seed (000002 + 000012) grants
		// product.publish to super_admin only.
		{perm: models.PermProductPublish, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermProductPublish, roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},
		{perm: models.PermProductPublish, roles: []string{models.RoleContentEditor}, wantStatus: 403},

		// --- super_admin bypasses EVERY gate, even ones not in the map ---
		{perm: models.PermProductRead, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermSettingsManage, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},
		{perm: models.PermItineraryConfirm, roles: []string{models.RoleSuperAdmin}, wantStatus: 200},

		// --- multi-role: holding ANY allowed role passes ---
		{perm: models.PermOrderRead, roles: []string{models.RoleContentEditor, models.RoleEcommerceOperator}, wantStatus: 200},
		{perm: models.PermOrderRead, roles: []string{models.RoleCustomerService, models.RoleContentEditor}, wantStatus: 200},

		// --- no roles → 403 (unauthenticated requests are blocked by JWT first,
		//     but the gate must still deny if Locals is empty) ---
		{perm: models.PermProductRead, roles: nil, wantStatus: 403},
		{perm: models.PermProductRead, roles: []string{}, wantStatus: 403},

		// --- unknown permission key (not in the map, not super) → 403 ---
		{perm: "nonexistent.perm", roles: []string{models.RoleEcommerceOperator}, wantStatus: 403},
		{perm: "nonexistent.perm", roles: []string{models.RoleSuperAdmin}, wantStatus: 200}, // bypass
	}
	for _, tc := range cases {
		t.Run(tc.name+"_"+tc.perm, func(t *testing.T) {
			got := runGate(t, tc.perm, tc.roles)
			require.Equalf(t, tc.wantStatus, got,
				"perm=%s roles=%v: want %d got %d", tc.perm, tc.roles, tc.wantStatus, got)
		})
	}
}

// TestRequireRole gates on a single role key (Super Admin bypasses). This is
// the older gate; RequirePermission is preferred for new routes, but RequireRole
// is still in use so its behavior is locked here too.
func TestRequireRole(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		roles []string
		want  int
	}{
		{"exact_role", models.RoleEcommerceOperator, []string{models.RoleEcommerceOperator}, 200},
		{"wrong_role", models.RoleEcommerceOperator, []string{models.RoleContentEditor}, 403},
		{"super_bypasses", models.RoleEcommerceOperator, []string{models.RoleSuperAdmin}, 200},
		{"super_holds_target_too", models.RoleSuperAdmin, []string{models.RoleSuperAdmin}, 200},
		{"no_roles", models.RoleEcommerceOperator, nil, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{DisableStartupMessage: true})
			app.Use(func(c *fiber.Ctx) error { c.Locals("userRoles", tc.roles); return c.Next() },
				middleware.RequireRole(tc.role))
			app.Use(func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
			resp, err := app.Test(mustRequest(), -1)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.StatusCode)
		})
	}
}
