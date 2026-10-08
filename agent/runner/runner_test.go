package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kkEo/g-mk8s/wire"
)

func TestShellRunnerUsesEnvAndDir(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	rc := &Context{
		Task: &wire.AgentTask{Kind: wire.KindShell, Params: json.RawMessage(`{"script":"echo $SECRET_X; pwd; exit 4"}`)},
		Dir:  dir,
		Out:  &out,
		Env:  []string{"PATH=/usr/bin:/bin", "SECRET_X=s3cr3t"},
	}
	r, err := For(wire.KindShell)
	require.NoError(t, err)
	o, err := r.Run(context.Background(), rc)
	require.NoError(t, err)
	assert.Equal(t, 4, o.ExitCode)
	assert.Contains(t, out.String(), "s3cr3t\n")
	assert.Contains(t, out.String(), "[goran] $ echo $SECRET_X")
}

func TestForRejectsUnknownKind(t *testing.T) {
	_, err := For("docker")
	assert.Error(t, err)
}

func TestTerraformApplyWithoutPlanFails(t *testing.T) {
	var out bytes.Buffer
	rc := &Context{
		Task: &wire.AgentTask{Kind: wire.KindTerraform, Phase: wire.PhaseApply, Params: json.RawMessage(`{"source":{"dir":"."}}`)},
		Dir:  t.TempDir(),
		Out:  &out,
	}
	_, err := (&Terraform{}).Run(context.Background(), rc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no saved plan")
}

// A fake terraform binary records its arguments so the plan/apply protocol can
// be verified without the real tool.
func fakeTerraform(t *testing.T) (string, string) {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := "#!/bin/sh\necho \"$@\" >> " + log + "\necho \"TF_VAR_region=$TF_VAR_region\" >> " + log + "\n" +
		"for a in \"$@\"; do case $a in -out=*) : > \"${a#-out=}\";; esac; done\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "terraform"), []byte(script), 0o755))
	return filepath.Join(bin, "terraform"), log
}

func TestTerraformPlanThenApply(t *testing.T) {
	binary, calls := fakeTerraform(t)
	src := t.TempDir()
	taskDir := t.TempDir()
	params, _ := json.Marshal(map[string]interface{}{
		"source": map[string]string{"dir": src},
		"binary": binary,
		"vars":   map[string]string{"region": "eu-central"},
	})
	var out bytes.Buffer
	rc := &Context{
		Task: &wire.AgentTask{Kind: wire.KindTerraform, Phase: wire.PhasePlan, Params: params},
		Dir:  taskDir, Out: &out, Env: []string{"PATH=/usr/bin:/bin"},
	}
	o, err := (&Terraform{}).Run(context.Background(), rc)
	require.NoError(t, err)
	assert.True(t, o.AwaitApproval)
	assert.Equal(t, 0, o.ExitCode)

	rc.Task.Phase = wire.PhaseApply
	o, err = (&Terraform{}).Run(context.Background(), rc)
	require.NoError(t, err)
	assert.False(t, o.AwaitApproval)

	b, _ := os.ReadFile(calls)
	s := string(b)
	assert.Contains(t, s, "init -input=false -no-color")
	assert.Contains(t, s, "plan -input=false -no-color -out="+filepath.Join(taskDir, "tfplan"))
	assert.Contains(t, s, "apply -input=false -no-color "+filepath.Join(taskDir, "tfplan"))
	assert.Contains(t, s, "TF_VAR_region=eu-central")
	assert.NotContains(t, out.String(), "eu-central", "variable values must not be echoed into the log")
}
