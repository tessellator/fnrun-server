package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/tessellator/executil"
	"github.com/tessellator/fnrun"
)

type contextDefaults struct {
	MaxRunnableTime time.Duration
}

func main() {
	cmd, err := executil.ParseCmd(os.Getenv("FUNCTION_COMMAND"))
	if err != nil {
		panic(err)
	}
	cmd.Env = os.Environ()

	maxFuncCount := 8
	maxFuncCountStr := os.Getenv("MAX_FUNCTION_COUNT")
	if maxFuncCountStr != "" {
		i, err := strconv.Atoi(maxFuncCountStr)
		if err == nil {
			maxFuncCount = i
		}
	}

	maxWaitMillis := 500
	maxWaitMillisStr := os.Getenv("MAX_WAIT_MILLIS")
	if maxWaitMillisStr != "" {
		i, err := strconv.Atoi(maxWaitMillisStr)
		if err == nil {
			maxWaitMillis = i
		}
	}

	maxExecMillis := 30000
	maxExecMillisStr := os.Getenv("MAX_EXEC_MILLIS")
	if maxExecMillisStr != "" {
		i, err := strconv.Atoi(maxExecMillisStr)
		if err == nil {
			maxExecMillis = i
		}
	}

	config := fnrun.InvokerPoolConfig{
		MaxInvokerCount: maxFuncCount,
		InvokerFactory:  fnrun.NewCmdInvokerFactory(cmd),
		MaxWaitDuration: time.Duration(maxWaitMillis) * time.Millisecond,
	}
	pool, err := fnrun.NewInvokerPool(config)
	if err != nil {
		panic(err)
	}

	defaults := contextDefaults{
		MaxRunnableTime: time.Duration(maxExecMillis) * time.Millisecond,
	}
	handler := makeHandler(pool, &defaults)

	http.HandleFunc("/", handler)

	http.ListenAndServe(":8080", nil)
}

func makeHandler(pool *fnrun.InvokerPool, defaults *contextDefaults) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO Define and load up the context with information from the request
		ctx := fnrun.ExecutionContext{
			MaxRunnableTime: defaults.MaxRunnableTime,
			Env:             make(map[string]string),
		}

		data, err := ioutil.ReadAll(r.Body)
		r.Body.Close()

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		input := fnrun.Input{Data: data}

		result, err := pool.Invoke(&input, &ctx)
		if err != nil {
			if err == fnrun.ErrAvailabilityTimeout {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			os.Stdout.WriteString(err.Error() + "\n")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(result.Status)
		w.Write(result.Data)
		// TODO Write the returned env vars as headers
	}
}
