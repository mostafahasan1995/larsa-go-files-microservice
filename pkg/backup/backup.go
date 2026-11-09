package backup

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"larsa-files/domain"

	"git.larsa.io/mahdawi/microservices-commons.git/common"

	"larsa-files/pkg/compress"

	"github.com/google/uuid"

	"larsa-files/utils"
)

func Backup(headerParams *common.HeaderParams, d domain.BackupData, c chan domain.BackupData) {
	replay := d
	folder := utils.UploadPath(headerParams)
	source := filepath.Join(folder, headerParams.Client)

	t := time.Now()
	year := t.Year()
	month := int(t.Month())
	destination := filepath.Join(folder, headerParams.Client, "backups", "files", strconv.Itoa(year), strconv.Itoa(month))
	filename := uuid.New().String()
	err1 := os.MkdirAll(destination, os.ModePerm)
	if err1 != nil {
		replay.Status = "failed"
		replay.Error = err1.Error()
	} else {
		err := compress.Tar(source, destination, filename)
		if err != nil {
			replay.Status = "failed"
			replay.Error = err.Error()
		} else {
			tarFile := filepath.Join(destination, filename+".tar")
			err := compress.Gzip(tarFile, destination)
			if err != nil {
				replay.Status = "failed"
				replay.Error = err.Error()
			} else {
				replay.Status = "success"
				replay.Path = filepath.Join(strings.TrimPrefix(destination, folder+"/"), filename+".tar.gz")
			}
			os.Remove(tarFile)
		}
	}
	c <- replay
}

func Restore(headerParams *common.HeaderParams, d domain.BackupData, c chan domain.BackupData) {
	replay := d
	folder := utils.UploadPath(headerParams)
	tmpFolder := filepath.Join("tmp", uuid.New().String())
	err1 := os.MkdirAll(tmpFolder, os.ModePerm)
	if err1 != nil {
		replay.Status = "failed"
		replay.Error = err1.Error()
	}

	err := compress.UnGzip(filepath.Join(folder, d.Path), tmpFolder)
	if err != nil {
		replay.Status = "failed"
		replay.Error = err.Error()
	} else {
		tarFile := filepath.Join(tmpFolder, strings.TrimSuffix(filepath.Base(d.Path), ".gz"))
		err := compress.Untar(tarFile, folder)
		if err != nil {
			replay.Status = "failed"
			replay.Error = err.Error()
		} else {
			replay.Status = "success"
		}
		os.RemoveAll(tmpFolder)
	}
	c <- replay
}

func Delete(headerParams *common.HeaderParams, d domain.BackupData, c chan domain.BackupData) {
	replay := d
	if d.Path != "" {
		folder := utils.UploadPath(headerParams)
		err := os.RemoveAll(filepath.Join(folder, d.Path))
		if err != nil {
			replay.Status = "failed"
			replay.Error = err.Error()
		} else {
			replay.Status = "success"
		}
	} else {
		replay.Status = "failed"
		replay.Error = "backup path is empty!"
	}

	c <- replay
}
