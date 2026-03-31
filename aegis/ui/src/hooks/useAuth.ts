import { useCallback, useEffect, useState } from "react";

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
