package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// Store implements store.Store backed by MySQL.
type Store struct {
	db *sql.DB
}

func NewStore(cfg config.DatabaseConfig) (*Store, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
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
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			nodes JSON,
			edges JSON,
			start_type VARCHAR(32) DEFAULT '',
			cron_expr VARCHAR(128) DEFAULT '',
			last_run_at DATETIME NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS instances (
			id VARCHAR(64) PRIMARY KEY,
			template_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			state JSON,
			node_states JSON,
			current_node_id VARCHAR(64),
			error TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS human_tasks (
			id VARCHAR(64) PRIMARY KEY,
			instance_id VARCHAR(64) NOT NULL,
			template_id VARCHAR(64) NOT NULL,
			node_id VARCHAR(64) NOT NULL,
			node_description VARCHAR(255),
			assignee_group VARCHAR(128),
			status VARCHAR(32) NOT NULL,
			input_data JSON,
			result JSON,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
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
		tmpl.ID, tmpl.Name, tmpl.Description, nodes, edges, tmpl.CreatedAt, tmpl.UpdatedAt,
	)
	return err
}

func (s *Store) GetTemplate(id string) (*model.Template, error) {
	row := s.db.QueryRow(
		`SELECT id, name, description, nodes, edges, start_type, cron_expr, last_run_at, created_at, updated_at FROM templates WHERE id = ?`, id,
	)

	var tmpl model.Template
	var nodesJSON, edgesJSON []byte
	err := row.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &nodesJSON, &edgesJSON, &tmpl.StartType, &tmpl.CronExpr, &tmpl.LastRunAt, &tmpl.CreatedAt, &tmpl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(nodesJSON, &tmpl.Nodes)
	json.Unmarshal(edgesJSON, &tmpl.Edges)
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
		var nodesJSON, edgesJSON []byte
		if err := rows.Scan(&tmpl.ID, &tmpl.Name, &tmpl.Description, &nodesJSON, &edgesJSON, &tmpl.StartType, &tmpl.CronExpr, &tmpl.LastRunAt, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(nodesJSON, &tmpl.Nodes)
		json.Unmarshal(edgesJSON, &tmpl.Edges)
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
		inst.ID, inst.TemplateID, inst.Status, state, nodeStates, inst.CurrentNodeID, inst.Error, inst.CreatedAt, inst.UpdatedAt,
	)
	return err
}

func (s *Store) UpdateTemplate(tmpl *model.Template) error {
	tmpl.UpdatedAt = time.Now()
	_, err := s.db.Exec(`UPDATE templates SET name=?, description=?, nodes=?, edges=?, start_type=?, cron_expr=?, last_run_at=?, updated_at=? WHERE id=?`,
		tmpl.Name, tmpl.Description, tmpl.Nodes, tmpl.Edges, tmpl.StartType, tmpl.CronExpr, tmpl.LastRunAt, tmpl.UpdatedAt, tmpl.ID)
	return err
}

func (s *Store) GetInstance(id string) (*model.Instance, error) {
	row := s.db.QueryRow(
		`SELECT id, template_id, status, state, node_states, current_node_id, error, created_at, updated_at FROM instances WHERE id = ?`, id,
	)

	var inst model.Instance
	var stateJSON, nodeStatesJSON []byte
	err := row.Scan(&inst.ID, &inst.TemplateID, &inst.Status, &stateJSON, &nodeStatesJSON, &inst.CurrentNodeID, &inst.Error, &inst.CreatedAt, &inst.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("instance not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(stateJSON, &inst.State)
	json.Unmarshal(nodeStatesJSON, &inst.NodeStates)
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
		var stateJSON, nodeStatesJSON []byte
		if err := rows.Scan(&inst.ID, &inst.TemplateID, &inst.Status, &stateJSON, &nodeStatesJSON, &inst.CurrentNodeID, &inst.Error, &inst.CreatedAt, &inst.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(stateJSON, &inst.State)
		json.Unmarshal(nodeStatesJSON, &inst.NodeStates)
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
		inst.Status, state, nodeStates, inst.CurrentNodeID, inst.Error, inst.UpdatedAt, inst.ID,
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
		task.ID, task.InstanceID, task.TemplateID, task.NodeID, task.NodeDescription, task.AssigneeGroup, task.Status, inputData, result, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *Store) GetHumanTask(id string) (*model.HumanTask, error) {
	row := s.db.QueryRow(
		`SELECT id, instance_id, template_id, node_id, node_description, assignee_group, status, input_data, result, created_at, updated_at FROM human_tasks WHERE id = ?`, id,
	)

	var task model.HumanTask
	var inputJSON, resultJSON []byte
	err := row.Scan(&task.ID, &task.InstanceID, &task.TemplateID, &task.NodeID, &task.NodeDescription, &task.AssigneeGroup, &task.Status, &inputJSON, &resultJSON, &task.CreatedAt, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("human task not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(inputJSON, &task.InputData)
	json.Unmarshal(resultJSON, &task.Result)
	return &task, nil
}

func (s *Store) ListHumanTasks(statuses ...model.HumanTaskStatus) ([]*model.HumanTask, error) {
	var rows *sql.Rows
	var err error

	if len(statuses) > 0 {
		// Build placeholder string
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
		var inputJSON, resultJSON []byte
		if err := rows.Scan(&task.ID, &task.InstanceID, &task.TemplateID, &task.NodeID, &task.NodeDescription, &task.AssigneeGroup, &task.Status, &inputJSON, &resultJSON, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(inputJSON, &task.InputData)
		json.Unmarshal(resultJSON, &task.Result)
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
		task.Status, inputData, result, task.UpdatedAt, task.ID,
	)
	return err
}

// join is a simple strings.Join replacement to avoid importing strings.
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