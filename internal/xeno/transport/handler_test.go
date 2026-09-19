package transport_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"github.com/ix1ax/nwstep-hackaton-2026/golang/internal/xeno"
)

func setupTestApp() *fiber.App {
	app := fiber.New()
	v2 := app.Group("/api/v2")
	xenoMod := xeno.NewModule(zerolog.Nop())
	xenoMod.RegisterV2(v2)
	return app
}

func TestTransport_APIV2Endpoints(t *testing.T) {
	app := setupTestApp()

	// 1. GET /api/v2/worlds
	req := httptest.NewRequest("GET", "/api/v2/worlds", nil)
	resp, err := app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /api/v2/worlds failed: %v, status: %d", err, resp.StatusCode)
	}

	// 2. POST /api/v2/experiments
	createBody := []byte(`{"name":"Earth-Exp","worldId":"earth","mode":"adaptive","seed":42}`)
	req = httptest.NewRequest("POST", "/api/v2/experiments", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("POST /api/v2/experiments failed: %v, status: %d", err, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var expResp struct {
		Data struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			WorldID string `json:"worldId"`
			Mode    string `json:"mode"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &expResp); err != nil || expResp.Data.ID == "" {
		t.Fatalf("Failed to parse create experiment response: %v, body: %s", err, string(body))
	}
	expID := expResp.Data.ID

	// 3. GET /api/v2/experiments/:id
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID, nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /experiments/:id failed: %v, status: %d", err, resp.StatusCode)
	}

	// 4. POST /api/v2/experiments/:id/commands (step)
	cmdBody := []byte(`{"command":"step"}`)
	req = httptest.NewRequest("POST", "/api/v2/experiments/"+expID+"/commands", bytes.NewReader(cmdBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("POST /commands (step) failed: %v, status: %d", err, resp.StatusCode)
	}

	// 5. GET /api/v2/experiments/:id/state
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID+"/state", nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /state failed: %v, status: %d", err, resp.StatusCode)
	}

	// 6. GET /api/v2/experiments/:id/colonies/colony-01
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID+"/colonies/colony-01", nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /colonies/:colonyId failed: %v, status: %d", err, resp.StatusCode)
	}

	// 7. GET /api/v2/experiments/:id/individuals/ind-01
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID+"/individuals/ind-01", nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /individuals/:individualId failed: %v, status: %d", err, resp.StatusCode)
	}

	// 8. GET /api/v2/experiments/:id/metrics
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID+"/metrics", nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /metrics failed: %v, status: %d", err, resp.StatusCode)
	}

	// 9. GET /api/v2/experiments/:id/export?format=csv
	req = httptest.NewRequest("GET", "/api/v2/experiments/"+expID+"/export?format=csv", nil)
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET /export?format=csv failed: %v, status: %d", err, resp.StatusCode)
	}

	// 10. POST /api/v2/experiments/:id/replay
	replayBody := []byte(`{"targetTick":5}`)
	req = httptest.NewRequest("POST", "/api/v2/experiments/"+expID+"/replay", bytes.NewReader(replayBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, 2000)
	if err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("POST /replay failed: %v, status: %d", err, resp.StatusCode)
	}
}
