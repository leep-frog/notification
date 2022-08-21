package notification

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		fmt.Sprintf("python3 -c \"%s\" %q", pythonFileContents, filepath.Join("TEMP_DIR", filename)),
	}
}

func TestExecute(t *testing.T) {
	for _, test := range []struct {
		name             string
		n                *notifier
		etc              *command.ExecuteTestCase
		httpResponses    []*httpResponse
		want             *notifier
		wantHTTPRequests []*httpRequest
	}{
		// Audio
		{
			name: "works with file",
			etc: &command.ExecuteTestCase{
				Args: []string{
					"audio",
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
					"a",
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
		// Slack
		{
			name: "handles http error",
			etc: &command.ExecuteTestCase{
				Args: []string{
					"slack",
					"https://hooks.slack.com/services/un/deux/trois",
					"hello",
					"there",
				},
				WantData: &command.Data{Values: map[string]interface{}{
					slackURL.Name(): "https://hooks.slack.com/services/un/deux/trois",
					slackText.Name(): []string{
						"hello",
						"there",
					},
				}},
				WantStderr: "slack http post failed: oopsie\n",
				WantErr:    fmt.Errorf("slack http post failed: oopsie"),
			},
			httpResponses: []*httpResponse{
				newResponse(200, fmt.Errorf("oopsie"), "body oops"),
			},
			wantHTTPRequests: []*httpRequest{
				{
					"https://hooks.slack.com/services/un/deux/trois",
					contentType,
					`{"text":"hello there"}`,
				},
			},
		},
		{
			name: "handles bad status code",
			etc: &command.ExecuteTestCase{
				Args: []string{
					"slack",
					"https://hooks.slack.com/services/un/deux/trois",
					"hello",
					"there",
				},
				WantData: &command.Data{Values: map[string]interface{}{
					slackURL.Name(): "https://hooks.slack.com/services/un/deux/trois",
					slackText.Name(): []string{
						"hello",
						"there",
					},
				}},
				WantStderr: "failed with status code 456:\nbody oops\n",
				WantErr:    fmt.Errorf("failed with status code 456:\nbody oops"),
			},
			httpResponses: []*httpResponse{
				newResponse(456, nil, "body oops"),
			},
			wantHTTPRequests: []*httpRequest{
				{
					"https://hooks.slack.com/services/un/deux/trois",
					contentType,
					`{"text":"hello there"}`,
				},
			},
		},
		{
			name: "sends slack message",
			etc: &command.ExecuteTestCase{
				Args: []string{
					"slack",
					"https://hooks.slack.com/services/un/deux/trois",
					"hello",
					"there",
				},
				WantData: &command.Data{Values: map[string]interface{}{
					slackURL.Name(): "https://hooks.slack.com/services/un/deux/trois",
					slackText.Name(): []string{
						"hello",
						"there",
					},
				}},
			},
			httpResponses: []*httpResponse{
				successResponse("success"),
			},
			wantHTTPRequests: []*httpRequest{
				{
					"https://hooks.slack.com/services/un/deux/trois",
					contentType,
					`{"text":"hello there"}`,
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			command.StubValue(t, &mkTempDir, func(string, string) (string, error) {
				return "TEMP_DIR", nil
			})
			fhc := &fakeHTTPClient{
				t:         t,
				responses: test.httpResponses,
			}
			command.StubValue(t, &getHTTPClient, func() httpInterface { return fhc })
			n := test.n
			if test.n == nil {
				n = &notifier{}
			}
			test.etc.Node = n.Node()
			command.ExecuteTest(t, test.etc)
			command.ChangeTest(t, test.want, n)
			fhc.test(t, test.wantHTTPRequests)
		})
	}
}

func successResponse(body string) *httpResponse {
	return &httpResponse{
		&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
		},
		nil,
	}
}

func newResponse(code int, err error, body string) *httpResponse {
	return &httpResponse{
		&http.Response{
			StatusCode: code,
			Body:       io.NopCloser(strings.NewReader(body)),
		},
		err,
	}
}

type httpRequest struct {
	url         string
	contentType string
	contents    string
}

type httpResponse struct {
	response *http.Response
	err      error
}

type fakeHTTPClient struct {
	t           *testing.T
	gotRequests []*httpRequest
	responses   []*httpResponse
}

func (fhc *fakeHTTPClient) test(t *testing.T, wantRequests []*httpRequest) {
	if len(fhc.responses) != 0 {
		t.Errorf("Unused http responses: %v", fhc.responses)
	}

	if diff := cmp.Diff(wantRequests, fhc.gotRequests, cmp.AllowUnexported(httpRequest{})); diff != "" {
		t.Errorf("Recieved incorrect http requests (-want, +got):\n%s", diff)
	}
}

func (fhc *fakeHTTPClient) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	b, err := io.ReadAll(reader)
	if err != nil {
		fhc.t.Fatalf("failed to read response: %v", err)
	}
	fhc.gotRequests = append(fhc.gotRequests, &httpRequest{
		url,
		contentType,
		string(b),
	})

	if len(fhc.responses) == 0 {
		return nil, fmt.Errorf("ran out of stubbed responses")
	}

	r := fhc.responses[0]
	fhc.responses = fhc.responses[1:]
	return r.response, r.err
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
				Args: "cmd audio b ",
				WantData: &command.Data{Values: map[string]interface{}{
					builtinArg.Name(): "",
					mediaDir:          filepathAbs(t, "media"),
				}},
				Want: []string{
					"break.wav",
					"error.wav",
					"laser.wav",
					"success.wav",
					"warning.wav",
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
