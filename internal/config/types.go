package config

type Config struct {
	Projects map[string]*Project `json:"projects"`
	Settings *GlobalSettings     `json:"settings,omitempty"`
}

type GlobalSettings struct {
	DefaultBaseImage    string            `json:"default_base_image,omitempty"`
	DefaultEnvironment  map[string]string `json:"default_environment,omitempty"`
	ConfigTemplatesPath string            `json:"config_templates_path,omitempty"`
	AutoUpdate          bool              `json:"auto_update,omitempty"`
	AutoStopOnExit      bool              `json:"auto_stop_on_exit,omitempty"`
	AutoApplyLock       bool              `json:"auto_apply_lock,omitempty"`
}

type Project struct {
	Name          string `json:"name"`
	IslandName    string `json:"island_name"`
	BaseImage     string `json:"base_image"`
	WorkspacePath string `json:"workspace_path"`
	Status        string `json:"status,omitempty"`
	ConfigFile    string `json:"config_file,omitempty"`
}

type ProjectConfig struct {
	Name          string            `json:"name"`
	BaseImage     string            `json:"base_image,omitempty"`
	SetupCommands []string          `json:"setup_commands,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	Ports         []string          `json:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	Dotfiles      []string          `json:"dotfiles,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	Shell         string            `json:"shell,omitempty"`
	User          string            `json:"user,omitempty"`
	Capabilities  []string          `json:"capabilities,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Network       string            `json:"network,omitempty"`
	Restart       string            `json:"restart,omitempty"`
	HealthCheck   *HealthCheck      `json:"health_check,omitempty"`
	Resources     *Resources        `json:"resources,omitempty"`
	Gpus          string            `json:"gpus,omitempty"`
}

type HealthCheck struct {
	Test        []string `json:"test,omitempty"`
	Interval    string   `json:"interval,omitempty"`
	Timeout     string   `json:"timeout,omitempty"`
	StartPeriod string   `json:"start_period,omitempty"`
	Retries     int      `json:"retries,omitempty"`
}

type Resources struct {
	CPUs   string `json:"cpus,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ConfigTemplate struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Config      ProjectConfig `json:"config"`
}

const ProjectConfigJSONSchema = `{
	"$schema": "http://json-schema.org/draft-07/schema#",
	"title": "Coderaft Project Config",
	"type": "object",
	"required": ["name"],
	"properties": {
		"name": {"type": "string", "minLength": 1},
		"base_image": {"type": "string"},
		"setup_commands": {"type": "array", "items": {"type": "string"}},
		"environment": {"type": "object", "additionalProperties": {"type": "string"}},
		"ports": {"type": "array", "items": {"type": "string"}},
		"volumes": {"type": "array", "items": {"type": "string"}},
		"dotfiles": {"type": "array", "items": {"type": "string"}},
		"working_dir": {"type": "string"},
		"shell": {"type": "string"},
		"user": {"type": "string"},
		"capabilities": {"type": "array", "items": {"type": "string"}},
		"labels": {"type": "object", "additionalProperties": {"type": "string"}},
		"network": {"type": "string"},
		"restart": {"type": "string"},
		"health_check": {
			"type": "object",
			"properties": {
				"test": {"type": "array", "items": {"type": "string"}},
				"interval": {"type": "string"},
				"timeout": {"type": "string"},
				"start_period": {"type": "string"},
				"retries": {"type": "integer", "minimum": 0}
			},
			"additionalProperties": false
		},
		"resources": {
			"type": "object",
			"properties": {
				"cpus": {"type": "string"},
				"memory": {"type": "string"}
			},
			"additionalProperties": false
		},
		"gpus": {"type": "string"}
	},
	"additionalProperties": false
}`
