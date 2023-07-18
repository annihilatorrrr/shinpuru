package wsutil

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/zekroTJA/shinpuru/internal/services/database"
	"github.com/zekroTJA/shinpuru/internal/util/embedded"
	"github.com/zekrotja/rogu/log"
)

// GetQueryInt tries to get a value from request query
// and transforms it to an integer value.
//
// If the query value is not provided, def is returened.
//
// If the integer value is smaller than min or larger
// than max (if max is larger than 0), a bounds error
// is returned.
//
// Returned errors are in form of fiber errors with
// appropriate error codes.
func GetQueryInt(ctx *fiber.Ctx, key string, def, min, max int) (int, error) {
	valStr := ctx.Query(key)
	if valStr == "" {
		return def, nil
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if val < min || (max > 0 && val > max) {
		return 0, fiber.NewError(fiber.StatusBadRequest,
			fmt.Sprintf("value of '%s' must be in bounds [%d, %d]", key, min, max))
	}

	return val, nil
}

// GetQueryBool tries to get a value from request query
// and transforms it to an bool value.
//
// If the query value is not provided, def is returened.
//
// Valid string values for <true> are 'true', '1' or
// 'yes. Valid values for <false> are 'false', '0'
// or 'no'.
//
// Returned errors are in form of fiber errors with
// appropriate error codes.
func GetQueryBool(ctx *fiber.Ctx, key string, def bool) (bool, error) {
	v := ctx.Query(key)

	switch strings.ToLower(v) {
	case "":
		return def, nil
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fiber.NewError(fiber.StatusBadRequest, "invalid boolean value")
	}
}

// ErrInternalOrNotFound returns a fiber not found
// error when the passed error is a ErrDatabaseNotFound
// error. Otherwise, the passed error is returned
// unchanged.
func ErrInternalOrNotFound(err error) error {
	if database.IsErrDatabaseNotFound(err) {
		return fiber.ErrNotFound
	}
	return err
}

func GetFS() (f http.FileSystem, err error) {
	fsys, err := fs.Sub(embedded.FrontendFiles, "webdist")
	if err != nil {
		return
	}
	_, err = fsys.Open("index.html")
	if os.IsNotExist(err) {
		log.Info().Tag("WebServer").Msg("Using web files from web/dist/web")
		f = http.Dir("web/dist/web")
		err = nil
		return
	}
	if err != nil {
		return
	}
	log.Info().Tag("WebServer").Msg("Using embedded web files")
	f = http.FS(fsys)
	return
}

func ParseBase64Data(b64Data string) (mime string, data []byte, err error) {
	split := strings.SplitN(b64Data, ",", 2)

	var dataS string
	if len(split) == 1 {
		dataS = split[0]
	} else {
		mime = split[0]
		mime = strings.TrimPrefix(mime, "data:")
		if i := strings.IndexRune(mime, ';'); i != -1 {
			mime = mime[:i]
		}
		dataS = split[1]
	}

	data, err = base64.StdEncoding.DecodeString(dataS)
	return
}
