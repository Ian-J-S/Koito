package summary

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"os"
	"path"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

var (
	assetPath         = path.Join("..", "..", "assets")
	titleFontPath     = path.Join(assetPath, "LeagueSpartan-Medium.ttf")
	textFontPath      = path.Join(assetPath, "Jost-Regular.ttf")
	paddingLg         = 30
	paddingMd         = 20
	paddingSm         = 6
	featuredImageSize = 180
	titleFontSize     = 48.0
	textFontSize      = 16.0
	featureTextStart  = paddingLg + paddingMd + featuredImageSize
)

type addTextOpts struct {
	Text     string
	Subtext  string
	Point    image.Point
	FontFile string
	FontSize float64
}

func addImage(baseImage *image.RGBA, path string, point image.Point, height int) error {
	templateFile, err := os.Open(path)
	if err != nil {
		return err
	}

	template, _, err := image.Decode(templateFile)
	if err != nil {
		return err
	}

	resized := resize(template, height, height)

	draw.Draw(baseImage, baseImage.Bounds(), resized, point, draw.Over)

	return nil
}

func addText(baseImage *image.RGBA, opts addTextOpts) error {
	text := opts.Text
	subtext := opts.Subtext
	point := opts.Point
	fontFile := opts.FontFile
	fontSize := opts.FontSize

	fontBytes, err := os.ReadFile(fontFile)
	if err != nil {
		return err
	}

	ttf, err := opentype.Parse(fontBytes)
	if err != nil {
		return err
	}

	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    fontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return err
	}

	drawer := &font.Drawer{
		Dst:  baseImage,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(point.X),
			Y: fixed.I(point.Y),
		},
	}

	drawer.DrawString(text)
	if subtext != "" {
		face, err = opentype.NewFace(ttf, &opentype.FaceOptions{
			Size:    textFontSize,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		drawer.Face = face
		if err != nil {
			return err
		}
		drawer.Src = image.NewUniform(color.RGBA{200, 200, 200, 255})
		drawer.DrawString(" - ")
		drawer.DrawString(subtext)
	}

	return nil
}

func resize(m image.Image, w, h int) *image.RGBA {
	if w < 0 || h < 0 {
		return nil
	}
	r := m.Bounds()
	if w == 0 || h == 0 || r.Dx() <= 0 || r.Dy() <= 0 {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}
	curw, curh := r.Dx(), r.Dy()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			// Get a source pixel.
			subx := x * curw / w
			suby := y * curh / h
			r32, g32, b32, a32 := m.At(subx, suby).RGBA()
			r := uint8(r32 >> 8)
			g := uint8(g32 >> 8)
			b := uint8(b32 >> 8)
			a := uint8(a32 >> 8)
			img.SetRGBA(x, y, color.RGBA{r, g, b, a})
		}
	}
	return img
}
