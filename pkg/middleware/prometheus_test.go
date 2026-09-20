package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestPrometheusMetrics(t *testing.T) {
	app := fiber.New()
	app.Use(NewPrometheusMiddleware(app, "nwstep-api"))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Send a request to /test
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Get /metrics
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	respMetrics, err := app.Test(reqMetrics)
	if err != nil {
		t.Fatal(err)
	}
	defer respMetrics.Body.Close()

	body, _ := io.ReadAll(respMetrics.Body)
	t.Logf("METRICS OUTPUT:\n%s", string(body))
}
