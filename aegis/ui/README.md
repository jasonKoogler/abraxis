# Service Manager UI

This React-based UI provides a modern interface for managing services and routes in the GAuth system.

## Prerequisites

- Node.js 16 or higher
- npm or yarn

## Dependencies

Install the following dependencies:

```bash
# React and core dependencies
npm install react react-dom

# Material UI dependencies
npm install @mui/material @mui/icons-material @emotion/react @emotion/styled

# For HTTP requests and routing
npm install axios react-router-dom
```

## Authentication Hook

Create a `useAuth` hook to handle authentication. Here's a minimal implementation:

```tsx
// src/hooks/useAuth.ts
import { useState, useEffect, useCallback } from "react";

interface UseAuthReturn {
  isAuthenticated: boolean;
  getAccessToken: () => Promise<string>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

export const useAuth = (): UseAuthReturn => {
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);

  useEffect(() => {
    // Check if token exists and is valid on mount
    const token = localStorage.getItem("accessToken");
    if (token) {
      // You might want to validate the token here
      setIsAuthenticated(true);
    }
  }, []);

  const getAccessToken = useCallback(async (): Promise<string> => {
    const token = localStorage.getItem("accessToken");
    if (!token) {
      throw new Error("No access token available");
    }
    return token;
  }, []);

  const login = useCallback(
    async (username: string, password: string): Promise<void> => {
      try {
        const response = await fetch("/auth/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ username, password }),
        });

        if (!response.ok) {
          throw new Error(
            `Login failed: ${response.status} ${response.statusText}`
          );
        }

        const data = await response.json();
        localStorage.setItem("accessToken", data.access_token);
        setIsAuthenticated(true);
      } catch (error) {
        console.error("Login error:", error);
        throw error;
      }
    },
    []
  );

  const logout = useCallback((): void => {
    localStorage.removeItem("accessToken");
    setIsAuthenticated(false);
  }, []);

  return { isAuthenticated, getAccessToken, login, logout };
};
```

## Usage

Import and use the ServiceManager component in your application:

```tsx
// src/App.tsx
import React from "react";
import { ServiceManager } from "./components/ServiceManager";
import { AuthProvider } from "./context/AuthContext";

const App: React.FC = () => {
  return (
    <AuthProvider>
      <ServiceManager />
    </AuthProvider>
  );
};

export default App;
```

## Features

The Service Manager UI provides:

1. **Service Management**

   - List all services with their health status
   - Add new services
   - Edit existing services
   - Delete services

2. **Route Management** (Requires Authentication)
   - View all routes across services
   - Add new routes
   - Configure route properties (public/protected, required scopes)
   - Edit existing routes
   - Delete routes

## Security Notes

- Routes management requires authentication with appropriate permissions (`service:admin`)
- Service listing is publicly available, but management actions require authentication
- All sensitive operations are protected by requiring a valid access token

## Customization

You can customize the UI appearance by:

1. Creating a custom Material UI theme
2. Extending the component with additional features
3. Modifying the service and route data models as needed

## Screenshots

![Service Manager UI - Services Tab](./docs/screenshots/services-tab.png)
![Service Manager UI - Routes Tab](./docs/screenshots/routes-tab.png)
![Service Manager UI - Add Service Dialog](./docs/screenshots/add-service-dialog.png)
