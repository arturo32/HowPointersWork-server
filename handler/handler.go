package handler

import (
	"encoding/json"
	"net/http"
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
	var gdbCommands string

	switch strings.TrimSpace(er.Language) {
	case "":
		return input.Task{}, errors.Errorf("require: language")
	case "python":
		image = "python:3"
		filename = "script.py"
		run = "python script.py > $TORK_OUTPUT"
	case "go":
		image = "golang:1.19"
		filename = "main.go"
		run = "go run main.go > $TORK_OUTPUT"
	case "bash":
		image = "alpine:3.18.3"
		filename = "script"
		run = "sh ./script > $TORK_OUTPUT"
	case "c":
		image = "gcc:latest"
		filename = "main.c"
		run = "gcc main.c -o main;./main > $TORK_OUTPUT"
	case "gdb":
		image = "gcc-compiler:latest"
		filename = "usercode.c"
		gdbCommands = "d.gdb"
		//
		run = "mv usercode.c /tmp/user_code/usercode.c; " +
			"gcc -ggdb -O0 -fno-omit-frame-pointer -o /tmp/user_code/usercode /tmp/user_code/usercode.c; "

		if debug_valgrind {
			run += "python3 /tmp/parser/wsgi_backend.py c > $TORK_OUTPUT"
		} else {
			run += "python3 /tmp/parser/wsgi_backend.py c > $TORK_OUTPUT"
		}

		//

	default:
		return input.Task{}, errors.Errorf("unknown language: %s", er.Language)
	}

	return input.Task{
		Name:    "execute code",
		Image:   image,
		Run:     run,
		Timeout: "20s",
		Limits: &input.Limits{
			CPUs:   "1",
			Memory: "2000m",
		},
		Files: map[string]string{
			filename:    er.Code,
			gdbCommands: "set disable-randomization off\nset auto-solib-add off\nstart\nn\nbt full\n",
		},
	}, nil
}