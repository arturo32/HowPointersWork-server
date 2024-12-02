package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/runabol/tork"
	"github.com/runabol/tork/engine"
	"github.com/runabol/tork/input"
	"github.com/runabol/tork/middleware/web"
)

type ExecRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

var debug_valgrind = false

func Handler(c web.Context) error {
	er := ExecRequest{}

	if err := c.Bind(&er); err != nil {
		c.Error(http.StatusBadRequest, errors.Wrapf(err, "error binding request"))
		return nil
	}

	log.Debug().Msgf("%s", er.Code)

	task, err := buildTask(er)
	if err != nil {
		c.Error(http.StatusBadRequest, err)
		return nil
	}

	result := make(chan string)

	listener := func(j *tork.Job) {
		if j.State == tork.JobStateCompleted {
			result <- j.Execution[0].Result
		} else {
			result <- j.Execution[0].Error
		}
	}

	input := &input.Job{
		Name:  "code execution",
		Tasks: []input.Task{task},
	}

	job, err := engine.SubmitJob(c.Request().Context(), input, listener)

	if err != nil {
		c.Error(http.StatusBadRequest, errors.Wrapf(err, "error executing code"))
		return nil
	}

	log.Debug().Msgf("job %s submitted", job.ID)

	select {
	case r := <-result:
		r = handleGccError(er.Code, r)

		if debug_valgrind {
			return c.JSON(http.StatusOK, r)
		} else {
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(r), &jsonData); err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"message": "Error parsing JSON: " + err.Error()})
			}
			return c.JSON(http.StatusOK, jsonData)
		}

	case <-c.Done():
		return c.JSON(http.StatusGatewayTimeout, map[string]string{"message": "timeout"})
	}
}

func buildTask(er ExecRequest) (input.Task, error) {
	var image string
	var run string
	var filename string
	var compiler string
	var language string

	image = "gcc-compiler:latest"

	switch strings.TrimSpace(er.Language) {
	case "":
		return input.Task{}, errors.Errorf("require: language")
	case "c++":
		compiler = "g++"
		filename = "usercode.cpp"
		language = "c++"

	case "c":
		compiler = "gcc"
		filename = "usercode.c"
		language = "c"

	default:
		return input.Task{}, errors.Errorf("unknown language: %s", er.Language)
	}

	run = "mv " + filename + " /tmp/user_code/" + filename + "; " +
		compiler + " -ggdb -O0 -fno-omit-frame-pointer -o /tmp/user_code/usercode /tmp/user_code/" + filename + " 2> $TORK_OUTPUT; " +
		"[ -s \"${TORK_OUTPUT}\" ] || "

	if debug_valgrind {
		run += "cat /tmp/user_code/usercode.vgtrace > $TORK_OUTPUT"
	} else {
		run += "python3 /tmp/parser/wsgi_backend.py " + language + " > $TORK_OUTPUT"
	}

	return input.Task{
		Name:    "execute code",
		Image:   image,
		Run:     run,
		Timeout: "20s",
		Limits: &input.Limits{
			CPUs:   "1",
			Memory: "1000m",
		},
		Files: map[string]string{
			filename: er.Code,
		},
	}, nil
}

// Helper function to safely convert string to integer
func toInt(s string) int {
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}

type Trace struct {
	Event        string `json:"event"`
	ExceptionMsg string `json:"exception_msg"`
	Line         int    `json:"line"`
}

type Ret struct {
	Code  string  `json:"code"`
	Trace []Trace `json:"trace"`
}

func handleGccError(code string, gccStderr string) string {
	// Define the regex pattern with the filename "usercode.c"
	pattern := `usercode(.c|.cpp):(\d+):(\d+):.+?(error:.*)`

	// Compile the regular expression
	re := regexp.MustCompile(pattern)

	// Check if the regex matches the input string
	isMatch := re.MatchString(gccStderr)

	if !isMatch {
		return gccStderr
	}

	exceptionMsg := "unknown compiler error"
	lineno := 0
	//column := 0

	// Split gccStderr into lines and process
	lines := strings.Split(gccStderr, "\n")
	for _, line := range lines {
		// Try to match the error format
		re := regexp.MustCompile(`usercode(.c|.cpp):(\d+):(\d+):.+?(error:.*$)`)
		matches := re.FindStringSubmatch(line)
		if matches != nil {
			// Extract the line and column number and the error message
			lineno = toInt(matches[1])
			//column = toInt(matches[2])
			exceptionMsg = strings.TrimSpace(matches[3])
			break
		}

		// Handle custom-defined errors from include path
		if strings.Contains(line, "#error") {
			// Extract the error message after '#error'
			exceptionMsg = strings.TrimSpace(strings.Split(line, "#error")[1])
			break
		}

		// Handle linker errors (undefined reference)
		if strings.Contains(line, "undefined ") {
			parts := strings.Split(line, ":")
			exceptionMsg = strings.TrimSpace(parts[len(parts)-1])
			// Match file path and line number
			if strings.Contains(parts[0], "usercode.c") || strings.Contains(parts[0], "usercode.cpp") {
				lineno = toInt(parts[1])
			}
			break
		}
	}

	// Prepare the return value
	ret := Ret{
		Code: code,
		Trace: []Trace{
			{
				Event:        "uncaught_exception",
				ExceptionMsg: exceptionMsg,
				Line:         lineno,
			},
		},
	}

	// Convert to JSON
	retJson, _ := json.Marshal(ret)

	return string(retJson)
}
