import React, { createContext, ReactNode, useContext } from "react";
import { useAuth } from "../hooks/useAuth";

// Auth context type
interface AuthContextType {
  isAuthenticated: boolean;
  getAccessToken: () => Promise<string>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

// Create the context with defaults
const AuthContext = createContext<AuthContextType>({
  isAuthenticated: false,
  getAccessToken: async () => "",
  login: async () => {},
  logout: () => {},
});

// Provider props type
interface AuthProviderProps {
  children: ReactNode;
}

// Auth provider component
export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const auth = useAuth();

  return <AuthContext.Provider value={auth}>{children}</AuthContext.Provider>;
};

// Custom hook to use the auth context
export const useAuthContext = () => useContext(AuthContext);
