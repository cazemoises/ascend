//go:build integration

package worker

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const labSolution = `from fastapi import FastAPI
from fastapi.responses import RedirectResponse
import os,socket
app = FastAPI()
items={}
@app.get('/hello/{name}')
def hello(name:str,greeting:str='Hello'):
 print('student log',flush=True)
 return {'message':greeting+', '+name+'!'}
@app.post('/items/{key}',status_code=201)
def create(key:str,data:dict):
 items[key]=data
 return data
@app.get('/items/{key}')
def get(key:str):
 return items[key]
@app.get('/identity')
def identity():
 return {'uid':os.getuid()}
@app.get('/network')
def network():
 try:
  socket.create_connection(('1.1.1.1',80),timeout=.1)
  return {'isolated':False}
 except OSError:
  return {'isolated':True}
@app.get('/redirect')
def redirect():
 return RedirectResponse('http://example.com',status_code=307)
@app.get('/huge')
def huge():
 return 'x'*20000
@app.get('/loop')
async def loop():
 while True: pass
`

func TestHTTPLabPodman(t *testing.T) {
	if os.Getenv("ASCEND_TEST_PODMAN") != "1" {
		t.Skip("private rootless Podman opt-in")
	}
	info, err := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output()
	if err != nil || strings.TrimSpace(string(info)) != "true" {
		t.Fatal("rootless Podman required")
	}
	containers := func() string {
		t.Helper()
		out, err := exec.Command("podman", "ps", "-a", "--filter", "name=ascend-run-", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	before := containers()
	for _, tc := range []struct {
		name, source, status string
		cases                []testCaseRecord
		timeout              int
	}{
		{"stateful methods path query JSON", labSolution, "accepted", []testCaseRecord{
			{Input: `{"method":"GET","path":"/hello/Ana?greeting=Oi"}`, ExpectedOutput: `{"status":200,"json":{"message":"Oi, Ana!"}}`},
			{Input: `{"method":"POST","path":"/items/7","json":{"title":"A","count":2}}`, ExpectedOutput: `{"status":201,"json":{"count":2,"title":"A"}}`},
			{Input: `{"method":"GET","path":"/items/7"}`, ExpectedOutput: `{"status":200,"json":{"title":"A","count":2}}`},
			{Input: `{"method":"GET","path":"/identity"}`, ExpectedOutput: `{"status":200,"json":{"uid":65534}}`},
			{Input: `{"method":"GET","path":"/network"}`, ExpectedOutput: `{"status":200,"json":{"isolated":true}}`},
		}, 15000},
		{"status mismatch", labSolution, "wrong_answer", []testCaseRecord{{Input: `{"method":"GET","path":"/hello/Ana"}`, ExpectedOutput: `{"status":201,"json":{"message":"Hello, Ana!"}}`, IsSample: true}}, 15000},
		{"redirect not followed", labSolution, "accepted", []testCaseRecord{{Input: `{"method":"GET","path":"/redirect"}`, ExpectedOutput: `{"status":307,"json":null}`}}, 15000},
		{"exploration", labSolution, "accepted", []testCaseRecord{{Input: `{"method":"GET","path":"/hello/Ana"}`, IsSample: true}}, 15000},
		{"response budget", labSolution, "runtime_error", []testCaseRecord{{Input: `{"method":"GET","path":"/huge"}`}}, 15000},
		{"startup error", "raise ValueError('fixture startup failure')", "runtime_error", []testCaseRecord{{Input: `{"method":"GET","path":"/"}`}}, 15000},
		{"deadline", labSolution, "time_limit_exceeded", []testCaseRecord{{Input: `{"method":"GET","path":"/loop"}`}}, 1200},
		{"logs budget", "print('x'*100000,flush=True)\n" + labSolution, "runtime_error", []testCaseRecord{{Input: `{"method":"GET","path":"/hello/Ana"}`}}, 15000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := evaluateHTTP(context.Background(), NewDockerExecutor("podman"), tc.source, tc.cases, tc.timeout, 128)
			if res.status != tc.status {
				t.Fatalf("status %s want %s; logs %s", res.status, tc.status, res.stderr)
			}
			if tc.name == "stateful methods path query JSON" && res.stderr != "" {
				t.Error("hidden request logs leaked")
			}
			if tc.name == "status mismatch" && res.failedInput == "" {
				t.Error("missing HTTP feedback")
			}
			if after := containers(); after != before {
				t.Fatalf("sandbox remained: %q", after)
			}
		})
	}
}
