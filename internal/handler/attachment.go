package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/httpx"
	"github.com/LinuxSuRen/tick-flow/internal/service"
)

// Attachment wires upload and download (issue #15).
type Attachment struct {
	Svc *service.AttachmentService
}

// Register mounts attachment routes.
func (h *Attachment) Register(rg *gin.RouterGroup) {
	rg.POST("/tickets/:id/attachments", h.upload)
	rg.GET("/attachments/:id/content", h.download)
}

func (h *Attachment) upload(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, httpx.CodeValidationFailed, "multipart field file is required")
		return
	}
	var timelineID *int64
	if v := c.PostForm("timeline_id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			timelineID = &n
		}
	}
	f, err := fileHeader.Open()
	if err != nil {
		httpx.ErrInternal(c, err)
		return
	}
	defer f.Close()

	brief, err := h.Svc.Upload(c.Request.Context(), id, fileHeader.Filename, fileHeader.Header.Get("Content-Type"), f, timelineID)
	if err != nil {
		mapError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": brief})
}

func (h *Attachment) download(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	att, content, err := h.Svc.Download(c.Request.Context(), id)
	if err != nil {
		mapError(c, err)
		return
	}
	defer content.Close()
	c.Header("Content-Disposition", "attachment; filename=\""+sanitizeFilename(att.Filename)+"\"")
	c.Header("Content-Type", att.ContentType)
	c.Status(http.StatusOK)
	_, _ = copyStream(c.Writer, content)
}

// sanitizeFilename strips characters that could break the disposition
// header; quoting above handles the rest.
func sanitizeFilename(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if r == '"' || r == '\\' || r == '\r' || r == '\n' {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func copyStream(dst interface{ Write([]byte) (int, error) }, src interface{ Read([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return total, nil
			}
			return total, err
		}
	}
}
