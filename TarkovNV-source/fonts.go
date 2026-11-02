package main

import _ "embed"

//go:embed assets/fonts/BarlowCondensed-Medium.ttf
var fontBarlowMedium []byte

//go:embed assets/fonts/BarlowCondensed-SemiBold.ttf
var fontBarlowSemi []byte

//go:embed assets/fonts/BarlowCondensed-Bold.ttf
var fontBarlowBold []byte

//go:embed assets/fonts/SairaStencilOne-Regular.ttf
var fontStencil []byte

//go:embed assets/fonts/ShareTechMono-Regular.ttf
var fontMono []byte

type embFont struct {
	face   string
	weight int
	data   []byte
}

func embeddedFonts() []embFont {
	return []embFont{
		{"Barlow Condensed", 500, fontBarlowMedium},
		{"Barlow Condensed", 600, fontBarlowSemi},
		{"Barlow Condensed", 700, fontBarlowBold},
		{"Saira Stencil One", 400, fontStencil},
		{"Share Tech Mono", 400, fontMono},
	}
}
