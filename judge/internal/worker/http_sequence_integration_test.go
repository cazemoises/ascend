//go:build integration

package worker

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHTTPSequencePodman checks pedagogical contracts against executable
// reference implementations and verifies every starter produces public feedback.
// Reference solutions stay in test code, never in the mentor catalog response.
func TestHTTPSequencePodman(t *testing.T) {
	if os.Getenv("ASCEND_TEST_PODMAN") != "1" {
		t.Skip("private rootless Podman opt-in")
	}
	info, err := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output()
	if err != nil || strings.TrimSpace(string(info)) != "true" {
		t.Fatal("rootless Podman required")
	}
	data, err := os.ReadFile("../../../api/internal/handler/activity_templates/fastapi-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Items []struct {
			Title string
			Lab   struct {
				Starter string `json:"starter_code"`
				Cases   []struct {
					Input    string `json:"input"`
					Expected string `json:"expected_output"`
					Public   bool   `json:"is_sample"`
				} `json:"test_cases"`
			} `json:"http_lab"`
		} `json:"items"`
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	solutions := []string{
		`from fastapi import FastAPI
app=FastAPI()
@app.get('/health')
def health(): return {'service':'biblioteca','ok':True}
`,
		`from fastapi import FastAPI
from pydantic import BaseModel
app=FastAPI()
class Message(BaseModel):
 text:str
@app.post('/echo',status_code=201)
def echo(message:Message): return {'text':message.text,'length':len(message.text)}
`,
		`from fastapi import FastAPI
app=FastAPI()
@app.get('/books/{book_id}')
def book(book_id:int,lang:str='pt'): return {'id':book_id,'lang':lang,'available':True}
`,
		`from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel,Field
app=FastAPI()
class Loan(BaseModel):
 reader:str=Field(min_length=1)
 days:int=Field(ge=1,le=14)
@app.exception_handler(RequestValidationError)
async def invalid(request:Request,exc:RequestValidationError):
 return JSONResponse(status_code=422,content={'error':'invalid_payload'})
@app.post('/loans',status_code=201)
def loan(data:Loan): return data.model_dump()
`,
		`from fastapi import FastAPI,HTTPException
from pydantic import BaseModel
app=FastAPI()
books={}
next_id=1
class BookInput(BaseModel):
 title:str
def find(book_id):
 if book_id not in books: raise HTTPException(status_code=404,detail='book_not_found')
 return books[book_id]
@app.get('/books')
def all_books(): return [books[key] for key in sorted(books)]
@app.post('/books',status_code=201)
def create(data:BookInput):
 global next_id
 book={'id':next_id,'title':data.title}
 books[next_id]=book
 next_id+=1
 return book
@app.get('/books/{book_id}')
def get(book_id:int): return find(book_id)
@app.put('/books/{book_id}')
def update(book_id:int,data:BookInput):
 find(book_id)
 books[book_id]={'id':book_id,'title':data.title}
 return books[book_id]
@app.delete('/books/{book_id}')
def delete(book_id:int):
 find(book_id)
 del books[book_id]
 return {'deleted':book_id}
`,
		`from fastapi import FastAPI,HTTPException
from pydantic import BaseModel
app=FastAPI()
available=2
class Reservation(BaseModel):
 book_id:int
 quantity:int
@app.get('/stock/{book_id}')
def stock(book_id:int):
 if book_id!=1: raise HTTPException(status_code=404,detail='book_not_found')
 return {'book_id':1,'available':available}
@app.post('/reservations',status_code=201)
def reserve(data:Reservation):
 global available
 if data.quantity<1: raise HTTPException(status_code=400,detail='invalid_quantity')
 if data.book_id!=1: raise HTTPException(status_code=404,detail='book_not_found')
 if data.quantity>available: raise HTTPException(status_code=409,detail='insufficient_stock')
 available-=data.quantity
 return {'book_id':1,'reserved':data.quantity,'remaining':available}
`,
	}
	if len(catalog.Items) != len(solutions) {
		t.Fatal("catalog/reference mismatch")
	}
	for i, item := range catalog.Items {
		t.Run(item.Title, func(t *testing.T) {
			cases := make([]testCaseRecord, 0, len(item.Lab.Cases))
			for _, tc := range item.Lab.Cases {
				cases = append(cases, testCaseRecord{Input: tc.Input, ExpectedOutput: tc.Expected, IsSample: tc.Public})
			}
			executor := NewDockerExecutor("podman")
			good := evaluateHTTP(context.Background(), executor, solutions[i], cases, 10000, 128)
			if good.status != "accepted" {
				t.Fatalf("reference: %s logs=%s", good.status, good.stderr)
			}
			starter := evaluateHTTP(context.Background(), executor, item.Lab.Starter, cases, 10000, 128)
			if starter.status != "wrong_answer" || starter.failedInput == "" || starter.expectedOutput == "" {
				t.Fatalf("starter must yield public debugger feedback: %+v", starter)
			}
		})
	}
}
