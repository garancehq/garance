package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	enginev1 "github.com/garancehq/garance/proto/gen/go/engine/v1"
	"github.com/garancehq/garance/services/gateway/internal/handler"
	"github.com/garancehq/garance/services/gateway/internal/middleware"
)

type EngineProxy struct {
	client enginev1.EngineServiceClient
}

func NewEngineProxy(addr string) (*EngineProxy, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &EngineProxy{client: enginev1.NewEngineServiceClient(conn)}, nil
}

func (p *EngineProxy) RegisterRoutes(mux *http.ServeMux) {
	// Meta endpoints (literal paths take priority over {table} in Go 1.22+)
	mux.HandleFunc("GET /api/v1/_tables", p.ListTables)
	mux.HandleFunc("GET /api/v1/_schema", p.GetSchema)
	mux.HandleFunc("GET /api/v1/_schema/{table}", p.GetSchemaTable)
	mux.HandleFunc("POST /api/v1/_reload", p.ReloadSchema)
	mux.HandleFunc("POST /api/v1/rpc/query", p.ExecuteSQL)
	// Migrate endpoints (pass-through to Engine HTTP)
	mux.HandleFunc("POST /api/v1/_migrate/preview", p.MigratePreview)
	mux.HandleFunc("POST /api/v1/_migrate/apply", p.MigrateApply)
	// CRUD
	mux.HandleFunc("GET /api/v1/{table}", p.ListRows)
	mux.HandleFunc("POST /api/v1/{table}", p.InsertRow)
	mux.HandleFunc("GET /api/v1/{table}/{id}", p.GetRow)
	mux.HandleFunc("PATCH /api/v1/{table}/{id}", p.UpdateRow)
	mux.HandleFunc("DELETE /api/v1/{table}/{id}", p.DeleteRow)
}

func (p *EngineProxy) ListRows(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	query := r.URL.Query()

	filters := make(map[string]string)
	for key, vals := range query {
		if key != "select" && key != "order" && key != "limit" && key != "offset" {
			filters[key] = vals[0]
		}
	}

	req := &enginev1.ListRowsRequest{
		Table:   table,
		Filters: filters,
		Select:  query.Get("select"),
		Order:   query.Get("order"),
	}
	if l := query.Get("limit"); l != "" {
		var limit int64
		fmt.Sscanf(l, "%d", &limit)
		req.Limit = limit
	}
	if o := query.Get("offset"); o != "" {
		var offset int64
		fmt.Sscanf(o, "%d", &offset)
		req.Offset = offset
	}

	// Inject auth context from JWT
	if uid, ok := r.Context().Value(middleware.UserIDKey).(string); ok {
		req.UserId = uid
	}
	if pid, ok := r.Context().Value(middleware.ProjectIDKey).(string); ok {
		req.ProjectId = pid
	}
	if role, ok := r.Context().Value(middleware.RoleKey).(string); ok {
		req.Role = role
	}

	resp, err := p.client.ListRows(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	handler.WriteRawJSON(w, 200, resp.RowsJson)
}

func (p *EngineProxy) GetRow(w http.ResponseWriter, r *http.Request) {
	req := &enginev1.GetRowRequest{
		Table: r.PathValue("table"),
		Id:    r.PathValue("id"),
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.GetRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	handler.WriteRawJSON(w, 200, resp.RowJson)
}

func (p *EngineProxy) InsertRow(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "failed to read body", 400)
		return
	}

	req := &enginev1.InsertRowRequest{
		Table:    r.PathValue("table"),
		BodyJson: body,
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.InsertRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteRawJSON(w, 201, resp.RowJson)
}

func (p *EngineProxy) UpdateRow(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "failed to read body", 400)
		return
	}

	req := &enginev1.UpdateRowRequest{
		Table:    r.PathValue("table"),
		Id:       r.PathValue("id"),
		BodyJson: body,
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.UpdateRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	handler.WriteRawJSON(w, 200, resp.RowJson)
}

func (p *EngineProxy) DeleteRow(w http.ResponseWriter, r *http.Request) {
	req := &enginev1.DeleteRowRequest{
		Table: r.PathValue("table"),
		Id:    r.PathValue("id"),
	}
	injectAuth(r, func(uid, pid, role string) {
		req.UserId = uid
		req.ProjectId = pid
		req.Role = role
	})

	resp, err := p.client.DeleteRow(r.Context(), req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	if !resp.Found {
		handler.WriteError(w, "NOT_FOUND", "row not found", 404)
		return
	}
	w.WriteHeader(204)
}

// ─── Meta Endpoints ──────────────────────────────────────────────────────

func (p *EngineProxy) ListTables(w http.ResponseWriter, r *http.Request) {
	resp, err := p.client.ListTables(r.Context(), &enginev1.ListTablesRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp.Tables)
}

func (p *EngineProxy) GetSchema(w http.ResponseWriter, r *http.Request) {
	resp, err := p.client.GetSchema(r.Context(), &enginev1.GetSchemaRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteRawJSON(w, 200, resp.SchemaJson)
}

func (p *EngineProxy) GetSchemaTable(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("table")
	resp, err := p.client.GetSchema(r.Context(), &enginev1.GetSchemaRequest{Table: tableName})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteRawJSON(w, 200, resp.SchemaJson)
}

func (p *EngineProxy) ReloadSchema(w http.ResponseWriter, r *http.Request) {
	resp, err := p.client.ReloadSchema(r.Context(), &enginev1.ReloadSchemaRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	handler.WriteJSON(w, 200, resp)
}

func (p *EngineProxy) ExecuteSQL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "failed to read body", 400)
		return
	}

	var sqlReq struct {
		SQL string `json:"sql"`
	}
	if err := json.Unmarshal(body, &sqlReq); err != nil {
		handler.WriteError(w, "VALIDATION_ERROR", "invalid request body", 400)
		return
	}

	readwrite := false
	if mode := r.Header.Get("X-Garance-SQL-Mode"); mode == "readwrite" {
		readwrite = true
	}

	resp, err := p.client.ExecuteSQL(r.Context(), &enginev1.ExecuteSQLRequest{
		Sql:       sqlReq.SQL,
		Readwrite: readwrite,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build response matching the Engine HTTP format
	result := map[string]interface{}{
		"columns":     resp.Columns,
		"rows":        json.RawMessage(resp.RowsJson),
		"row_count":   resp.RowCount,
		"duration_ms": resp.DurationMs,
	}
	handler.WriteJSON(w, 200, result)
}

// ─── Migrate Endpoints (pass-through to Engine HTTP) ─────────────────────────

func (p *EngineProxy) MigratePreview(w http.ResponseWriter, r *http.Request) {
	proxyToEngineHTTP(w, r, "POST", "/api/v1/_migrate/preview")
}

func (p *EngineProxy) MigrateApply(w http.ResponseWriter, r *http.Request) {
	proxyToEngineHTTP(w, r, "POST", "/api/v1/_migrate/apply")
}

func proxyToEngineHTTP(w http.ResponseWriter, r *http.Request, method, path string) {
	engineHTTPURL := os.Getenv("ENGINE_HTTP_URL")
	if engineHTTPURL == "" {
		engineHTTPURL = "http://engine:4000"
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handler.WriteError(w, "PROXY_ERROR", "failed to read request body", 500)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), method, engineHTTPURL+path, bytes.NewReader(body))
	if err != nil {
		handler.WriteError(w, "PROXY_ERROR", "failed to create request", 500)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		handler.WriteError(w, "PROXY_ERROR", fmt.Sprintf("failed to reach Engine: %v", err), 502)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}
