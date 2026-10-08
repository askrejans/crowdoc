package pipeline

import (
	"strings"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/support/hyph"
	"github.com/askrejans/crowdoc/v2/internal/support/locale"
)

type localeInfo struct {
	typstLang, typstRegion string
	terms                  map[string]string
	labels                 map[string][2]string
	pageOf                 string
	longDate               func(time.Time) string
}

var closings = map[string]string{
	"en": "Sincerely,", "lv": "Ar cieņu,", "lt": "Pagarbiai,", "et": "Lugupidamisega", "fi": "Ystävällisin terveisin",
	"sv": "Med vänliga hälsningar", "nb": "Med vennlig hilsen", "da": "Med venlig hilsen", "is": "Virðingarfyllst,",
	"de": "Mit freundlichen Grüßen", "nl": "Met vriendelijke groet,", "lb": "Mat beschte Gréiss", "fr": "Cordialement,",
	"it": "Cordiali saluti,", "es": "Atentamente,", "pt": "Com os melhores cumprimentos,", "pl": "Z poważaniem",
	"cs": "S pozdravem", "sk": "S pozdravom", "sl": "Lep pozdrav,", "hr": "S poštovanjem,", "cnr": "S poštovanjem,",
	"sq": "Me respekt,", "mk": "Со почит,", "bg": "С уважение,", "el": "Με εκτίμηση,", "ro": "Cu stimă,",
	"hu": "Tisztelettel:", "tr": "Saygılarımla,", "uk": "З повагою,", "ru": "С уважением,", "sr": "С поштовањем,",
	"ca": "Atentament,", "eu": "Adeitasunez,", "ga": "Is mise le meas,", "ja": "敬具", "zh": "此致敬礼", "ko": "감사합니다.",
	"ar": "مع خالص التحية،", "he": "בברכה,",
}

func localeFor(lang string) localeInfo {
	t := locale.Get(lang)
	terms := map[string]string{
		"contents": t.Contents, "list-of-figures": t.ListOfFigures, "list-of-tables": t.ListOfTables,
		"list-of-listings": t.ListOfListings, "figure": t.Figure, "table": t.Table, "listing": t.Listing,
		"equation": t.Equation, "chapter": t.Chapter, "section": t.Section, "part": t.Part, "appendix": t.Appendix,
		"abstract": t.Abstract, "keywords": t.Keywords, "references": t.References, "bibliography": t.Bibliography,
		"index": t.Index, "page": t.Page, "continued": t.Continued, "notes": t.Notes, "caption-sep": t.CaptionSep,
		"note": t.Note, "tip": t.Tip, "info": t.Info, "important": t.Important, "warning": t.Warning,
		"caution": t.Caution, "danger": t.Danger, "success": t.Success, "example": t.Example, "summary": t.Summary,
		"theorem": t.Theorem, "lemma": t.Lemma, "corollary": t.Corollary, "proposition": t.Proposition,
		"definition": t.Definition, "remark": t.Remark, "proof": t.Proof, "exercise": t.Exercise, "solution": t.Solution,
		"version": t.Version, "status": t.Status, "date": t.Date, "author": t.Author, "authors": t.Authors,
		"prepared-by": t.PreparedBy, "prepared-for": t.PreparedFor, "classification": t.Classification,
		"confidential": t.Confidential, "draft": t.Draft, "final": t.Final, "subject": t.Subject, "to": t.To,
		"from": t.From, "cc": t.CC, "re": t.Re, "enclosure": t.Enclosure, "attachment": t.Attachment,
		"signature": t.Signature, "name": t.Name, "position": t.Position, "place": t.Place, "invoice": t.Invoice,
		"invoice-number": t.InvoiceNumber, "issue-date": t.IssueDate, "due-date": t.DueDate, "total": t.Total,
		"subtotal": t.Subtotal, "tax": t.Tax, "amount": t.Amount, "quantity": t.Quantity, "unit-price": t.UnitPrice,
		"description": t.Description, "memorandum": t.Memorandum, "minutes": t.Minutes, "attendees": t.Attendees,
		"agenda": t.Agenda, "action-items": t.ActionItems, "decisions": t.Decisions, "agreement": t.Agreement,
		"parties": t.Parties, "between": t.Between, "and": t.And, "signed-on": t.SignedOn, "for-party": t.ForParty,
		"supervisor": t.Supervisor, "advisor": t.Advisor, "faculty": t.Faculty, "department": t.Department,
		"institution": t.Institution, "degree": t.Degree, "submitted-by": t.SubmittedBy, "submitted-to": t.SubmittedTo,
		"thesis": t.Thesis, "dissertation": t.Dissertation, "masters-thesis": t.MastersThesis,
		"bachelors-thesis": t.BachelorsThesis, "declaration": t.Declaration, "acknowledgements": t.Acknowledgements,
		"dedication": t.Dedication,
	}
	base := strings.ToLower(strings.SplitN(t.Tag, "-", 2)[0])
	if c, ok := closings[base]; ok {
		terms["closing"] = c
	}
	// Numbered labels as prefix/suffix around the number: "Figure 3",
	// "3. attēls", "図3", "第3章".
	labels := map[string][2]string{}
	for key, term := range map[string]string{"figure": t.Figure, "table": t.Table, "listing": t.Listing, "chapter": t.Chapter, "section": t.Section, "appendix": t.Appendix} {
		l := t.Label(term, "\x00")
		if pre, post, ok := strings.Cut(l, "\x00"); ok {
			labels[key] = [2]string{pre, post}
		}
	}
	tag := t.Tag
	return localeInfo{
		typstLang: t.TypstLang, typstRegion: t.TypstRegion, terms: terms, labels: labels, pageOf: t.PageOfFmt,
		longDate: func(d time.Time) string { return locale.FormatDate(d, tag, true) },
	}
}

// hyphenator inserts soft hyphens for languages the engine cannot
// hyphenate itself (Latvian, Romanian, Macedonian, Irish, Basque, …).
func hyphenator(lang string) func(string) string {
	if lang == "" || !hyph.Supported(lang) || hyph.TypstHyphenates(lang) {
		return nil
	}
	return func(s string) string { return hyph.InsertSoftHyphens(s, lang) }
}

// documentLanguage decides the document language: an explicit choice
// (frontmatter, caller metadata) wins; a language declared by office
// metadata is checked against the text; otherwise it is detected.
func documentLanguage(p *prepared, opts Options) (string, string) {
	doc := p.doc
	explicit := false
	for k := range opts.Meta {
		if k := strings.ToLower(k); k == "lang" || k == "language" {
			explicit = true
		}
	}
	switch p.format {
	case FormatMarkdown, FormatNotebook, FormatText:
		explicit = explicit || doc.Meta.Lang != ""
	}
	if doc.Meta.Lang != "" && explicit {
		return doc.Meta.Lang, ""
	}
	if doc.Meta.Lang == "" {
		return detectLanguage(doc), ""
	}
	lang, changed := locale.Verify(doc.Meta.Lang, documentText(doc))
	if changed {
		return lang, "the file declares language " + doc.Meta.Lang + " but the text reads as " + lang + "; using " + lang + " (set --lang to override)"
	}
	return lang, ""
}

// detectLanguage guesses the language from the document's own text.
func detectLanguage(doc *ast.Document) string { return locale.Detect(documentText(doc)) }

func documentText(doc *ast.Document) string {
	var sb strings.Builder
	sb.WriteString(doc.Meta.Title)
	sb.WriteByte('\n')
	ast.WalkBlocks(doc.Blocks, func(b ast.Block) bool {
		if sb.Len() > 20000 {
			return false
		}
		switch n := b.(type) {
		case *ast.Para:
			sb.WriteString(ast.PlainText(n.Inlines))
			sb.WriteByte('\n')
		case *ast.Plain:
			sb.WriteString(ast.PlainText(n.Inlines))
			sb.WriteByte('\n')
		case *ast.Heading:
			sb.WriteString(ast.PlainText(n.Inlines))
			sb.WriteByte('\n')
		case *ast.CodeBlock, *ast.MathBlock:
			return false
		}
		return true
	})
	return sb.String()
}
