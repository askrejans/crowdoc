package fonts

import "strings"

// typstFamily derives the family name Typst 0.15 assigns to a font: a
// PostScript-name exception if one exists, otherwise name ID 1 with trailing
// style words trimmed. Typst deliberately ignores name ID 16 because it
// groups optical-size and width variants that must stay addressable.
func typstFamily(postscript, family string) string {
	if f, ok := typstExceptions[postscript]; ok {
		return f
	}
	return typographicFamily(family)
}

var (
	styleSeparators = " -_"
	styleModifiers  = []string{"extra", "ext", "ex", "x", "semi", "sem", "sm", "demi", "dem", "ultra"}
	styleSuffixes   = []string{
		"normal", "italic", "oblique", "slanted",
		"thin", "th", "hairline", "light", "lt", "regular", "medium", "med",
		"md", "bold", "bd", "demi", "extb", "black", "blk", "bk", "heavy",
		"narrow", "condensed", "cond", "cn", "cd", "compressed", "expanded", "exp",
		"vf", "var", "variable",
	}
)

// typographicFamily is a port of Typst's suffix trimming; it must stay
// byte-for-byte compatible so Scan and Typst agree on names.
func typographicFamily(family string) string {
	family = strings.TrimLeft(strings.TrimSpace(family), ".")
	lower := asciiLower(family)
	n := -1
	trimmed := lower
	for n < 0 || len(trimmed) < n {
		n = len(trimmed)
		t := trimmed
		shortened := false
		for {
			s, ok := cutAnySuffix(t, styleSuffixes)
			if !ok {
				break
			}
			t, shortened = s, true
		}
		if !shortened {
			break
		}
		if s, ok := cutSeparator(t); ok {
			trimmed, t = s, s
		}
		if s, ok := cutAnySuffix(t, styleModifiers); ok {
			if s, ok := cutSeparator(s); ok {
				trimmed = s
			}
		}
	}
	return family[:n]
}

// cutAnySuffix strips the first suffix in list order that matches, like
// Rust's iter().find_map(strip_suffix).
func cutAnySuffix(s string, suffixes []string) (string, bool) {
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			return s[:len(s)-len(suf)], true
		}
	}
	return s, false
}

func cutSeparator(s string) (string, bool) {
	if s != "" && strings.IndexByte(styleSeparators, s[len(s)-1]) >= 0 {
		return s[:len(s)-1], true
	}
	return s, false
}

// asciiLower lowercases ASCII only so byte offsets stay valid for slicing
// the original string.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// typstExceptions mirrors the family overrides in Typst 0.15's font
// exception table (keyed by PostScript name). Most matter for the TeX Live
// Computer Modern and Noto Display fonts.
var typstExceptions = map[string]string{
	"ArchivoNarrow-Regular":                          "Archivo Narrow",
	"ArchivoNarrow-Italic":                           "Archivo Narrow",
	"ArchivoNarrow-Bold":                             "Archivo Narrow",
	"ArchivoNarrow-BoldItalic":                       "Archivo Narrow",
	"NotoNaskhArabicUISemi-Bold":                     "Noto Naskh Arabic UI",
	"NotoSansSoraSompengSemi-Bold":                   "Noto Sans Sora Sompeng",
	"NotoSans-DisplayBlackItalic":                    "Noto Sans Display",
	"NotoSans-DisplayCondensedBlackItalic":           "Noto Sans Display",
	"NotoSans-DisplayCondensedBold":                  "Noto Sans Display",
	"NotoSans-DisplayCondensedBoldItalic":            "Noto Sans Display",
	"NotoSans-DisplayCondensedExtraBoldItalic":       "Noto Sans Display",
	"NotoSans-DisplayCondensedExtraLightItalic":      "Noto Sans Display",
	"NotoSans-DisplayCondensedItalic":                "Noto Sans Display",
	"NotoSans-DisplayCondensedLightItalic":           "Noto Sans Display",
	"NotoSans-DisplayCondensedMediumItalic":          "Noto Sans Display",
	"NotoSans-DisplayCondensedSemiBoldItalic":        "Noto Sans Display",
	"NotoSans-DisplayCondensedThinItalic":            "Noto Sans Display",
	"NotoSans-DisplayExtraBoldItalic":                "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedBlackItalic":      "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedBold":             "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedBoldItalic":       "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedExtraBoldItalic":  "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedExtraLightItalic": "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedItalic":           "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedLightItalic":      "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedMediumItalic":     "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedSemiBoldItalic":   "Noto Sans Display",
	"NotoSans-DisplayExtraCondensedThinItalic":       "Noto Sans Display",
	"NotoSans-DisplayExtraLightItalic":               "Noto Sans Display",
	"NotoSans-DisplayLightItalic":                    "Noto Sans Display",
	"NotoSans-DisplayMediumItalic":                   "Noto Sans Display",
	"NotoSans-DisplaySemiBoldItalic":                 "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedBlackItalic":       "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedBold":              "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedBoldItalic":        "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedExtraBoldItalic":   "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedExtraLightItalic":  "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedItalic":            "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedLightItalic":       "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedMediumItalic":      "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedSemiBoldItalic":    "Noto Sans Display",
	"NotoSans-DisplaySemiCondensedThinItalic":        "Noto Sans Display",
	"NotoSans-DisplayThinItalic":                     "Noto Sans Display",
	"NotoSerif-DisplayCondensedBold":                 "Noto Serif Display",
	"NotoSerif-DisplayExtraCondensedBold":            "Noto Serif Display",
	"NotoSerif-DisplaySemiCondensedBold":             "Noto Serif Display",
	"NewCM08-Book":                                   "New Computer Modern 08",
	"NewCM08-BookItalic":                             "New Computer Modern 08",
	"NewCM08-Italic":                                 "New Computer Modern 08",
	"NewCM08-Regular":                                "New Computer Modern 08",
	"NewCM10-Bold":                                   "New Computer Modern",
	"NewCM10-BoldItalic":                             "New Computer Modern",
	"NewCM10-Book":                                   "New Computer Modern",
	"NewCM10-BookItalic":                             "New Computer Modern",
	"NewCM10-Italic":                                 "New Computer Modern",
	"NewCM10-Regular":                                "New Computer Modern",
	"NewCMMath-Bold":                                 "New Computer Modern Math",
	"NewCMMath-Book":                                 "New Computer Modern Math",
	"NewCMMath-Regular":                              "New Computer Modern Math",
	"NewCMMono10-Bold":                               "New Computer Modern Mono",
	"NewCMMono10-BoldOblique":                        "New Computer Modern Mono",
	"NewCMMono10-Book":                               "New Computer Modern Mono",
	"NewCMMono10-BookItalic":                         "New Computer Modern Mono",
	"NewCMMono10-Italic":                             "New Computer Modern Mono",
	"NewCMMono10-Regular":                            "New Computer Modern Mono",
	"NewCMSans08-Book":                               "New Computer Modern Sans 08",
	"NewCMSans08-BookOblique":                        "New Computer Modern Sans 08",
	"NewCMSans08-Oblique":                            "New Computer Modern Sans 08",
	"NewCMSans08-Regular":                            "New Computer Modern Sans 08",
	"NewCMSans10-Bold":                               "New Computer Modern Sans",
	"NewCMSans10-BoldOblique":                        "New Computer Modern Sans",
	"NewCMSans10-Book":                               "New Computer Modern Sans",
	"NewCMSans10-BookOblique":                        "New Computer Modern Sans",
	"NewCMSans10-Oblique":                            "New Computer Modern Sans",
	"NewCMSans10-Regular":                            "New Computer Modern Sans",
	"NewCMSansMath-Regular":                          "New Computer Modern Sans Math",
	"NewCMUncial08-Bold":                             "New Computer Modern Uncial 08",
	"NewCMUncial08-Book":                             "New Computer Modern Uncial 08",
	"NewCMUncial08-Regular":                          "New Computer Modern Uncial 08",
	"NewCMUncial10-Bold":                             "New Computer Modern Uncial",
	"NewCMUncial10-Book":                             "New Computer Modern Uncial",
	"NewCMUncial10-Regular":                          "New Computer Modern Uncial",
	"LMMono8-Regular":                                "Latin Modern Mono 8",
	"LMMono9-Regular":                                "Latin Modern Mono 9",
	"LMMono12-Regular":                               "Latin Modern Mono 12",
	"LMRoman5-Regular":                               "Latin Modern Roman 5",
	"LMRoman6-Regular":                               "Latin Modern Roman 6",
	"LMRoman7-Regular":                               "Latin Modern Roman 7",
	"LMRoman8-Regular":                               "Latin Modern Roman 8",
	"LMRoman9-Regular":                               "Latin Modern Roman 9",
	"LMRoman12-Regular":                              "Latin Modern Roman 12",
	"LMRoman17-Regular":                              "Latin Modern Roman 17",
	"LMRoman7-Italic":                                "Latin Modern Roman 7",
	"LMRoman8-Italic":                                "Latin Modern Roman 8",
	"LMRoman9-Italic":                                "Latin Modern Roman 9",
	"LMRoman12-Italic":                               "Latin Modern Roman 12",
	"LMRoman5-Bold":                                  "Latin Modern Roman 5",
	"LMRoman6-Bold":                                  "Latin Modern Roman 6",
	"LMRoman7-Bold":                                  "Latin Modern Roman 7",
	"LMRoman8-Bold":                                  "Latin Modern Roman 8",
	"LMRoman9-Bold":                                  "Latin Modern Roman 9",
	"LMRoman12-Bold":                                 "Latin Modern Roman 12",
	"LMRomanSlant8-Regular":                          "Latin Modern Roman 8",
	"LMRomanSlant9-Regular":                          "Latin Modern Roman 9",
	"LMRomanSlant12-Regular":                         "Latin Modern Roman 12",
	"LMRomanSlant17-Regular":                         "Latin Modern Roman 17",
	"LMSans8-Regular":                                "Latin Modern Sans 8",
	"LMSans9-Regular":                                "Latin Modern Sans 9",
	"LMSans12-Regular":                               "Latin Modern Sans 12",
	"LMSans17-Regular":                               "Latin Modern Sans 17",
	"LMSans8-Oblique":                                "Latin Modern Sans 8",
	"LMSans9-Oblique":                                "Latin Modern Sans 9",
	"LMSans12-Oblique":                               "Latin Modern Sans 12",
	"LMSans17-Oblique":                               "Latin Modern Sans 17",
	"SimSun-ExtB":                                    "SimSun-ExtB",
}
