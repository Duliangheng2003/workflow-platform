package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// Store implements store.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable WAL mode for better performance
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			nodes TEXT,
			edges TEXT,
			start_type TEXT DEFAULT '',
			cron_expr TEXT DEFAULT '',
			last_run_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS instances (
			id TEXT PRIMARY KEY,
			template_id TEXT NOT NULL,
			status TEXT NOT NULL,
			state TEXT,
			node_states TEXT,
			current_node_id TEXT,
			error TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS human_tasks (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			template_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			node_description TEXT,
			assignee_group TEXT,
			status TEXT NOT NULL,
			input_data TEXT,
			result TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// ——————————————————————————————————————————————————————————————
// Templates
// ——————————————————————————————————————————————————————————————

func (s *Store) CreateTemplate(tmpl *model.Template) error {
	if tmpl.ID == "" {
		tmpl.ID = fmt.Sprintf("tmpl_%d", time.Now().UnixNano())
	}
	now := time.Now()
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now

	nodes, _ := json.Marshal(tmpl.Nodes)
	edges, _ := json.Marshal(tmpl.Edges)

	_, err := s.db.Exec(
		`INSERT INTO templates (id, name, description, nodes, edges, start_type, cron_expr, last_run_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tmpl.ID, tmpl.Name, tmpl.Description, string(nodes), string(edges), tmpl.StartType, tmpl.CronExpr, tmpl.LastRunAt, tmpl.CreatedAt, tmpl.UpdatedAt,
	)
	return err
}

func (s *Store) GetTemplate(id string) (*model.Template, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, nodes, edges, start_type, cron_expr, last_run_at, created_at, updated_at FROM templates WHERE id = ?`, id,
	)

	var tmpl model.Template
	var nodesStr, edgesStr string
	err := row.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &nodesStr, &edgesStr, &tmpl.StartType, &tmpl.CronExpr, &tmpl.LastRunAt, &tmpl.CreatedAt, &tmpl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(nodesStr), &tmpl.Nodes)
	json.Unmarshal([]byte(edgesStr), &tmpl.Edges)
	return &tmpl, nil
}

func (s *Store) ListTemplates() ([]*model.Template, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, nodes, edges, start_type, cron_expr, last_run_at, created_at, updated_at FROM templates ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.Template
	for rows.Next() {
		var tmpl model.Template
		var nodesStr, edgesStr string
		if err := rows.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &nodesStr, &edgesStr, &tmpl.StartType, &tmpl.CronExpr, &tmpl.LastRunAt, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(nodesStr), &tmpl.Nodes)
		json.Unmarshal([]byte(edgesStr), &tmpl.Edges)
		result = append(result, &tmpl)
	}
	return result, rows.Err()
}

func (s *Store) DeleteTemplate(id string) error {
	res, err := s.db.Exec(`DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("template not found: %s", id)
	}
	return nil
}

func (s *Store) UpdateTemplate(tmpl *model.Template) error {
	tmpl.UpdatedAt = time.Now()
	nodes, _ := json.Marshal(tmpl.Nodes)
	edges, _ := json.Marshal(tmpl.Edges)
	_, err := s.db.Exec(`UPDATE templates SET name=?, description=?, nodes=?, edges=?, start_type=?, cron_expr=?, last_run_at=?, updated_at=? WHERE id=?`,
		tmpl.Name, tmpl.Description, string(nodes), string(edges), tmpl.StartType, tmpl.CronExpr, tmpl.LastRunAt, tmpl.UpdatedAt, tmpl.ID)
	return err
}

// ——————————————————————————————————————————————————————————————
// Instances
// ——————————————————————————————————————————————————————————————

func (s *Store) CreateInstance(inst *model.Instance) error {
	if inst.ID == "" {
		inst.ID = fmt.Sprintf("inst_%d", time.Now().UnixNano())
	}
	now := time.Now()
	inst.CreatedAt = now
	inst.UpdatedAt = now

	state, _ := json.Marshal(inst.State)
	nodeStates, _ := json.Marshal(inst.NodeStates)

	_, err := s.db.Exec(
		`INSERT INTO instances (id, template_id, status, state, node_states, current_node_id, error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inst.ID, inst.TemplateID, inst.Status, string(state), string(nodeStates), inst.CurrentNodeID, inst.Error, inst.CreatedAt, inst.UpdatedAt,
	)
	return err
}

func (s *Store) GetInstance(id string) (*model.Instance, error) {
	row := s.db.QueryRow(
		`SELECT id, template_id, status, state, node_states, current_node_id, error, created_at, updated_at FROM instances WHERE id = ?`, id,
	)

	var inst model.Instance
	var stateStr, nodeStatesStr string
	err := row.Scan(&inst.ID, &inst.TemplateID, &inst.Status, &stateStr, &nodeStatesStr, &inst.CurrentNodeID, &inst.Error, &inst.CreatedAt, &inst.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("instance not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(stateStr), &inst.State)
	json.Unmarshal([]byte(nodeStatesStr), &inst.NodeStates)
	return &inst, nil
}

func (s *Store) ListInstances() ([]*model.Instance, error) {
	rows, err := s.db.Query(
		`SELECT id, template_id, status, state, node_states, current_node_id, error, created_at, updated_at FROM instances ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.Instance
	for rows.Next() {
		var inst model.Instance
		var stateStr, nodeStatesStr string
		if err := rows.Scan(&inst.ID, &inst.TemplateID, &inst.Status, &stateStr, &nodeStatesStr, &inst.CurrentNodeID, &inst.Error, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(stateStr), &inst.State)
		json.Unmarshal([]byte(nodeStatesStr), &inst.NodeStates)
		result = append(result, &inst)
	}
	return result, rows.Err()
}

func (s *Store) UpdateInstance(inst *model.Instance) error {
	inst.UpdatedAt = time.Now()

	state, _ := json.Marshal(inst.State)
	nodeStates, _ := json.Marshal(inst.NodeStates)

	_, err := s.db.Exec(
		`UPDATE instances SET status=?, state=?, node_states=?, current_node_id=?, error=?, updated_at=? WHERE id=?`,
		inst.Status, string(state), string(nodeStates), inst.CurrentNodeID, inst.Error, inst.UpdatedAt, inst.ID,
	)
	return err
}

// ——————————————————————————————————————————————————————————————
// Human Tasks
// ——————————————————————————————————————————————————————————————

func (s *Store) CreateHumanTask(task *model.HumanTask) error {
	if task.ID == "" {
		task.ID = fmt.Sprintf("ht_%d", time.Now().UnixNano())
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	inputData, _ := json.Marshal(task.InputData)
	result, _ := json.Marshal(task.Result)

	_, err := s.db.Exec(
		`INSERT INTO human_tasks (id, instance_id, template_id, node_id, node_description, assignee_group, status, input_data, result, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.InstanceID, task.TemplateID, task.NodeID, task.NodeDescription, task.AssigneeGroup, task.Status, string(inputData), string(result), task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *Store) GetHumanTask(id string) (*model.HumanTask, error) {
	row := s.db.QueryRow(
		`SELECT id, instance_id, template_id, node_id, node_description, assignee_group, status, input_data, result, created_at, updated_at FROM human_tasks WHERE id = ?`, id,
	)

	var task model.HumanTask
	var inputStr, resultStr string
	err := row.Scan(&task.ID, &task.InstanceID, &task.TemplateID, &task.NodeID, &task.NodeDescription, &task.AssigneeGroup, &task.Status, &inputStr, &resultStr, &task.CreatedAt, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("human task not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(inputStr), &task.InputData)
	json.Unmarshal([]byte(resultStr), &task.Result)
	return &task, nil
}

func (s *Store) ListHumanTasks(statuses ...model.HumanTaskStatus) ([]*model.HumanTask, error) {
	var rows *sql.Rows
	var err error

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		args := make([]interface{}, len(statuses))
		for i, st := range statuses {
			placeholders[i] = "?"
			args[i] = string(st)
		}
		query := fmt.Sprintf(
			`SELECT id, instance_id, template_id, node_id, node_description, assignee_group, status, input_data, result, created_at, updated_at FROM human_tasks WHERE status IN (%s) ORDER BY created_at DESC`,
			join(placeholders, ","),
		)
		rows, err = s.db.Query(query, args...)
	} else {
		rows, err = s.db.Query(
			`SELECT id, instance_id, template_id, node_id, node_description, assignee_group, status, input_data, result, created_at, updated_at FROM human_tasks ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.HumanTask
	for rows.Next() {
		var task model.HumanTask
		var inputStr, resultStr string
		if err := rows.Scan(&task.ID, &task.InstanceID, &task.TemplateID, &task.NodeID, &task.NodeDescription, &task.AssigneeGroup, &task.Status, &inputStr, &resultStr, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(inputStr), &task.InputData)
		json.Unmarshal([]byte(resultStr), &task.Result)
		result = append(result, &task)
	}
	return result, rows.Err()
}

func (s *Store) UpdateHumanTask(task *model.HumanTask) error {
	task.UpdatedAt = time.Now()

	inputData, _ := json.Marshal(task.InputData)
	result, _ := json.Marshal(task.Result)

	_, err := s.db.Exec(
		`UPDATE human_tasks SET status=?, input_data=?, result=?, updated_at=? WHERE id=?`,
		task.Status, string(inputData), string(result), task.UpdatedAt, task.ID,
	)
	return err
}

func join(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	b := elems[0]
	for _, e := range elems[1:] {
		b += sep + e
	}
	return b
}