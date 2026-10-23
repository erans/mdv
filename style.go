package main

import (
	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func upp(u uint) *uint    { return &u }

// StyleConfig maps palette roles onto glamour's token names. This is the only
// place the two vocabularies meet; the rest of the viewer speaks palette roles.
func (p Palette) StyleConfig() ansi.StyleConfig {
	s := styles.DarkStyleConfig
	if p.Light {
		s = styles.LightStyleConfig
	}

	s.Document.Color = sp(p.Fg)
	s.Document.BlockPrefix = "" // no blank first line in the viewer
	s.Document.BlockSuffix = ""
	s.Document.Margin = upp(0)
	s.Text.Color = nil // inherit the document color
	s.Document.BackgroundColor = nil

	s.Heading.Color = sp(p.Accent)
	s.Heading.Bold = bp(true)
	// Headings are styled, not marked up: no `#` prefixes, no background bands.
	s.H1.BackgroundColor = nil
	s.H1.Prefix = ""
	s.H1.Suffix = ""
	s.H1.Color = sp(p.Accent)
	s.H1.Bold = bp(true)
	s.H1.Underline = bp(true)
	s.H2.Color = sp(p.Accent)
	s.H2.Prefix = ""
	for _, h := range []*ansi.StyleBlock{&s.H3, &s.H4, &s.H5, &s.H6} {
		h.Color = sp(blend(p.Accent, p.Fg, 0.35))
		h.Prefix = ""
	}

	s.Link.Color = sp(p.Blue)
	s.Link.Underline = bp(true)
	s.LinkText.Color = sp(p.Blue)
	s.LinkText.Underline = bp(true)

	s.Code.Color = sp(p.Orange)
	s.Code.BackgroundColor = sp(p.Selection)
	s.Code.Prefix = ""
	s.Code.Suffix = ""

	s.CodeBlock.Color = sp(p.Fg)
	s.CodeBlock.Margin = upp(0)
	s.CodeBlock.Chroma = &ansi.Chroma{
		Text:                ansi.StylePrimitive{Color: sp(p.Fg)},
		Comment:             ansi.StylePrimitive{Color: sp(p.Muted)},
		CommentPreproc:      ansi.StylePrimitive{Color: sp(p.Orange)},
		Keyword:             ansi.StylePrimitive{Color: sp(p.Blue)},
		KeywordReserved:     ansi.StylePrimitive{Color: sp(p.Magenta)},
		KeywordNamespace:    ansi.StylePrimitive{Color: sp(p.Magenta)},
		KeywordType:         ansi.StylePrimitive{Color: sp(p.Cyan)},
		Operator:            ansi.StylePrimitive{Color: sp(p.Cyan)},
		Punctuation:         ansi.StylePrimitive{Color: sp(p.Muted)},
		Name:                ansi.StylePrimitive{Color: sp(p.Fg)},
		NameBuiltin:         ansi.StylePrimitive{Color: sp(p.Cyan)},
		NameTag:             ansi.StylePrimitive{Color: sp(p.Blue)},
		NameAttribute:       ansi.StylePrimitive{Color: sp(p.Yellow)},
		NameClass:           ansi.StylePrimitive{Color: sp(p.Orange), Bold: bp(true)},
		NameConstant:        ansi.StylePrimitive{Color: sp(p.Yellow)},
		NameDecorator:       ansi.StylePrimitive{Color: sp(p.Orange)},
		NameException:       ansi.StylePrimitive{Color: sp(p.Red)},
		NameFunction:        ansi.StylePrimitive{Color: sp(p.Green)},
		LiteralNumber:       ansi.StylePrimitive{Color: sp(p.Magenta)},
		LiteralString:       ansi.StylePrimitive{Color: sp(p.Yellow)},
		LiteralStringEscape: ansi.StylePrimitive{Color: sp(p.Cyan)},
		GenericDeleted:      ansi.StylePrimitive{Color: sp(p.Red)},
		GenericEmph:         ansi.StylePrimitive{Italic: bp(true)},
		GenericInserted:     ansi.StylePrimitive{Color: sp(p.Green)},
		GenericStrong:       ansi.StylePrimitive{Bold: bp(true)},
		GenericSubheading:   ansi.StylePrimitive{Color: sp(p.Muted)},
		Error:               ansi.StylePrimitive{Color: sp(p.Red)},
	}

	s.Table.Color = sp(p.Fg)
	s.Table.Bold = nil // header weight comes from glamour's own table defaults
	s.Table.CenterSeparator = sp("┼")
	s.Table.ColumnSeparator = sp("│")
	s.Table.RowSeparator = sp("─")

	s.BlockQuote.Color = sp(p.Muted)
	s.BlockQuote.IndentToken = sp("│ ")
	s.HorizontalRule.Color = sp(p.Muted)
	s.Item.Color = sp(p.Accent)
	s.Enumeration.Color = sp(p.Accent)
	s.Image.Color = sp(p.Muted)
	s.ImageText.Color = sp(p.Muted)
	return s
}
