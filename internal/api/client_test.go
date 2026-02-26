package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicolasacchi/ebcli/internal/auth"
)

func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	key, err := auth.LoadPrivateKey("../../testdata/pkcs8.pem")
	if err != nil {
		t.Fatalf("loading test key: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return NewClient("test-app-id", key,
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
	)
}

func TestClient_ListASPSPs(t *testing.T) {
	aspsps := []ASPSPData{
		{Name: "Nordea", Country: "FI", PSUTypes: []string{"personal"}},
		{Name: "S-Pankki", Country: "FI", PSUTypes: []string{"personal"}},
	}

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Error("missing Bearer token")
		}

		// Verify path and query
		if r.URL.Path != "/aspsps" {
			t.Errorf("path = %q, want /aspsps", r.URL.Path)
		}
		if r.URL.Query().Get("country") != "FI" {
			t.Errorf("country = %q, want FI", r.URL.Query().Get("country"))
		}
		if r.URL.Query().Get("service") != "AIS" {
			t.Errorf("service = %q, want AIS", r.URL.Query().Get("service"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(aspsps)
	}))

	result, err := client.ListASPSPs(context.Background(), "FI", "")
	if err != nil {
		t.Fatalf("ListASPSPs: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Name != "Nordea" {
		t.Errorf("Name = %q, want Nordea", result[0].Name)
	}
}

func TestClient_GetBalances(t *testing.T) {
	resp := BalancesResponse{
		Balances: []Balance{
			{
				BalanceAmount: Amount{Currency: "EUR", Amount: "1234.56"},
				BalanceType:   "CLBD",
			},
		},
	}

	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/test-uid/balances" {
			t.Errorf("path = %q, want /accounts/test-uid/balances", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))

	result, err := client.GetBalances(context.Background(), "test-uid", nil)
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	if len(result.Balances) != 1 {
		t.Fatalf("Balances = %d, want 1", len(result.Balances))
	}
	if result.Balances[0].BalanceAmount.Amount != "1234.56" {
		t.Errorf("Amount = %q, want 1234.56", result.Balances[0].BalanceAmount.Amount)
	}
}

func TestClient_GetTransactions_Pagination(t *testing.T) {
	callCount := 0
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch callCount {
		case 1:
			json.NewEncoder(w).Encode(TransactionsResponse{
				Transactions: []Transaction{
					{TransactionID: "txn1", TransactionAmount: Amount{Currency: "EUR", Amount: "10.00"}},
				},
				ContinuationKey: "page2",
			})
		case 2:
			if r.URL.Query().Get("continuation_key") != "page2" {
				t.Errorf("continuation_key = %q, want page2", r.URL.Query().Get("continuation_key"))
			}
			json.NewEncoder(w).Encode(TransactionsResponse{
				Transactions: []Transaction{
					{TransactionID: "txn2", TransactionAmount: Amount{Currency: "EUR", Amount: "20.00"}},
				},
			})
		default:
			t.Errorf("unexpected call %d", callCount)
		}
	}))

	// First page
	result1, err := client.GetTransactions(context.Background(), "test-uid", TransactionParams{
		DateFrom:          "2024-01-01",
		DateTo:            "2024-01-31",
		TransactionStatus: "BOOK",
	}, nil)
	if err != nil {
		t.Fatalf("GetTransactions page 1: %v", err)
	}
	if len(result1.Transactions) != 1 {
		t.Fatalf("page 1 transactions = %d, want 1", len(result1.Transactions))
	}
	if result1.ContinuationKey != "page2" {
		t.Errorf("ContinuationKey = %q, want page2", result1.ContinuationKey)
	}

	// Second page
	result2, err := client.GetTransactions(context.Background(), "test-uid", TransactionParams{
		DateFrom:          "2024-01-01",
		DateTo:            "2024-01-31",
		ContinuationKey:   "page2",
		TransactionStatus: "BOOK",
	}, nil)
	if err != nil {
		t.Fatalf("GetTransactions page 2: %v", err)
	}
	if len(result2.Transactions) != 1 {
		t.Fatalf("page 2 transactions = %d, want 1", len(result2.Transactions))
	}
	if result2.ContinuationKey != "" {
		t.Errorf("ContinuationKey should be empty on last page")
	}
}

func TestClient_APIError(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "not_found",
			"error_description": "Resource not found",
		})
	}))

	_, err := client.GetApplication(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

func TestClient_RateLimitHeaders(t *testing.T) {
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ratelimit-Remaining", "3")
		w.Header().Set("X-Ratelimit-Limit", "10")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BalancesResponse{Balances: []Balance{}})
	}))

	_, err := client.GetBalances(context.Background(), "test-uid", nil)
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}
	// Rate limit headers are tracked silently — no error expected
}

func TestExtractAccountUID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/accounts/abc-123/balances", "abc-123"},
		{"/accounts/abc-123/transactions?date_from=2024-01-01", "abc-123"},
		{"/aspsps", ""},
		{"/sessions/sess-123", ""},
	}

	for _, tt := range tests {
		got := extractAccountUID(tt.path)
		if got != tt.want {
			t.Errorf("extractAccountUID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractEndpoint(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/accounts/abc/balances", "balances"},
		{"/accounts/abc/transactions?foo=bar", "transactions"},
		{"/aspsps", "aspsps"},
		{"/application", "application"},
	}

	for _, tt := range tests {
		got := extractEndpoint(tt.path)
		if got != tt.want {
			t.Errorf("extractEndpoint(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
