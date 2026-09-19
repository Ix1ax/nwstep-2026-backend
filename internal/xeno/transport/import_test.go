package transport_test

import (
	"bytes"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/export"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno/model"
	"github.com/rs/zerolog"
	"io"
	"net/http/httptest"
	"testing"
	"time"
)

func TestImportRoundTripAndCorruption(t *testing.T) {
	mod := xeno.NewModule(zerolog.Nop())
	app := fiber.New(fiber.Config{BodyLimit: 20 * 1024 * 1024})
	mod.RegisterV2(app.Group("/api/v2"))
	exp := mod.Manager.CreateExperiment("roundtrip", "earth", model.ModeEvolutionary, 2048, nil)
	if _, err := exp.AddIntervention(model.Intervention{Type: "add_inoculum", Name: "Placed", Value: 45, Params: map[string]float64{"count": 4, "lat": 20, "lng": 35}}); err != nil {
		t.Fatal(err)
	}
	if err := exp.SendCommand("step", 1); err != nil {
		t.Fatal(err)
	}
	var channel string
	for _, ch := range exp.View().LatestSnapshot.Channels {
		if ch.FromID == "ind-0013" {
			channel = ch.ID
			break
		}
	}
	if channel == "" {
		t.Fatal("missing created colony channel")
	}
	if _, err := exp.AddIntervention(model.Intervention{Type: "set_channel", TargetID: channel, Value: 1, Params: map[string]float64{"power": 3}}); err != nil {
		t.Fatal(err)
	}
	if _, err := exp.Replay(400); err != nil {
		t.Fatal(err)
	}
	// Include a future accepted event: import must preserve it without executing early.
	if _, err := exp.AddIntervention(model.Intervention{Type: "impulse", Value: 2, Duration: 60}); err != nil {
		t.Fatal(err)
	}
	data, err := export.ExportJSON(exp)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string, body []byte) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, 2000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		var envelope map[string]any
		if err = json.Unmarshal(b, &envelope); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, envelope
	}
	run := func(payload []byte, want string) string {
		t.Helper()
		code, body := request("POST", "/api/v2/experiments/import", payload)
		if code != 202 {
			t.Fatalf("import status %d: %v", code, body)
		}
		id := body["data"].(map[string]any)["id"].(string)
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			_, body = request("GET", "/api/v2/imports/"+id, nil)
			job := body["data"].(map[string]any)
			if job["status"] != "running" {
				if job["status"] != want {
					t.Fatalf("job: %v", job)
				}
				if result, ok := job["experimentId"].(string); ok {
					return result
				}
				return ""
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("import timed out")
		return ""
	}
	id := run(data, "completed")
	restored, err := mod.Manager.GetExperiment(id)
	if err != nil {
		t.Fatal(err)
	}
	if restored.View().LatestSnapshot.Checksum != exp.View().LatestSnapshot.Checksum {
		t.Fatal("checksum mismatch")
	}
	for _, tick := range []int64{0, 10, 30, 5, 100, 400} {
		a, err := restored.Preview(tick)
		if err != nil {
			t.Fatal(err)
		}
		b, err := exp.Preview(tick)
		if err != nil {
			t.Fatal(err)
		}
		if a.Checksum != b.Checksum {
			t.Fatalf("preview %d differs", tick)
		}
	}
	var bundle export.ExportBundle
	json.Unmarshal(data, &bundle)
	bundle.FinalSnapshot.Checksum = "corrupt"
	bad, _ := json.Marshal(bundle)
	run(bad, "failed")
	if len(mod.Manager.ListExperiments()) != 2 {
		t.Fatal("failed import leaked an experiment")
	}
	code, _ := request("POST", "/api/v2/experiments/import", []byte(`{"broken":true}`))
	if code != 400 {
		t.Fatalf("bad json accepted: %d", code)
	}
}
