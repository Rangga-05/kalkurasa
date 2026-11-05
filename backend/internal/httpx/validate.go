package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidUUID melaporkan apakah s berformat UUID.
func ValidUUID(s string) bool { return uuidPattern.MatchString(s) }

// ParamUUID membaca parameter path bertipe UUID. ID yang tidak valid tidak
// mungkin ada, sehingga dibalas 404.
func ParamUUID(c *gin.Context, name string) (string, bool) {
	id := c.Param(name)
	if !ValidUUID(id) {
		NotFound(c, "Data tidak ditemukan.")
		return "", false
	}
	return strings.ToLower(id), true
}

// BindJSON membaca isi permintaan sebagai JSON secara ketat: maksimal 1 MB,
// field tidak dikenal ditolak, dan hanya satu nilai JSON yang diizinkan.
// Bila gagal, respons 400/413 sudah dikirim dan hasilnya false.
func BindJSON(c *gin.Context, dst any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err == nil {
		if _, extra := dec.Token(); !errors.Is(extra, io.EOF) {
			err = errors.New("data tambahan setelah JSON")
		}
	}
	if err != nil {
		respondBindError(c, err)
		return false
	}
	return true
}

func respondBindError(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	var typeErr *json.UnmarshalTypeError
	msg := err.Error()

	switch {
	case errors.As(err, &maxErr):
		RespondError(c, http.StatusRequestEntityTooLarge, "payload_too_large", "Ukuran data terlalu besar (maksimal 1 MB).")
	case errors.Is(err, io.EOF):
		BadRequest(c, "invalid_json", "Isi permintaan kosong. Kirim data dalam format JSON.")
	case strings.HasPrefix(msg, "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(msg, "json: unknown field "), `"`)
		BadRequest(c, "invalid_json", fmt.Sprintf("Field tidak dikenal: %s.", field))
	case errors.As(err, &typeErr):
		BadRequest(c, "invalid_json", fmt.Sprintf("Nilai field %q tidak sesuai tipe yang diharapkan.", typeErr.Field))
	default:
		BadRequest(c, "invalid_json", "Format data tidak valid. Kirim JSON yang benar.")
	}
}

// Validation mengumpulkan pesan kesalahan input (Bahasa Indonesia) agar semua
// masalah dapat dilaporkan sekaligus. Validasi berjalan sebelum akses database.
type Validation struct{ msgs []string }

// NewValidation membuat Validation kosong.
func NewValidation() *Validation { return &Validation{} }

// Add menambahkan satu pesan kesalahan.
func (v *Validation) Add(msg string) { v.msgs = append(v.msgs, msg) }

// OK melaporkan apakah tidak ada kesalahan.
func (v *Validation) OK() bool { return len(v.msgs) == 0 }

// Message menggabungkan semua pesan.
func (v *Validation) Message() string { return strings.Join(v.msgs, "; ") }

// Respond membalas 422 dengan semua pesan.
func (v *Validation) Respond(c *gin.Context) {
	UnprocessableEntity(c, "validation_error", v.Message())
}

// Str memvalidasi teks wajib (di-trim, maksimal max karakter).
func (v *Validation) Str(label, val string, max int) string {
	s := strings.TrimSpace(val)
	if s == "" {
		v.Add(label + " wajib diisi.")
		return ""
	}
	if utf8.RuneCountInString(s) > max {
		v.Add(fmt.Sprintf("%s maksimal %d karakter.", label, max))
	}
	return s
}

// OptStr memvalidasi teks opsional (boleh kosong).
func (v *Validation) OptStr(label, val string, max int) string {
	s := strings.TrimSpace(val)
	if utf8.RuneCountInString(s) > max {
		v.Add(fmt.Sprintf("%s maksimal %d karakter.", label, max))
	}
	return s
}

// OneOf memastikan val salah satu dari opts.
func (v *Validation) OneOf(label, val string, opts ...string) string {
	for _, o := range opts {
		if val == o {
			return val
		}
	}
	if val == "" {
		v.Add(label + " wajib diisi.")
	} else {
		v.Add(fmt.Sprintf("%s harus salah satu dari: %s.", label, strings.Join(opts, ", ")))
	}
	return ""
}

// Email memvalidasi alamat email dan mengembalikannya dalam huruf kecil.
func (v *Validation) Email(label, val string) string {
	s := strings.ToLower(strings.TrimSpace(val))
	if s == "" {
		v.Add(label + " wajib diisi.")
		return ""
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || utf8.RuneCountInString(s) > 254 || !strings.Contains(s[strings.Index(s, "@"):], ".") {
		v.Add(label + " bukan alamat email yang valid.")
		return ""
	}
	return s
}

// Password memvalidasi kata sandi: 8 sampai 72 byte (batas bcrypt).
func (v *Validation) Password(label, val string) string {
	switch {
	case val == "":
		v.Add(label + " wajib diisi.")
	case utf8.RuneCountInString(val) < 8:
		v.Add(label + " minimal 8 karakter.")
	case len(val) > 72:
		v.Add(label + " maksimal 72 byte.")
	}
	return val
}

// Int memvalidasi bilangan bulat wajib dalam rentang [min, max].
func (v *Validation) Int(label string, p *int, min, max int) int {
	if p == nil {
		v.Add(label + " wajib diisi.")
		return 0
	}
	if *p < min || *p > max {
		v.Add(fmt.Sprintf("%s harus antara %d dan %d.", label, min, max))
	}
	return *p
}

// DecOpts mengatur validasi bilangan desimal.
type DecOpts struct {
	Min          decimal.Decimal
	Max          decimal.Decimal
	MinExclusive bool  // true: nilai harus > Min
	MaxScale     int32 // jumlah maksimal angka di belakang koma
	Optional     bool  // true: nil dianggap 0
}

// MoneyOpts adalah aturan uang numeric(16,4): tidak negatif (atau > 0 bila
// positive), maksimal 4 angka di belakang koma, di bawah 1 triliun.
func MoneyOpts(positive bool, optional bool) DecOpts {
	return DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(999_999_999_999),
		MinExclusive: positive,
		MaxScale:     4,
		Optional:     optional,
	}
}

// Dec memvalidasi bilangan desimal. Pointer dipakai agar field yang tidak
// dikirim dapat dibedakan dari nilai 0.
func (v *Validation) Dec(label string, p *decimal.Decimal, o DecOpts) decimal.Decimal {
	if p == nil {
		if !o.Optional {
			v.Add(label + " wajib diisi.")
		}
		return decimal.Zero
	}
	d := *p
	ok := true
	if !d.Round(o.MaxScale).Equal(d) {
		v.Add(fmt.Sprintf("%s maksimal %d angka di belakang koma.", label, o.MaxScale))
		ok = false
	}
	if d.LessThan(o.Min) || (o.MinExclusive && d.Equal(o.Min)) {
		switch {
		case o.MinExclusive && o.Min.IsZero():
			v.Add(label + " harus lebih dari 0.")
		case o.Min.IsZero():
			v.Add(label + " tidak boleh negatif.")
		case o.MinExclusive:
			v.Add(fmt.Sprintf("%s harus lebih dari %s.", label, o.Min))
		default:
			v.Add(fmt.Sprintf("%s tidak boleh kurang dari %s.", label, o.Min))
		}
		ok = false
	}
	if d.GreaterThan(o.Max) {
		v.Add(fmt.Sprintf("%s tidak boleh lebih dari %s.", label, o.Max))
		ok = false
	}
	if !ok {
		return decimal.Zero
	}
	return d
}

