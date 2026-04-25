package parser

type WorkflowFile struct {
	Entrypoint string              `yaml:"entrypoint"`
	Namespace  string              `yaml:"namespace"`
	Forks      int                 `yaml:"forks"`
	Vars       map[string]any      `yaml:"vars"`
	Rules      []Rule              `yaml:"rules"`
	StepTypes  map[string]StepType `yaml:"step_types"`
	Workflows  []Workflow          `yaml:"workflows"`
}

type Rule struct {
	Name        string         `yaml:"name"`
	File        string         `yaml:"file"`
	Description string         `yaml:"description"`
	Salience    int            `yaml:"salience"`
	When        string         `yaml:"when"`
	Action      string         `yaml:"action"`
	SetFact     map[string]any `yaml:"set_fact"`
}

type StepType struct {
	Base        string         `yaml:"base"`
	Description string         `yaml:"description"`
	Defaults    map[string]any `yaml:"defaults"`
	Job         map[string]any `yaml:"job"`
	Params      map[string]any `yaml:"params"`
}

type Workflow struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Vars        map[string]any `yaml:"vars"`
	Steps       []Step         `yaml:"steps"`
}

type Step struct {
	Name        string              `yaml:"name"`
	Type        string              `yaml:"type"`
	Params      map[string]any      `yaml:"params"`
	When        string              `yaml:"when"`
	Register    string              `yaml:"register"`
	Timeout     int                 `yaml:"timeout"`
	ForEach     string              `yaml:"for_each"`
	ForEachKey  string              `yaml:"for_each_key"`
	ForEachSort string              `yaml:"for_each_sort"`
	As          string              `yaml:"as"`
	Concurrency int                 `yaml:"concurrency"`
	Workflow    string              `yaml:"workflow"`
	Vars        map[string]any      `yaml:"vars"`
	Artifacts   map[string]Artifact `yaml:"artifacts"`
	That        []string            `yaml:"that"`
	Msg         string              `yaml:"msg"`
	Rules       []string            `yaml:"rules"`
	Facts       map[string]any      `yaml:"facts"`
}

type Artifact struct {
	Path     string `yaml:"path"`
	Format   string `yaml:"format"`
	Source   string `yaml:"source"`
	Optional bool   `yaml:"optional"`
}
