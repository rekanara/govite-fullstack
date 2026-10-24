// Package httpx holds the response envelope, typed errors and request binding
// shared by every handler. Every response, success or error, uses Envelope.
package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Status struct {
	Code       int    `json:"code"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	MessageInd string `json:"messageInd"`
	TimeStamp  string `json:"timeStamp"`
}

type ErrorBody struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

type Envelope struct {
	Data   any        `json:"data"`
	Meta   any        `json:"meta,omitempty"`
	Error  *ErrorBody `json:"error,omitempty"`
	Status Status     `json:"status"`
}

type PageMeta struct {
	Total       int64  `json:"total"`
	Page        int    `json:"page"`
	Limit       int    `json:"limit"`
	TotalPages  int64  `json:"totalPages"`
	UnreadCount *int64 `json:"unreadCount,omitempty"`
}

func NewPageMeta(total int64, page, limit int) PageMeta {
	pages := (total + int64(limit) - 1) / int64(limit)
	return PageMeta{Total: total, Page: page, Limit: limit, TotalPages: max(1, pages)}
}

var jakarta = time.FixedZone("WIB", 7*60*60)

// Indonesian copy for the UI, keyed by error code, message or status name.
var messageInd = map[string]string{
	"OK":                           "OK",
	"Created":                      "Berhasil dibuat",
	"Bad Request":                  "Permintaan tidak valid",
	"Unauthorized":                 "Belum terautentikasi",
	"Forbidden":                    "Tidak memiliki akses",
	"Not Found":                    "Data tidak ditemukan",
	"Conflict":                     "Terjadi konflik data",
	"Request Entity Too Large":     "Ukuran data terlalu besar",
	"Unprocessable Entity":         "Data tidak dapat diproses",
	"Too Many Requests":            "Terlalu banyak permintaan",
	"Internal Server Error":        "Terjadi kesalahan server",
	"Service Unavailable":          "Layanan belum siap",
	"NOT_READY":                    "Layanan belum siap",
	"NOT_FOUND":                    "Data tidak ditemukan",
	"CONFLICT":                     "Terjadi konflik data",
	"TOO_MANY_REQUESTS":            "Terlalu banyak percobaan, coba lagi nanti",
	"MISSING_REFRESH_TOKEN":        "Refresh token tidak ditemukan",
	"INVALID_CREDENTIALS":          "Email atau password salah",
	"INVALID_REFRESH_TOKEN":        "Refresh token tidak valid atau sudah kedaluwarsa",
	"REFRESH_TOKEN_REUSE_DETECTED": "Refresh token digunakan ulang; sesi dicabut",
	"SESSION_USER_NOT_FOUND":       "Sesi pengguna tidak ditemukan atau tidak aktif",
	"PERMISSION_NOT_DECLARED":      "Permission route belum dideklarasikan",
	"NOT_AUTHENTICATED":            "Belum terautentikasi",
	"PERMISSION_NOT_GRANTED":       "Permission tidak diberikan",
	"TEAM_SCOPE_UNAVAILABLE":       "Scope team membutuhkan provider hierarki",
	"ROLE_NOT_FOUND":               "Role tidak ditemukan",
	"ROLE_NAME_TAKEN":              "Nama role sudah dipakai",
	"SYSTEM_ROLE_IMMUTABLE":        "System role tidak dapat diubah",
	"ROLE_IN_USE":                  "Role masih dipakai pengguna",
	"LAST_PERMISSION_HOLDER":       "Permission harus punya minimal satu pemegang",
	"USER_NOT_FOUND":               "Pengguna tidak ditemukan",
	"EMAIL_TAKEN":                  "Email sudah digunakan",
	"CANNOT_MODIFY_SELF":           "Tidak dapat mengubah role atau menonaktifkan diri sendiri",
	"INVALID_ROLE":                 "Role tidak valid",
	"VALIDATION_ERROR":             "Validasi gagal",
	"INVALID_TOKEN":                "Token tidak valid atau sudah kedaluwarsa",
	"INVALID_TWO_FACTOR_CODE":      "Kode verifikasi dua langkah salah",
	"STORAGE_UNAVAILABLE":          "Penyimpanan file belum dikonfigurasi",
}

func statusName(code int) string {
	if code == fiber.StatusRequestEntityTooLarge {
		return "Request Entity Too Large"
	}
	if t := http.StatusText(code); t != "" {
		return t
	}
	return "Unknown"
}

func buildStatus(code int, typ, message, errorCode string) Status {
	name := statusName(code)
	if message == "" {
		message = name
	}
	ind := messageInd[errorCode]
	if ind == "" {
		ind = messageInd[message]
	}
	if ind == "" {
		ind = messageInd[name]
	}
	if ind == "" {
		ind = message
	}
	return Status{
		Code: code, Name: name, Type: typ, Message: message, MessageInd: ind,
		TimeStamp: time.Now().In(jakarta).Format("2006-01-02T15:04:05.000-07:00"),
	}
}

// OK writes 200 with data.
func OK(c *fiber.Ctx, data any) error { return write(c, fiber.StatusOK, data, nil) }

// Created writes 201 with data.
func Created(c *fiber.Ctx, data any) error { return write(c, fiber.StatusCreated, data, nil) }

// Page writes 200 with rows and list metadata.
func Page(c *fiber.Ctx, rows any, meta PageMeta) error {
	return write(c, fiber.StatusOK, rows, meta)
}

func NoContent(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) }

// File sends a download outside the envelope.
func File(c *fiber.Ctx, filename, contentType string, body []byte) error {
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+strings.ReplaceAll(filename, `"`, "")+`"`)
	return c.Send(body)
}

func write(c *fiber.Ctx, code int, data, meta any) error {
	return c.Status(code).JSON(Envelope{Data: data, Meta: meta, Status: buildStatus(code, "SUCCESS", "", "")})
}
