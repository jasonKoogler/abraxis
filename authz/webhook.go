package authz

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
)

// WebhookConfig contains configuration for the policy update webhook
type WebhookConfig struct {
	// Endpoint is the path where the webhook will listen for policy updates
	Endpoint string

	// Secret is used to validate webhook requests
	Secret string

	// AllowedSources defines IP addresses or CIDR ranges allowed to push updates
	AllowedSources []string
}

// PolicyUpdate represents a policy update received via webhook
type PolicyUpdate struct {
	// Policies is a map of policy module names to their content
	Policies map[string]string `json:"policies"`

	// Signature for validating the update
	Signature string `json:"signature,omitempty"`
}

// RegisterWebhook registers the webhook handler for policy updates
func (a *Agent) RegisterWebhook(mux *http.ServeMux) {
	if a.config.WebhookConfig == nil {
		a.config.Logger.Println("No webhook configuration provided")
		return
	}

	if a.config.Source != PolicySourceLocal {
		a.config.Logger.Println("Webhook only supported for local policy source")
		return
	}

	mux.HandleFunc(a.config.WebhookConfig.Endpoint, a.webhookHandler)
	a.config.Logger.Printf("Registered policy update webhook on %s", a.config.WebhookConfig.Endpoint)
}

// webhookHandler handles policy update webhooks
func (a *Agent) webhookHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate source IP if allowed sources are configured
	if len(a.config.WebhookConfig.AllowedSources) > 0 {
		clientIP, err := getClientIP(r)
		if err != nil {
			a.config.Logger.Printf("Error getting client IP: %v", err)
			http.Error(w, "Error validating source", http.StatusInternalServerError)
			return
		}

		if !isAllowedIP(clientIP, a.config.WebhookConfig.AllowedSources) {
			a.config.Logger.Printf("Unauthorized request from IP: %s", clientIP)
			http.Error(w, "Unauthorized source", http.StatusForbidden)
			return
		}
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Parse the policy update
	var update PolicyUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	// Validate signature if secret is configured
	if a.config.WebhookConfig.Secret != "" {
		// Extract policies for signature validation
		policiesJSON, err := json.Marshal(update.Policies)
		if err != nil {
			a.config.Logger.Printf("Error marshaling policies for signature validation: %v", err)
			http.Error(w, "Error validating signature", http.StatusInternalServerError)
			return
		}

		// Verify the signature
		if !verifySignature(policiesJSON, update.Signature, a.config.WebhookConfig.Secret) {
			a.config.Logger.Println("Invalid signature in webhook request")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Update the policies
	if err := a.UpdatePolicies(update.Policies); err != nil {
		a.config.Logger.Printf("Error updating policies: %v", err)
		http.Error(w, "Error updating policies", http.StatusInternalServerError)
		return
	}

	a.config.Logger.Println("Policies updated successfully via webhook")
	w.WriteHeader(http.StatusOK)
}

// getClientIP extracts the client IP address from the request
func getClientIP(r *http.Request) (string, error) {
	// Check X-Forwarded-For header first (for proxied requests)
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0]), nil
	}

	// If no X-Forwarded-For, use RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If there's no port in the RemoteAddr, just use it as is
		return r.RemoteAddr, nil
	}
	return ip, nil
}

// isAllowedIP checks if the given IP is in the list of allowed sources
func isAllowedIP(ip string, allowedSources []string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, source := range allowedSources {
		// Check if source is a CIDR range
		if strings.Contains(source, "/") {
			_, ipNet, err := net.ParseCIDR(source)
			if err != nil {
				continue
			}
			if ipNet.Contains(clientIP) {
				return true
			}
		} else {
			// Check if source is a single IP
			allowedIP := net.ParseIP(source)
			if allowedIP != nil && allowedIP.Equal(clientIP) {
				return true
			}
		}
	}
	return false
}

// verifySignature validates the signature of the webhook payload
func verifySignature(payload []byte, signature string, secret string) bool {
	// Decode the hex-encoded signature
	signatureBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	// Create a new HMAC with SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	expectedSignature := h.Sum(nil)

	// Compare the expected signature with the provided one
	return hmac.Equal(signatureBytes, expectedSignature)
}
