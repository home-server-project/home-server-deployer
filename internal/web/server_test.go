package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMutationRequiresCSRF(t *testing.T) {
	s, err := NewServer(NewAgentClient("/definitely/not/a/socket"))
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/plan", strings.NewReader("operation=install&appId=alpha-smoke&instanceId=alpha-smoke"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("status=%d", w.Code)
	}
}
