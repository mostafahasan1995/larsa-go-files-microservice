package streaming

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"larsa-files/domain"
	"larsa-files/pkg/optimization"
	"larsa-files/pkg/uploadS3"
	"larsa-files/utils"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
)

func GenerateMainfestHLS(headerParams *common.HeaderParams, f domain.File) (string, error) {

	if !optimization.IsSupportedFormat(filepath.Ext(f.Path), optimization.SupportedExtentions) {
		return "", errors.New("File is not a video or format is not supported: " + filepath.Ext(f.Path))
	}

	clientPath := utils.UploadPath(headerParams)
	fileName := filepath.Base(f.Path)
	folder := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	path := filepath.Join(clientPath, strings.TrimSuffix(f.Path, filepath.Base(f.Path)))
	mfp := filepath.Join(path, folder+".m3u8")

	mf, err := os.Create(mfp)
	if err != nil {
		return "", err
	}

	mf.WriteString("#EXTM3U \n")
	mf.WriteString("#EXT-X-VERSION:7 \n")

	for _, resultion := range optimization.TranscodingResolutions {

		axises := strings.Split(resultion, "x")
		hight := axises[1]

		rpf := filepath.Join(path, folder+"-"+hight+".m3u8")
		// imp4f := filepath.Join("/", path, folder+"-"+hight+"-init.mp4")
		sfp := filepath.Join(path, folder+"-"+hight+"-%d.m4s")

		// rpf := folder + "-" + hight + ".m3u8"
		imp4f := folder + "-" + hight + "-init.mp4" //docker
		// sfp := folder + "-" + hight + "-%d.m4s"
		fmt.Println("impf4", imp4f)
		fmt.Println("rpf", rpf)
		fmt.Println("spf", sfp)
		fmt.Println("path", path)

		flags := []string{
			"-i", filepath.Join(path, fileName),
			"-force_key_frames", "expr:gte(t,n_forced*2)",
			"-sc_threshold", "0",
			"-s", resultion,
			"-c:v", "libx264",
			"-b:v", "5000k",
			"-c:a", "copy",
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4",
			"-hls_segment_filename", sfp,
			"-hls_fmp4_init_filename", imp4f,
			rpf,
		}

		mf.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=" + resultion + "\n")
		mf.WriteString(folder + "-" + hight + ".m3u8" + "\n")

		// fmt.Println("ffmpeg flags:", flags)
		cmd := exec.Command("/usr/bin/ffmpeg", flags...)

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println("hls ffmpeg: " + fmt.Sprint(err) + ": " + stderr.String())
			return "", errors.New(fmt.Sprint(err) + " : " + stderr.String())
		}
		// fmt.Println("Result: " + out.String())

		input, err := ioutil.ReadFile(rpf)
		if err != nil {
			return "", err
		}
		output := bytes.Replace(input, []byte(imp4f), []byte(folder+"-"+hight+"-init.mp4"), -1)
		if err = ioutil.WriteFile(rpf, output, os.ModePerm); err != nil {
			return "", err
		}
	}

	return strings.TrimSuffix(f.Path, filepath.Ext(f.Path)) + ".m3u8", nil
}
func GenerateMainfestHLSS3(headerParams *common.HeaderParams, f domain.File) (string, error) {

	if !optimization.IsSupportedFormat(filepath.Ext(f.Path), optimization.SupportedExtentions) {
		return "", errors.New("File is not a video or format is not supported: " + filepath.Ext(f.Path))
	}

	clientPath := utils.UploadPath(headerParams)
	fileName := filepath.Base(f.Path)
	folder := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	path := filepath.Join(clientPath, strings.TrimSuffix(f.Path, filepath.Base(f.Path)))
	mfp := filepath.Join(path, folder+".m3u8")

	mf, err := os.Create(mfp)
	if err != nil {
		return "", err
	}

	mf.WriteString("#EXTM3U \n")
	mf.WriteString("#EXT-X-VERSION:7 \n")
	var filesToUpload []string
	filesToUpload = append(filesToUpload, mfp)
	for _, resultion := range optimization.TranscodingResolutions {

		axises := strings.Split(resultion, "x")
		hight := axises[1]

		rpf := filepath.Join(path, folder+"-"+hight+".m3u8")
		// imp4f := filepath.Join("/", path, folder+"-"+hight+"-init.mp4")
		sfp := filepath.Join(path, folder+"-"+hight+"-%d.m4s")

		// rpf := folder + "-" + hight + ".m3u8"
		imp4f := folder + "-" + hight + "-init.mp4" //docker
		// sfp := folder + "-" + hight + "-%d.m4s"
		fmt.Println("impf4", imp4f)
		fmt.Println("rpf", rpf)
		fmt.Println("spf", sfp)
		fmt.Println("path", path)

		flags := []string{
			"-i", filepath.Join(path, fileName),
			"-force_key_frames", "expr:gte(t,n_forced*2)",
			"-sc_threshold", "0",
			"-s", resultion,
			"-c:v", "libx264",
			"-b:v", "5000k",
			"-c:a", "copy",
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4",
			"-hls_segment_filename", sfp,
			"-hls_fmp4_init_filename", imp4f,
			rpf,
		}

		mf.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=5000000,RESOLUTION=" + resultion + "\n")
		mf.WriteString(folder + "-" + hight + ".m3u8" + "\n")

		// fmt.Println("ffmpeg flags:", flags)
		cmd := exec.Command("/usr/bin/ffmpeg", flags...)

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			fmt.Println("hls ffmpeg: " + fmt.Sprint(err) + ": " + stderr.String())
			return "", errors.New(fmt.Sprint(err) + " : " + stderr.String())
		}
		// fmt.Println("Result: " + out.String())

		input, err := ioutil.ReadFile(rpf)
		if err != nil {
			return "", err
		}
		output := bytes.Replace(input, []byte(imp4f), []byte(folder+"-"+hight+"-init.mp4"), -1)
		if err = ioutil.WriteFile(rpf, output, os.ModePerm); err != nil {
			return "", err
		}
		filesToUpload = append(filesToUpload, rpf)
		filesToUpload = append(filesToUpload, filepath.Join(path, imp4f))
	}
	for _, filePath := range filesToUpload {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println("Error reading file:", err)
		}
		uploadS3.StoreFileS3(fileData, filePath)
	}
	return strings.TrimSuffix(f.Path, filepath.Ext(f.Path)) + ".m3u8", nil
}
