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

func executble(t *testing.T, filename string) []string {
	absFile := filepathAbs(t, "media", filename)
	return []string{
		fmt.Sprintf(`cp %q "TEMP_DIR"`, absFile),
		fmt.Sprintf("python -c \"%s\" %q", pythonFileContents, filepath.Join("TEMP_DIR", filename)),
	}
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
					Executable: executble(t, "break.wav"),
				},
			},
		},
		{
			name: "works with built-in",
			etc: &command.ExecuteTestCase{
				Args: []string{
					"b",
					"error.wav",
				},
				WantData: &command.Data{Values: map[string]interface{}{
					builtinArg.Name(): "error.wav",
					mediaDir:          filepathAbs(t, "media"),
				}},
				WantExecuteData: &command.ExecuteData{
					Executable: executble(t, "error.wav"),
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command.StubValue(t, &mkTempDir, func(string, string) (string, error) {
				return "TEMP_DIR", nil
			})
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

func TestComplete(t *testing.T) {
	for _, test := range []struct {
		name string
		n    *notifier
		ctc  *command.CompleteTestCase
	}{
		{
			name: "completes built-in audio file names",
			ctc: &command.CompleteTestCase{
				Args: "cmd b ",
				WantData: &command.Data{Values: map[string]interface{}{
					builtinArg.Name(): "",
					mediaDir:          filepathAbs(t, "media"),
				}},
				Want: []string{
					"break.wav",
					"error.wav",
					" ",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command.StubValue(t, &mkTempDir, func(string, string) (string, error) {
				return "TEMP_DIR", nil
			})
			n := test.n
			if test.n == nil {
				n = &notifier{}
			}
			test.ctc.Node = n.Node()
			command.CompleteTest(t, test.ctc)
		})
	}
}
