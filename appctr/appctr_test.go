package appctr

import (
	"os/exec"
	"testing"
)

func TestCommandEnv(t *testing.T) {
	cmd := exec.Command("sh", "-c", "export")
	cmd.Env = []string{"ENV_T=hello", "A=SSS", "B=ZZZ"}

	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(data))
}
