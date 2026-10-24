package domain

import (
	"context"

	"govite/internal/kernel"
)

type Grant struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Scope    string `json:"scope"`
}

type Scope struct {
	Kind               string
	UserID             string
	SubordinateUserIDs []string
}

type Principal struct {
	ID     string
	RoleID string
}

type GrantRepository interface {
	Grants(context.Context, string) ([]Grant, error)
}

type GrantCache interface {
	Get(context.Context, string) ([]Grant, bool)
	Set(context.Context, string, []Grant)
	Delete(context.Context, string)
}

type TeamHierarchy func(context.Context, string) ([]string, error)

var ErrTeamScope = kernel.E(kernel.Forbidden, "TEAM_SCOPE_UNAVAILABLE", "Team scope requires a hierarchy provider")

func ErrNotGranted(permission string) error {
	return kernel.E(kernel.Forbidden, "PERMISSION_NOT_GRANTED", "Permission not granted: "+permission)
}
