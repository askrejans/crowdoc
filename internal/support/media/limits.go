package media

import (
	"bytes"
	"errors"
	"image"
)

// A compact bitmap can expand to gigabytes when decoded. Bound pixels before
// allocating a raster, independently of the compressed image byte limit.
const maxRasterPixels = 64 << 20

func checkRasterSize(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("could not read image dimensions")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxRasterPixels/cfg.Height {
		return errors.New("image dimensions are too large")
	}
	return nil
}

func decodeRaster(data []byte) (image.Image, error) {
	if err := checkRasterSize(data); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}
