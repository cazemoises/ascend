//go:build integration

package router_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	appmw "github.com/caze/ascend/api/internal/middleware"
	"github.com/caze/ascend/api/internal/router"
	"github.com/caze/ascend/api/internal/store"
	"github.com/redis/go-redis/v9"
)

func TestHTTPLabs(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("disposable PostgreSQL required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var rdb *redis.Client
	real := os.Getenv("ASCEND_HTTP_LAB_REDIS_ADDR") != ""
	if real {
		rdb = redis.NewClient(&redis.Options{Addr: os.Getenv("ASCEND_HTTP_LAB_REDIS_ADDR")})
		defer rdb.Close()
	}
	s := store.New(db, rdb)
	suffix := fmt.Sprint(time.Now().UnixNano())
	var users []string
	var listID, challengeID string
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if challengeID != "" {
			db.ExecContext(ctx, `DELETE FROM submissions WHERE challenge_id=$1`, challengeID)
			db.ExecContext(ctx, `UPDATE list_items SET linked_challenge_id=NULL WHERE linked_challenge_id=$1`, challengeID)
			db.ExecContext(ctx, `DELETE FROM challenges WHERE id=$1`, challengeID)
		}
		if listID != "" {
			db.ExecContext(ctx, `DELETE FROM problem_lists WHERE id=$1`, listID)
		}
		for _, id := range users {
			db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, id)
		}
	}()
	user := func(name, role string) store.User {
		t.Helper()
		u, err := s.CreateUser(ctx, "http-lab-"+name+suffix+"@example.com", "unused")
		if err != nil {
			t.Fatal(err)
		}
		users = append(users, u.ID)
		if _, err = db.ExecContext(ctx, `UPDATE users SET role=$1 WHERE id=$2`, role, u.ID); err != nil {
			t.Fatal(err)
		}
		return u
	}
	mentor, other, student, peer := user("mentor", "teacher"), user("other", "teacher"), user("student", "student"), user("peer", "student")
	pl, err := s.CreateProblemList(ctx, store.CreateProblemListRequest{TeacherID: mentor.ID, Title: "HTTP fixture " + suffix})
	if err != nil {
		t.Fatal(err)
	}
	listID = pl.ID
	item, err := s.CreateListItem(ctx, listID, mentor.ID, store.CreateListItemRequest{Title: "HTTP lab", Difficulty: "easy", Body: "Implemente GET /hello/{name}?greeting= e POST /items/{id}."})
	if err != nil {
		t.Fatal(err)
	}
	limiterRedis := rdb
	if limiterRedis == nil {
		mini := miniredis.RunT(t)
		limiterRedis = redis.NewClient(&redis.Options{Addr: mini.Addr()})
		defer limiterRedis.Close()
	}
	limiter := appmw.NewRateLimiter(limiterRedis, 100, time.Minute, appmw.UserKey)
	server := httptest.NewServer(router.New(s, appmw.NewPangolinAuth(s, ""), limiter, nil, nil))
	defer server.Close()
	base := os.Getenv("LIVE_ROOM_DEPLOYED_URL")
	if base == "" {
		base = server.URL + "/api/v1"
	}
	request := func(method, path, email, viewAs string, body any, want int, out any) string {
		t.Helper()
		var encoded []byte
		if body != nil {
			encoded, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, base+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if email != "" {
			req.Header.Set("Remote-Email", email)
		}
		if viewAs != "" {
			req.Header.Set("X-View-As", viewAs)
		}
		client := http.Client{Timeout: 10 * time.Second}
		rsp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer rsp.Body.Close()
		raw, _ := io.ReadAll(rsp.Body)
		if rsp.StatusCode != want {
			t.Fatalf("%s %s: status=%d want %d: %s", method, path, rsp.StatusCode, want, raw)
		}
		if out != nil {
			if err = json.Unmarshal(raw, out); err != nil {
				t.Fatal(err)
			}
		}
		return string(raw)
	}
	source := `from fastapi import FastAPI
app=FastAPI()
items={}
@app.get('/hello/{name}')
def hello(name:str,greeting:str='Hello'):
 print('lab fixture log',flush=True)
 return {'message':greeting+', '+name+'!'}
@app.post('/items/{key}',status_code=201)
def create(key:str,data:dict):
 items[key]=data
 return data
@app.get('/items/{key}')
def get(key:str):
 return items[key]
`
	in := map[string]any{"starter_code": "from fastapi import FastAPI\napp=FastAPI()\n", "time_limit_ms": 15000, "memory_limit_mb": 128, "test_cases": []map[string]any{
		{"input": `{"method":"GET","path":"/hello/Ana?greeting=Oi"}`, "expected_output": `{"status":200,"json":{"message":"Oi, Ana!"}}`, "is_sample": true},
		{"input": `{"method":"POST","path":"/items/7","json":{"title":"secret-test"}}`, "expected_output": `{"status":201,"json":{"title":"secret-test"}}`, "is_sample": false},
		{"input": `{"method":"GET","path":"/items/7"}`, "expected_output": `{"status":200,"json":{"title":"secret-test"}}`, "is_sample": false},
	}}
	path := "/list-items/" + item.ID + "/http-lab"
	request("POST", path, "", "", in, 401, nil)
	request("POST", path, student.Email, "", in, 403, nil)
	request("POST", path, mentor.Email, "student", in, 403, nil)
	request("POST", path, other.Email, "", in, 404, nil)
	var created map[string]string
	request("POST", path, mentor.Email, "", in, 201, &created)
	challengeID = created["challenge_id"]
	challengePath := "/challenges/" + challengeID
	request("POST", path, mentor.Email, "", in, 409, nil)
	request("GET", challengePath, "", "", nil, 404, nil)
	request("GET", challengePath, student.Email, "", nil, 404, nil)
	request("GET", challengePath, other.Email, "", nil, 404, nil)
	request("GET", challengePath, mentor.Email, "student", nil, 404, nil)
	request("GET", challengePath, mentor.Email, "", nil, 200, nil)
	var persistedPublished bool
	if err = db.QueryRowContext(ctx, `SELECT published FROM problem_lists WHERE id=$1`, listID).Scan(&persistedPublished); err != nil || persistedPublished {
		t.Fatal("creating lab changed publication")
	}
	// Explicit fixture publication, never real editorial content.
	if _, err = db.ExecContext(ctx, `UPDATE problem_lists SET published=true WHERE id=$1`, listID); err != nil {
		t.Fatal(err)
	}
	raw := request("GET", challengePath, student.Email, "", nil, 200, nil)
	if strings.Contains(raw, "secret-test") {
		t.Fatal("hidden HTTP tests leaked")
	}
	request("GET", challengePath+"/test-cases", other.Email, "", nil, 404, nil)
	request("GET", challengePath+"/test-cases", mentor.Email, "", nil, 200, nil)
	request("PUT", challengePath, mentor.Email, "", map[string]string{}, 409, nil)
	request("GET", "/challenges", student.Email, "", nil, 200, nil)
	// Labs cannot accidentally enter collaborative or timed code execution.
	if _, err = s.CreateLiveSessionWithMode(ctx, listID, mentor.ID, 2, "collaborative"); err == nil {
		t.Fatal("lab-only list allowed live session")
	}
	submit := func(email string, explore bool, code string) store.Submission {
		t.Helper()
		body := map[string]any{"language": "fastapi", "source_code": code}
		if explore {
			body["http_requests"] = []map[string]string{{"method": "GET", "path": "/hello/Ana?greeting=Oi"}}
		}
		var queued map[string]string
		request("POST", challengePath+"/submissions", email, "", body, 202, &queued)
		if !real {
			_, err = db.ExecContext(ctx, `UPDATE submissions SET status='accepted',passed_count=1,total_test_cases=1 WHERE id=$1`, queued["submission_id"])
			if err != nil {
				t.Fatal(err)
			}
		}
		var result store.Submission
		deadline := time.Now().Add(30 * time.Second)
		for {
			request("GET", "/submissions/"+queued["submission_id"], email, "", nil, 200, &result)
			if result.Status != "pending" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("Judge did not finish HTTP lab")
			}
			time.Sleep(100 * time.Millisecond)
		}
		return result
	}
	completed := func() bool {
		detail, err := s.GetProblemListDetail(ctx, listID, student.ID, "student")
		if err != nil {
			t.Fatal(err)
		}
		return detail.Items[0].Completed != nil && *detail.Items[0].Completed
	}
	exploratory := submit(student.Email, true, source)
	if exploratory.Status != "accepted" || !exploratory.IsTestRun || completed() {
		t.Fatalf("exploration affected progress: %+v", exploratory)
	}
	request("GET", "/submissions/"+exploratory.ID, peer.Email, "", nil, 403, nil)
	if real && (exploratory.Stdout == nil || !strings.Contains(*exploratory.Stdout, "200") || exploratory.Stderr == nil || !strings.Contains(*exploratory.Stderr, "lab fixture log")) {
		t.Fatal("missing real HTTP response/logs")
	}
	if real {
		wrong := submit(student.Email, false, strings.Replace(source, "greeting+', '+name+'!'", "'incorrect'", 1))
		if wrong.Status != "wrong_answer" || wrong.FailedInput == nil || wrong.ExpectedOutput == nil || completed() {
			t.Fatalf("missing failure feedback: %+v", wrong)
		}
	}
	graded := submit(student.Email, false, source)
	if graded.Status != "accepted" || graded.IsTestRun || !completed() {
		t.Fatalf("grade did not update progress: %+v", graded)
	}
	teacherRun := submit(mentor.Email, false, source)
	if !teacherRun.IsTestRun {
		t.Fatal("mentor generated graded student run")
	}
	var attempts int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM submissions WHERE user_id=$1 AND NOT is_test_run`, student.ID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	want := 1
	if real {
		want = 2
	}
	if attempts != want {
		t.Fatalf("graded attempts %d want %d", attempts, want)
	}
	request("POST", challengePath+"/submissions", student.Email, "", map[string]any{"language": "fastapi", "source_code": source, "http_requests": []map[string]string{{"method": "GET", "path": "http://example.com"}}}, 422, nil)
}
