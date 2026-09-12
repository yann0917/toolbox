package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxUploadBytes 上传文件大小上限：500MB。
const maxUploadBytes = 500 << 20

// errInvalidFileID 标记 file_id 非法（非 UUID，疑似路径注入）。
var errInvalidFileID = errors.New("非法 file_id")

// uploadsDir 返回上传文件目录：dataDir/uploads。
func (s *Server) uploadsDir() string {
	return filepath.Join(s.svc.Config().DataDir, "uploads")
}

// uploadFile 处理 POST /api/uploads：multipart 字段 file 落盘为
// <dataDir>/uploads/<uuid><ext>，返回 {file_id}（即 uuid，不含扩展名）。
// 上传端不做格式限制（格式合法性由 Tool 侧报错），仅限制大小与要求扩展名。
func (s *Server) uploadFile(c *gin.Context) {
	// 前置截断：MaxBytesReader 含 10MB multipart 编码开销余量，
	// 超大请求体在 multipart 解析前即被拒绝，不再落临时文件。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes+10<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		fail(c, CodeBadRequest, "上传失败: "+err.Error())
		return
	}
	if fh.Size > maxUploadBytes {
		fail(c, CodeBadRequest, "文件超过大小上限（500MB）")
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ext == "" {
		// 无扩展名落盘为 <uuid>（无点），解析端 Glob "<uuid>.*" 永不匹配，
		// file_id 必然不可用，直接拒绝（此分支在 MkdirAll 之前，不落盘）。
		fail(c, CodeBadRequest, "文件缺少扩展名（用于识别格式判断），请上传带扩展名的音频文件")
		return
	}
	dir := s.uploadsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		failErr(c, err)
		return
	}
	id := uuid.NewString()
	dst := filepath.Join(dir, id+ext)
	if err := c.SaveUploadedFile(fh, dst); err != nil {
		failErr(c, err)
		return
	}
	ok(c, gin.H{"file_id": id})
}

// fileIDsToFiles 将上传 file_id 解析为本地绝对路径（key 固定 "audio"）。
// 按 <id><ext> 在 uploads 目录 Glob 查找（不落 DB，简单可靠）。
// 防御：id 必须是合法 UUID——id 会进 Glob 模式，非 UUID 一律拒绝防路径注入。
func fileIDsToFiles(dataDir string, fileIDs []string) (map[string]string, error) {
	if len(fileIDs) == 0 {
		return nil, nil
	}
	dir := filepath.Join(dataDir, "uploads")
	files := make(map[string]string, len(fileIDs))
	for _, id := range fileIDs {
		if _, err := uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidFileID, id)
		}
		matches, err := filepath.Glob(filepath.Join(dir, id+".*"))
		if err != nil || len(matches) == 0 {
			return nil, fmt.Errorf("上传文件不存在或已清理: %s", id)
		}
		files["audio"] = matches[0]
	}
	return files, nil
}

// streamUpload 处理 GET /api/uploads/:id/stream：按 id 查找上传文件并以
// 二进制流返回（与 artifacts stream 同为二进制流端点，不套 JSON 包络，
// 404 用真实 HTTP 404）。
func (s *Server) streamUpload(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(404, gin.H{"error": "上传文件不存在"})
		return
	}
	dir := s.uploadsDir()
	matches, err := filepath.Glob(filepath.Join(dir, id+".*"))
	if err != nil || len(matches) == 0 {
		c.JSON(404, gin.H{"error": "上传文件不存在或已清理"})
		return
	}
	abs := filepath.Clean(matches[0])
	// jail 校验：路径必须封闭在 uploads 目录内（id 已是 UUID，此处兜底）。
	jail := filepath.Clean(dir) + string(os.PathSeparator)
	if !strings.HasPrefix(abs+string(os.PathSeparator), jail) {
		c.JSON(404, gin.H{"error": "非法上传路径"})
		return
	}
	c.Header("Accept-Ranges", "bytes")
	// 同源直出二进制流的安全头：禁 MIME 嗅探 + 强制附件下载（XSS 防护面）。
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(abs)+"\"")
	http.ServeFile(c.Writer, c.Request, abs)
}
