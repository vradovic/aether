package contacts

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vradovic/aether/services/api/internal/auth"
)

func bearerToken(t *testing.T) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		Issuer:    "aether",
		Subject:   testUserID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return value
}

func testHandler(t *testing.T, queries *fakeQuerier) *http.ServeMux {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHandler(NewService(queries), logger)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux, auth.NewMiddleware("secret", "aether").Authenticate)
	return mux
}

func TestHandlerSendContactRequest(t *testing.T) {
	queries := &fakeQuerier{sendID: testRequestID}
	request := httptest.NewRequest(
		http.MethodPost,
		"/contact-requests",
		strings.NewReader(`{"username":"petar"}`),
	)
	request.Header.Set("Authorization", "Bearer "+bearerToken(t))
	response := httptest.NewRecorder()

	testHandler(t, queries).ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusCreated, response.Body.String())
	}
	var body sendResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != testRequestID.String() {
		t.Errorf("response ID = %q, want %q", body.ID, testRequestID.String())
	}
	if queries.sendParams.SenderID.String() != testUserID {
		t.Errorf("sender ID = %q, want %q", queries.sendParams.SenderID.String(), testUserID)
	}
}

func TestHandlerContactRequestMutations(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		check  func(*fakeQuerier) bool
	}{
		{name: "cancel", method: http.MethodDelete, path: "/contact-requests/" + testRequestID.String(), check: func(q *fakeQuerier) bool { return q.cancelParams.RequestID == testRequestID }},
		{name: "accept", method: http.MethodPost, path: "/contact-requests/" + testRequestID.String() + "/accept", check: func(q *fakeQuerier) bool { return q.acceptParams.RequestID == testRequestID }},
		{name: "decline", method: http.MethodPost, path: "/contact-requests/" + testRequestID.String() + "/decline", check: func(q *fakeQuerier) bool { return q.declineParams.RequestID == testRequestID }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := &fakeQuerier{accepted: true}
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Header.Set("Authorization", "Bearer "+bearerToken(t))
			response := httptest.NewRecorder()

			testHandler(t, queries).ServeHTTP(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, http.StatusNoContent, response.Body.String())
			}
			if !tt.check(queries) {
				t.Error("request ID was not passed to the service query")
			}
		})
	}
}

func TestHandlerRequiresAuthentication(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/contact-requests", strings.NewReader(`{"username":"petar"}`))
	response := httptest.NewRecorder()

	testHandler(t, &fakeQuerier{}).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
