package docx

import (
	"encoding/json"
	"html"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

// ---------------------------------------------------------------------------
// Reference registry

// addReference registers a cited work and returns its citation key. ident
// identifies the work across citations (a URI, a source tag); ref may be
// nil when the field carries no bibliographic data.
func (r *reader) addReference(id, ident string, ref *ast.Reference) string {
	if ident == "" && ref != nil {
		ident = refIdentity(*ref)
	}
	if ident == "" {
		ident = "id:" + id
	}
	if key, ok := r.refIdent[ident]; ok {
		if ref != nil {
			if _, have := r.refIndex[key]; !have {
				r.storeReference(key, *ref)
			}
		}
		return key
	}
	base := sanitizeKey(id)
	if base == "" && ref != nil {
		base = sanitizeKey(defaultKey(*ref))
	}
	if base == "" {
		r.refSerial++
		base = "ref-" + strconv.Itoa(r.refSerial)
	}
	key := base
	for n := 2; ; n++ {
		if owner, taken := r.usedKeys[key]; !taken || owner == ident {
			break
		}
		key = base + "-" + strconv.Itoa(n)
	}
	r.usedKeys[key] = ident
	r.refIdent[ident] = key
	if ref != nil {
		r.storeReference(key, *ref)
	}
	return key
}

func (r *reader) storeReference(key string, ref ast.Reference) {
	ref.ID = key
	r.refIndex[key] = len(r.doc.References)
	r.doc.References = append(r.doc.References, ref)
}

func refIdentity(ref ast.Reference) string {
	t := strings.ToLower(strings.Join(strings.Fields(ref.Title), " "))
	if t == "" {
		return ""
	}
	fam := ""
	if len(ref.Author) > 0 {
		fam = strings.ToLower(ref.Author[0].Family + ref.Author[0].Literal)
	}
	return "work:" + t + "|" + strconv.Itoa(ref.Issued.Year) + "|" + fam
}

// defaultKey derives an author-year key ("smith2020").
func defaultKey(ref ast.Reference) string {
	name := ""
	if len(ref.Author) > 0 {
		name = ref.Author[0].Family
		if name == "" {
			name = ref.Author[0].Literal
		}
	}
	if name == "" {
		name = ref.Title
	}
	name = strings.ReplaceAll(sanitizeID(name), "-", "")
	if len(name) > 24 {
		name = name[:24]
	}
	if ref.Issued.Year != 0 {
		name += strconv.Itoa(ref.Issued.Year)
	}
	return name
}

// sanitizeKey keeps citation keys to letters, digits and -_:.
func sanitizeKey(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == ':', r == '.':
			b.WriteRune(r)
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	return strings.Trim(b.String(), "-.")
}

// ---------------------------------------------------------------------------
// CSL-JSON citations (reference-manager field codes)

// cslCitation parses the CSL citation object embedded in a field
// instruction such as `ADDIN ... CSL_CITATION {...}`.
func (r *reader) cslCitation(instr string) ([]ast.CiteItem, bool) {
	i := strings.IndexByte(instr, '{')
	if i < 0 {
		return nil, false
	}
	var c struct {
		CitationItems []map[string]json.RawMessage `json:"citationItems"`
	}
	if err := json.NewDecoder(strings.NewReader(instr[i:])).Decode(&c); err != nil {
		return nil, false
	}
	var items []ast.CiteItem
	for _, ci := range c.CitationItems {
		var data map[string]json.RawMessage
		if raw, ok := ci["itemData"]; ok {
			if err := json.Unmarshal(raw, &data); err != nil {
				data = nil
			}
		}
		id := jsonString(ci["id"])
		if id == "" {
			id = jsonString(data["id"])
		}
		uri := ""
		if uris := jsonStrings(ci["uris"]); len(uris) > 0 {
			uri = uris[0]
		} else if uris := jsonStrings(ci["uri"]); len(uris) > 0 {
			uri = uris[0]
		}
		var ref *ast.Reference
		if data != nil {
			x := cslReference(data)
			ref = &x
		}
		if generatedItemID(id) {
			// Per-document item numbers are reused for different works;
			// the stable URI (or the work itself) is a better key.
			id = ""
			if uri != "" {
				id = path.Base(strings.TrimRight(uri, "/"))
				if i := strings.LastIndexAny(id, "=?"); i >= 0 {
					id = id[i+1:]
				}
			}
		}
		if id == "" && uri == "" && ref == nil {
			continue
		}
		key := r.addReference(id, uri, ref)
		items = append(items, ast.CiteItem{
			Key:            key,
			Prefix:         cleanMarkup(jsonString(ci["prefix"])),
			Suffix:         cleanMarkup(jsonString(ci["suffix"])),
			Locator:        jsonString(ci["locator"]),
			LocatorLabel:   jsonString(ci["label"]),
			SuppressAuthor: jsonBool(ci["suppress-author"]),
		})
	}
	return items, len(items) > 0
}

var generatedIDRe = regexp.MustCompile(`^(?i)item-?\d+$`)

func generatedItemID(id string) bool { return generatedIDRe.MatchString(id) }

// cslReference converts a CSL-JSON item.
func cslReference(m map[string]json.RawMessage) ast.Reference {
	s := func(keys ...string) string {
		for _, k := range keys {
			if v := cleanMarkup(jsonString(m[k])); v != "" {
				return v
			}
		}
		return ""
	}
	ref := ast.Reference{
		Type:            s("type"),
		Title:           s("title"),
		ShortTitle:      s("title-short", "shortTitle"),
		ContainerTitle:  s("container-title", "container_title"),
		CollectionTitle: s("collection-title"),
		Publisher:       s("publisher"),
		PublisherPlace:  s("publisher-place"),
		Edition:         s("edition"),
		Volume:          s("volume"),
		Issue:           s("issue"),
		Page:            s("page"),
		Number:          s("number"),
		Genre:           s("genre"),
		Event:           s("event-title", "event"),
		EventPlace:      s("event-place"),
		Medium:          s("medium"),
		Version:         s("version"),
		Language:        s("language"),
		URL:             s("URL", "url"),
		DOI:             s("DOI", "doi"),
		ISBN:            s("ISBN"),
		ISSN:            s("ISSN"),
		Note:            s("note"),
		Abstract:        s("abstract"),
		Author:          cslNames(m["author"]),
		Editor:          cslNames(m["editor"]),
		Translator:      cslNames(m["translator"]),
		Issued:          cslDate(m["issued"]),
		Accessed:        cslDate(m["accessed"]),
	}
	if ref.Type == "" {
		ref.Type = "article"
	}
	return ref
}

func cslNames(raw json.RawMessage) []ast.Name {
	if len(raw) == 0 {
		return nil
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	var out []ast.Name
	for _, n := range arr {
		get := func(k string) string { return cleanMarkup(jsonString(n[k])) }
		name := ast.Name{
			Family:   get("family"),
			Given:    get("given"),
			Particle: get("non-dropping-particle"),
			Suffix:   get("suffix"),
			Literal:  get("literal"),
		}
		if dp := get("dropping-particle"); dp != "" {
			name.Given = strings.TrimSpace(name.Given + " " + dp)
		}
		if name.Family == "" && name.Given == "" && name.Literal == "" {
			continue
		}
		if name.Literal != "" {
			name.Family, name.Given = "", ""
		}
		out = append(out, name)
	}
	return out
}

func cslDate(raw json.RawMessage) ast.Date {
	if len(raw) == 0 {
		return ast.Date{}
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		if s := jsonString(raw); s != "" {
			return parseDate(s)
		}
		return ast.Date{}
	}
	var d ast.Date
	if dp, ok := m["date-parts"]; ok {
		var parts [][]json.RawMessage
		if json.Unmarshal(dp, &parts) == nil && len(parts) > 0 && len(parts[0]) > 0 {
			p := parts[0]
			if first := jsonString(p[0]); len(p) == 1 && strings.ContainsAny(first, "-/") {
				d = parseDate(first)
			} else {
				nums := make([]int, 0, 3)
				for _, x := range p {
					n, err := strconv.Atoi(strings.TrimSpace(jsonString(x)))
					if err != nil {
						break
					}
					nums = append(nums, n)
				}
				d = dateFromParts(nums)
			}
		}
	}
	if d.Year == 0 {
		if y, err := strconv.Atoi(jsonString(m["year"])); err == nil {
			mo, _ := strconv.Atoi(jsonString(m["month"]))
			day, _ := strconv.Atoi(jsonString(m["day"]))
			d = dateFromParts([]int{y, mo, day})
		}
	}
	if d.Year == 0 {
		if lit := cleanMarkup(jsonString(m["literal"])); lit != "" {
			d = ast.Date{Literal: lit}
		} else if raw := jsonString(m["raw"]); raw != "" {
			d = parseDate(raw)
		}
	}
	if c, ok := m["circa"]; ok {
		d.Circa = jsonBool(c) || jsonString(c) == "1"
	}
	return d
}

func dateFromParts(nums []int) ast.Date {
	var d ast.Date
	if len(nums) > 0 && nums[0] > 0 && nums[0] < 10000 {
		d.Year = nums[0]
	} else {
		return d
	}
	if len(nums) > 1 && nums[1] >= 1 && nums[1] <= 12 {
		d.Month = nums[1]
		if len(nums) > 2 && nums[2] >= 1 && nums[2] <= 31 {
			d.Day = nums[2]
		}
	}
	return d
}

var (
	isoDateRe = regexp.MustCompile(`^(\d{4})(?:[-/.](\d{1,2})(?:[-/.](\d{1,2}))?)?`)
	dmyDateRe = regexp.MustCompile(`^(\d{1,2})[./](\d{1,2})[./](\d{4})$`)
	yearRe    = regexp.MustCompile(`^\d{4}$`)
)

// parseDate parses "2020", "2020-03", "2020-03-15", "15.03.2020"; anything
// else is kept as a literal.
func parseDate(s string) ast.Date {
	s = strings.TrimSpace(s)
	if m := isoDateRe.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		return dateFromParts([]int{y, mo, day})
	}
	if m := dmyDateRe.FindStringSubmatch(s); m != nil {
		day, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		y, _ := strconv.Atoi(m[3])
		return dateFromParts([]int{y, mo, day})
	}
	if s == "" {
		return ast.Date{}
	}
	return ast.Date{Literal: s}
}

// jsonString returns a JSON string or number as text (first element of an
// array of strings).
func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s)
		}
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
			return jsonString(arr[0])
		}
	case '{', 'n', 't', 'f':
		return ""
	default:
		var n json.Number
		if json.Unmarshal(raw, &n) == nil {
			return n.String()
		}
	}
	return ""
}

func jsonStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) != nil {
		if s := jsonString(raw); s != "" {
			return []string{s}
		}
		return nil
	}
	var out []string
	for _, x := range arr {
		if s := jsonString(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func jsonBool(raw json.RawMessage) bool {
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	return jsonString(raw) == "true"
}

// cleanMarkup removes the rich-text tags reference managers allow in
// titles and decodes entities.
func cleanMarkup(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(rd.CleanText(html.UnescapeString(b.String())))
}

// ---------------------------------------------------------------------------
// Built-in citations (CITATION fields + customXml b:Sources)

// readSources loads bibliography sources stored in custom XML parts.
func (r *reader) readSources() {
	var names []string
	for _, rl := range sortedRels(r.main.rels) {
		if strings.HasSuffix(rl.typ, "/customXml") && !rl.external {
			names = append(names, rl.target)
		}
	}
	for _, n := range r.z.Names() {
		ln := strings.ToLower(n)
		if strings.HasPrefix(ln, "customxml/item") && strings.HasSuffix(ln, ".xml") && !strings.Contains(ln, "props") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	seen := map[string]bool{}
	for _, name := range names {
		if seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		root := r.optionalPart(name)
		if root == nil || root.name != "Sources" {
			continue
		}
		for _, src := range root.childrenNamed("Source") {
			tag, ref := bibSource(src)
			if tag != "" {
				r.sources[tag] = ref
			}
		}
	}
}

var sourceTypes = map[string]string{
	"Book": "book", "BookSection": "chapter", "JournalArticle": "article-journal",
	"ArticleInAPeriodical": "article-magazine", "ConferenceProceedings": "paper-conference",
	"Report": "report", "SoundRecording": "song", "Performance": "speech", "Art": "graphic",
	"DocumentFromInternetSite": "webpage", "InternetSite": "webpage", "Film": "motion_picture",
	"Interview": "interview", "Patent": "patent", "ElectronicSource": "article", "Case": "legal_case",
	"Misc": "article",
}

// bibSource converts a b:Source element.
func bibSource(n *node) (string, ast.Reference) {
	get := func(names ...string) string {
		for _, name := range names {
			if v := strings.TrimSpace(rd.CleanText(n.child(name).textContent())); v != "" {
				return v
			}
		}
		return ""
	}
	typ := get("SourceType")
	ref := ast.Reference{
		Type:           sourceTypes[typ],
		Title:          get("Title"),
		ShortTitle:     get("ShortTitle"),
		Publisher:      get("Publisher", "Institution", "ProductionCompany", "Distributor", "Broadcaster", "Court"),
		PublisherPlace: get("City", "StateProvince", "CountryRegion"),
		Edition:        get("Edition"),
		Volume:         get("Volume"),
		Issue:          get("Issue"),
		Page:           get("Pages"),
		Number:         get("PatentNumber", "CaseNumber"),
		Genre:          get("ThesisType", "ReportType"),
		Medium:         get("Medium"),
		Version:        get("Version"),
		URL:            get("URL"),
		DOI:            get("DOI"),
		Note:           get("Comments"),
	}
	if ref.Type == "" {
		ref.Type = "article"
	}
	if get("ThesisType") != "" {
		ref.Type = "thesis"
	}
	switch typ {
	case "JournalArticle":
		ref.ContainerTitle = get("JournalName")
	case "ArticleInAPeriodical":
		ref.ContainerTitle = get("PeriodicalTitle")
	case "ConferenceProceedings":
		ref.ContainerTitle = get("ConferenceName", "BookTitle")
	case "BookSection":
		ref.ContainerTitle = get("BookTitle")
	case "InternetSite", "DocumentFromInternetSite":
		ref.ContainerTitle = get("InternetSiteTitle", "ProductionCompany")
	}
	if ref.ContainerTitle == "" {
		ref.ContainerTitle = get("JournalName", "PeriodicalTitle", "BookTitle", "ConferenceName",
			"InternetSiteTitle", "AlbumTitle", "BroadcastTitle", "PublicationTitle")
	}
	if ref.Publisher == ref.ContainerTitle {
		ref.Publisher = ""
	}
	if sn := get("StandardNumber"); sn != "" {
		if strings.Contains(strings.ToUpper(sn), "ISSN") || issnRe.MatchString(sn) {
			ref.ISSN = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(sn), "ISSN"))
		} else {
			ref.ISBN = strings.TrimSpace(strings.TrimPrefix(strings.ToUpper(sn), "ISBN"))
		}
	}
	ref.Issued = sourceDate(get("Year"), get("Month"), get("Day"))
	ref.Accessed = sourceDate(get("YearAccessed"), get("MonthAccessed"), get("DayAccessed"))
	if roles := n.child("Author"); roles != nil {
		ref.Author = sourceNames(roles.child("Author"))
		ref.Editor = sourceNames(roles.child("Editor"))
		ref.Translator = sourceNames(roles.child("Translator"))
		if len(ref.Author) == 0 {
			for _, role := range []string{"Artist", "Composer", "Director", "Interviewee", "Inventor", "Performer", "Writer", "Compiler"} {
				if names := sourceNames(roles.child(role)); len(names) > 0 {
					ref.Author = names
					break
				}
			}
		}
	}
	return get("Tag"), ref
}

var issnRe = regexp.MustCompile(`^\d{4}-\d{3}[\dXx]$`)

func sourceNames(role *node) []ast.Name {
	if role == nil {
		return nil
	}
	var out []ast.Name
	if corp := strings.TrimSpace(role.child("Corporate").textContent()); corp != "" {
		out = append(out, ast.Name{Literal: corp})
	}
	for _, p := range role.child("NameList").childrenNamed("Person") {
		get := func(k string) string { return strings.TrimSpace(p.child(k).textContent()) }
		given := strings.TrimSpace(get("First") + " " + get("Middle"))
		name := ast.Name{Family: get("Last"), Given: given}
		if name.Family == "" && name.Given == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

var monthNames = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6, "july": 7,
	"august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
	"janvāris": 1, "februāris": 2, "marts": 3, "aprīlis": 4, "maijs": 5, "jūnijs": 6,
	"jūlijs": 7, "augusts": 8, "septembris": 9, "oktobris": 10, "novembris": 11, "decembris": 12,
	"januar": 1, "februar": 2, "märz": 3, "mai": 5, "juni": 6, "juli": 7, "oktober": 10, "dezember": 12,
	"janvier": 1, "février": 2, "mars": 3, "avril": 4, "juin": 6, "juillet": 7, "août": 8,
	"septembre": 9, "octobre": 10, "novembre": 11, "décembre": 12,
}

func monthNumber(s string) int {
	s = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(s, ".")))
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 12 {
		return n
	}
	if n, ok := monthNames[s]; ok {
		return n
	}
	if len(s) >= 3 {
		for name, n := range monthNames {
			if strings.HasPrefix(name, s) {
				return n
			}
		}
	}
	return 0
}

func sourceDate(year, month, day string) ast.Date {
	if year == "" {
		return ast.Date{}
	}
	y, err := strconv.Atoi(year)
	if err != nil || y <= 0 || y >= 10000 {
		if yearRe.MatchString(year) {
			y, _ = strconv.Atoi(year)
		} else {
			return ast.Date{Literal: year}
		}
	}
	d, _ := strconv.Atoi(strings.TrimSpace(day))
	return dateFromParts([]int{y, monthNumber(month), d})
}

// builtinCitation converts a CITATION field instruction:
// CITATION Tag [\l lcid] [\p pages] [\s suffix] [\f prefix] [\n] [\m Tag2 ...]
func (r *reader) builtinCitation(in instr) []ast.CiteItem {
	var items []ast.CiteItem
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		key := ""
		if src, ok := r.sources[tag]; ok {
			key = r.addReference(tag, "source:"+tag, &src)
		} else {
			r.warn.Addf("citation source %q is not in the document's source list", tag)
			key = r.addReference(tag, "source:"+tag, nil)
		}
		items = append(items, ast.CiteItem{Key: key})
	}
	toks := in.toks
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if !t.sw {
			if len(items) == 0 {
				add(t.s)
			}
			continue
		}
		arg, hasArg := "", switchTakesArg(toks, i)
		if hasArg {
			arg = toks[i+1].s
		}
		cur := len(items) - 1
		switch strings.ToLower(t.s) {
		case `\m`:
			add(arg)
		case `\p`:
			if cur >= 0 {
				items[cur].Locator = arg
			}
		case `\s`:
			if cur >= 0 {
				items[cur].Suffix = arg
			}
		case `\f`:
			if cur >= 0 {
				items[cur].Prefix = arg
			}
		case `\v`:
			if cur >= 0 && items[cur].Locator == "" {
				items[cur].Locator, items[cur].LocatorLabel = arg, "volume"
			}
		case `\n`:
			if cur >= 0 {
				items[cur].SuppressAuthor = true
			}
			continue
		case `\t`, `\y`:
			continue
		}
		if hasArg {
			i++
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// Citations with embedded record XML (`ADDIN EN.CITE <...>`)

// recordCitation converts a citation field that embeds its bibliographic
// records as XML in the field instruction.
func (r *reader) recordCitation(instr string) ([]ast.CiteItem, bool) {
	i := strings.Index(instr, "<EndNote")
	if i < 0 {
		return nil, false
	}
	root, _ := parseXML([]byte(instr[i:]))
	if root == nil {
		return nil, false
	}
	var items []ast.CiteItem
	for _, c := range root.childrenNamed("Cite") {
		rec := c.child("record")
		num := strings.TrimSpace(c.child("RecNum").textContent())
		if num == "" {
			num = strings.TrimSpace(rec.child("rec-number").textContent())
		}
		var ref *ast.Reference
		if rec != nil {
			x := recordReference(rec)
			ref = &x
		}
		id, ident := "", ""
		if num != "" {
			id, ident = "rec-"+num, "record:"+num
		}
		if id == "" && ref == nil {
			continue
		}
		key := r.addReference(id, ident, ref)
		item := ast.CiteItem{
			Key:     key,
			Locator: strings.TrimSpace(c.child("Pages").textContent()),
			Prefix:  strings.TrimSpace(strings.ReplaceAll(c.child("Prefix").textContent(), "`", "")),
			Suffix:  strings.TrimSpace(strings.ReplaceAll(c.child("Suffix").textContent(), "`", "")),
		}
		item.SuppressAuthor = truthy(c.attr("ExcludeAuth"))
		items = append(items, item)
	}
	return items, len(items) > 0
}

var recordTypes = map[string]string{
	"journal article": "article-journal", "book": "book", "book section": "chapter",
	"edited book": "book", "conference proceedings": "paper-conference",
	"conference paper": "paper-conference", "thesis": "thesis", "report": "report",
	"web page": "webpage", "newspaper article": "article-newspaper",
	"magazine article": "article-magazine", "patent": "patent", "dataset": "dataset",
	"computer program": "software", "generic": "article", "electronic article": "article-journal",
	"standard": "standard", "statute": "legislation", "case": "legal_case",
}

func recordReference(rec *node) ast.Reference {
	text := func(n *node) string { return strings.TrimSpace(rd.CleanText(n.textContent())) }
	titles := rec.child("titles")
	ref := ast.Reference{
		Type:           recordTypes[strings.ToLower(rec.child("ref-type").attr("name"))],
		Title:          text(titles.child("title")),
		ShortTitle:     text(titles.child("short-title")),
		ContainerTitle: text(titles.child("secondary-title")),
		Page:           text(rec.child("pages")),
		Volume:         text(rec.child("volume")),
		Issue:          text(rec.child("number")),
		Edition:        text(rec.child("edition")),
		Publisher:      text(rec.child("publisher")),
		PublisherPlace: text(rec.child("pub-location")),
		DOI:            text(rec.child("electronic-resource-num")),
		Language:       text(rec.child("language")),
		Abstract:       text(rec.child("abstract")),
	}
	if ref.Type == "" {
		ref.Type = "article"
	}
	if ref.ContainerTitle == "" {
		ref.ContainerTitle = text(rec.child("periodical").child("full-title"))
	}
	if isbn := text(rec.child("isbn")); isbn != "" {
		if issnRe.MatchString(isbn) {
			ref.ISSN = isbn
		} else {
			ref.ISBN = isbn
		}
	}
	if u := rec.child("urls").child("related-urls").child("url"); u != nil {
		ref.URL = text(u)
	}
	contrib := rec.child("contributors")
	ref.Author = recordNames(contrib.child("authors"))
	ref.Editor = recordNames(contrib.child("secondary-authors"))
	ref.Translator = recordNames(contrib.child("translated-authors"))
	dates := rec.child("dates")
	ref.Issued = parseDate(text(dates.child("year")))
	if pd := text(dates.child("pub-dates").child("date")); pd != "" && ref.Issued.Year != 0 {
		if m := monthNumber(strings.Fields(pd)[0]); m > 0 {
			ref.Issued.Month = m
		}
	}
	return ref
}

func recordNames(list *node) []ast.Name {
	var out []ast.Name
	for _, a := range list.childrenNamed("author") {
		s := strings.TrimSpace(rd.CleanText(a.textContent()))
		switch {
		case s == "":
			continue
		case strings.HasSuffix(s, ","):
			out = append(out, ast.Name{Literal: strings.TrimSuffix(s, ",")})
		case strings.Contains(s, ","):
			fam, given, _ := strings.Cut(s, ",")
			out = append(out, ast.Name{Family: strings.TrimSpace(fam), Given: strings.TrimSpace(given)})
		case !strings.Contains(s, " "):
			out = append(out, ast.Name{Family: s})
		default:
			out = append(out, ast.Name{Literal: s})
		}
	}
	return out
}
