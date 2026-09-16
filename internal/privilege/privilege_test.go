package privilege

import (
	"os/exec"
	"testing"
)

func TestConfineNilCommand(t *testing.T) {
	Confine(nil)
	Confine(&exec.Cmd{})
}
