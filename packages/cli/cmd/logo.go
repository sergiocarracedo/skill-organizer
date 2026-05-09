package cmd

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"math"
	"os"
	"strings"
)

const minASCIIBrightness = 150

//go:embed assets/logo_color.png
var logoPNG []byte

//go:embed assets/ascii-text-art.txt
var asciiLogo string

func cliLogo() string {
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return asciiLogo
	}
	if os.Getenv("NO_COLOR") != "" {
		return asciiLogo
	}
	return renderGradientASCII()
}

func cliHeader() string {
	return fmt.Sprintf("skill-organizer v%s", version)
}

func cliHelpHeader() string {
	return fmt.Sprintf("%s\n%s\ncommit %s, built %s\n", cliLogo(), cliHeader(), commit, date)
}

func printCLIHeader(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, cliLogo())
	_, _ = fmt.Fprintln(writer, cliHeader())
	_, _ = fmt.Fprintf(writer, "commit %s, built %s\n\n", commit, date)
}

func decodeLogoImage() (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func renderGradientASCII() string {
	img, err := decodeLogoImage()
	if err != nil {
		return asciiLogo
	}

	lines := strings.Split(strings.TrimRight(asciiLogo, "\n"), "\n")
	gradient := sampleGradient(img, maxLineWidth(lines))

	var builder strings.Builder
	for lineIndex, line := range lines {
		for i, r := range line {
			if r == ' ' {
				builder.WriteRune(r)
				continue
			}
			idx := i
			if idx >= len(gradient) {
				idx = len(gradient) - 1
			}
			builder.WriteString(ansiForeground(gradient[idx]))
			builder.WriteRune(r)
		}
		builder.WriteString("\x1b[0m")
		if lineIndex < len(lines)-1 {
			builder.WriteByte('\n')
		}
	}

	return builder.String()
}

func sampleGradient(img image.Image, width int) []colorRGBA {
	if width <= 0 {
		return []colorRGBA{{r: 255, g: 255, b: 255, a: 255}}
	}

	bounds := img.Bounds()
	colors := make([]colorRGBA, width)
	for i := range width {
		x := bounds.Min.X
		if width > 1 {
			x = bounds.Min.X + int(math.Round(float64(i)*float64(bounds.Dx()-1)/float64(width-1)))
		}
		colors[i] = averageOpaqueColumnColor(img, x)
		if colors[i].a == 0 {
			colors[i] = colorRGBA{r: 255, g: 255, b: 255, a: 255}
			continue
		}
		colors[i] = brightenASCIIColor(colors[i])
	}
	return colors
}

func brightenASCIIColor(c colorRGBA) colorRGBA {
	brightness := (299*int(c.r) + 587*int(c.g) + 114*int(c.b)) / 1000
	if brightness >= minASCIIBrightness {
		return c
	}
	if brightness <= 0 {
		return colorRGBA{r: 255, g: 255, b: 255, a: c.a}
	}

	scale := float64(minASCIIBrightness) / float64(brightness)
	return colorRGBA{
		r: uint8(minInt(int(math.Round(float64(c.r)*scale)), 255)),
		g: uint8(minInt(int(math.Round(float64(c.g)*scale)), 255)),
		b: uint8(minInt(int(math.Round(float64(c.b)*scale)), 255)),
		a: c.a,
	}
}

func averageOpaqueColumnColor(img image.Image, x int) colorRGBA {
	bounds := img.Bounds()
	var red, green, blue, alphaTotal uint64
	var count uint64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		c := rgbaFromColor(img.At(x, y))
		if c.a == 0 {
			continue
		}
		red += uint64(c.r) * uint64(c.a)
		green += uint64(c.g) * uint64(c.a)
		blue += uint64(c.b) * uint64(c.a)
		alphaTotal += uint64(c.a)
		count++
	}
	if count == 0 || alphaTotal == 0 {
		return colorRGBA{}
	}
	return colorRGBA{
		r: uint8(red / alphaTotal),
		g: uint8(green / alphaTotal),
		b: uint8(blue / alphaTotal),
		a: 255,
	}
}

func maxLineWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	return maxWidth
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type colorRGBA struct {
	r uint8
	g uint8
	b uint8
	a uint8
}

func rgbaFromColor(c color.Color) colorRGBA {
	r, g, b, a := c.RGBA()
	return colorRGBA{
		r: uint8(r >> 8),
		g: uint8(g >> 8),
		b: uint8(b >> 8),
		a: uint8(a >> 8),
	}
}

func ansiForeground(c colorRGBA) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b)
}
