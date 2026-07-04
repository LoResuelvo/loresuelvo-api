package test_handler

import (
	"net/http"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/gin-gonic/gin"
)

type TestHandler struct {
	clock *clock.SystemClock
}

func NewTestHandler(clock *clock.SystemClock) *TestHandler {
	return &TestHandler{
		clock: clock,
	}
}

func (h *TestHandler) SetTime(c *gin.Context) {
	var req SetTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Now == nil {
		httphandler.RespondError(c, http.StatusBadRequest, "now is required")
		return
	}

	t, err := time.Parse(time.RFC3339Nano, *req.Now)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid datetime format, expected RFC3339")
		return
	}

	h.clock.SetTime(t)
	c.Status(http.StatusOK)
}

func (h *TestHandler) ClearTestData(c *gin.Context) {
	h.clock.Reset()
	c.Status(http.StatusOK)
}
