package auth

// Entitlement is a type representation of a permission as it applies to a particular ObjectType.
type Entitlement string

const (
	// Entitlements that apply to all resources.
	EntitlementCanCreate Entitlement = "can_create"
	EntitlementCanDelete Entitlement = "can_delete"
	EntitlementCanEdit   Entitlement = "can_edit"
	EntitlementCanView   Entitlement = "can_view"
)

// ObjectType is a type of resource within the migration manager.
type ObjectType string

const (
	// ObjectTypeUser represents a user.
	ObjectTypeUser ObjectType = "user"

	// ObjectTypeServer represents a server.
	ObjectTypeServer ObjectType = "server"
)
