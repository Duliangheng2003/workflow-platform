package api

import (
	"encoding/json"
	"net/http"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/engine"
	"github.com/Duliangheng2003/workflow-platform/internal/model"
	"github.com/Duliangheng2003/workflow-platform/internal/store"
)

type Handler struct {
	store      store.Store
	engine     *engine.Engine
	llmConfig  config.LLMConfig
}

func NewHandler(s store.Store, e *engine.Engine, llmCfg config.LLMConfig) *Handler {
	return &Handler{store: s, engine: e, llmConfig: llmCfg}
}

// RegisterRoutes registers all HTTP routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Templates
	mux.HandleFunc("POST /api/v1/templates", h.CreateTemplate)
	mux.HandleFunc("GET /api/v1/templates", h.ListTemplates)
	mux.HandleFunc("GET /api/v1/templates/{id}", h.GetTemplate)
	mux.HandleFunc("PUT /api/v1/templates/{id}", h.UpdateTemplate)
	mux.HandleFunc("DELETE /api/v1/templates/{id}", h.DeleteTemplate)

	// Instances
	mux.HandleFunc("POST /api/v1/templates/{id}/instances", h.StartInstance)
	mux.HandleFunc("GET /api/v1/instances", h.ListInstances)
	mux.HandleFunc("GET /api/v1/instances/{id}", h.GetInstance)
		mux.HandleFunc("DELETE /api/v1/instances/{id}", h.DeleteInstance)

	// Human Tasks
	mux.HandleFunc("GET /api/v1/human-tasks", h.ListHumanTasks)
	mux.HandleFunc("POST /api/v1/human-tasks/{id}/resume", h.ResumeTask)

	// LLM profiles (server-side config, exposed for frontend dropdown)
	mux.HandleFunc("GET /api/v1/llm/profiles", h.ListLLMProfiles)
		mux.HandleFunc("POST /api/v1/llm/profiles", h.CreateLLMProfile)
		mux.HandleFunc("PUT /api/v1/llm/profiles/{id}", h.UpdateLLMProfile)
		mux.HandleFunc("DELETE /api/v1/llm/profiles/{id}", h.DeleteLLMProfile)
}

// ——————————————————————————————————————————————————————————————
// Template handlers
// ——————————————————————————————————————————————————————————————

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "template name is required")
		return
	}

	// Check name uniqueness
	existing, err := h.store.ListTemplates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, t := range existing {
		if t.Name == req.Name {
			writeError(w, http.StatusConflict, "template name \""+req.Name+"\" already exists")
			return
		}
	}

	tmpl := &model.Template{
		Name:        req.Name,
		Description: req.Description,
		Nodes:       req.Nodes,
		Edges:       req.Edges,
		StartType:   req.StartType,
		CronExpr:    req.CronExpr,
		StartInput:  req.StartInput,
	}

	if err := h.store.CreateTemplate(tmpl); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, tmpl)
}
func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tmpl, err := h.store.GetTemplate(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tmpl)
}

func (h *Handler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.store.ListTemplates()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteTemplate(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req model.CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "template name is required")
		return
	}

	tmpl, err := h.store.GetTemplate(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Check name uniqueness (skip if name unchanged)
	if req.Name != tmpl.Name {
		templates, err := h.store.ListTemplates()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, t := range templates {
			if t.Name == req.Name {
				writeError(w, http.StatusConflict, "template name \""+req.Name+"\" already exists")
				return
			}
		}
	}

	tmpl.Name = req.Name
	tmpl.Description = req.Description
	tmpl.Nodes = req.Nodes
	tmpl.Edges = req.Edges
	tmpl.StartType = req.StartType
	tmpl.CronExpr = req.CronExpr
	tmpl.StartInput = req.StartInput

	if err := h.store.UpdateTemplate(tmpl); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tmpl)
}
func (h *Handler) StartInstance(w http.ResponseWriter, r *http.Request) {
	tmplID := r.PathValue("id")

	var req model.StartInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	inst, err := h.engine.StartInstance(r.Context(), tmplID, req.Input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, inst)
}

func (h *Handler) GetInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := h.store.GetInstance(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteInstance(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListInstances(w http.ResponseWriter, r *http.Request) {
	instances, err := h.store.ListInstances()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, instances)
}

// ——————————————————————————————————————————————————————————————
// Human Task handlers
// ——————————————————————————————————————————————————————————————

func (h *Handler) ListHumanTasks(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	var tasks []*model.HumanTask
	var err error

	if statusFilter != "" {
		tasks, err = h.store.ListHumanTasks(model.HumanTaskStatus(statusFilter))
	} else {
		tasks, err = h.store.ListHumanTasks()
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) ResumeTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var req model.ResumeTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Action != "approve" && req.Action != "reject" {
		writeError(w, http.StatusBadRequest, "action must be 'approve' or 'reject'")
		return
	}

	if err := h.engine.ResumeTask(r.Context(), taskID, req.Action, req.Result); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ——————————————————————————————————————————————————————————————
// LLM Profiles
// ——————————————————————————————————————————————————————————————

func (h *Handler) ListLLMProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.store.ListLLMProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type safeProfile struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
		KeyHint  string `json:"key_hint"`
	}
	safe := make([]safeProfile, len(profiles))
	for i, p := range profiles {
		keyHint := ""
		if len(p.APIKey) > 4 {
			keyHint = p.APIKey[:2] + "****" + p.APIKey[len(p.APIKey)-4:]
		}
		safe[i] = safeProfile{ID: p.ID, Name: p.Name, Provider: p.Provider, Model: p.Model, BaseURL: p.BaseURL, KeyHint: keyHint}
	}
	writeJSON(w, http.StatusOK, safe)
}

func (h *Handler) CreateLLMProfile(w http.ResponseWriter, r *http.Request) {
	var req model.CreateLLMProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	p := &model.LLMProfile{Name: req.Name, Provider: req.Provider, Model: req.Model, APIKey: req.APIKey, BaseURL: req.BaseURL}
	if err := h.store.CreateLLMProfile(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) UpdateLLMProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req model.CreateLLMProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	p := &model.LLMProfile{ID: id, Name: req.Name, Provider: req.Provider, Model: req.Model, APIKey: req.APIKey, BaseURL: req.BaseURL}
	if err := h.store.UpdateLLMProfile(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) DeleteLLMProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteLLMProfile(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ——————————————————————————————————————————————————————————————
// Response helpers
// ——————————————————————————————————————————————————————————————

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}