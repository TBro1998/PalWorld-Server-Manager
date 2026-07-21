package api

import (
	"net/http"
	"strconv"

	"github.com/TBro1998/PalWorld-Server-Manager/internal/process"
	"github.com/TBro1998/PalWorld-Server-Manager/internal/sysstat"
	"github.com/gin-gonic/gin"
)

// GetSystemStats returns whole-host resource usage (CPU, memory, and the data
// disk). Each metric group degrades independently: a failure in one leaves its
// fields zeroed rather than failing the request, so the response is always 200.
// @Summary      Get host resource stats
// @Description  Returns host CPU, memory, and data-disk usage
// @Tags         system
// @Produce      json
// @Success      200  {object}  sysstat.HostStats
// @Security     BearerAuth
// @Router       /system/stats [get]
func (r *Router) GetSystemStats(c *gin.Context) {
	c.JSON(http.StatusOK, r.sys.Host(c.Request.Context()))
}

// GetServerStats returns CPU / memory usage of a server's process tree. When the
// server is not running it responds 200 with {running:false, reason:"not_running"}
// (mirroring the REST-status degradation style); a missing server id is 404.
// @Summary      Get server process stats
// @Description  Returns CPU and memory usage aggregated over the server's process tree
// @Tags         servers
// @Produce      json
// @Param        id   path      int  true  "Server ID"
// @Success      200  {object}  sysstat.ProcessStats
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /servers/{id}/stats [get]
func (r *Router) GetServerStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	if !r.serverExists(c, id) {
		return // serverExists already wrote the 404/500 response.
	}

	// last_error drives DeriveStatus; a query failure here is non-fatal — treat
	// it as no error so status derivation still works off in-memory state.
	_, lastError, _, _, _ := r.loadServerPathState(id)
	if r.process.DeriveStatus(id, lastError) != process.StatusRunning {
		c.JSON(http.StatusOK, sysstat.ProcessStats{
			Running: false,
			Reason:  reasonNotRunning,
			NumCPU:  r.sys.NumCPU(),
		})
		return
	}

	pid := r.process.PID(id)
	if pid <= 0 {
		c.JSON(http.StatusOK, sysstat.ProcessStats{
			Running: false,
			Reason:  reasonNotRunning,
			NumCPU:  r.sys.NumCPU(),
		})
		return
	}

	stats := r.sys.Process(c.Request.Context(), strconv.FormatInt(id, 10), pid)
	c.JSON(http.StatusOK, stats)
}
