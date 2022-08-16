package notification

import (
	"fmt"
	"path/filepath"
	"runtime"

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

const (
	mediaDir = "MEDIA_DIR"
)

var (
	fileTypes = []string{".wav", ".mp3"}
	fileArg   = command.FileNode("FILE", "Audio file to play", &command.FileCompletor[string]{
		FileTypes: fileTypes,
	})
	builtinArg = command.Arg[string]("BUILTIN", "Built-in audio file to play", command.CompletorFromFunc(func(s string, d *command.Data) (*command.Completion, error) {
		fc := &command.FileCompletor[string]{
			FileTypes:         fileTypes,
			Directory:         d.String(mediaDir),
			IgnoreDirectories: true,
		}
		return fc.Complete(s, d)
	}))
	pythonFileContents = `
from playsound import playsound
import os
import sys

p = os.path.abspath(sys.argv[1])
if not os.path.isfile(p):
  # TODO: use problem matcher here?
  print('not a file')
  exit(0)

# See the following answer for why this logic is needed:
# https://stackoverflow.com/a/68937955/18162937
if os.name == 'nt':
  p = p.replace('\\\\', '\\\\\\\\', 1)

playsound(p)
`
)

func (n *notifier) executable(file string) ([]string, error) {
	return []string{
		fmt.Sprintf("python -c \"%s\" %q", pythonFileContents, file),
	}, nil
}

func getMediaDir(d *command.Data) error {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get notification directory")
	}
	d.Set(mediaDir, filepath.Join(filepath.Dir(thisFile), "media"))
	return nil
}

func (n *notifier) Node() *command.Node {
	// TODO: Eventually have other notification formats ??? (text to phone, slack, etc.) ???
	return command.BranchNode(map[string]*command.Node{
		"built-in b": command.SerialNodes(
			command.SimpleProcessor(func(i *command.Input, o command.Output, d *command.Data, ed *command.ExecuteData) error {
				return getMediaDir(d)
			}, func(i *command.Input, d *command.Data) (*command.Completion, error) {
				return nil, getMediaDir(d)
			}),
			builtinArg,
			command.ExecutableNode(func(o command.Output, d *command.Data) ([]string, error) {
				return n.executable(filepath.Join(d.String(mediaDir), builtinArg.Get(d)))
			}),
		),
	}, command.ShortcutNode("notifier-shortcuts", n, command.SerialNodes(
		fileArg,
		command.ExecutableNode(func(o command.Output, d *command.Data) ([]string, error) {
			return n.executable(fileArg.Get(d))
		}),
	)))
}
