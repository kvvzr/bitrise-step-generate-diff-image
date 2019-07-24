package main

import (
	"errors"
	"image"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	png "image/png"

	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	diffimage "github.com/murooka/go-diff-image"
)

type Config struct {
	BeforeImages string `env:"before_images"`
	AfterImages  string `env:"after_images"`
}

func loadImage(fileName string) (image.Image, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		emptyImage := image.NewRGBA(image.Rect(0, 0, 0, 0))
		return emptyImage, nil
	}

	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func saveImage(img image.Image, fileName string) error {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0644)
	defer file.Close()
	if err != nil {
		return err
	}
	png.Encode(file, img)
	return nil
}

func generateDiffImage(beforeFn string, afterFn string, outputDir string) error {
	beforeImage, err := loadImage(beforeFn)
	if err != nil {
		return err
	}
	afterImage, err := loadImage(afterFn)
	if err != nil {
		return err
	}
	diffImage := diffimage.DiffImage(beforeImage, afterImage)
	if beforeImage.Bounds() == diffImage.Bounds() {
		return nil
	}

	outputFn := path.Join(outputDir, path.Base(afterFn))
	err = saveImage(diffImage, outputFn)
	if err != nil {
		return err
	}
	return nil
}

func validateFileModes(before string, after string) (os.FileMode, error) {
	beforeStats, err := os.Stat(before)
	if err != nil {
		return 0, err
	}
	afterStats, err := os.Stat(after)
	if err != nil {
		return 0, err
	}
	if beforeStats.Mode() != afterStats.Mode() {
		return 0, errors.New("File Mode of before_images and after_images are different.")
	}
	return afterStats.Mode(), nil
}

func main() {
	var conf Config
	if err := stepconf.Parse(&conf); err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}
	stepconf.Print(conf)

	outputDir := path.Join(os.Getenv("BITRISE_SOURCE_DIR"), "diff_image_output")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, os.ModePerm)
	}

	before := conf.BeforeImages
	after := conf.AfterImages
	fileMode, err := validateFileModes(before, after)
	if err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}
	if fileMode.IsDir() {
		files, err := ioutil.ReadDir(after)
		if err != nil {
			log.Errorf("Error: %s\n", err)
			os.Exit(1)
		}
		for _, file := range files {
			if filepath.Ext(file.Name()) != ".png" {
				continue
			}
			beforeFn := path.Join(before, file.Name())
			afterFn := path.Join(after, file.Name())
			generateDiffImage(beforeFn, afterFn, outputDir)
		}
	} else {
		generateDiffImage(before, after, outputDir)
	}

	cmdLog, err := exec.Command("bitrise", "envman", "add", "--key", "GENERATED_DIFF_IMAGES_DIR", "--value", outputDir).CombinedOutput()
	if err != nil {
		log.Printf("Failed to expose output with envman, error: %#v | output: %s", err, cmdLog)
		os.Exit(1)
	}
	os.Exit(0)
}
