package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/jasonKoogler/abraxis/prism/internal/common/api"
	commonlog "github.com/jasonKoogler/abraxis/prism/internal/common/log"
	"github.com/jasonKoogler/abraxis/prism/internal/config"
)

// ServiceAPIHandler provides HTTP handlers for service management
type ServiceAPIHandler struct {
	serviceProxy *ServiceProxy
	logger       *commonlog.Logger
}

// NewServiceAPIHandler creates a new service API handler
func NewServiceAPIHandler(serviceProxy *ServiceProxy, logger *commonlog.Logger) *ServiceAPIHandler {
	return &ServiceAPIHandler{
		serviceProxy: serviceProxy,
		logger:       logger,
	}
}

// RegisterRoutes registers all service and route management endpoints on the chi router.
func (h *ServiceAPIHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/services", func(r chi.Router) {
		r.Get("/", api.Make(h.listServices, h.logger))
		r.Post("/", api.Make(h.createService, h.logger))
		r.Get("/{name}", api.Make(h.getService, h.logger))
		r.Put("/{name}", api.Make(h.updateService, h.logger))
		r.Delete("/{name}", api.Make(h.deleteService, h.logger))
	})
	r.Route("/api/routes", func(r chi.Router) {
		r.Get("/", api.Make(h.listRoutes, h.logger))
		r.Get("/{service}", api.Make(h.listServiceRoutes, h.logger))
		r.Post("/{service}", api.Make(h.addServiceRoute, h.logger))
	})
}

// listServices godoc
// @Summary      List all services
// @Description  Get all registered backend services
// @Tags         services
// @Produce      json
// @Success      200  {array}   config.ServiceConfig
// @Failure      500  {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/services [get]
func (h *ServiceAPIHandler) listServices(w http.ResponseWriter, r *http.Request) error {
	services := h.serviceProxy.ListServices()
	return api.Respond(w, http.StatusOK, services)
}

// createService godoc
// @Summary      Register a new service
// @Description  Register a new backend service with the gateway
// @Tags         services
// @Accept       json
// @Produce      json
// @Param        service  body      config.ServiceConfig  true  "Service configuration"
// @Success      201      {object}  config.ServiceConfig
// @Failure      400      {object}  api.APIError
// @Failure      500      {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/services [post]
func (h *ServiceAPIHandler) createService(w http.ResponseWriter, r *http.Request) error {
	var svcConfig config.ServiceConfig
	if err := json.NewDecoder(r.Body).Decode(&svcConfig); err != nil {
		return api.InvalidJSONError()
	}

	if err := h.serviceProxy.RegisterService(svcConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, svcConfig)
}

// getService godoc
// @Summary      Get a service by name
// @Description  Retrieve configuration for a specific registered service
// @Tags         services
// @Produce      json
// @Param        name  path      string  true  "Service name"
// @Success      200   {object}  config.ServiceConfig
// @Failure      404   {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/services/{name} [get]
func (h *ServiceAPIHandler) getService(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")
	for _, svc := range h.serviceProxy.ListServices() {
		if svc.Name == name {
			return api.Respond(w, http.StatusOK, svc)
		}
	}
	return api.NotFound(nil)
}

// updateService godoc
// @Summary      Update a service
// @Description  Update configuration for an existing service
// @Tags         services
// @Accept       json
// @Produce      json
// @Param        name     path      string                true  "Service name"
// @Param        service  body      config.ServiceConfig  true  "Updated service configuration"
// @Success      200      {object}  config.ServiceConfig
// @Failure      400      {object}  api.APIError
// @Failure      500      {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/services/{name} [put]
func (h *ServiceAPIHandler) updateService(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")

	var svcConfig config.ServiceConfig
	if err := json.NewDecoder(r.Body).Decode(&svcConfig); err != nil {
		return api.InvalidJSONError()
	}

	if svcConfig.Name != name {
		return api.NewError(
			"name-mismatch",
			"Service name in URL does not match body",
			http.StatusBadRequest,
			api.ErrorTypeBadRequest,
			nil,
		)
	}

	if err := h.serviceProxy.UpdateService(svcConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, svcConfig)
}

// deleteService godoc
// @Summary      Delete a service
// @Description  Deregister a service from the gateway
// @Tags         services
// @Produce      json
// @Param        name  path      string  true  "Service name"
// @Success      200   {object}  object{status=string}
// @Failure      500   {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/services/{name} [delete]
func (h *ServiceAPIHandler) deleteService(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "name")

	if err := h.serviceProxy.DeregisterService(name); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// listRoutes godoc
// @Summary      List all routes
// @Description  Get all registered routes, optionally filtered by type (public or protected)
// @Tags         routes
// @Produce      json
// @Param        type  query     string  false  "Filter by route type"  Enums(public, protected)
// @Success      200   {array}   RouteMetadata
// @Failure      500   {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/routes [get]
func (h *ServiceAPIHandler) listRoutes(w http.ResponseWriter, r *http.Request) error {
	filterType := r.URL.Query().Get("type")

	var routes []RouteMetadata
	switch filterType {
	case "public":
		routes = h.serviceProxy.ListPublicRoutes()
	case "protected":
		routes = h.serviceProxy.ListProtectedRoutes()
	default:
		routes = h.serviceProxy.ListRoutes()
	}

	return api.Respond(w, http.StatusOK, routes)
}

// listServiceRoutes godoc
// @Summary      List routes for a service
// @Description  Get all registered routes for a specific service
// @Tags         routes
// @Produce      json
// @Param        service  path      string  true  "Service name"
// @Success      200      {array}   RouteMetadata
// @Failure      404      {object}  api.APIError
// @Failure      500      {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/routes/{service} [get]
func (h *ServiceAPIHandler) listServiceRoutes(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "service")

	routes, err := h.serviceProxy.ListServiceRoutes(serviceName)
	if err != nil {
		return api.NotFound(err)
	}

	return api.Respond(w, http.StatusOK, routes)
}

// addServiceRoute godoc
// @Summary      Add a route to a service
// @Description  Register a new custom route for a specific service
// @Tags         routes
// @Accept       json
// @Produce      json
// @Param        service  path      string             true  "Service name"
// @Param        route    body      config.RouteConfig  true  "Route configuration"
// @Success      201      {object}  config.RouteConfig
// @Failure      400      {object}  api.APIError
// @Failure      500      {object}  api.APIError
// @Security     BearerAuth
// @Router       /api/routes/{service} [post]
func (h *ServiceAPIHandler) addServiceRoute(w http.ResponseWriter, r *http.Request) error {
	serviceName := chi.URLParam(r, "service")

	var routeConfig config.RouteConfig
	if err := json.NewDecoder(r.Body).Decode(&routeConfig); err != nil {
		return api.InvalidJSONError()
	}

	if err := h.serviceProxy.RegisterServiceRoute(serviceName, routeConfig); err != nil {
		return api.InternalError(err)
	}

	return api.Respond(w, http.StatusCreated, routeConfig)
}
