package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/runabol/tork/cli"
	"github.com/runabol/tork/engine"
	"github.com/runabol/tork/input"
	"github.com/runabol/tork/middleware/web"
	"net/http"
	"os"

	"github.com/runabol/tork/conf"
)

func main() {
	// Load the Tork config file (if exists)
	if err := conf.LoadConfig(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	engine.RegisterEndpoint(http.MethodPost, "/myendpoint", handler)

	if err := cli.New().Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type ExecRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

func handler(c web.Context) error {
	req := ExecRequest{}
	if err := c.Bind(&req); err != nil {
		c.Error(http.StatusBadRequest, err)

		return nil
	}

	task, err := buildTask(req)
	if err != nil {
		c.Error(http.StatusBadRequest, err)
		return nil
	}

	taskInput := &input.Job{
		Name:  "code execution",
		Tasks: []input.Task{task},
	}

	job, err := engine.SubmitJob(c.Request().Context(), taskInput)
	if err != nil {
		return err
	}

	fmt.Printf("job %s submitted!\n", job.ID)

	return c.JSON(http.StatusOK, req)
}

func buildTask(er ExecRequest) (input.Task, error) {
	var image string
	var run string
	var filename string

	switch er.Language {
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
		image = "ubuntu:mantic"
		filename = "script"
		run = "sh ./script > $TORK_OUTPUT"
	default:
		return input.Task{}, errors.Errorf("unknown language: %s", er.Language)
	}

	return input.Task{
		Name:  "execute code",
		Image: image,
		Run:   run,
		Files: map[string]string{
			filename: er.Code,
		},
	}, nil
}
