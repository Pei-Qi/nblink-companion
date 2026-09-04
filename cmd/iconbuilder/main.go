package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"
)

func main() {
	svgPath := flag.String("svg", "", "source SVG path")
	pngPath := flag.String("png", "", "optional 1024px PNG output path")
	pngSize := flag.Int("png-size", 1024, "PNG output size")
	icoPath := flag.String("ico", "", "optional multi-size ICO output path")
	icnsPath := flag.String("icns", "", "optional multi-size ICNS output path")
	iconsetPath := flag.String("iconset", "", "optional macOS iconset output directory")
	flag.Parse()
	if *svgPath == "" || (*pngPath == "" && *icoPath == "" && *icnsPath == "" && *iconsetPath == "") {
		flag.Usage()
		os.Exit(2)
	}

	if *pngPath != "" {
		if *pngSize <= 0 || *pngSize > 4096 {
			must(fmt.Errorf("png size must be between 1 and 4096"))
		}
		must(writePNG(*svgPath, *pngPath, *pngSize))
	}
	if *icoPath != "" {
		must(writeICO(*svgPath, *icoPath, []int{16, 24, 32, 48, 64, 128, 256}))
	}
	if *icnsPath != "" {
		must(writeICNS(*svgPath, *icnsPath))
	}
	if *iconsetPath != "" {
		must(writeIconset(*svgPath, *iconsetPath))
	}
}

func writeICNS(svgPath, output string) error {
	types := []string{
		"icp4", "icp5", "icp6", "ic07", "ic08", "ic09", "ic10",
		"ic11", "ic12", "ic13", "ic14",
	}
	sizes := []int{16, 32, 64, 128, 256, 512, 1024, 32, 64, 256, 512}
	images := make([][]byte, len(sizes))
	total := 8
	for i, size := range sizes {
		data, err := encodePNG(svgPath, size)
		if err != nil {
			return err
		}
		images[i] = data
		total += 8 + len(data)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.WriteString("icns"); err != nil {
		return err
	}
	if err := binary.Write(file, binary.BigEndian, uint32(total)); err != nil {
		return err
	}
	for i, data := range images {
		if _, err := file.WriteString(types[i]); err != nil {
			return err
		}
		if err := binary.Write(file, binary.BigEndian, uint32(8+len(data))); err != nil {
			return err
		}
		if _, err := file.Write(data); err != nil {
			return err
		}
	}
	return file.Close()
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func render(svgPath string) (*image.RGBA, error) {
	file, err := os.Open(svgPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	icon, err := oksvg.ReadIconStream(file)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", svgPath, err)
	}
	const size = 1024
	icon.SetTarget(0, 0, size, size)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1)
	return img, nil
}

func encodePNG(svgPath string, size int) ([]byte, error) {
	source, err := render(svgPath)
	if err != nil {
		return nil, err
	}
	img := source
	if size != source.Bounds().Dx() {
		img = image.NewRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(img, img.Bounds(), source, source.Bounds(), draw.Over, nil)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func writePNG(svgPath, output string, size int) error {
	data, err := encodePNG(svgPath, size)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return os.WriteFile(output, data, 0o644)
}

func writeIconset(svgPath, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	outputs := map[string]int{
		"icon_16x16.png": 16, "icon_16x16@2x.png": 32,
		"icon_32x32.png": 32, "icon_32x32@2x.png": 64,
		"icon_128x128.png": 128, "icon_128x128@2x.png": 256,
		"icon_256x256.png": 256, "icon_256x256@2x.png": 512,
		"icon_512x512.png": 512, "icon_512x512@2x.png": 1024,
	}
	for name, size := range outputs {
		if err := writePNG(svgPath, filepath.Join(outputDir, name), size); err != nil {
			return err
		}
	}
	return nil
}

func writeICO(svgPath, output string, sizes []int) error {
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		data, err := encodePNG(svgPath, size)
		if err != nil {
			return err
		}
		images = append(images, data)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := binary.Write(file, binary.LittleEndian, uint16(0)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(len(images))); err != nil {
		return err
	}
	offset := uint32(6 + len(images)*16)
	for i, data := range images {
		sizeByte := byte(sizes[i])
		if sizes[i] == 256 {
			sizeByte = 0
		}
		entry := []byte{sizeByte, sizeByte, 0, 0}
		if _, err := file.Write(entry); err != nil {
			return err
		}
		for _, value := range []any{uint16(1), uint16(32), uint32(len(data)), offset} {
			if err := binary.Write(file, binary.LittleEndian, value); err != nil {
				return err
			}
		}
		offset += uint32(len(data))
	}
	for _, data := range images {
		if _, err := file.Write(data); err != nil {
			return err
		}
	}
	return file.Close()
}
