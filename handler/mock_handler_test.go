package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMockSetAndGetVerbatim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewMockHandler()
	r := gin.New()
	r.POST("/api/util/mock/set", h.SetMockResponse)
	r.GET("/api/util/mock/:id", h.GetMockResponseByID)

	raw := `{"a":1,"b":{"c":"x"},"float":1.50,"list":[1,2,3]}`

	// set
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/util/mock/set", strings.NewReader(raw))
	req.Host = "localhost:8080"
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("set status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal set response: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("set response missing id")
	}
	wantURL := "http://localhost:8080/api/util/mock/" + resp.ID
	if resp.URL != wantURL {
		t.Errorf("url = %q, want %q", resp.URL, wantURL)
	}

	// get by id: should echo the raw body verbatim
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/util/mock/"+resp.ID, nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", w2.Code, w2.Body.String())
	}
	if w2.Body.String() != raw {
		t.Errorf("get body = %q, want %q", w2.Body.String(), raw)
	}

	// unknown id -> 404
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/api/util/mock/doesnotexist", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Errorf("unknown id status = %d, want 404", w3.Code)
	}

	// invalid JSON -> 400
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/api/util/mock/set", strings.NewReader("not json"))
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusBadRequest {
		t.Errorf("invalid json status = %d, want 400", w4.Code)
	}
}
