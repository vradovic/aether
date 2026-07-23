package realtime_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vradovic/aether/services/api/internal/core"
	"github.com/vradovic/aether/services/api/internal/realtime"
)

type TokenTest struct {
	goodSecret string
	badSecret  string
	goodToken  string
	badToken   string
	userID     string
}

func SetupTokens(t *testing.T) TokenTest {
	t.Helper()

	goodSecret := "asfjlksadjfklsdajfsladkfjklsdajflkasdjfslkdfjsdalkfjasdlkfjsdalkfjasdlfkjjflaksfjlsdkafjlkasdjfkl"
	badSecret := "asfjlksadjfklsdajfsladkfjklsdajflkasdjfslkdfjsdalkfjasdlkfjsdalkfjafjkasdhfiksdjflaksfjlsdkafjlkasdjfkl"
	userID := "df32ce22-6685-4f95-89fd-7df5aab51f91"
	goodToken, err := core.IssueToken(goodSecret, userID)
	if err != nil {
		t.Fatalf("Error issuing good token: %v", err)
	}
	badToken, err := core.IssueToken(badSecret, userID)
	if err != nil {
		t.Fatalf("Error issuing bad token: %v", err)
	}

	return TokenTest{
		goodSecret: goodSecret,
		badSecret:  badSecret,
		goodToken:  goodToken,
		badToken:   badToken,
		userID:     userID,
	}
}

func TestParseToken(t *testing.T) {
	tokenTest := SetupTokens(t)

	tests := []struct {
		name string
		url  string
		want int
	}{
		{
			name: "should return bad request when no token provided",
			url:  "/ws?ohboy",
			want: http.StatusBadRequest,
		},
		{
			name: "should return unauthorized when bad token",
			url:  "/ws?token=" + tokenTest.badToken,
			want: http.StatusUnauthorized,
		},
		{
			name: "should return nothing when good token provided",
			url:  "/ws?token=" + tokenTest.goodToken,
			want: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()

			_, _ = realtime.ParseToken(w, r, tokenTest.goodSecret)
			if got := w.Code; got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
