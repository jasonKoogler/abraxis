// Initiate login
const response = await fetch("/auth/password/initiate");
const { state } = await response.json();

// Submit login form
const loginResponse = await fetch("/auth/password/login", {
  method: "POST",
  body: JSON.stringify({ email, password, state }),
  headers: { "Content-Type": "application/json" },
});
