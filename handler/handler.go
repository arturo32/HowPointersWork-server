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
				if strings.HasPrefix(r, "/tmp/user_code/usercode.c:") {
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "compilation_error:" + r})
				} else {
					log.Debug().Msgf("unknown_error: %s", err.Error())
					return c.JSON(http.StatusBadRequest, map[string]string{"message": "unknown_error"})
				}
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

	switch strings.TrimSpace(er.Language) {
	case "":
		return input.Task{}, errors.Errorf("require: language")
	case "c++":
		image = "gcc-compiler:latest"
		filename = "usercode.cpp"
		run = "mv usercode.cpp /tmp/user_code/usercode.cpp; " +

			// creating file for storing possible compilation errors
			"touch compilation_error.txt; " +

			// compiling user code (without warnings); sending possible compilation error to compilation_error.txt
			"g++ -w -ggdb -O0 -fno-omit-frame-pointer -o /tmp/user_code/usercode /tmp/user_code/usercode.cpp; 2> compilation_error.txt; " +

			// if compilation_error.txt is not empty, send its output to TORK's output
			"if [ -s compilation_error.txt ]; then " +
			"cat compilation_error.txt > $TORK_OUTPUT; else "

		if debug_valgrind {
			run += "cat /tmp/user_code/usercode.vgtrace > $TORK_OUTPUT"
		} else {
			run += "python3 /tmp/parser/wsgi_backend.py c++ > $TORK_OUTPUT"
		}
	case "c":
		image = "gcc-compiler:latest"
		filename = "usercode.c"
		run = "mv usercode.c /tmp/user_code/usercode.c; " +

			// creating file for storing possible compilation errors
			"touch compilation_error.txt; " +

			// compiling user code (without warnings); sending possible compilation error to compilation_error.txt
			"gcc -w -ggdb -O0 -fno-omit-frame-pointer -o /tmp/user_code/usercode /tmp/user_code/usercode.c 2> compilation_error.txt; " +

			// if compilation_error.txt is not empty, send its output to TORK's output
			"if [ -s compilation_error.txt ]; then " +
			"cat compilation_error.txt > $TORK_OUTPUT; else "

		if debug_valgrind {
			run += "cat /tmp/user_code/usercode.vgtrace > $TORK_OUTPUT; fi"
		} else {
			run += "python3 /tmp/parser/wsgi_backend.py c > $TORK_OUTPUT; fi"
		}

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
			Memory: "1000m",
		},
		Files: map[string]string{
			filename: er.Code,
		},
	}, nil
}
