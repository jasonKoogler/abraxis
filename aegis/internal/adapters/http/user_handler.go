package http

import (
	"net/http"

	"github.com/jasonKoogler/aegis/internal/common/api"
	"github.com/jasonKoogler/aegis/internal/ports"
)

type userHandler struct {
	userService ports.UserService
}

func NewUserHandler(service ports.UserService) *userHandler {
	return &userHandler{
		userService: service,
	}
}

func (h *userHandler) getUsers(w http.ResponseWriter, r *http.Request, page, pageSize *int) error {
	api.ParsePagination(page, pageSize)

	users, err := h.userService.ListAll(r.Context(), *page, *pageSize)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, users)
}

func (h *userHandler) deleteUser(w http.ResponseWriter, r *http.Request, id string) error {
	if id == "" {
		return api.MissingIDError()
	}

	err := h.userService.Delete(r.Context(), id)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusNoContent, nil)
}

func (h *userHandler) getUserByID(w http.ResponseWriter, r *http.Request, id string) error {
	if id == "" {
		return api.MissingIDError()
	}

	user, err := h.userService.GetByID(r.Context(), id)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, user)
}

func (h *userHandler) getUserByEmail(w http.ResponseWriter, r *http.Request, email string) error {
	if email == "" {
		return api.MissingFieldError("email")
	}

	user, err := h.userService.GetByEmail(r.Context(), email)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, user)
}

func (h *userHandler) updateUserByID(w http.ResponseWriter, r *http.Request, id string) error {
	params, err := UpdateUserRequestToParams(r)
	if err != nil {
		return api.InvalidJSONError()
	}

	if id == "" {
		return api.MissingIDError()
	}

	user, err := h.userService.Update(r.Context(), id, params)
	if err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, user)
}

// func (h *userHandler) getByOrganizationID(w http.ResponseWriter, r *http.Request) error {
// 	// organizationID := chi.URLParam(r, "organizationID")
// 	// if organizationID == "" {
// 	// 	return api.MissingFieldError("organizationID")
// 	// }

// 	limit, offset, err := api.LimitAndOffsetFromRequest(r)
// 	if err != nil {
// 		return api.InvalidJSONError()
// 	}

// 	users, err := h.userService.GetByOrganizationID(r.Context(), organizationID, limit, offset)
// 	if err != nil {
// 		return api.InternalError(err)
// 	}

// 	return api.RespondWithJSON(w, http.StatusOK, users)
// }

// func (h *userHandler) getUserRoles(w http.ResponseWriter, r *http.Request, id string) error {
// 	if id == "" {
// 		return api.MissingIDError()
// 	}

// 	roles, err := h.userService.GetRoles(r.Context(), id)
// 	if err != nil {
// 		return api.InternalError(err)
// 	}

// 	return api.Respond(w, http.StatusOK, roles)
// }
