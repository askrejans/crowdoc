package locale

import (
	"strings"
	"unicode"
)

// stopwords are frequent function words used to tell Latin-script
// languages apart. Words shared by several languages still help: the
// highest total wins.
var stopwords = map[string][]string{
	"en": {"the", "and", "of", "to", "in", "is", "that", "for", "with", "as", "this", "are", "by", "be", "on", "it", "or", "which", "from"},
	"lv": {"un", "ir", "ar", "no", "uz", "par", "kas", "lai", "tas", "bet", "vai", "arī", "kā", "kur", "pēc", "tiek", "šis", "līdz", "jo", "nav"},
	"lt": {"ir", "kad", "su", "iš", "į", "tai", "kaip", "bet", "yra", "jo", "dėl", "nuo", "buvo", "arba", "apie", "kuris", "šis", "tik", "jau"},
	"et": {"ja", "on", "et", "ei", "see", "ka", "kui", "oli", "mis", "aga", "või", "ning", "selle", "nii", "seda", "kes", "veel", "pole"},
	"fi": {"ja", "on", "ei", "että", "se", "oli", "mutta", "kun", "ovat", "joka", "tai", "myös", "sen", "kuin", "jos", "tämä", "niin", "hän"},
	"sv": {"och", "att", "det", "som", "en", "är", "av", "för", "med", "till", "den", "på", "inte", "har", "om", "ett", "var", "kan"},
	"nb": {"og", "i", "det", "som", "en", "er", "av", "for", "med", "til", "den", "på", "ikke", "har", "om", "et", "var", "kan", "jeg"},
	"da": {"og", "i", "det", "som", "en", "er", "af", "for", "med", "til", "den", "på", "ikke", "har", "om", "et", "var", "kan", "jeg"},
	"is": {"og", "að", "er", "í", "á", "sem", "um", "við", "með", "ekki", "það", "til", "var", "fyrir", "eru", "hann"},
	"de": {"der", "die", "und", "in", "den", "von", "zu", "das", "mit", "sich", "des", "auf", "für", "ist", "im", "dem", "nicht", "ein", "eine"},
	"nl": {"de", "en", "van", "het", "een", "in", "is", "dat", "op", "te", "zijn", "voor", "met", "die", "niet", "aan", "er", "ook"},
	"lb": {"an", "de", "den", "dat", "ass", "vun", "mat", "op", "fir", "net", "och", "et", "ech", "mir", "sinn", "huet"},
	"fr": {"le", "la", "les", "de", "des", "et", "en", "un", "une", "du", "est", "que", "pour", "dans", "qui", "par", "sur", "pas", "au"},
	"it": {"il", "di", "che", "e", "la", "per", "un", "in", "del", "della", "non", "sono", "le", "con", "una", "si", "da", "gli"},
	"es": {"el", "la", "de", "que", "y", "en", "los", "las", "del", "se", "por", "un", "para", "con", "una", "es", "al", "como"},
	"pt": {"de", "que", "e", "o", "a", "do", "da", "em", "um", "para", "com", "não", "uma", "os", "no", "se", "na", "por", "mais"},
	"ca": {"el", "la", "de", "i", "que", "en", "els", "les", "del", "per", "amb", "una", "és", "als", "no", "més", "dels"},
	"pl": {"i", "w", "się", "nie", "na", "z", "do", "to", "że", "jest", "o", "jak", "ale", "po", "co", "tak", "za", "od"},
	"cs": {"a", "se", "na", "je", "že", "v", "to", "do", "ve", "s", "jako", "pro", "jsou", "z", "ale", "by", "jeho", "který"},
	"sk": {"a", "sa", "na", "je", "že", "v", "to", "do", "vo", "s", "ako", "pre", "sú", "z", "ale", "by", "jeho", "ktorý"},
	"sl": {"in", "je", "da", "na", "se", "za", "v", "so", "z", "ki", "pa", "ne", "tudi", "bi", "kot", "ali", "po", "ta"},
	"hr": {"i", "je", "u", "da", "se", "na", "za", "su", "od", "s", "koji", "ne", "kao", "ili", "što", "iz", "bi", "ali"},
	"sq": {"e", "të", "në", "dhe", "i", "me", "për", "që", "është", "nga", "një", "së", "si", "nuk", "ka", "u"},
	"ro": {"și", "de", "în", "la", "a", "cu", "o", "pe", "că", "nu", "este", "din", "pentru", "sunt", "mai", "care", "un", "se"},
	"hu": {"a", "az", "és", "hogy", "nem", "is", "egy", "van", "meg", "de", "ez", "csak", "már", "mint", "volt", "kell", "vagy"},
	"tr": {"ve", "bir", "bu", "da", "de", "için", "ile", "olarak", "çok", "daha", "gibi", "olan", "ne", "ama", "en", "kadar", "değil"},
	"eu": {"eta", "da", "ez", "ere", "bat", "du", "zen", "dira", "baina", "hau", "izan", "edo", "dute", "bere", "ditu"},
	"ga": {"agus", "an", "na", "ar", "is", "go", "le", "sa", "a", "ag", "ní", "atá", "bhí", "don", "seo", "sé"},
}

// letterHints are characters that point strongly at one language.
var letterHints = map[rune]string{
	'ā': "lv", 'ē': "lv", 'ī': "lv", 'ū': "lv", 'ģ': "lv", 'ķ': "lv", 'ļ': "lv", 'ņ': "lv",
	'ė': "lt", 'ų': "lt", 'į': "lt",
	'õ': "et",
	'ß': "de",
	'ł': "pl", 'ś': "pl", 'ź': "pl", 'ń': "pl", 'ę': "pl", 'ą': "pl",
	'ř': "cs", 'ů': "cs",
	'ľ': "sk", 'ĺ': "sk", 'ŕ': "sk", 'ô': "sk",
	'ő': "hu", 'ű': "hu",
	'ș': "ro", 'ț': "ro", 'ă': "ro",
	'ğ': "tr", 'ı': "tr", 'ş': "tr",
	'ð': "is", 'þ': "is",
	'ø': "nb", 'æ': "da", 'å': "sv",
	'ñ': "es", 'ã': "pt",
	'ë': "sq", 'ç': "fr",
	'đ': "hr",
}

// Detect guesses the BCP 47 language of text (one of the supported
// languages). It returns "en" when unsure.
func Detect(text string) string {
	if text == "" {
		return "en"
	}
	if len(text) > 20000 {
		text = text[:20000]
	}
	var latin, cyr, greek, arabic, hebrew, hangul, kana, han int
	special := map[rune]int{}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Hangul, r):
			hangul++
		case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			kana++
		case unicode.Is(unicode.Han, r):
			han++
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
			special[unicode.ToLower(r)]++
		case unicode.Is(unicode.Greek, r):
			greek++
		case unicode.Is(unicode.Arabic, r):
			arabic++
		case unicode.Is(unicode.Hebrew, r):
			hebrew++
		case unicode.IsLetter(r):
			latin++
		}
	}
	total := latin + cyr + greek + arabic + hebrew + hangul + kana + han
	if total == 0 {
		return "en"
	}
	switch max := maxOf(latin, cyr, greek, arabic, hebrew, hangul, kana+han); {
	case max == hangul:
		return "ko"
	case max == kana+han && kana > 0:
		return "ja"
	case max == kana+han:
		return "zh"
	case max == arabic:
		return "ar"
	case max == hebrew:
		return "he"
	case max == greek:
		return "el"
	case max == cyr:
		return cyrillic(special)
	}
	return latinLanguage(text)
}

func maxOf(v ...int) int {
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func cyrillic(c map[rune]int) string {
	switch {
	case c['ї']+c['є']+c['ґ']+c['і'] > 0:
		return "uk"
	case c['ѓ']+c['ќ']+c['ѕ'] > 0:
		return "mk"
	case c['ђ']+c['ћ'] > 0 || (c['џ']+c['љ']+c['њ'] > 0 && c['ы'] == 0):
		return "sr"
	case c['ъ'] > 2 && c['ы']+c['э'] == 0:
		return "bg"
	}
	return "ru"
}

func latinLanguage(text string) string {
	best, _, _ := latinScores(text)
	return best
}

// Verify checks a language declared by a document's metadata against its
// text. Office files often carry the author's interface language rather
// than the text's. It returns the declared tag unless the text clearly
// reads as another language.
func Verify(declared, text string) (string, bool) {
	base := strings.ToLower(strings.SplitN(strings.ReplaceAll(declared, "_", "-"), "-", 2)[0])
	if base == "" {
		return Detect(text), true
	}
	if len(strings.Fields(text)) < 120 {
		return declared, false
	}
	detected := Detect(text)
	if detected == base || (base == "nn" || base == "no") && detected == "nb" {
		return declared, false
	}
	declScript := Get(declared).Script
	if declScript != Get(detected).Script {
		return detected, true
	}
	_, top, scores := latinScores(text)
	if top >= 8 && top >= 3*scores[base] {
		return detected, true
	}
	return declared, false
}

func latinScores(text string) (string, float64, map[string]float64) {
	score := map[string]float64{}
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\''
	})
	if len(words) > 3000 {
		words = words[:3000]
	}
	index := map[string][]string{}
	for lang, ws := range stopwords {
		for _, w := range ws {
			index[w] = append(index[w], lang)
		}
	}
	for _, w := range words {
		for _, lang := range index[w] {
			score[lang] += 1 / float64(len(index[w]))
		}
		for _, r := range w {
			if lang := letterHints[r]; lang != "" {
				score[lang] += 0.5
			}
		}
	}
	best, bestScore := "en", 0.0
	for lang, s := range score {
		if s > bestScore || (s == bestScore && lang < best) {
			best, bestScore = lang, s
		}
	}
	if bestScore < 2 {
		return "en", bestScore, score
	}
	return best, bestScore, score
}
