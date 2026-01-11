// Package theme defines the dark theme colors for Spider UI.
package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Spider Dark Theme Colors
var (
	// Primary colors
	ColorBackground     = color.NRGBA{R: 18, G: 18, B: 18, A: 255}      // #121212 - Pure dark
	ColorSurface        = color.NRGBA{R: 30, G: 30, B: 30, A: 255}      // #1E1E1E - Cards/panels
	ColorSurfaceVariant = color.NRGBA{R: 45, G: 45, B: 45, A: 255}      // #2D2D2D - Elevated surfaces
	ColorBorder         = color.NRGBA{R: 60, G: 60, B: 60, A: 255}      // #3C3C3C - Borders

	// Accent colors (Green)
	ColorPrimary      = color.NRGBA{R: 0, G: 200, B: 83, A: 255}   // #00C853 - Primary green
	ColorPrimaryDark  = color.NRGBA{R: 0, G: 150, B: 60, A: 255}   // #00963C - Darker green
	ColorPrimaryLight = color.NRGBA{R: 105, G: 240, B: 174, A: 255} // #69F0AE - Light green

	// Text colors
	ColorTextPrimary   = color.NRGBA{R: 255, G: 255, B: 255, A: 255} // #FFFFFF - Primary text
	ColorTextSecondary = color.NRGBA{R: 179, G: 179, B: 179, A: 255} // #B3B3B3 - Secondary text
	ColorTextDisabled  = color.NRGBA{R: 100, G: 100, B: 100, A: 255} // #646464 - Disabled text
	ColorTextOnPrimary = color.NRGBA{R: 0, G: 0, B: 0, A: 255}       // #000000 - Text on green

	// Status colors
	ColorSuccess = color.NRGBA{R: 0, G: 200, B: 83, A: 255}    // #00C853 - Success/2xx
	ColorWarning = color.NRGBA{R: 255, G: 193, B: 7, A: 255}   // #FFC107 - Warning/3xx
	ColorError   = color.NRGBA{R: 244, G: 67, B: 54, A: 255}   // #F44336 - Error/4xx-5xx
	ColorInfo    = color.NRGBA{R: 33, G: 150, B: 243, A: 255}  // #2196F3 - Info

	// Table colors
	ColorTableHeader   = color.NRGBA{R: 38, G: 38, B: 38, A: 255}  // #262626
	ColorTableRowEven  = color.NRGBA{R: 25, G: 25, B: 25, A: 255}  // #191919
	ColorTableRowOdd   = color.NRGBA{R: 30, G: 30, B: 30, A: 255}  // #1E1E1E
	ColorTableRowHover = color.NRGBA{R: 45, G: 50, B: 45, A: 255}  // Slight green tint
	ColorTableSelected = color.NRGBA{R: 0, G: 60, B: 30, A: 255}   // Dark green selection

	// Sidebar/Navigation
	ColorSidebar       = color.NRGBA{R: 22, G: 22, B: 22, A: 255}  // #161616
	ColorSidebarActive = color.NRGBA{R: 0, G: 80, B: 40, A: 255}   // Dark green active

	// Input fields
	ColorInputBg      = color.NRGBA{R: 35, G: 35, B: 35, A: 255}   // #232323
	ColorInputBorder  = color.NRGBA{R: 70, G: 70, B: 70, A: 255}   // #464646
	ColorInputFocused = color.NRGBA{R: 0, G: 200, B: 83, A: 255}   // Green focus
)

// SpiderTheme implements fyne.Theme for Spider dark theme.
type SpiderTheme struct{}

var _ fyne.Theme = (*SpiderTheme)(nil)

// Color returns the color for the specified theme color name.
func (t *SpiderTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return ColorBackground
	case theme.ColorNameButton:
		return ColorSurface
	case theme.ColorNameDisabledButton:
		return ColorSurfaceVariant
	case theme.ColorNameDisabled:
		return ColorTextDisabled
	case theme.ColorNameError:
		return ColorError
	case theme.ColorNameFocus:
		return ColorPrimary
	case theme.ColorNameForeground:
		return ColorTextPrimary
	case theme.ColorNameHover:
		return ColorSurfaceVariant
	case theme.ColorNameInputBackground:
		return ColorInputBg
	case theme.ColorNameInputBorder:
		return ColorInputBorder
	case theme.ColorNameMenuBackground:
		return ColorSurface
	case theme.ColorNameOverlayBackground:
		return ColorSurface
	case theme.ColorNamePlaceHolder:
		return ColorTextSecondary
	case theme.ColorNamePressed:
		return ColorPrimaryDark
	case theme.ColorNamePrimary:
		return ColorPrimary
	case theme.ColorNameScrollBar:
		return ColorBorder
	case theme.ColorNameSelection:
		return ColorTableSelected
	case theme.ColorNameSeparator:
		return ColorBorder
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 100}
	case theme.ColorNameSuccess:
		return ColorSuccess
	case theme.ColorNameWarning:
		return ColorWarning
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

// Font returns the font for the specified text style.
func (t *SpiderTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Icon returns the icon for the specified icon name.
func (t *SpiderTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Size returns the size for the specified size name.
func (t *SpiderTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameScrollBar:
		return 12
	case theme.SizeNameScrollBarSmall:
		return 4
	case theme.SizeNameSeparatorThickness:
		return 1
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameInputBorder:
		return 1
	default:
		return theme.DefaultTheme().Size(name)
	}
}
