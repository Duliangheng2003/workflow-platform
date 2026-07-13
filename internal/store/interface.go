package store

import "github.com/Duliangheng2003/workflow-platform/internal/model"

// Store defines the persistence interface for the workflow platform.
// MVP uses an in-memory implementation; swap this for a database-backed
// implementation without changing the rest of the system.
type Store interface {
	// Template operations
	CreateTemplate(tmpl *model.Template) error
	GetTemplate(id string) (*model.Template, error)
	ListTemplates() ([]*model.Template, error)
	UpdateTemplate(tmpl *model.Template) error
	DeleteTemplate(id string) error

	// Instance operations
	CreateInstance(inst *model.Instance) error
	GetInstance(id string) (*model.Instance, error)
	ListInstances() ([]*model.Instance, error)
	UpdateInstance(inst *model.Instance) error
		DeleteInstance(id string) error

	// HumanTask operations
	CreateHumanTask(task *model.HumanTask) error
	GetHumanTask(id string) (*model.HumanTask, error)
	ListHumanTasks(status ...model.HumanTaskStatus) ([]*model.HumanTask, error)
	UpdateHumanTask(task *model.HumanTask) error
}