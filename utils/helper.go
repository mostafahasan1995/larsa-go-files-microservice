package utils

import (
	"larsa-files/domain"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

func IntegerGreater(x interface{}, y interface{}) bool {

	if x == nil || y == nil {
		return false
	}

	var xint int = 0
	var yint int = 0

	xtyp := reflect.TypeOf(x)
	switch xtyp.Kind() {
	case reflect.Int:
		xint = int(x.(int))
	case reflect.Int32:
		xint = int(x.(int32))
	case reflect.Int16:
		xint = int(x.(int16))
	case reflect.Int64:
		xint = int(x.(int64))
	}

	ytyp := reflect.TypeOf(y)
	switch ytyp.Kind() {
	case reflect.Int:
		yint = int(y.(int))
	case reflect.Int32:
		yint = int(y.(int32))
	case reflect.Int16:
		yint = int(y.(int16))
	case reflect.Int64:
		yint = int(y.(int64))
	}

	if xint <= yint {
		return false
	}

	return true
}

func GetBytes(str string) (int, error) {
	units := map[string]int{
		"G":  1000 * 1000 * 1000,
		"Gi": 1024 * 1024 * 1024,
		"M":  1000 * 1000,
		"Mi": 1024 * 1024,
		"K":  1000,
		"Ki": 1024,
		"":   1,
	}
	re := regexp.MustCompile("[0-9]+")
	size := re.FindString(str)
	unit := strings.ReplaceAll(str, size, "")

	s, err := strconv.Atoi(size)
	if err != nil {
		return 0, err
	}

	if unit == "" {
		return s, nil
	} else {
		v := units[unit]
		return s * v, nil
	}
}

func Find(a string, s []string) (int, bool) {
	for i, v := range s {
		if v == a {
			return i, true
		}

	}
	return -1, false
}

func SetFilePath(headerParams *common.HeaderParams, files ...*domain.File) {
	var path string
	UPLOADS_PUBLIC_URL, err := common.GetOptionValue("UPLOADS_PUBLIC_URL", headerParams)
	if err == nil && UPLOADS_PUBLIC_URL != nil {
		path = UPLOADS_PUBLIC_URL.(string) + "/file/"
	} else {
		path = "/file/"
	}

	for _, f := range files {
		f.Path = path + f.Path
		if len(f.Variants) != 0 {
			for i, _ := range f.Variants {
				f.Variants[i] = path + f.Variants[i]
			}
		}
	}
}

// GetDriveHTTPPath returns the HTTP serving path for a drive file without modifying the original
func GetDriveHTTPPath(headerParams *common.HeaderParams, filePath string) string {
	var path string
	UPLOADS_PUBLIC_URL, err := common.GetOptionValue("UPLOADS_PUBLIC_URL", headerParams)
	if err == nil && UPLOADS_PUBLIC_URL != nil {
		path = UPLOADS_PUBLIC_URL.(string) + "/file/"
	} else {
		path = "/file/"
	}

	// If the path is already a full URL (http:// or https://), return it as is
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return filePath
	}

	// Only prepend if path doesn't already start with /file/ or the public URL
	if !strings.HasPrefix(filePath, "/file/") && !strings.HasPrefix(filePath, path) {
		return path + filePath
	}
	return filePath
}

// SetDriveFilePath sets the HTTP serving path for drive files by creating a copy with HTTP paths
// This preserves the original S3 path for S3 operations while providing HTTP paths for serving
func SetDriveFilePath(headerParams *common.HeaderParams, files ...*domain.File) {
	for _, f := range files {
		// Set HTTP serving path
		f.Path = GetDriveHTTPPath(headerParams, f.Path)

		// Handle variants
		if len(f.Variants) != 0 {
			for i, _ := range f.Variants {
				f.Variants[i] = GetDriveHTTPPath(headerParams, f.Variants[i])
			}
		}
	}
}

func UploadPath(headerParams *common.HeaderParams) string {
	var p string
	uploadPath, err := common.GetOptionValue("UPLOADS_PATH", headerParams)
	if err == nil && uploadPath != nil && uploadPath != "" {
		p = uploadPath.(string)
	} else {
		p = "files/"
	}

	return p
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
