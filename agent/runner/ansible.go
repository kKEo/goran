package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Ansible runs ansible-playbook against a checked out or local source.
// extra_vars are written to a file so values stay out of the command line.
type Ansible struct{}

type ansibleParams struct {
	Source    Source                 `json:"source"`
	Playbook  string                 `json:"playbook"`
	Inventory string                 `json:"inventory"`
	ExtraVars map[string]interface{} `json:"extra_vars"`
	Args      []string               `json:"args"`
	Binary    string                 `json:"binary"`
}

func (Ansible) Run(ctx context.Context, rc *Context) (Outcome, error) {
	var p ansibleParams
	if err := json.Unmarshal(rc.Task.Params, &p); err != nil {
		return Outcome{}, fmt.Errorf("ansible params: %w", err)
	}
	if p.Playbook == "" {
		return Outcome{}, errors.New("ansible task needs a playbook")
	}
	if p.Binary == "" {
		p.Binary = "ansible-playbook"
	}
	rc = rc.withEnv("ANSIBLE_NOCOLOR=1", "ANSIBLE_FORCE_COLOR=0")
	srcDir, err := p.Source.Prepare(ctx, rc)
	if err != nil {
		return Outcome{}, err
	}
	var args []string
	if p.Inventory != "" {
		args = append(args, "-i", p.Inventory)
	}
	if len(p.ExtraVars) > 0 {
		b, err := json.Marshal(p.ExtraVars)
		if err != nil {
			return Outcome{}, err
		}
		varsPath := filepath.Join(rc.Dir, "extra_vars.json")
		if err := os.WriteFile(varsPath, b, 0o600); err != nil {
			return Outcome{}, err
		}
		args = append(args, "-e", "@"+varsPath)
	}
	args = append(args, p.Args...)
	args = append(args, p.Playbook)
	code, err := rc.run(ctx, srcDir, p.Binary, args...)
	return Outcome{ExitCode: code}, err
}
