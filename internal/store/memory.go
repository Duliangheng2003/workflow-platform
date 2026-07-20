package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/Duliangheng2003/workflow-platform/internal/model"
)

// MemoryStore is an in-memory implementation of Store.
// Thread-safe via sync.RWMutex.
type MemoryStore struct {
	mu         sync.RWMutex
	templates  map[string]*model.Template
	instances  map[string]*model.Instance
	humanTasks map[string]*model.HumanTask
	llmProfiles map[string]*model.LLMProfile
	counter    int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		templates:  make(map[string]*model.Template),
		instances:  make(map[string]*model.Instance),
		humanTasks: make(map[string]*model.HumanTask),
		llmProfiles: make(map[string]*model.LLMProfile),
	}
}

func (m *MemoryStore) nextID(prefix string) string {
	m.counter++
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), m.counter)
}

// Template operations

func (m *MemoryStore) CreateTemplate(tmpl *model.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tmpl.ID == "" {
		tmpl.ID = m.nextID("tmpl")
	}
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

func (m *MemoryStore) GetTemplate(id string) (*model.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tmpl, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return tmpl, nil
}

func (m *MemoryStore) ListTemplates() ([]*model.Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.Template, 0, len(m.templates))
	for _, tmpl := range m.templates {
		result = append(result, tmpl)
	}
	return result, nil
}

func (m *MemoryStore) DeleteTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.templates[id]; !ok {
		return fmt.Errorf("template not found: %s", id)
	}
	delete(m.templates, id)
	return nil
}

func (m *MemoryStore) UpdateTemplate(tmpl *model.Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.templates[tmpl.ID]; !ok {
		return fmt.Errorf("template not found: %s", tmpl.ID)
	}
	tmpl.UpdatedAt = time.Now()
	m.templates[tmpl.ID] = tmpl
	return nil
}

// Instance operations

func (m *MemoryStore) CreateInstance(inst *model.Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inst.ID == "" {
		inst.ID = m.nextID("inst")
	}
	inst.CreatedAt = time.Now()
	inst.UpdatedAt = time.Now()
	m.instances[inst.ID] = inst
	return nil
}

func (m *MemoryStore) GetInstance(id string) (*model.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inst, ok := m.instances[id]
	if !ok {
		return nil, fmt.Errorf("instance not found: %s", id)
	}
	return inst, nil
}

func (m *MemoryStore) ListInstances() ([]*model.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		result = append(result, inst)
	}
	return result, nil
}

func (m *MemoryStore) UpdateInstance(inst *model.Instance) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[inst.ID]; !ok {
		return fmt.Errorf("instance not found: %s", inst.ID)
	}
	inst.UpdatedAt = time.Now()
	m.instances[inst.ID] = inst
	return nil
}

func (m *MemoryStore) DeleteInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.instances[id]; !ok {
		return fmt.Errorf("instance not found: %s", id)
	}
	delete(m.instances, id)
	return nil
}

// HumanTask operations

func (m *MemoryStore) CreateHumanTask(task *model.HumanTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = m.nextID("ht")
	}
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	m.humanTasks[task.ID] = task
	return nil
}

func (m *MemoryStore) GetHumanTask(id string) (*model.HumanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.humanTasks[id]
	if !ok {
		return nil, fmt.Errorf("human task not found: %s", id)
	}
	return task, nil
}

func (m *MemoryStore) ListHumanTasks(statuses ...model.HumanTaskStatus) ([]*model.HumanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*model.HumanTask, 0)
	filter := make(map[model.HumanTaskStatus]bool)
	for _, s := range statuses {
		filter[s] = true
	}

	for _, task := range m.humanTasks {
		if len(filter) == 0 || filter[task.Status] {
			result = append(result, task)
		}
	}
	return result, nil
}

func (m *MemoryStore) ListLLMProfiles() ([]model.LLMProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]model.LLMProfile, 0, len(m.llmProfiles))
	for _, p := range m.llmProfiles {
		result = append(result, *p)
	}
	return result, nil
}

func (m *MemoryStore) GetLLMProfile(id string) (*model.LLMProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.llmProfiles[id]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", id)
	}
	return p, nil
}

func (m *MemoryStore) CreateLLMProfile(p *model.LLMProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.ID == "" {
		p.ID = m.nextID("llm")
	}
	m.llmProfiles[p.ID] = p
	return nil
}

func (m *MemoryStore) UpdateLLMProfile(p *model.LLMProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.llmProfiles[p.ID]; !ok {
		return fmt.Errorf("profile not found: %s", p.ID)
	}
	m.llmProfiles[p.ID] = p
	return nil
}

func (m *MemoryStore) DeleteLLMProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.llmProfiles[id]; !ok {
		return fmt.Errorf("profile not found: %s", id)
	}
	delete(m.llmProfiles, id)
	return nil
}

func (m *MemoryStore) UpdateHumanTask(task *model.HumanTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.humanTasks[task.ID]; !ok {
		return fmt.Errorf("human task not found: %s", task.ID)
	}
	task.UpdatedAt = time.Now()
	m.humanTasks[task.ID] = task
	return nil
}