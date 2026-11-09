package optimization

import (
	"encoding/json"
	"fmt"
	"larsa-files/pkg/uploadS3"
	"os"

	// "larsa-files/pkg/upload"
	// "os"
	"os/exec"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	ffmpeg "github.com/floostack/transcoder/ffmpeg"
)

var SupportedExtentions = []string{".mp4", ".avi", ".mkv", ".mov", ".flv", ".wmv", ".mpg", ".mpeg", ".m4v", ".webm"}

var TranscodingResolutions = []string{"1920x1080", "1280x720", "854x480"}

// fast presets = less compression. finishes faster
// slow presets = more compression. finishes slower
// ultrafast presets = fastest possible encoding.
// veryslow presets = slowest possible encoding.
type videoSettings struct {
	resolution string
	format     string
	codec      string
	skipAudio  bool
	present    string
	skipVideo  bool
}

type VideoConvertOptions struct {
	Format     string
	Resolution string
	Stream     bool
}

func VideoOptimization(headerParams *common.HeaderParams, video_path string) bool {
	var videoConvertOptions VideoConvertOptions

	convertOptions := headerParams.VideoConvertingOptions

	err := json.Unmarshal([]byte(convertOptions), &videoConvertOptions)
	if err != nil {
		fmt.Println("Decode error!", err.Error())
	}

	fileData := GetFileData(video_path)

	if !IsSupportedFormat(fileData.extension, SupportedExtentions) {
		fmt.Println("File is not a video or format is not supported: ", fileData.extension)
		return false
	}
	if videoConvertOptions.Format != "" && videoConvertOptions.Resolution != "" {
		resolution := videoConvertOptions.Resolution
		format := videoConvertOptions.Format

		videoSettings := videoSettings{
			resolution: resolution,
			format:     format,
			codec:      "libx264",
			skipAudio:  false,
			skipVideo:  false,
			present:    "ultrafast",
		}

		newName := fileData.savePath + "/" + fileData.uuid + "-" + resolution + "." + format
		variant := []string{fileData.destination + "/" + fileData.uuid + "-" + resolution + "." + format}
		fileToUpdate := fileData.destination + "/" + fileData.fileName

		transcodeVideo(video_path, newName, videoSettings)
		updateDatabase(headerParams, fileToUpdate, variant)
	} else {
		fmt.Println("no options selected - video operations aborted!")
	}
	return true
}

func GenerateStream(headerParams *common.HeaderParams, fileData fileData, video_path string) {
	fmt.Println("internal generate stream")
	videoPathes := []string{}
	// generate a video for each resolution
	for _, resolution := range TranscodingResolutions {
		videoSettings := videoSettings{
			resolution: resolution,
			format:     "mp4",
			codec:      "libx264",
			skipAudio:  true,
			present:    "ultrafast",
			skipVideo:  false,
		}
		newName := fileData.savePath + "/" + fileData.uuid + "-" + resolution + fileData.extension
		transcodeVideo(video_path, newName, videoSettings)
		videoPathes = append(videoPathes, newName)
	}
	stg := videoSettings{
		skipAudio: false,
		skipVideo: true,
	}
	newName := fileData.savePath + "/" + fileData.uuid + "-" + "audio" + fileData.extension
	videoPathes = append(videoPathes, newName)
	transcodeVideo(video_path, newName, stg)
	generateManifest(videoPathes, fileData)

	variant := []string{fileData.destination + "/" + fileData.uuid + ".mpd"}
	fileToUpdate := fileData.destination + "/" + fileData.fileName

	updateDatabase(headerParams, fileToUpdate, variant)
	for _, videoPath := range videoPathes {
		fmt.Println("videoPath remove :")

		fmt.Println(videoPath)
		removeFile(videoPath)
	}
}

func generateManifest(inputs []string, fileData fileData) {
	output := fileData.savePath + "/" + fileData.uuid + ".mpd"
	segmentName := fileData.uuid + "-" + "$RepresentationID$_$Number%03d$"
	fmt.Println("generating manifest")
	flags := []string{"-dash", "5000", "-frag", "5000", "-segment-name", segmentName, "-out", output}

	flags = append(flags, inputs...)

	cmd := exec.Command("MP4Box", flags...)
	_, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

func transcodeVideo(videoPath string, newVideoPath string, videoSettings videoSettings) {
	var opts ffmpeg.Options

	fmt.Println("transcodeVideo not s3 ")
	fmt.Println("videoPath")
	fmt.Println(videoPath)

	fmt.Println("newVideoPath")
	fmt.Println(newVideoPath)

	hideBanner := true
	overwrite := true
	audioCodec := "aac"
	audioBitrate := "128k"

	if videoSettings.skipVideo {
		opts = ffmpeg.Options{
			SkipVideo:    &videoSettings.skipVideo,
			AudioCodec:   &audioCodec,
			AudioBitrate: &audioBitrate,
			Overwrite:    &overwrite,
		}
	} else if videoSettings.skipAudio {
		opts = ffmpeg.Options{
			OutputFormat: &videoSettings.format,
			HideBanner:   &hideBanner,
			VideoCodec:   &videoSettings.codec,
			Preset:       &videoSettings.present,
			SkipAudio:    &videoSettings.skipAudio,
			Resolution:   &videoSettings.resolution,
			Overwrite:    &overwrite,
		}
	} else {
		opts = ffmpeg.Options{
			OutputFormat: &videoSettings.format,
			HideBanner:   &hideBanner,
			VideoCodec:   &videoSettings.codec,
			Preset:       &videoSettings.present,
			Resolution:   &videoSettings.resolution,
			Overwrite:    &overwrite,
		}
	}

	ffmpegConf := &ffmpeg.Config{
		FfmpegBinPath:   "/usr/bin/ffmpeg",
		FfprobeBinPath:  "/usr/bin/ffprobe",
		ProgressEnabled: true,
	}
	fmt.Println("dossssssssssssssssss")
	progress, err := ffmpeg.
		New(ffmpegConf).
		Input(videoPath).
		Output(newVideoPath).
		WithOptions(opts).
		Start(opts)
	if err != nil {
		fmt.Println(err)
	}
	fileData, err := os.ReadFile(newVideoPath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	fmt.Println(fileData)

	for msg := range progress {
		fmt.Println("doooooooooooooooooooooo")
		fmt.Println(msg.GetProgress())
	}
}

func VideoOptimizationS3(headerParams *common.HeaderParams, video_path string) bool {
	var videoConvertOptions VideoConvertOptions

	convertOptions := headerParams.VideoConvertingOptions

	err := json.Unmarshal([]byte(convertOptions), &videoConvertOptions)
	if err != nil {
		fmt.Println("Decode error!", err.Error())
	}

	fileData := GetFileData(video_path)

	if !IsSupportedFormat(fileData.extension, SupportedExtentions) {
		fmt.Println("File is not a video or format is not supported: ", fileData.extension)
		return false
	}
	if videoConvertOptions.Format != "" && videoConvertOptions.Resolution != "" {
		resolution := videoConvertOptions.Resolution
		format := videoConvertOptions.Format

		videoSettings := videoSettings{
			resolution: resolution,
			format:     format,
			codec:      "libx264",
			skipAudio:  false,
			skipVideo:  false,
			present:    "ultrafast",
		}

		newName := fileData.savePath + "/" + fileData.uuid + "-" + resolution + "." + format
		variant := []string{fileData.destination + "/" + fileData.uuid + "-" + resolution + "." + format}
		fileToUpdate := fileData.destination + "/" + fileData.fileName

		transcodeVideoS3(video_path, newName, videoSettings)
		updateDatabase(headerParams, fileToUpdate, variant)
	} else {
		fmt.Println("no options selected - video operations aborted!")
	}
	return true
}
func GenerateStreamS3(headerParams *common.HeaderParams, fileData fileData, video_path string) {
	fmt.Println("internal generate stream")
	videoPathes := []string{}
	// generate a video for each resolution
	for _, resolution := range TranscodingResolutions {
		videoSettings := videoSettings{
			resolution: resolution,
			format:     "mp4",
			codec:      "libx264",
			skipAudio:  true,
			present:    "ultrafast",
			skipVideo:  false,
		}
		newName := fileData.savePath + "/" + fileData.uuid + "-" + resolution + fileData.extension
		transcodeVideoS3(video_path, newName, videoSettings)
		videoPathes = append(videoPathes, newName)
	}
	stg := videoSettings{
		skipAudio: false,
		skipVideo: true,
	}
	newName := fileData.savePath + "/" + fileData.uuid + "-" + "audio" + fileData.extension
	videoPathes = append(videoPathes, newName)
	transcodeVideoS3(video_path, newName, stg)
	generateManifestS3(videoPathes, fileData)

	variant := []string{fileData.destination + "/" + fileData.uuid + ".mpd"}
	fileToUpdate := fileData.destination + "/" + fileData.fileName

	updateDatabase(headerParams, fileToUpdate, variant)
	for _, videoPath := range videoPathes {
		fmt.Println("videoPath remove :")

		fmt.Println(videoPath)
		removeFile(videoPath)
	}
}
func generateManifestS3(inputs []string, fileData fileData) {
	output := fileData.savePath + "/" + fileData.uuid + ".mpd"
	segmentName := fileData.uuid + "-" + "$RepresentationID$_$Number%03d$"
	fmt.Println("generating manifest")
	flags := []string{"-dash", "5000", "-frag", "5000", "-segment-name", segmentName, "-out", output}

	flags = append(flags, inputs...)

	cmd := exec.Command("MP4Box", flags...)
	_, err := cmd.Output()

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fileDataBuffer, err := os.ReadFile(output)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	uploadS3.StoreFileS3(fileDataBuffer, output)

	if err != nil {
		fmt.Println(err)
	}

}
func transcodeVideoS3(videoPath string, newVideoPath string, videoSettings videoSettings) {
	fmt.Println("transcodeVideoS3")
	fmt.Println("videoPath")
	fmt.Println(videoPath)

	fmt.Println("newVideoPath")
	fmt.Println(newVideoPath)

	var opts ffmpeg.Options

	hideBanner := true
	overwrite := true
	audioCodec := "aac"
	audioBitrate := "128k"

	if videoSettings.skipVideo {
		opts = ffmpeg.Options{
			SkipVideo:    &videoSettings.skipVideo,
			AudioCodec:   &audioCodec,
			AudioBitrate: &audioBitrate,
			Overwrite:    &overwrite,
		}
	} else if videoSettings.skipAudio {
		opts = ffmpeg.Options{
			OutputFormat: &videoSettings.format,
			HideBanner:   &hideBanner,
			VideoCodec:   &videoSettings.codec,
			Preset:       &videoSettings.present,
			SkipAudio:    &videoSettings.skipAudio,
			Resolution:   &videoSettings.resolution,
			Overwrite:    &overwrite,
		}
	} else {
		opts = ffmpeg.Options{
			OutputFormat: &videoSettings.format,
			HideBanner:   &hideBanner,
			VideoCodec:   &videoSettings.codec,
			Preset:       &videoSettings.present,
			Resolution:   &videoSettings.resolution,
			Overwrite:    &overwrite,
		}
	}

	ffmpegConf := &ffmpeg.Config{
		FfmpegBinPath:   "/usr/bin/ffmpeg",
		FfprobeBinPath:  "/usr/bin/ffprobe",
		ProgressEnabled: true,
	}
	fmt.Println("dossssssssssssssssss")
	progress, err := ffmpeg.
		New(ffmpegConf).
		Input(videoPath).
		Output(newVideoPath).
		WithOptions(opts).
		Start(opts)
	if err != nil {
		fmt.Println("Error starting ffmpeg process:", err)
		return
	}
	fileData, err := os.ReadFile(newVideoPath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	err = uploadS3.StoreFileS3(fileData, newVideoPath)
	if err != nil {
		fmt.Println(err)
	}

	for msg := range progress {
		fmt.Println("doooooooooooooooooooooo")
		fmt.Println(msg.GetProgress())
	}
}
