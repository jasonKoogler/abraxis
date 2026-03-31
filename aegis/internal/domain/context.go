package domain

import (
	"context"
)

// Context key for storing/retrieving the user.
type contextKey string

const userContextDataContextKey = contextKey("user_context_data")

// UserContextDataFromContext retrieves the user context data from the context.
func UserContextDataFromContext(ctx context.Context) (*UserContextData, bool) {
	usr, ok := ctx.Value(userContextDataContextKey).(*UserContextData)
	return usr, ok
}

// ContextWithUserContextData adds the user context data to the context.
func ContextWithUserContextData(ctx context.Context, usr *UserContextData) context.Context {
	return context.WithValue(ctx, userContextDataContextKey, usr)
}

// UserContextData carries essential user info for authorization and logging.
type UserContextData struct {
	UserID       string
	Roles        RoleMap
	AuthProvider string
	SessionID    string
}

func RolesFromContext(ctx context.Context) RoleMap {
	userContextData, ok := UserContextDataFromContext(ctx)
	if !ok {
		return nil
	}
	return userContextData.Roles
}
