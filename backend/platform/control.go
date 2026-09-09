package platform

import (
	"net/http"
	"strconv"
)

func (a *API) control(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	switch r.URL.Path {
	case "/api/human-sessions":
		if r.Method != "POST" {
			problem(w, 405, "POST required")
			return
		}
		if p.SubjectType != "human" {
			problem(w, 403, "human identity required")
			return
		}
		var req struct {
			Expires int64 `json:"expires"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "invalid session")
			return
		}
		meta, secret, err := a.Store.CreateHumanSession(r.Context(), p, req.Expires)
		if err != nil {
			problem(w, 400, err.Error())
			return
		}
		respond(w, 201, map[string]any{"metadata": meta, "session_credential": secret})
	case "/api/organizations":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		v, err := a.Store.Organizations(r.Context(), p)
		if err != nil {
			problem(w, 503, "organizations unavailable")
			return
		}
		respond(w, 200, v)
	case "/api/workspaces":
		if r.Method == "GET" {
			v, err := a.Store.Workspaces(r.Context(), p)
			if err != nil {
				problem(w, 503, "workspaces unavailable")
				return
			}
			respond(w, 200, v)
			return
		}
		if r.Method != "POST" {
			problem(w, 405, "method not allowed")
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "invalid workspace")
			return
		}
		v, err := a.Store.CreateWorkspace(r.Context(), p, req.Name)
		if err != nil {
			problem(w, 400, "valid unique workspace name required")
			return
		}
		respond(w, 201, v)
	case "/api/memberships":
		if r.Method == "GET" {
			v, err := a.Store.Memberships(r.Context(), p)
			if err != nil {
				problem(w, 503, "memberships unavailable")
				return
			}
			respond(w, 200, v)
			return
		}
		if r.Method == "POST" {
			var req struct {
				Issuer      string `json:"issuer"`
				ExternalID  string `json:"external_id"`
				DisplayName string `json:"display_name"`
			}
			if decode(r, &req) != nil {
				problem(w, 400, "invalid membership")
				return
			}
			v, err := a.Store.CreateMembership(r.Context(), p, req.Issuer, req.ExternalID, req.DisplayName)
			if err != nil {
				problem(w, 400, "valid unique human identity required")
				return
			}
			respond(w, 201, v)
			return
		}
		if r.Method == "DELETE" {
			if err := a.Store.SetMembershipStatus(r.Context(), p, r.URL.Query().Get("id"), "revoked"); err != nil {
				problem(w, 404, "membership not found")
				return
			}
			w.WriteHeader(204)
			return
		}
		problem(w, 405, "method not allowed")
	case "/api/role-assignments":
		if r.Method == "GET" {
			v, err := a.Store.RoleAssignments(r.Context(), p)
			if err != nil {
				problem(w, 503, "role assignments unavailable")
				return
			}
			respond(w, 200, v)
			return
		}
		if r.Method != "POST" {
			problem(w, 405, "method not allowed")
			return
		}
		var req struct {
			WorkspaceID string `json:"workspace_id"`
			SubjectType string `json:"subject_type"`
			SubjectID   string `json:"subject_id"`
			Role        string `json:"role"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "invalid role assignment")
			return
		}
		v, err := a.Store.AssignRole(r.Context(), p, req.WorkspaceID, req.SubjectType, req.SubjectID, req.Role)
		if err != nil {
			problem(w, 400, "valid in-organization assignment required")
			return
		}
		respond(w, 201, v)
	case "/api/collector-enrollments":
		if r.Method == "GET" {
			v, err := a.Store.CredentialKeys(r.Context(), p, CredentialEnrollment)
			if err != nil {
				problem(w, 503, "enrollments unavailable")
				return
			}
			respond(w, 200, v)
			return
		}
		if r.Method == "POST" {
			var req struct {
				Name    string `json:"name"`
				Expires int64  `json:"expires"`
			}
			if decode(r, &req) != nil {
				problem(w, 400, "invalid enrollment")
				return
			}
			meta, secret, err := a.Store.CreateEnrollment(r.Context(), p, req.Name, req.Expires)
			if err != nil {
				problem(w, 400, err.Error())
				return
			}
			respond(w, 201, map[string]any{"metadata": meta, "enrollment_credential": secret})
			return
		}
		if r.Method == "DELETE" {
			if err := a.Store.RevokeEnrollment(r.Context(), p, r.URL.Query().Get("id")); err != nil {
				problem(w, 404, "enrollment not found")
				return
			}
			w.WriteHeader(204)
			return
		}
		problem(w, 405, "method not allowed")
	case "/api/collectors/enroll":
		if r.Method != "POST" {
			problem(w, 405, "POST required")
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "invalid collector")
			return
		}
		identity, meta, secret, err := a.Store.EnrollCollector(r.Context(), p, req.Name)
		if err != nil {
			problem(w, 401, "enrollment unavailable")
			return
		}
		respond(w, 201, map[string]any{"identity": identity, "metadata": meta, "collector_credential": secret})
	case "/api/collector-credentials/rotate":
		if r.Method != "POST" {
			problem(w, 405, "POST required")
			return
		}
		var req struct {
			CredentialID string `json:"credential_id"`
		}
		if decode(r, &req) != nil {
			problem(w, 400, "invalid rotation")
			return
		}
		meta, secret, err := a.Store.RotateCollector(r.Context(), p, req.CredentialID)
		if err != nil {
			problem(w, 404, "collector credential not found")
			return
		}
		respond(w, 201, map[string]any{"metadata": meta, "collector_credential": secret})
	case "/api/collector-credentials":
		if r.Method == "GET" {
			v, err := a.Store.CredentialKeys(r.Context(), p, CredentialCollector)
			if err != nil {
				problem(w, 503, "collector credentials unavailable")
				return
			}
			respond(w, 200, v)
			return
		}
		if r.Method == "DELETE" {
			if err := a.Store.revokeCredential(r.Context(), p, r.URL.Query().Get("id"), CredentialCollector); err != nil {
				problem(w, 404, "collector credential not found")
				return
			}
			w.WriteHeader(204)
			return
		}
		problem(w, 405, "method not allowed")
	case "/api/workload-identities":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		v, err := a.Store.Workloads(r.Context(), p)
		if err != nil {
			problem(w, 503, "workload identities unavailable")
			return
		}
		respond(w, 200, v)
	case "/api/audit-events":
		if r.Method != "GET" {
			problem(w, 405, "GET required")
			return
		}
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			var err error
			limit, err = strconv.Atoi(raw)
			if err != nil {
				problem(w, 400, "invalid limit")
				return
			}
		}
		v, err := a.Store.AuditEvents(r.Context(), p, limit, r.URL.Query().Get("cursor"))
		if err != nil {
			problem(w, 400, "invalid audit query")
			return
		}
		respond(w, 200, v)
	default:
		problem(w, 404, "endpoint not available")
	}
}
