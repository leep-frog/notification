package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

const (
	mediaDir = "MEDIA_DIR"
)

var (
	fileTypes = []string{".wav", ".mp3"}
	fileArg   = command.FileNode("FILE", "Audio file to play", &command.FileCompleter[string]{
		FileTypes: fileTypes,
	})
	builtinArg = command.Arg[string]("BUILTIN", "Built-in audio file to play", command.CompleterFromFunc(func(s string, d *command.Data) (*command.Completion, error) {
		fc := &command.FileCompleter[string]{
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
  print('not a file')
  exit(0)

# See the following answer for why this logic is needed:
# https://stackoverflow.com/a/68937955/18162937
if os.name == 'nt':
  p = p.replace('\\\\', '\\\\\\\\', 1)

playsound(p)
`
)

var (
	// variable so it can be stubbed out for testing.
	mkTempDir = os.MkdirTemp

	slackURL  = command.Arg[string]("URL", "Slack URL to which to send messages")
	slackText = command.ListArg[string]("TEXT", "Text to send", 1, command.UnboundedList)
)

const (
	contentType = "application/json"
)

// SlackAliaser returns an aliaser to a specific slack url
func SlackAliaser(alias, url string) *sourcerer.Aliaser {
	return sourcerer.NewAliaser(alias, "n", "slack", url)
}

func (n *notifier) executable(file string) ([]string, error) {
	// There are issues if certain characters (e.g. '@') are in the full path.
	// This is always the case with built-in audio files (since go module folders
	// include '@version' as part of the name).
	// By copying to a temporary directory, this no longer becomes an issue.
	dir, err := mkTempDir("", "leep-frog-notification")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for notification: %v", err)
	}
	return []string{
		fmt.Sprintf("cp %q %q", file, dir),
		fmt.Sprintf("python3 -c \"%s\" %q", pythonFileContents, filepath.Join(dir, filepath.Base(file))),
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

func (n *notifier) Node() command.Node {
	return &command.BranchNode{
		Branches: map[string]command.Node{
			"slack s": n.slackNode(),
			"audio a": n.audioNode(),
		},
	}
}

type SlackMessage struct {
	Text string `json:"text"`
}

func (n *notifier) slackNode() command.Node {
	return command.ShortcutNode("slack-shortcuts", n, command.SerialNodes(command.SerialNodes(
		command.Description("Send a slack message"),
		slackURL,
		slackText,
		&command.ExecutorProcessor{F: func(o command.Output, d *command.Data) error {
			client := getHTTPClient()
			msg := &SlackMessage{strings.Join(slackText.Get(d), " ")}

			data, err := json.Marshal(msg)
			if err != nil {
				return o.Annotatef(err, "failed to marhsal object to json")
			}

			resp, err := client.Post(slackURL.Get(d), contentType, bytes.NewBuffer(data))
			if err != nil {
				return o.Annotatef(err, "slack http post failed")
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return o.Annotatef(err, "failed to read response body")
			}

			if resp.StatusCode != 200 {
				return o.Stderrf("failed with status code %d:\n%v\n", resp.StatusCode, string(body))
			}

			return nil
		}},
	)))
}

type httpInterface interface {
	Post(string, string, io.Reader) (*http.Response, error)
}

var (
	// stubbed out for testing purposes
	getHTTPClient = func() httpInterface {
		return &http.Client{}
	}
)

func (n *notifier) audioNode() command.Node {
	return &command.BranchNode{
		Branches: map[string]command.Node{
			// Note: built-in audio files obtained from VS Code audio files:
			// https://github.com/microsoft/vscode/tree/main/src/vs/workbench/contrib/audioCues/browser/media
			"built-in b": command.SerialNodes(
				command.Description("Play a built-in audio file"),
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
		},
		Default: command.ShortcutNode("audio-shortcuts", n, command.SerialNodes(
			command.Description("Play the provided audio file"),
			fileArg,
			command.ExecutableNode(func(o command.Output, d *command.Data) ([]string, error) {
				return n.executable(fileArg.Get(d))
			}),
		)),
		DefaultCompletion: true,
	}
}
