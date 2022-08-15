package notification

import (
	"path/filepath"
	"testing"

	"github.com/leep-frog/command"
)

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
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			n := test.n
			if test.n == nil {
				n = &notifier{}
			}
			command.ExecuteTest(t, test.etc)
			command.ChangeTest(t, test.want, n)
		})
	}
}
