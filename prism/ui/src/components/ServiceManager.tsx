import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
  CheckCircle as HealthyIcon,
  Refresh as RefreshIcon,
  Error as UnhealthyIcon,
} from "@mui/icons-material";
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  FormGroup,
  IconButton,
  Paper,
  Snackbar,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import React, { useEffect, useState } from "react";
import { useAuth } from "../hooks/useAuth";

// Types
interface Service {
  id: string;
  name: string;
  url: string;
  health_status: "healthy" | "unhealthy";
  last_checked: string;
}

interface Route {
  path: string;
  method: string;
  service_id: string;
  service_name: string;
  public: boolean;
  required_scopes: string[];
  priority: number;
}

interface ServiceFormData {
  id: string;
  name: string;
  url: string;
  health_check_path: string;
  requires_auth: boolean;
  allowed_methods: string[];
  timeout: string;
  retry_count: number;
}

interface RouteFormData {
  service_id: string;
  path: string;
  method: string;
  public: boolean;
  required_scopes: string[];
  priority: number;
}

const defaultServiceForm: ServiceFormData = {
  id: "",
  name: "",
  url: "",
  health_check_path: "/health",
  requires_auth: false,
  allowed_methods: ["GET", "POST", "PUT", "DELETE"],
  timeout: "30s",
  retry_count: 3,
};

const defaultRouteForm: RouteFormData = {
  service_id: "",
  path: "",
  method: "GET",
  public: false,
  required_scopes: [],
  priority: 10,
};

const httpMethods = [
  "GET",
  "POST",
  "PUT",
  "DELETE",
  "PATCH",
  "OPTIONS",
  "HEAD",
];

export const ServiceManager: React.FC = () => {
  const { isAuthenticated, getAccessToken } = useAuth();
  const [activeTab, setActiveTab] = useState(0);
  const [services, setServices] = useState<Service[]>([]);
  const [routes, setRoutes] = useState<Route[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [openServiceDialog, setOpenServiceDialog] = useState(false);
  const [openRouteDialog, setOpenRouteDialog] = useState(false);
  const [serviceForm, setServiceForm] =
    useState<ServiceFormData>(defaultServiceForm);
  const [routeForm, setRouteForm] = useState<RouteFormData>(defaultRouteForm);
  const [editingService, setEditingService] = useState<string | null>(null);
  const [editingRoute, setEditingRoute] = useState<string | null>(null);
  const [scopeInput, setScopeInput] = useState("");
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Fetch data when component mounts
  useEffect(() => {
    fetchServices();
    if (isAuthenticated) {
      fetchRoutes();
    }
  }, [isAuthenticated]);

  // Handle tab change
  const handleTabChange = (_event: React.SyntheticEvent, newValue: number) => {
    setActiveTab(newValue);
  };

  // Fetch services
  const fetchServices = async () => {
    setLoading(true);
    try {
      const response = await fetch("/api/services");
      if (!response.ok) {
        throw new Error(
          `Failed to fetch services: ${response.status} ${response.statusText}`
        );
      }
      const data = await response.json();
      setServices(data.services || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Fetch routes
  const fetchRoutes = async () => {
    if (!isAuthenticated) return;

    setLoading(true);
    try {
      const token = await getAccessToken();
      const response = await fetch("/api/routes", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch routes: ${response.status} ${response.statusText}`
        );
      }

      const data = await response.json();
      setRoutes(data.routes || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Open service dialog for creating/editing
  const openServiceForm = (service?: Service) => {
    if (service) {
      // Editing existing service
      setEditingService(service.id);
      setServiceForm({
        id: service.id,
        name: service.name,
        url: service.url,
        health_check_path: "/health", // Default, would need to get actual from API
        requires_auth: false, // Default, would need to get actual from API
        allowed_methods: ["GET", "POST", "PUT", "DELETE"], // Default
        timeout: "30s", // Default
        retry_count: 3, // Default
      });
    } else {
      // Creating new service
      setEditingService(null);
      setServiceForm(defaultServiceForm);
    }
    setOpenServiceDialog(true);
  };

  // Open route dialog for creating/editing
  const openRouteForm = (route?: Route) => {
    if (route) {
      // Editing existing route
      setEditingRoute(`${route.service_id}-${route.method}-${route.path}`);
      setRouteForm({
        service_id: route.service_id,
        path: route.path,
        method: route.method,
        public: route.public,
        required_scopes: [...route.required_scopes],
        priority: route.priority,
      });
    } else {
      // Creating new route
      setEditingRoute(null);
      setRouteForm({
        ...defaultRouteForm,
        service_id: services.length > 0 ? services[0].id : "", // Set first service as default
      });
    }
    setOpenRouteDialog(true);
  };

  // Handle service form input change
  const handleServiceChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value, checked, type } = e.target;
    setServiceForm((prev) => ({
      ...prev,
      [name]: type === "checkbox" ? checked : value,
    }));
  };

  // Handle route form input change
  const handleRouteChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value, checked, type } = e.target;
    setRouteForm((prev) => ({
      ...prev,
      [name]: type === "checkbox" ? checked : value,
    }));
  };

  // Handle method checkbox change
  const handleMethodCheckbox = (method: string, checked: boolean) => {
    setServiceForm((prev) => {
      const methods = new Set(prev.allowed_methods);
      if (checked) {
        methods.add(method);
      } else {
        methods.delete(method);
      }
      return {
        ...prev,
        allowed_methods: Array.from(methods),
      };
    });
  };

  // Add a scope to the required_scopes list
  const addScope = () => {
    if (!scopeInput.trim()) return;

    setRouteForm((prev) => ({
      ...prev,
      required_scopes: [...prev.required_scopes, scopeInput.trim()],
    }));
    setScopeInput("");
  };

  // Remove a scope from the required_scopes list
  const removeScope = (scope: string) => {
    setRouteForm((prev) => ({
      ...prev,
      required_scopes: prev.required_scopes.filter((s) => s !== scope),
    }));
  };

  // Submit service form
  const submitServiceForm = async () => {
    if (!serviceForm.id || !serviceForm.name || !serviceForm.url) {
      setError("Service ID, name, and URL are required");
      return;
    }

    setLoading(true);
    try {
      const token = await getAccessToken();
      const url = editingService
        ? `/api/services/${editingService}`
        : "/api/services";

      const method = editingService ? "PUT" : "POST";

      const response = await fetch(url, {
        method,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(serviceForm),
      });

      if (!response.ok) {
        throw new Error(
          `Failed to ${editingService ? "update" : "create"} service: ${
            response.status
          } ${response.statusText}`
        );
      }

      setSuccessMessage(
        `Service ${editingService ? "updated" : "created"} successfully`
      );
      setOpenServiceDialog(false);
      fetchServices();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Submit route form
  const submitRouteForm = async () => {
    if (!routeForm.service_id || !routeForm.path || !routeForm.method) {
      setError("Service ID, path, and method are required");
      return;
    }

    setLoading(true);
    try {
      const token = await getAccessToken();
      const url = editingRoute ? `/api/routes/${editingRoute}` : "/api/routes";

      const method = editingRoute ? "PUT" : "POST";

      const response = await fetch(url, {
        method,
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(routeForm),
      });

      if (!response.ok) {
        throw new Error(
          `Failed to ${editingRoute ? "update" : "create"} route: ${
            response.status
          } ${response.statusText}`
        );
      }

      setSuccessMessage(
        `Route ${editingRoute ? "updated" : "created"} successfully`
      );
      setOpenRouteDialog(false);
      fetchRoutes();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Delete a service
  const deleteService = async (serviceId: string) => {
    if (!confirm(`Are you sure you want to delete service "${serviceId}"?`)) {
      return;
    }

    setLoading(true);
    try {
      const token = await getAccessToken();
      const response = await fetch(`/api/services/${serviceId}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error(
          `Failed to delete service: ${response.status} ${response.statusText}`
        );
      }

      setSuccessMessage("Service deleted successfully");
      fetchServices();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  // Delete a route
  const deleteRoute = async (routeId: string) => {
    if (!confirm("Are you sure you want to delete this route?")) {
      return;
    }

    setLoading(true);
    try {
      const token = await getAccessToken();
      const response = await fetch(`/api/routes/${routeId}`, {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error(
          `Failed to delete route: ${response.status} ${response.statusText}`
        );
      }

      setSuccessMessage("Route deleted successfully");
      fetchRoutes();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Container maxWidth="lg">
      <Box sx={{ mt: 4, mb: 2 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Service Manager
        </Typography>

        <Tabs value={activeTab} onChange={handleTabChange} sx={{ mb: 2 }}>
          <Tab label="Services" />
          <Tab label="Routes" disabled={!isAuthenticated} />
        </Tabs>

        {/* Error alert */}
        {error && (
          <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
            {error}
          </Alert>
        )}

        {/* Loading indicator */}
        {loading && (
          <Box sx={{ display: "flex", justifyContent: "center", mt: 2, mb: 2 }}>
            <CircularProgress />
          </Box>
        )}

        {/* Services tab */}
        {activeTab === 0 && (
          <Box>
            <Box
              sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}
            >
              <Typography variant="h6">Services</Typography>
              <Box>
                <Tooltip title="Refresh">
                  <IconButton
                    onClick={fetchServices}
                    size="small"
                    sx={{ mr: 1 }}
                  >
                    <RefreshIcon />
                  </IconButton>
                </Tooltip>
                {isAuthenticated && (
                  <Button
                    variant="contained"
                    startIcon={<AddIcon />}
                    onClick={() => openServiceForm()}
                  >
                    Add Service
                  </Button>
                )}
              </Box>
            </Box>

            <TableContainer component={Paper}>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>ID</TableCell>
                    <TableCell>Name</TableCell>
                    <TableCell>URL</TableCell>
                    <TableCell>Health</TableCell>
                    <TableCell>Last Checked</TableCell>
                    {isAuthenticated && (
                      <TableCell align="right">Actions</TableCell>
                    )}
                  </TableRow>
                </TableHead>
                <TableBody>
                  {services.length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={isAuthenticated ? 6 : 5}
                        align="center"
                      >
                        No services found
                      </TableCell>
                    </TableRow>
                  ) : (
                    services.map((service) => (
                      <TableRow key={service.id}>
                        <TableCell>{service.id}</TableCell>
                        <TableCell>{service.name}</TableCell>
                        <TableCell>{service.url}</TableCell>
                        <TableCell>
                          {service.health_status === "healthy" ? (
                            <Chip
                              icon={<HealthyIcon />}
                              label="Healthy"
                              color="success"
                              size="small"
                            />
                          ) : (
                            <Chip
                              icon={<UnhealthyIcon />}
                              label="Unhealthy"
                              color="error"
                              size="small"
                            />
                          )}
                        </TableCell>
                        <TableCell>
                          {new Date(service.last_checked).toLocaleString()}
                        </TableCell>
                        {isAuthenticated && (
                          <TableCell align="right">
                            <Tooltip title="Edit">
                              <IconButton
                                size="small"
                                onClick={() => openServiceForm(service)}
                              >
                                <EditIcon />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="Delete">
                              <IconButton
                                size="small"
                                onClick={() => deleteService(service.id)}
                                color="error"
                              >
                                <DeleteIcon />
                              </IconButton>
                            </Tooltip>
                          </TableCell>
                        )}
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        )}

        {/* Routes tab */}
        {activeTab === 1 && isAuthenticated && (
          <Box>
            <Box
              sx={{ display: "flex", justifyContent: "space-between", mb: 2 }}
            >
              <Typography variant="h6">Routes</Typography>
              <Box>
                <Tooltip title="Refresh">
                  <IconButton onClick={fetchRoutes} size="small" sx={{ mr: 1 }}>
                    <RefreshIcon />
                  </IconButton>
                </Tooltip>
                <Button
                  variant="contained"
                  startIcon={<AddIcon />}
                  onClick={() => openRouteForm()}
                  disabled={services.length === 0}
                >
                  Add Route
                </Button>
              </Box>
            </Box>

            <TableContainer component={Paper}>
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>Path</TableCell>
                    <TableCell>Method</TableCell>
                    <TableCell>Service</TableCell>
                    <TableCell>Public</TableCell>
                    <TableCell>Required Scopes</TableCell>
                    <TableCell>Priority</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {routes.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} align="center">
                        No routes found
                      </TableCell>
                    </TableRow>
                  ) : (
                    routes.map((route) => (
                      <TableRow
                        key={`${route.service_id}-${route.method}-${route.path}`}
                      >
                        <TableCell>{route.path}</TableCell>
                        <TableCell>
                          <Chip
                            label={route.method}
                            size="small"
                            color={
                              route.method === "GET"
                                ? "success"
                                : route.method === "POST"
                                ? "primary"
                                : route.method === "PUT"
                                ? "warning"
                                : route.method === "DELETE"
                                ? "error"
                                : "default"
                            }
                          />
                        </TableCell>
                        <TableCell>{route.service_name}</TableCell>
                        <TableCell>
                          {route.public ? (
                            <Chip label="Public" color="success" size="small" />
                          ) : (
                            <Chip
                              label="Protected"
                              color="warning"
                              size="small"
                            />
                          )}
                        </TableCell>
                        <TableCell>
                          {route.required_scopes.length === 0 ? (
                            <Typography variant="body2" color="text.secondary">
                              None
                            </Typography>
                          ) : (
                            <Box
                              sx={{
                                display: "flex",
                                flexWrap: "wrap",
                                gap: 0.5,
                              }}
                            >
                              {route.required_scopes.map((scope) => (
                                <Chip key={scope} label={scope} size="small" />
                              ))}
                            </Box>
                          )}
                        </TableCell>
                        <TableCell>{route.priority}</TableCell>
                        <TableCell align="right">
                          <Tooltip title="Edit">
                            <IconButton
                              size="small"
                              onClick={() => openRouteForm(route)}
                            >
                              <EditIcon />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="Delete">
                            <IconButton
                              size="small"
                              onClick={() =>
                                deleteRoute(
                                  `${route.service_id}-${route.method}-${route.path}`
                                )
                              }
                              color="error"
                            >
                              <DeleteIcon />
                            </IconButton>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        )}

        {/* Service Dialog */}
        <Dialog
          open={openServiceDialog}
          onClose={() => setOpenServiceDialog(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            {editingService ? "Edit Service" : "Add New Service"}
          </DialogTitle>
          <DialogContent>
            <TextField
              margin="normal"
              required
              fullWidth
              label="Service ID"
              name="id"
              value={serviceForm.id}
              onChange={handleServiceChange}
              disabled={!!editingService}
            />
            <TextField
              margin="normal"
              required
              fullWidth
              label="Service Name"
              name="name"
              value={serviceForm.name}
              onChange={handleServiceChange}
            />
            <TextField
              margin="normal"
              required
              fullWidth
              label="Service URL"
              name="url"
              value={serviceForm.url}
              onChange={handleServiceChange}
              placeholder="http://service-host:port"
            />
            <TextField
              margin="normal"
              fullWidth
              label="Health Check Path"
              name="health_check_path"
              value={serviceForm.health_check_path}
              onChange={handleServiceChange}
              placeholder="/health"
            />
            <TextField
              margin="normal"
              fullWidth
              label="Timeout"
              name="timeout"
              value={serviceForm.timeout}
              onChange={handleServiceChange}
              placeholder="30s"
            />
            <TextField
              margin="normal"
              fullWidth
              type="number"
              label="Retry Count"
              name="retry_count"
              value={serviceForm.retry_count}
              onChange={handleServiceChange}
              inputProps={{ min: 0, max: 10 }}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={serviceForm.requires_auth}
                  onChange={handleServiceChange}
                  name="requires_auth"
                />
              }
              label="Requires Authentication (Default)"
            />
            <Typography variant="subtitle2" sx={{ mt: 2, mb: 1 }}>
              Allowed HTTP Methods
            </Typography>
            <FormGroup row>
              {httpMethods.map((method) => (
                <FormControlLabel
                  key={method}
                  control={
                    <Checkbox
                      checked={serviceForm.allowed_methods.includes(method)}
                      onChange={(e) =>
                        handleMethodCheckbox(method, e.target.checked)
                      }
                      name={`method_${method}`}
                    />
                  }
                  label={method}
                />
              ))}
            </FormGroup>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setOpenServiceDialog(false)}>Cancel</Button>
            <Button
              onClick={submitServiceForm}
              variant="contained"
              disabled={loading}
            >
              {loading ? <CircularProgress size={24} /> : "Save"}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Route Dialog */}
        <Dialog
          open={openRouteDialog}
          onClose={() => setOpenRouteDialog(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            {editingRoute ? "Edit Route" : "Add New Route"}
          </DialogTitle>
          <DialogContent>
            <TextField
              select
              margin="normal"
              required
              fullWidth
              label="Service"
              name="service_id"
              value={routeForm.service_id}
              onChange={handleRouteChange}
              disabled={!!editingRoute}
              SelectProps={{ native: true }}
            >
              <option value="">Select Service</option>
              {services.map((service) => (
                <option key={service.id} value={service.id}>
                  {service.name}
                </option>
              ))}
            </TextField>
            <TextField
              margin="normal"
              required
              fullWidth
              label="Path"
              name="path"
              value={routeForm.path}
              onChange={handleRouteChange}
              placeholder="/api/resource/{param}"
              disabled={!!editingRoute}
            />
            <TextField
              select
              margin="normal"
              required
              fullWidth
              label="HTTP Method"
              name="method"
              value={routeForm.method}
              onChange={handleRouteChange}
              disabled={!!editingRoute}
              SelectProps={{ native: true }}
            >
              {httpMethods.map((method) => (
                <option key={method} value={method}>
                  {method}
                </option>
              ))}
            </TextField>
            <TextField
              margin="normal"
              fullWidth
              type="number"
              label="Priority"
              name="priority"
              value={routeForm.priority}
              onChange={handleRouteChange}
              inputProps={{ min: 0, max: 100 }}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={routeForm.public}
                  onChange={handleRouteChange}
                  name="public"
                />
              }
              label="Public Route (No Authentication Required)"
            />

            <Typography variant="subtitle2" sx={{ mt: 2, mb: 1 }}>
              Required Scopes (for protected routes)
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", mb: 1 }}>
              <TextField
                fullWidth
                size="small"
                label="Add Scope"
                value={scopeInput}
                onChange={(e) => setScopeInput(e.target.value)}
                onKeyPress={(e) => e.key === "Enter" && addScope()}
                disabled={routeForm.public}
              />
              <Button
                onClick={addScope}
                disabled={!scopeInput.trim() || routeForm.public}
                sx={{ ml: 1 }}
              >
                Add
              </Button>
            </Box>
            {routeForm.required_scopes.length > 0 ? (
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, mt: 1 }}>
                {routeForm.required_scopes.map((scope) => (
                  <Chip
                    key={scope}
                    label={scope}
                    onDelete={() => removeScope(scope)}
                    disabled={routeForm.public}
                  />
                ))}
              </Box>
            ) : (
              <Typography variant="body2" color="text.secondary">
                No scopes required
              </Typography>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setOpenRouteDialog(false)}>Cancel</Button>
            <Button
              onClick={submitRouteForm}
              variant="contained"
              disabled={loading}
            >
              {loading ? <CircularProgress size={24} /> : "Save"}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Success message snackbar */}
        <Snackbar
          open={!!successMessage}
          autoHideDuration={5000}
          onClose={() => setSuccessMessage(null)}
          anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        >
          <Alert
            onClose={() => setSuccessMessage(null)}
            severity="success"
            sx={{ width: "100%" }}
          >
            {successMessage}
          </Alert>
        </Snackbar>
      </Box>
    </Container>
  );
};
