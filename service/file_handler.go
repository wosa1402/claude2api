package service

import (
	"claude2api/genfile"
	"claude2api/openlist"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GeneratedFileDownloadHandler(c *gin.Context) {
	record, ok := genfile.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "file not found or expired"})
		return
	}

	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")

	if record.RemotePath != "" {
		body, contentType, contentLength, err := openlist.DownloadFile(record.RemotePath)
		if err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "failed to fetch file from openlist"})
			return
		}
		defer body.Close()

		if contentType == "" {
			contentType = record.ContentType
		}
		headers := map[string]string{
			"Content-Disposition": mime.FormatMediaType("attachment", map[string]string{"filename": record.Name}),
		}
		c.DataFromReader(http.StatusOK, contentLength, contentType, body, headers)
		return
	}

	c.Header("Content-Type", record.ContentType)
	c.FileAttachment(record.Path, record.Name)
}
