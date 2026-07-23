package models

// Role key constants — must match the `roles` table seed in migration 000002_rbac.
const (
	RoleSuperAdmin        = "super_admin"
	RoleContentEditor     = "content_editor"
	RoleTravelPlanner     = "travel_planner"
	RoleEcommerceOperator = "ecommerce_operator"
	RoleCustomerService   = "customer_service"
)

// StaffRoles is the set of fixed staff roles for v1 (PRD §3.4.1 — no custom roles).
var StaffRoles = []string{
	RoleSuperAdmin,
	RoleContentEditor,
	RoleTravelPlanner,
	RoleEcommerceOperator,
	RoleCustomerService,
}

// IsStaffRole reports whether key is one of the fixed staff roles.
func IsStaffRole(key string) bool {
	for _, r := range StaffRoles {
		if r == key {
			return true
		}
	}
	return false
}

// Permission key constants — must match the `permissions` table seed (000002_rbac).
// Reference these in middleware.RequirePermission(...) so renames surface at compile time.
const (
	PermUsersManage      = "users.manage"
	PermContentWrite     = "content.write"
	PermContentPublish   = "content.publish"
	PermProductRead      = "product.read"
	PermProductWrite     = "product.write"
	PermProductPublish   = "product.publish"   // Super Admin only — approve/publish products (PRD §3.1.1)
	PermCertificateManage = "certificate.manage" // List/view/regenerate product certificates (PRD §3.2.1)
	PermOrderRead        = "order.read"
	PermOrderWrite       = "order.write"
	PermOrderRefund      = "order.refund"
	PermItineraryRead    = "itinerary.read"
	PermItineraryWrite   = "itinerary.write"
	PermItineraryConfirm = "itinerary.confirm"
	PermChatHandle       = "chat.handle"
	PermDashboardView    = "dashboard.view"
	PermSettingsManage   = "settings.manage"
)
