package notification

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/leep-frog/command"
)

func filepathAbs(t *testing.T, path ...string) string {
	p, err := filepath.Abs(filepath.Join(path...))
	if err != nil {
		t.Fatalf("failed to get absolute file path: %v", err)
	}
	return p
}

func TestExecute(t *testing.T) {
	for _, test := range []struct {
		name string
		n    *notifier
		etc  *command.ExecuteTestCase
		want *notifier
	}{
		{
			name: "works with file",
			etc: &command.ExecuteTestCase{
				Args: []string{
					filepath.Join("media", "break.wav"),
				},
				WantData: &command.Data{Values: map[string]interface{}{
					fileArg.Name(): filepathAbs(t, "media", "break.wav"),
				}},
				WantExecuteData: &command.ExecuteData{
					Executable: []string{
						fmt.Sprintf("python -c \"%s\" %q", pythonFileContents, filepathAbs(t, "media", "break.wav")),
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			n := test.n
			if test.n == nil {
				n = &notifier{}
			}
			test.etc.Node = n.Node()
			command.ExecuteTest(t, test.etc)
			command.ChangeTest(t, test.want, n)
		})
	}
}
