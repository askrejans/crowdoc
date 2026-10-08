package cite

import (
	"strconv"
	"strings"
	"unicode"
)

// locale holds the language-dependent vocabulary and typography of
// reference formatting. Missing terms fall back to English.
type locale struct {
	lang        string
	terms       map[string]string
	months      [12]string
	monthsShort [12]string
	// monthsDate holds month forms required after a day number (Polish and
	// Lithuanian genitive, Finnish partitive). Nil means months.
	monthsDate *[12]string
	// monthsLoc is the Latvian locative used in "viewed on" phrases.
	monthsLoc     *[12]string
	open, close   string // primary quotation marks
	open2, close2 string // secondary marks (nested quotes, British single quotes)
	// postfix lists terms written after their number ("33. lpp.").
	postfix map[string]bool
}

var enTerms = map[string]string{
	"and": "and", "et-al": "et al.", "no-date": "n.d.", "circa": "ca.", "in": "In",
	"editor": "ed.", "editors": "eds.", "edited-by": "edited by",
	"translator": "trans.", "translators": "trans.", "translated-by": "translated by",
	"edition": "ed.",
	"page":    "p.", "pages": "pp.", "volume": "vol.", "volumes": "vols.", "issue": "no.",
	"chapter": "chap.", "section": "sec.", "figure": "fig.", "table": "tab.",
	"paragraph": "para.", "line": "l.", "lines": "ll.", "note": "n.", "column": "col.",
	"part": "pt.", "verse": "v.", "book": "bk.",
	"retrieved": "Retrieved", "from": "from", "accessed": "Accessed", "available-at": "Available at",
	"available-from": "Available from:", "online": "[Online]", "internet": "Internet", "cited": "cited",
	"thesis-phd": "Doctoral dissertation", "thesis-master": "Master's thesis",
	"thesis-bachelor": "Bachelor's thesis", "thesis": "Thesis",
	"report": "Report", "number": "No.", "report-no": "Report No.", "patent": "Patent",
	"standard": "Standard", "version": "Version", "preprint": "Preprint", "dataset": "Data set",
	"software": "Computer software", "manuscript": "Unpublished manuscript",
	"presentation": "Paper presentation", "presented-at": "Paper presented at",
	"unpublished": "unpublished", "in-press": "in press",
}

var enMonths = [12]string{"January", "February", "March", "April", "May", "June", "July",
	"August", "September", "October", "November", "December"}

var locales = map[string]*locale{
	"en": {
		lang: "en", terms: enTerms, months: enMonths,
		monthsShort: [12]string{"Jan.", "Feb.", "Mar.", "Apr.", "May", "June", "July", "Aug.", "Sept.", "Oct.", "Nov.", "Dec."},
		open:        "“", close: "”", open2: "‘", close2: "’",
	},
	"lv": {
		lang: "lv",
		terms: map[string]string{
			"and": "un", "et-al": "u. c.", "no-date": "b. g.", "circa": "ap.", "in": "No:",
			"editor": "red.", "editors": "red.", "edited-by": "red.",
			"translator": "tulk.", "translators": "tulk.", "translated-by": "tulk.",
			"edition": "izd.",
			"page":    "lpp.", "pages": "lpp.", "volume": "sēj.", "volumes": "sēj.", "issue": "nr.",
			"chapter": "nod.", "section": "sad.", "figure": "att.", "table": "tab.",
			"paragraph": "rindk.", "line": "r.", "lines": "r.", "note": "piez.", "column": "sl.",
			"part": "d.", "verse": "p.", "book": "grām.",
			"retrieved": "Skatīts", "from": "no", "accessed": "Skatīts", "available-at": "Pieejams",
			"available-from": "Pieejams:", "online": "[Tiešsaiste]", "internet": "Tiešsaiste", "cited": "skatīts",
			"thesis-phd": "Promocijas darbs", "thesis-master": "Maģistra darbs",
			"thesis-bachelor": "Bakalaura darbs", "thesis": "Noslēguma darbs",
			"report": "Ziņojums", "number": "Nr.", "report-no": "Ziņojums Nr.", "patent": "Patents",
			"standard": "Standarts", "version": "Versija", "preprint": "Priekšdruka", "dataset": "Datu kopa",
			"software": "Datorprogramma", "manuscript": "Nepublicēts manuskripts",
			"presentation": "Referāts", "presented-at": "Referāts konferencē",
			"unpublished": "nepublicēts", "in-press": "presē",
		},
		months: [12]string{"janvāris", "februāris", "marts", "aprīlis", "maijs", "jūnijs", "jūlijs",
			"augusts", "septembris", "oktobris", "novembris", "decembris"},
		monthsShort: [12]string{"janv.", "febr.", "marts", "apr.", "maijs", "jūn.", "jūl.", "aug.",
			"sept.", "okt.", "nov.", "dec."},
		monthsLoc: &[12]string{"janvārī", "februārī", "martā", "aprīlī", "maijā", "jūnijā", "jūlijā",
			"augustā", "septembrī", "oktobrī", "novembrī", "decembrī"},
		open: "„", close: "“", open2: "‚", close2: "‘",
		postfix: map[string]bool{"page": true, "pages": true, "volume": true, "volumes": true,
			"chapter": true, "section": true, "part": true, "edition": true},
	},
	"de": {
		lang: "de",
		terms: map[string]string{
			"and": "und", "et-al": "et al.", "no-date": "o. J.", "circa": "ca.", "in": "In:",
			"editor": "Hrsg.", "editors": "Hrsg.", "edited-by": "hrsg. von",
			"translator": "Übers.", "translators": "Übers.", "translated-by": "übers. von",
			"edition": "Aufl.",
			"page":    "S.", "pages": "S.", "volume": "Bd.", "volumes": "Bde.", "issue": "Nr.",
			"chapter": "Kap.", "section": "Abschn.", "figure": "Abb.", "table": "Tab.",
			"paragraph": "Abs.", "line": "Z.", "lines": "Z.", "note": "Anm.", "column": "Sp.",
			"part": "T.", "verse": "V.", "book": "Buch",
			"retrieved": "Abgerufen am", "from": "von", "accessed": "Zugriff am", "available-at": "Verfügbar unter",
			"available-from": "Verfügbar unter:", "online": "[Online]", "internet": "Internet", "cited": "zitiert am",
			"thesis-phd": "Dissertation", "thesis-master": "Masterarbeit",
			"thesis-bachelor": "Bachelorarbeit", "thesis": "Abschlussarbeit",
			"report": "Bericht", "number": "Nr.", "report-no": "Bericht Nr.", "patent": "Patent",
			"standard": "Norm", "version": "Version", "preprint": "Preprint", "dataset": "Datensatz",
			"software": "Software", "manuscript": "Unveröffentlichtes Manuskript",
			"presentation": "Vortrag", "presented-at": "Vortrag auf",
			"unpublished": "unveröffentlicht", "in-press": "im Druck",
		},
		months: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August",
			"September", "Oktober", "November", "Dezember"},
		monthsShort: [12]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.",
			"Sept.", "Okt.", "Nov.", "Dez."},
		open: "„", close: "“", open2: "‚", close2: "‘",
	},
	"fr": {
		lang: "fr",
		terms: map[string]string{
			"and": "et", "et-al": "et al.", "no-date": "s. d.", "circa": "vers", "in": "In",
			"editor": "éd.", "editors": "éds.", "edited-by": "édité par",
			"translator": "trad.", "translators": "trad.", "translated-by": "traduit par",
			"edition": "éd.",
			"page":    "p.", "pages": "p.", "volume": "vol.", "volumes": "vol.", "issue": "no",
			"chapter": "chap.", "section": "sect.", "figure": "fig.", "table": "tabl.",
			"paragraph": "paragr.", "line": "l.", "lines": "l.", "note": "n.", "column": "col.",
			"part": "partie", "verse": "v.", "book": "livre",
			"retrieved": "Consulté le", "from": "à l'adresse", "accessed": "Consulté le",
			"available-at": "Disponible à l'adresse", "available-from": "Disponible sur :",
			"online": "[En ligne]", "internet": "En ligne", "cited": "cité le",
			"thesis-phd": "Thèse de doctorat", "thesis-master": "Mémoire de master",
			"thesis-bachelor": "Mémoire de licence", "thesis": "Mémoire",
			"report": "Rapport", "number": "no", "report-no": "Rapport no", "patent": "Brevet",
			"standard": "Norme", "version": "Version", "preprint": "Prépublication", "dataset": "Jeu de données",
			"software": "Logiciel", "manuscript": "Manuscrit non publié",
			"presentation": "Communication", "presented-at": "Communication présentée à",
			"unpublished": "non publié", "in-press": "sous presse",
		},
		months: [12]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août",
			"septembre", "octobre", "novembre", "décembre"},
		monthsShort: [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juill.", "août",
			"sept.", "oct.", "nov.", "déc."},
		open: "«\u00a0", close: "\u00a0»", open2: "“", close2: "”",
	},
	"es": {
		lang: "es",
		terms: map[string]string{
			"and": "y", "et-al": "et al.", "no-date": "s. f.", "circa": "ca.", "in": "En",
			"editor": "ed.", "editors": "eds.", "edited-by": "editado por",
			"translator": "trad.", "translators": "trads.", "translated-by": "traducido por",
			"edition": "ed.",
			"page":    "p.", "pages": "pp.", "volume": "vol.", "volumes": "vols.", "issue": "n.º",
			"chapter": "cap.", "section": "secc.", "figure": "fig.", "table": "tabla",
			"paragraph": "párr.", "line": "lín.", "lines": "líns.", "note": "n.", "column": "col.",
			"retrieved": "Recuperado el", "from": "de", "accessed": "Consultado el", "available-at": "Disponible en",
			"available-from": "Disponible en:", "online": "[En línea]", "internet": "Internet", "cited": "citado",
			"thesis-phd": "Tesis doctoral", "thesis-master": "Tesis de maestría",
			"thesis-bachelor": "Trabajo de grado", "thesis": "Tesis",
			"report": "Informe", "number": "n.º", "report-no": "Informe n.º", "patent": "Patente",
			"standard": "Norma", "version": "Versión", "preprint": "Preimpresión", "dataset": "Conjunto de datos",
			"software": "Software", "manuscript": "Manuscrito no publicado",
			"presentation": "Ponencia", "presented-at": "Ponencia presentada en",
			"unpublished": "inédito", "in-press": "en prensa",
		},
		months: [12]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto",
			"septiembre", "octubre", "noviembre", "diciembre"},
		monthsShort: [12]string{"ene.", "feb.", "mar.", "abr.", "may.", "jun.", "jul.", "ago.",
			"sept.", "oct.", "nov.", "dic."},
		open: "“", close: "”", open2: "‘", close2: "’",
	},
	"it": {
		lang: "it",
		terms: map[string]string{
			"and": "e", "et-al": "et al.", "no-date": "s.d.", "circa": "ca.", "in": "In",
			"editor": "a cura di", "editors": "a cura di", "edited-by": "a cura di",
			"translator": "trad.", "translators": "trad.", "translated-by": "tradotto da",
			"edition": "ed.",
			"page":    "p.", "pages": "pp.", "volume": "vol.", "volumes": "voll.", "issue": "n.",
			"chapter": "cap.", "section": "sez.", "figure": "fig.", "table": "tab.",
			"paragraph": "par.", "line": "r.", "lines": "rr.", "note": "n.", "column": "col.",
			"retrieved": "Consultato il", "from": "da", "accessed": "Consultato il", "available-at": "Disponibile su",
			"available-from": "Disponibile su:", "online": "[Online]", "internet": "Internet", "cited": "citato il",
			"thesis-phd": "Tesi di dottorato", "thesis-master": "Tesi di laurea magistrale",
			"thesis-bachelor": "Tesi di laurea", "thesis": "Tesi",
			"report": "Rapporto", "number": "n.", "report-no": "Rapporto n.", "patent": "Brevetto",
			"standard": "Norma", "version": "Versione", "preprint": "Preprint", "dataset": "Set di dati",
			"software": "Software", "manuscript": "Manoscritto inedito",
			"presentation": "Relazione", "presented-at": "Relazione presentata a",
			"unpublished": "inedito", "in-press": "in stampa",
		},
		months: [12]string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno", "luglio",
			"agosto", "settembre", "ottobre", "novembre", "dicembre"},
		monthsShort: [12]string{"gen.", "feb.", "mar.", "apr.", "mag.", "giu.", "lug.", "ago.",
			"set.", "ott.", "nov.", "dic."},
		open: "“", close: "”", open2: "‘", close2: "’",
	},
	"pt": {
		lang: "pt",
		terms: map[string]string{
			"and": "e", "et-al": "et al.", "no-date": "s.d.", "circa": "ca.", "in": "In:",
			"editor": "ed.", "editors": "eds.", "edited-by": "editado por",
			"translator": "trad.", "translators": "trad.", "translated-by": "traduzido por",
			"edition": "ed.",
			"page":    "p.", "pages": "p.", "volume": "v.", "volumes": "v.", "issue": "n.",
			"chapter": "cap.", "section": "seç.", "figure": "fig.", "table": "tab.",
			"paragraph": "par.", "line": "l.", "lines": "l.", "note": "n.", "column": "col.",
			"retrieved": "Recuperado em", "from": "de", "accessed": "Acesso em", "available-at": "Disponível em",
			"available-from": "Disponível em:", "online": "[Online]", "internet": "Internet", "cited": "citado",
			"thesis-phd": "Tese de doutorado", "thesis-master": "Dissertação de mestrado",
			"thesis-bachelor": "Trabalho de conclusão de curso", "thesis": "Tese",
			"report": "Relatório", "number": "n.", "report-no": "Relatório n.", "patent": "Patente",
			"standard": "Norma", "version": "Versão", "preprint": "Preprint", "dataset": "Conjunto de dados",
			"software": "Software", "manuscript": "Manuscrito não publicado",
			"presentation": "Apresentação", "presented-at": "Apresentado em",
			"unpublished": "não publicado", "in-press": "no prelo",
		},
		months: [12]string{"janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho",
			"agosto", "setembro", "outubro", "novembro", "dezembro"},
		monthsShort: [12]string{"jan.", "fev.", "mar.", "abr.", "maio", "jun.", "jul.", "ago.",
			"set.", "out.", "nov.", "dez."},
		open: "“", close: "”", open2: "‘", close2: "’",
	},
	"nl": {
		lang: "nl",
		terms: map[string]string{
			"and": "en", "et-al": "et al.", "no-date": "z.d.", "circa": "ca.", "in": "In",
			"editor": "red.", "editors": "red.", "edited-by": "onder redactie van",
			"translator": "vert.", "translators": "vert.", "translated-by": "vertaald door",
			"edition": "dr.",
			"page":    "p.", "pages": "pp.", "volume": "vol.", "volumes": "vols.", "issue": "nr.",
			"chapter": "hfst.", "section": "par.", "figure": "fig.", "table": "tab.",
			"paragraph": "alin.", "line": "r.", "lines": "rr.", "note": "noot", "column": "kol.",
			"retrieved": "Geraadpleegd op", "from": "van", "accessed": "Geraadpleegd op", "available-at": "Beschikbaar op",
			"available-from": "Beschikbaar op:", "online": "[Online]", "internet": "Internet", "cited": "geciteerd",
			"thesis-phd": "Proefschrift", "thesis-master": "Masterscriptie",
			"thesis-bachelor": "Bachelorscriptie", "thesis": "Scriptie",
			"report": "Rapport", "number": "nr.", "report-no": "Rapport nr.", "patent": "Octrooi",
			"standard": "Norm", "version": "Versie", "preprint": "Preprint", "dataset": "Dataset",
			"software": "Software", "manuscript": "Ongepubliceerd manuscript",
			"presentation": "Presentatie", "presented-at": "Gepresenteerd op",
			"unpublished": "ongepubliceerd", "in-press": "in druk",
		},
		months: [12]string{"januari", "februari", "maart", "april", "mei", "juni", "juli", "augustus",
			"september", "oktober", "november", "december"},
		monthsShort: [12]string{"jan.", "feb.", "mrt.", "apr.", "mei", "jun.", "jul.", "aug.",
			"sep.", "okt.", "nov.", "dec."},
		open: "“", close: "”", open2: "‘", close2: "’",
	},
	"pl": {
		lang: "pl",
		terms: map[string]string{
			"and": "i", "et-al": "i in.", "no-date": "b.d.", "circa": "ok.", "in": "W:",
			"editor": "red.", "editors": "red.", "edited-by": "pod red.",
			"translator": "tłum.", "translators": "tłum.", "translated-by": "tłum.",
			"edition": "wyd.",
			"page":    "s.", "pages": "s.", "volume": "t.", "volumes": "t.", "issue": "nr",
			"chapter": "rozdz.", "section": "par.", "figure": "rys.", "table": "tab.",
			"paragraph": "ust.", "line": "w.", "lines": "w.", "note": "przyp.", "column": "kol.",
			"retrieved": "Pobrano", "from": "z", "accessed": "Dostęp", "available-at": "Dostępne na",
			"available-from": "Dostępne na:", "online": "[Online]", "internet": "Internet", "cited": "cytowane",
			"thesis-phd": "Rozprawa doktorska", "thesis-master": "Praca magisterska",
			"thesis-bachelor": "Praca licencjacka", "thesis": "Praca dyplomowa",
			"report": "Raport", "number": "nr", "report-no": "Raport nr", "patent": "Patent",
			"standard": "Norma", "version": "Wersja", "preprint": "Preprint", "dataset": "Zbiór danych",
			"software": "Oprogramowanie", "manuscript": "Niepublikowany rękopis",
			"presentation": "Referat", "presented-at": "Referat wygłoszony na",
			"unpublished": "niepublikowane", "in-press": "w druku",
		},
		months: [12]string{"styczeń", "luty", "marzec", "kwiecień", "maj", "czerwiec", "lipiec",
			"sierpień", "wrzesień", "październik", "listopad", "grudzień"},
		monthsShort: [12]string{"sty.", "lut.", "mar.", "kwi.", "maj", "cze.", "lip.", "sie.",
			"wrz.", "paź.", "lis.", "gru."},
		monthsDate: &[12]string{"stycznia", "lutego", "marca", "kwietnia", "maja", "czerwca", "lipca",
			"sierpnia", "września", "października", "listopada", "grudnia"},
		open: "„", close: "”", open2: "‚", close2: "’",
	},
	"lt": {
		lang: "lt",
		terms: map[string]string{
			"and": "ir", "et-al": "ir kt.", "no-date": "b. d.", "circa": "apie", "in": "Iš",
			"editor": "sud.", "editors": "sud.", "edited-by": "sudarė",
			"translator": "vert.", "translators": "vert.", "translated-by": "vertė",
			"edition": "leid.",
			"page":    "p.", "pages": "p.", "volume": "t.", "volumes": "t.", "issue": "nr.",
			"chapter": "sk.", "section": "sk.", "figure": "pav.", "table": "lent.",
			"paragraph": "pastr.", "line": "eil.", "lines": "eil.", "note": "pastaba",
			"retrieved": "Žiūrėta", "from": "iš", "accessed": "Žiūrėta", "available-at": "Prieiga per internetą",
			"available-from": "Prieiga per internetą:", "online": "[Interaktyvus]", "internet": "Interaktyvus",
			"cited":      "žiūrėta",
			"thesis-phd": "Daktaro disertacija", "thesis-master": "Magistro darbas",
			"thesis-bachelor": "Bakalauro darbas", "thesis": "Baigiamasis darbas",
			"report": "Ataskaita", "number": "Nr.", "report-no": "Ataskaita Nr.", "patent": "Patentas",
			"standard": "Standartas", "version": "Versija", "dataset": "Duomenų rinkinys",
			"software": "Programinė įranga", "manuscript": "Nepublikuotas rankraštis",
			"presentation": "Pranešimas", "unpublished": "nepublikuota", "in-press": "spaudoje",
		},
		months: [12]string{"sausis", "vasaris", "kovas", "balandis", "gegužė", "birželis", "liepa",
			"rugpjūtis", "rugsėjis", "spalis", "lapkritis", "gruodis"},
		monthsShort: [12]string{"saus.", "vas.", "kov.", "bal.", "geg.", "birž.", "liep.", "rugpj.",
			"rugs.", "spal.", "lapkr.", "gruod."},
		monthsDate: &[12]string{"sausio", "vasario", "kovo", "balandžio", "gegužės", "birželio",
			"liepos", "rugpjūčio", "rugsėjo", "spalio", "lapkričio", "gruodžio"},
		open: "„", close: "“", open2: "‚", close2: "‘",
	},
	"et": {
		lang: "et",
		terms: map[string]string{
			"and": "ja", "et-al": "jt", "no-date": "s.a.", "circa": "u", "in": "Rmt:",
			"editor": "toim.", "editors": "toim.", "edited-by": "toim.",
			"translator": "tlk.", "translators": "tlk.", "translated-by": "tlk.",
			"edition": "tr.",
			"page":    "lk", "pages": "lk", "volume": "kd", "volumes": "kd", "issue": "nr",
			"chapter": "ptk", "section": "jaot.", "figure": "joon.", "table": "tabel",
			"paragraph": "lõik", "line": "rida", "lines": "read", "note": "märkus",
			"retrieved": "Vaadatud", "from": "", "accessed": "Vaadatud", "available-at": "Kättesaadav",
			"available-from": "Kättesaadav:", "online": "[Võrgus]", "internet": "Võrgus", "cited": "vaadatud",
			"thesis-phd": "Doktoritöö", "thesis-master": "Magistritöö",
			"thesis-bachelor": "Bakalaureusetöö", "thesis": "Lõputöö",
			"report": "Aruanne", "number": "nr", "report-no": "Aruanne nr", "patent": "Patent",
			"standard": "Standard", "version": "Versioon", "dataset": "Andmestik",
			"software": "Tarkvara", "manuscript": "Avaldamata käsikiri",
			"unpublished": "avaldamata", "in-press": "trükis",
		},
		months: [12]string{"jaanuar", "veebruar", "märts", "aprill", "mai", "juuni", "juuli", "august",
			"september", "oktoober", "november", "detsember"},
		monthsShort: [12]string{"jaan", "veebr", "märts", "apr", "mai", "juuni", "juuli", "aug",
			"sept", "okt", "nov", "dets"},
		open: "„", close: "“", open2: "‚", close2: "‘",
	},
	"sv": {
		lang: "sv",
		terms: map[string]string{
			"and": "och", "et-al": "et al.", "no-date": "u.å.", "circa": "ca", "in": "I",
			"editor": "red.", "editors": "red.", "edited-by": "redigerad av",
			"translator": "övers.", "translators": "övers.", "translated-by": "översatt av",
			"edition": "uppl.",
			"page":    "s.", "pages": "s.", "volume": "vol.", "volumes": "vol.", "issue": "nr",
			"chapter": "kap.", "section": "avsn.", "figure": "fig.", "table": "tab.",
			"paragraph": "st.", "line": "rad", "lines": "rader", "note": "not",
			"retrieved": "Hämtad", "from": "från", "accessed": "Hämtad", "available-at": "Tillgänglig på",
			"available-from": "Tillgänglig på:", "online": "[Online]", "internet": "Internet", "cited": "citerad",
			"thesis-phd": "Doktorsavhandling", "thesis-master": "Masteruppsats",
			"thesis-bachelor": "Kandidatuppsats", "thesis": "Uppsats",
			"report": "Rapport", "number": "nr", "report-no": "Rapport nr", "patent": "Patent",
			"standard": "Standard", "version": "Version", "dataset": "Datamängd",
			"software": "Programvara", "manuscript": "Opublicerat manuskript",
			"unpublished": "opublicerad", "in-press": "under tryckning",
		},
		months: [12]string{"januari", "februari", "mars", "april", "maj", "juni", "juli", "augusti",
			"september", "oktober", "november", "december"},
		monthsShort: [12]string{"jan.", "feb.", "mars", "apr.", "maj", "juni", "juli", "aug.",
			"sep.", "okt.", "nov.", "dec."},
		open: "”", close: "”", open2: "’", close2: "’",
	},
	"fi": {
		lang: "fi",
		terms: map[string]string{
			"and": "ja", "et-al": "ym.", "no-date": "s.a.", "circa": "n.", "in": "Teoksessa",
			"editor": "toim.", "editors": "toim.", "edited-by": "toim.",
			"translator": "suom.", "translators": "suom.", "translated-by": "suom.",
			"edition": "p.",
			"page":    "s.", "pages": "s.", "volume": "vol.", "volumes": "vol.", "issue": "nro",
			"chapter": "luku", "section": "osa", "figure": "kuva", "table": "taulukko",
			"paragraph": "kappale", "line": "rivi", "lines": "rivit", "note": "huom.",
			"retrieved": "Haettu", "from": "osoitteesta", "accessed": "Luettu", "available-at": "Saatavilla",
			"available-from": "Saatavilla:", "online": "[Verkossa]", "internet": "Verkossa", "cited": "luettu",
			"thesis-phd": "Väitöskirja", "thesis-master": "Pro gradu -tutkielma",
			"thesis-bachelor": "Kandidaatintutkielma", "thesis": "Opinnäytetyö",
			"report": "Raportti", "number": "nro", "report-no": "Raportti nro", "patent": "Patentti",
			"standard": "Standardi", "version": "Versio", "dataset": "Aineisto",
			"software": "Ohjelmisto", "manuscript": "Julkaisematon käsikirjoitus",
			"unpublished": "julkaisematon", "in-press": "painossa",
		},
		months: [12]string{"tammikuu", "helmikuu", "maaliskuu", "huhtikuu", "toukokuu", "kesäkuu",
			"heinäkuu", "elokuu", "syyskuu", "lokakuu", "marraskuu", "joulukuu"},
		monthsShort: [12]string{"tammik.", "helmik.", "maalisk.", "huhtik.", "toukok.", "kesäk.",
			"heinäk.", "elok.", "syysk.", "lokak.", "marrask.", "jouluk."},
		monthsDate: &[12]string{"tammikuuta", "helmikuuta", "maaliskuuta", "huhtikuuta", "toukokuuta",
			"kesäkuuta", "heinäkuuta", "elokuuta", "syyskuuta", "lokakuuta", "marraskuuta", "joulukuuta"},
		open: "”", close: "”", open2: "’", close2: "’",
	},
}

// getLocale returns the locale for a BCP 47 tag ("lv-LV" → "lv"),
// falling back to English.
func getLocale(tag string) *locale {
	lang := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	switch lang {
	case "nb", "nn", "no", "da":
		// Close enough for the few terms that differ from Swedish.
		lang = "sv"
	}
	if l, ok := locales[lang]; ok {
		return l
	}
	return locales["en"]
}

// ordinal renders an edition number as an ordinal ("2nd", "2.", "2e").
func (l *locale) ordinal(n int) string {
	s := strconv.Itoa(n)
	switch l.lang {
	case "en":
		suf := "th"
		if n%100 < 11 || n%100 > 13 {
			switch n % 10 {
			case 1:
				suf = "st"
			case 2:
				suf = "nd"
			case 3:
				suf = "rd"
			}
		}
		return s + suf
	case "fr":
		if n == 1 {
			return "1re"
		}
		return s + "e"
	case "nl":
		return s + "e"
	case "es", "pt":
		return s + ".ª"
	case "it":
		return s + "ª"
	case "sv":
		if (n%10 == 1 || n%10 == 2) && n%100 != 11 && n%100 != 12 {
			return s + ":a"
		}
		return s + ":e"
	case "lt":
		return s + "-asis"
	}
	return s + "."
}

// editionLabel renders "2nd ed.", "2. izd.", "wyd. 2".
func (l *locale) editionLabel(n int, term string) string {
	if l.lang == "pl" {
		return term + " " + strconv.Itoa(n)
	}
	return l.ordinal(n) + " " + term
}

// dayMonth renders a day and month in the locale's order ("May 1",
// "1 May", "1. maijs", "gegužės 1 d."). month is the already chosen name.
func (l *locale) dayMonth(d, m int, month string, short, dmy bool) string {
	if d == 0 {
		return month
	}
	ds := strconv.Itoa(d)
	if !short && l.monthsDate != nil && m >= 1 && m <= 12 {
		month = l.monthsDate[m-1]
	}
	switch l.lang {
	case "en":
		if dmy {
			return ds + " " + month
		}
		return month + " " + ds
	case "lv", "de", "et", "fi":
		return ds + ". " + month
	case "lt":
		return month + " " + ds + " d."
	case "es", "pt":
		return ds + " de " + month
	case "fr":
		if d == 1 {
			ds = "1er"
		}
		return ds + " " + month
	}
	return ds + " " + month
}

// fullDate joins a day/month phrase (or month name) with the year.
func (l *locale) fullDate(y, d int, dm string, dmy bool) string {
	ys := strconv.Itoa(y)
	if dm == "" {
		return ys
	}
	switch l.lang {
	case "en":
		if d == 0 || dmy {
			return dm + " " + ys
		}
		return dm + ", " + ys
	case "lv":
		return ys + ". gada " + dm
	case "lt":
		return ys + " m. " + dm
	case "es", "pt":
		return dm + " de " + ys
	}
	return dm + " " + ys
}

// postfixNumber writes the Latvian-style ordinal dots after the numbers of
// a value placed before its term: "33–35" → "33.–35.".
func postfixNumber(v string) string {
	var sb strings.Builder
	digits := false
	for _, r := range v {
		if r >= '0' && r <= '9' {
			digits = true
			sb.WriteRune(r)
			continue
		}
		if unicode.IsLetter(r) {
			return v // identifiers such as "A12" stay as they are
		}
		if digits {
			sb.WriteByte('.')
			digits = false
		}
		sb.WriteRune(r)
	}
	if digits {
		sb.WriteByte('.')
	}
	return sb.String()
}
