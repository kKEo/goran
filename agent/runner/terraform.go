package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kkEo/g-mk8s/wire"
)

// Terraform runs init + plan, parks the task for approval (unless
// auto_approve) and applies the saved plan once the server hands the task
// back in the apply phase. Variables travel as TF_VAR_* so they never appear
// in the log.
type Terraform struct{}

type terraformParams struct {
	Source      Source            `json:"source"`
	Binary      string            `json:"binary"`
	Vars        map[string]string `json:"vars"`
	VarFiles    []string          `json:"var_files"`
	InitArgs    []string          `json:"init_args"`
	AutoApprove bool              `json:"auto_approve"`
	Destroy     bool              `json:"destroy"`
}

type savedPlan struct {
	SrcDir string `json:"src_dir"`
}

const (
	planFileName  = "tfplan"
	savedPlanName = "goran-plan.json"
)

func (Terraform) Run(ctx context.Context, rc *Context) (Outcome, error) {
	var p terraformParams
	if err := json.Unmarshal(rc.Task.Params, &p); err != nil {
		return Outcome{}, fmt.Errorf("terraform params: %w", err)
	}
	if p.Binary == "" {
		p.Binary = "terraform"
	}
	extra := []string{"TF_IN_AUTOMATION=1", "TF_INPUT=0"}
	for k, v := range p.Vars {
		extra = append(extra, "TF_VAR_"+k+"="+v)
	}
	rc = rc.withEnv(extra...)
	planPath := filepath.Join(rc.Dir, planFileName)
	statePath := filepath.Join(rc.Dir, savedPlanName)

	if rc.Task.Phase == wire.PhaseApply {
		b, err := os.ReadFile(statePath)
		if err != nil {
			return Outcome{}, fmt.Errorf("no saved plan in %s (the agent may have restarted or changed workdir); re-run the task", rc.Dir)
		}
		var saved savedPlan
		if err := json.Unmarshal(b, &saved); err != nil {
			return Outcome{}, fmt.Errorf("saved plan metadata is corrupt: %w", err)
		}
		if _, err := os.Stat(planPath); err != nil {
			return Outcome{}, fmt.Errorf("plan file %s is missing; re-run the task", planPath)
		}
		rc.Printf("applying approved plan")
		code, err := rc.run(ctx, saved.SrcDir, p.Binary, "apply", "-input=false", "-no-color", planPath)
		return Outcome{ExitCode: code}, err
	}

	srcDir, err := p.Source.Prepare(ctx, rc)
	if err != nil {
		return Outcome{}, err
	}
	initArgs := append([]string{"init", "-input=false", "-no-color"}, p.InitArgs...)
	code, err := rc.run(ctx, srcDir, p.Binary, initArgs...)
	if err != nil || code != 0 {
		return Outcome{ExitCode: code}, err
	}
	planArgs := []string{"plan", "-input=false", "-no-color", "-out=" + planPath}
	if p.Destroy {
		planArgs = append(planArgs, "-destroy")
	}
	for _, vf := range p.VarFiles {
		planArgs = append(planArgs, "-var-file="+vf)
	}
	code, err = rc.run(ctx, srcDir, p.Binary, planArgs...)
	if err != nil || code != 0 {
		return Outcome{ExitCode: code}, err
	}
	if _, err := os.Stat(planPath); err != nil {
		return Outcome{}, fmt.Errorf("%s plan exited 0 but wrote no plan file at %s", p.Binary, planPath)
	}
	meta, _ := json.Marshal(savedPlan{SrcDir: srcDir})
	if err := os.WriteFile(statePath, meta, 0o600); err != nil {
		return Outcome{}, fmt.Errorf("save plan metadata: %w", err)
	}
	if p.AutoApprove {
		rc.Printf("auto_approve is set: applying without waiting")
		code, err = rc.run(ctx, srcDir, p.Binary, "apply", "-input=false", "-no-color", planPath)
		return Outcome{ExitCode: code}, err
	}
	rc.Printf("plan saved to %s; waiting for approval", planPath)
	return Outcome{AwaitApproval: true}, nil
}
