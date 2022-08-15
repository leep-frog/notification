package notification

import (
	"strings"

	"github.com/leep-frog/command"
	"github.com/leep-frog/command/sourcerer"
)

func CLI() sourcerer.CLI {
	return &notifier{}
}

type notifier struct {
	Shortcuts map[string]map[string][]string
	changed   bool
}

func (n *notifier) Changed() bool   { return n.changed }
func (n *notifier) Setup() []string { return nil }
func (n *notifier) Name() string    { return "n" }

func (n *notifier) MarkChanged() {
	n.changed = true
}

func (n *notifier) ShortcutMap() map[string]map[string][]string {
	if n.Shortcuts == nil {
		n.Shortcuts = map[string]map[string][]string{}
	}
	return n.Shortcuts
}

var (
	fileArg = command.FileNode("FILE", "Audio file to play", &command.FileCompletor[string]{
		FileTypes: []string{".wav", ".mp3"},
	})
	pythonFileContents = strings.Join([]string{
		"from playsound import playsound",
		"import os",
		"import sys",
		"",
		"p = os.path.abspath(sys.argv[1])",
		"if not os.path.isfile(p):",
		"  # TODO: use problem matcher here?",
		"  print('not a file')",
		"  exit(0)",
		"",
		// See the following answer for why this logic is needed:
		// https://stackoverflow.com/a/68937955/18162937
		"if os.name == 'nt':",
		`  p = p.replace('\\', '\\\\', 1)`,
		"",
		"playsound(p)",
	}, "\n")
)

func (n *notifier) Node() *command.Node {
	return command.ShortcutNode("notifier-shortcuts", n, command.SerialNodes(
		fileArg,
		command.ExecutableNode(func(o command.Output, d *command.Data) ([]string, error) {
			return []string{
				"python",
				"-c",
				pythonFileContents,
				fileArg.Get(d),
			}, nil
		}),
	))
}
