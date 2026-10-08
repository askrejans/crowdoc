package rtf

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/askrejans/crowdoc/v2/ast"
	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func read(t *testing.T, src string) (*ast.Document, []string) {
	t.Helper()
	doc, warns, err := Read(context.Background(), []byte(src), rd.Options{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Resources == nil {
		t.Fatal("Resources not initialised")
	}
	return doc, warns
}

// body wraps paragraph-level RTF in a minimal document header.
func body(s string) string {
	return `{\rtf1\ansi\ansicpg1252\deff0{\fonttbl{\f0\froman\fcharset0 Times New Roman;}{\f1\fmodern\fcharset0\fprq1 Courier New;}{\f2\froman\fcharset2\fprq2 Symbol;}}` +
		`{\colortbl;\red0\green0\blue0;\red255\green255\blue255;\red255\green255\blue0;}` + "\n" + s + "}"
}

func checkDump(t *testing.T, got []ast.Block, want string) {
	t.Helper()
	if d := dump(got); d != want {
		t.Errorf("blocks mismatch\n got:\n%s\nwant:\n%s", d, want)
	}
}

// A document in the shape full-featured word processors write: theme and
// latent-style noise, a style
// sheet with built-in names, list tables, Baltic code page fonts, info.
const fullDoc = `{\rtf1\adeflang1025\ansi\ansicpg1257\uc1\adeff31507\deff0\stshfdbch31506\stshfloch31506\stshfhich31506\stshfbi31507\deflang1062\deflangfe1062\themelang1062
{\fonttbl{\f0\fbidi \froman\fcharset186\fprq2{\*\panose 02020603050405020304}Times New Roman;}{\f2\fbidi \fmodern\fcharset186\fprq1{\*\panose 02070309020205020404}Courier New;}
{\f3\fbidi \froman\fcharset2\fprq2{\*\panose 05050102010706020507}Symbol;}{\f37\fbidi \fswiss\fcharset186\fprq2{\*\panose 020f0502020204030204}Calibri{\*\falt Arial};}}
{\colortbl;\red0\green0\blue0;\red5\green99\blue193;\red255\green255\blue0;}
{\*\defchp \f37\fs22\lang1062\langfe1033 }{\*\defpap \ql \li0\ri0\sa160\sl259\slmult1\widctlpar\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 }
\noqfpromote {\stylesheet{\ql \li0\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 \snext0 \sqformat \spriority0 Normal;}
{\s1\ql \li0\ri0\sb240\keepn\widctlpar\wrapdefault\aspalpha\aspnum\faauto\outlinelevel0\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs32\alang1025 \ltrch\fcs0 \fs32\cf1\lang1062\langfe1033\cgrid \sbasedon0 \snext0 \slink15 \sqformat \spriority9 heading 1;}
{\s2\ql \li0\ri0\sb40\keepn\widctlpar\outlinelevel1\adjustright\rin0\lin0\itap0 \fs26\cf1 \sbasedon0 \snext0 \sqformat \spriority9 heading 2;}
{\*\cs10 \additive \ssemihidden \sunhideused \spriority1 Default Paragraph Font;}
{\*\ts11\tsrowd\trftsWidthB3\trpaddl108\trpaddr108\trpaddfl3\trpaddft3\trpaddfb3\trpaddfr3\tblind0\tblindtype3\tsvertalt\tsbrdrt\tsbrdrl\tsbrdrb\tsbrdrr\tsbrdrdgl\tsbrdrdgr\tsbrdrh\tsbrdrv \ql \li0\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 \snext11 \ssemihidden \sunhideused Normal Table;}
{\s15\ql \li0\ri0\contextualspace\widctlpar\adjustright\rin0\lin0\itap0 \fs56\expnd-2\expndtw-10\kerning28 \sbasedon0 \snext0 \slink16 \sqformat \spriority10 Title;}
{\s17\ql \li720\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin720\itap0\contextualspace \sbasedon0 \snext17 \sqformat \spriority34 List Paragraph;}
{\s18\ql \li0\ri0\sa200\widctlpar\adjustright\rin0\lin0\itap0 \i\fs18\cf1 \sbasedon0 \snext0 \sunhideused \sqformat \spriority35 caption;}
{\*\cs19 \additive \ul\cf2 \sbasedon10 \sunhideused Hyperlink;}
{\s20\ql \li0\ri0\sb240\keepn\widctlpar\adjustright\rin0\lin0\itap0 \fs32\cf1 \sbasedon1 \snext0 \sunhideused \sqformat \spriority39 TOC Heading;}
{\s21\ql \li0\ri0\sa100\widctlpar\tqr\tldot\tx9016\adjustright\rin0\lin0\itap0 \sbasedon0 \snext0 \sautoupd \sunhideused \spriority39 toc 1;}
{\s22\ql \li0\ri0\widctlpar\adjustright\rin0\lin0\itap0 \fs20 \sbasedon0 \snext22 \slink23 \ssemihidden \sunhideused footnote text;}
{\*\cs24 \additive \super \sbasedon10 \ssemihidden \sunhideused footnote reference;}
{\s25\ql \li720\ri720\sb200\sa160\widctlpar\adjustright\rin720\lin720\itap0 \i\cf1 \sbasedon0 \snext0 \slink26 \sqformat \spriority29 Quote;}}
{\*\listtable{\list\listtemplateid-1188457616\listhybrid{\listlevel\levelnfc23\levelnfcn23\leveljc0\leveljcn0\levelfollow0\levelstartat1\levelspace0\levelindent0{\leveltext\leveltemplateid68550657\'01\u-3913 ?;}{\levelnumbers;}\f3\fbias0\hres0\chhres0 \fi-360\li720\lin720 }
{\listlevel\levelnfc23\levelnfcn23\leveljc0\leveljcn0\levelfollow0\levelstartat1\levelspace0\levelindent0{\leveltext\leveltemplateid68550659\'01o;}{\levelnumbers;}\f2\fbias0\hres0\chhres0 \fi-360\li1440\lin1440 }{\listname ;}\listid1137187362}
{\list\listtemplateid-55443322\listhybrid{\listlevel\levelnfc0\levelnfcn0\leveljc0\leveljcn0\levelfollow0\levelstartat1\levelspace0\levelindent0{\leveltext\leveltemplateid68550671\'02\'00.;}{\levelnumbers\'01;}\fi-360\li720\lin720 }
{\listlevel\levelnfc4\levelnfcn4\leveljc0\leveljcn0\levelfollow0\levelstartat1\levelspace0\levelindent0{\leveltext\leveltemplateid68550681\'02\'01.;}{\levelnumbers\'01;}\fi-360\li1440\lin1440 }{\listname ;}\listid1500000000}}
{\*\listoverridetable{\listoverride\listid1137187362\listoverridecount0\ls1}{\listoverride\listid1500000000\listoverridecount0\ls2}}
{\*\rsidtbl \rsid1004563\rsid2232115}{\mmathPr\mmathFont34\mbrkBin0\mbrkBinSub0\msmallFrac0\mdispDef1\mlMargin0\mrMargin0\mdefJc1\mwrapIndent1440\mintLim0\mnaryLim1}
{\info{\title Info Title}{\subject Testing}{\author Anna K\'e2rkli\'f2a}{\operator Someone}{\keywords alpha, beta; gamma}{\doccomm A short summary}{\creatim\yr2024\mo3\dy15\hr10\min5}{\revtim\yr2024\mo3\dy16\hr9\min1}{\version2}{\edmins3}{\nofpages1}{\nofwords60}{\company Example Org}{\nofcharsws400}{\vern57}}
{\*\xmlnstbl {\xmlns1 http://example.com/ns/wordml}}
\paperw11906\paperh16838\margl1800\margr1800\margt1440\margb1440\gutter0\ltrsect
\widowctrl\ftnbj\aenddoc\trackmoves0\trackformatting1\donotembedsysfont1\relyonvml0\donotembedlingdata0\grfdocevents0\validatexml1\showplaceholdtext0\ignoremixedcontent0\saveinvalidxml0\showxmlerrors1\noxlattoyen
\expshrtn\noultrlspc\dntblnsbdb\nospaceforul\formshade\horzdoc\dgmargin\dghspace180\dgvspace180\dghorigin1800\dgvorigin1440\dghshow1\dgvshow1
\jexpand\viewkind1\viewscale100\pgbrdrhead\pgbrdrfoot\splytwnine\ftnlytwnine\htmautsp\nolnhtadjtbl\useltbaln\alntblind\lytcalctblwd\lyttblrtgr\lnbrkrule\nobrkwrptbl\snaptogridincell\allowfieldendsel\wrppunct
\asianbrkrule\rsidroot1004563 {\*\docvar {secret}{value}}{\*\wgrffmtfilter 2450}\nofeaturethrottle1\ilfomacatclean0\ltrpar \sectd \ltrsect\linex0\headery708\footery708\colsx708\endnhere\sectlinegrid360\sectdefaultcl\sftnbj
{\header \ltrpar \pard\plain \ltrpar\s27\ql \li0\ri0\widctlpar\tqc\tx4513\tqr\tx9026\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Running header\par }}
{\*\pnseclvl1\pnucrm\pnstart1\pnindent720\pnhang {\pntxta .}}{\*\pnseclvl2\pnucltr\pnstart1\pnindent720\pnhang {\pntxta .}}
\pard\plain \ltrpar\s15\ql \li0\ri0\contextualspace\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0\pararsid2232115 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \fs56\expnd-2\expndtw-10\lang1062\langfe1033\kerning28\cgrid\langnp1062\langfenp1033 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Gada p\'e2rskats}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115\charrsid2232115 \par }
\pard\plain \ltrpar\s20\ql \li0\ri0\sb240\keepn\widctlpar\wrapdefault\aspalpha\aspnum\faauto\outlinelevel9\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs32\alang1025 \ltrch\fcs0 \fs32\cf1 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Saturs\par }
\pard\plain \ltrpar\s21\ql \li0\ri0\sa100\widctlpar\tqr\tldot\tx9016\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22 {\field\fldedit{\*\fldinst {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  TOC \\o "1-3" \\h \\z \\u }}{\fldrslt {\field{\*\fldinst {\rtlch\fcs1 \af0 \ltrch\fcs0  HYPERLINK \\l "_Toc111" }}{\fldrslt {\rtlch\fcs1 \af0 \ltrch\fcs0 \cs19\ul\cf2 Ievads\tab 1}}}\par }}
\pard\plain \ltrpar\ql \li0\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \par }
\pard\plain \ltrpar\s1\ql \li0\ri0\sb240\keepn\widctlpar\wrapdefault\aspalpha\aspnum\faauto\outlinelevel0\adjustright\rin0\lin0\itap0\pararsid2232115 \rtlch\fcs1 \af0\afs32\alang1025 \ltrch\fcs0 \b\fs32\cf1\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 {\*\bkmkstart _Toc111}{\*\bkmkstart ievads}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Ievads}{\*\bkmkend _Toc111}{\*\bkmkend ievads}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \par }
\pard\plain \ltrpar\ql \li0\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \'d0is ir }{\rtlch\fcs1 \ab\af0 \ltrch\fcs0 \b\insrsid2232115 treknraksts}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  un }{\rtlch\fcs1 \ai\af0 \ltrch\fcs0 \i\insrsid2232115 sl\'eepraksts}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 , \u363\'fbdens}{\rtlch\fcs1 \af0 \ltrch\fcs0 \cs24\super\insrsid2232115 \chftn {\footnote \ltrpar \pard\plain \ltrpar\s22\ql \li0\ri0\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs20\alang1025 \ltrch\fcs0 \fs20\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \cs24\super\insrsid2232115 \chftn }{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  Zemsv\'eetras piez\'eeme.}}}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 {\v slepens} beigas.\par }
{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Saite uz }{\field\flddirty{\*\fldinst {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  HYPERLINK "https://example.com/path?q=1" \\o "Piem\'e7rs" }}{\fldrslt {\rtlch\fcs1 \af0 \ltrch\fcs0 \cs19\ul\cf2\insrsid2232115 vietni}}}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  un uz }{\field{\*\fldinst {\rtlch\fcs1 \af0 \ltrch\fcs0  REF ievads \\h }}{\fldrslt {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Ievads}}}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 . {\highlight3 Svar\'eegi}{\f3 \u-3913\'b7}\par }
\pard\plain \ltrpar\s2\ql \li0\ri0\sb40\keepn\widctlpar\outlinelevel1\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0 \ltrch\fcs0 \fs26\cf1 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Saraksti\par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f3\fs22\lang1062\langfe1033\langnp1062\insrsid2232115 \loch\af3\dbch\af0\hich\f3 \'b7\tab}\pard\plain \ltrpar\s17\ql \fi-360\li720\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\ls1\adjustright\rin0\lin720\itap0\pararsid2232115\contextualspace \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062\langfe1033\cgrid\langnp1062\langfenp1033 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Pirmais}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af2\afs22 \ltrch\fcs0 \f2\fs22\lang1062\langfe1033\langnp1062\insrsid2232115 \hich\af2\dbch\af0\loch\f2 o\tab}\pard\plain \ltrpar\s17\ql \fi-360\li1440\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\ls1\ilvl1\adjustright\rin0\lin1440\itap0\pararsid2232115\contextualspace \rtlch\fcs1 \af0\afs22\alang1025 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Ligzdots\par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f3\fs22\insrsid2232115 \loch\af3\dbch\af0\hich\f3 \'b7\tab}\pard\plain \ltrpar\s17\ql \fi-360\li720\ri0\sa160\widctlpar\ls1\adjustright\rin0\lin720\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Otrais\par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\insrsid2232115 \hich\af37\dbch\af0\loch\f37 1.\tab}\pard\plain \ltrpar\s17\ql \fi-360\li720\ri0\widctlpar\ls2\adjustright\rin0\lin720\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Viens\par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\insrsid2232115 \hich\af37\dbch\af0\loch\f37 a.\tab}\pard\plain \ltrpar\s17\ql \fi-360\li1440\ri0\widctlpar\ls2\ilvl1\adjustright\rin0\lin1440\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Apak\'f0punkts\par }
{\listtext\pard\plain\ltrpar \s17 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\insrsid2232115 \hich\af37\dbch\af0\loch\f37 2.\tab}\pard\plain \ltrpar\s17\ql \fi-360\li720\ri0\widctlpar\ls2\adjustright\rin0\lin720\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Divi\par }
\pard\plain \ltrpar\s25\ql \li720\ri720\sb200\sa160\widctlpar\adjustright\rin720\lin720\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \i\f37\fs22\cf1\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Citāts no grāmatas.\par }
\pard\plain \ltrpar\s18\ql \li0\ri0\sa200\widctlpar\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs18 \ltrch\fcs0 \i\f37\fs18\cf1\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Tabula }{\field{\*\fldinst {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115  SEQ Tabula \\* ARABIC }}{\fldrslt {\rtlch\fcs1 \af0 \ltrch\fcs0 \lang1024\langfe1024\noproof\insrsid2232115 1}}}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 : Rezult\'e2ti\par }
\trowd \irow0\irowband0\ltrrow\ts11\trgaph108\trleft-108\trhdr\trbrdrt\brdrs\brdrw10 \trbrdrl\brdrs\brdrw10 \trbrdrb\brdrs\brdrw10 \trbrdrr\brdrs\brdrw10 \trftsWidth1\trftsWidthB3\trftsWidthA3\trautofit1\trpaddl108\trpaddr108\trpaddfl3\trpaddft3\trpaddfb3\trpaddfr3\tblrsid2232115\tbllkhdrrows\tbllklastrow\tbllkhdrcols\tbllklastcol\tblind0\tblindtype3 \clvertalt\clbrdrt\brdrs\brdrw10 \clbrdrl\brdrs\brdrw10 \clbrdrb\brdrs\brdrw10 \clbrdrr\brdrs\brdrw10 \cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx2897\clvertalt\clbrdrt\brdrs\brdrw10 \cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx5902\clvertalt\cltxlrtb\clftsWidth3\clwWidth3006\clshdrawnil \cellx8908\pard\plain \ltrpar\ql \li0\ri0\widctlpar\intbl\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\pararsid2232115\yts11 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Nosaukums\cell Daudzums\cell Cena\cell }\pard\plain \ltrpar\ql \li0\ri0\sa160\sl259\slmult1\widctlpar\intbl\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \trowd \irow0\irowband0\ltrrow\ts11\trgaph108\trleft-108\trhdr\tblrsid2232115\tblind0\tblindtype3 \clvertalt\cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx2897\clvertalt\cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx5902\clvertalt\cltxlrtb\clftsWidth3\clwWidth3006\clshdrawnil \cellx8908\row }
\trowd \irow1\irowband1\ltrrow\ts11\trgaph108\trleft-108\tblrsid2232115\tblind0\tblindtype3 \clvertalt\cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx2897\clvertalt\cltxlrtb\clftsWidth3\clwWidth3005\clshdrawnil \cellx5902\clvertalt\cltxlrtb\clftsWidth3\clwWidth3006\clshdrawnil \cellx8908\pard\plain \ltrpar\ql \li0\ri0\widctlpar\intbl\wrapdefault\adjustright\rin0\lin0\yts11 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \'c2boli\cell }\pard\plain \ltrpar\qr \li0\ri0\widctlpar\intbl\adjustright\rin0\lin0\yts11 \f37\fs22 {\insrsid2232115 3\cell }\pard\plain \ltrpar\qr \li0\ri0\widctlpar\intbl\adjustright\rin0\lin0\yts11 \f37\fs22 {\insrsid2232115 1,20\cell }\pard\plain \ltrpar\ql \li0\ri0\sa160\widctlpar\intbl\adjustright\rin0\lin0 \f37\fs22 {\insrsid2232115 \trowd \irow1\irowband1\ltrrow\ts11\trgaph108\trleft-108\tblind0\tblindtype3 \cellx2897\cellx5902\cellx8908\row }
\trowd \irow2\irowband2\lastrow \ltrrow\ts11\trgaph108\trleft-108\tblind0\tblindtype3 \cellx5902\cellx8908\pard\plain \ltrpar\ql \li0\ri0\widctlpar\intbl\adjustright\rin0\lin0\yts11 \f37\fs22 {\insrsid2232115 Kop\'e2\cell }\pard\plain \ltrpar\qr \li0\ri0\widctlpar\intbl\adjustright\rin0\lin0\yts11 \f37\fs22 {\insrsid2232115 3,60\cell }\pard\plain \ltrpar\ql \li0\ri0\sa160\widctlpar\intbl\adjustright\rin0\lin0 \f37\fs22 {\insrsid2232115 \trowd \irow2\irowband2\lastrow \ltrrow\ts11\trgaph108\trleft-108\tblind0\tblindtype3 \cellx5902\cellx8908\row }
\pard\plain \ltrpar\ql \li0\ri0\sa160\sl259\slmult1\widctlpar\wrapdefault\aspalpha\aspnum\faauto\adjustright\rin0\lin0\itap0 \rtlch\fcs1 \af0\afs22 \ltrch\fcs0 \f2\fs20\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 x := 1\par }
{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \par }
{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 fmt.Println(x)\par }
\pard\plain \ltrpar\ql \li0\ri0\sa160\widctlpar\adjustright\rin0\lin0\itap0 \f37\fs22\lang1062 {\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 Beigu teksts}{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \page }{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 \par }
{\rtlch\fcs1 \af0 \ltrch\fcs0 \insrsid2232115 P\'e7c lapas p\'e2rtraukuma.\par }}`

func TestFullFeaturedDocument(t *testing.T) {
	doc, warns := read(t, fullDoc)
	m := doc.Meta
	if m.Title != "Gada pārskats" {
		t.Errorf("title = %q (Title-styled paragraph should win over \\info)", m.Title)
	}
	if m.Subject != "Testing" || m.Summary != "A short summary" || m.Organization != "Example Org" {
		t.Errorf("info meta = %+v", m)
	}
	if len(m.Authors) != 1 || m.Authors[0].Name != "Anna Kārkliņa" {
		t.Errorf("authors = %+v", m.Authors)
	}
	if !slices.Equal(m.Keywords, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("keywords = %q", m.Keywords)
	}
	if m.Date != "2024-03-15" || m.Lang != "lv" {
		t.Errorf("date/lang = %q %q", m.Date, m.Lang)
	}
	want := strings.Join([]string{
		"H1#ievads Ievads",
		"P Šis ir **treknraksts** un *slīpraksts*, ūdens^[P Zemsvītras piezīme.] beigas.",
		"P Saite uz [vietni](https://example.com/path?q=1) un uz [Ievads](#ievads). =={Svarīgi}•",
		"H2 Saraksti",
		"UL{Pirmais; UL{Ligzdots} | Otrais}",
		"OL(1,0){Viens; OL(1,1){Apakšpunkts} | Divi}",
		"Quote{P *Citāts no grāmatas.*}",
		"Table(Rezultāti)[-RR]{H[Nosaukums|Daudzums|Cena][Āboli|3|1,20][Kopā<c2>|3,60]}",
		"Code() \"x := 1\\n\\nfmt.Println(x)\"",
		"P Beigu teksts",
		"PB",
		"P Pēc lapas pārtraukuma.",
	}, "\n")
	checkDump(t, doc.Blocks, want)
	if len(warns) != 0 {
		t.Errorf("unexpected warnings: %v", warns)
	}
	tbl := findTable(doc.Blocks)
	if tbl == nil || len(tbl.Cols) != 3 || tbl.Cols[0].Width < 0.33 || tbl.Cols[0].Width > 0.34 {
		t.Errorf("column widths = %+v", tbl.Cols)
	}
	for _, b := range doc.Blocks {
		if p, ok := b.(*ast.Para); ok {
			for _, in := range p.Inlines {
				if l, ok := in.(*ast.Link); ok && l.URL == "https://example.com/path?q=1" && l.Title != "Piemērs" {
					t.Errorf("link title = %q", l.Title)
				}
			}
		}
	}
}

func findTable(blocks []ast.Block) *ast.Table {
	for _, b := range blocks {
		if t, ok := b.(*ast.Table); ok {
			return t
		}
	}
	return nil
}

func TestEncodings(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"cp1257 Baltic", `{\rtf1\ansi\ansicpg1257 \'e2\'e8\'e7\'ec\'ee\'ed\'ef\'f2\'f0\'fb\'fe\par}`, "P āčēģīķļņšūž"},
		{"cp1251 Cyrillic", `{\rtf1\ansi\ansicpg1251 \'cf\'f0\'e8\'e2\'e5\'f2\par}`, "P Привет"},
		{"cp1250 Central European", `{\rtf1\ansi\ansicpg1250 \'9a\'e8\'f8\par}`, "P ščř"},
		{"mac roman", `{\rtf1\mac \'8a\'9f\par}`, "P äü"},
		{"default 1252", `{\rtf1\ansi caf\'e9 \'93q\'94\par}`, "P café “q”"},
		{"font charset overrides doc code page", `{\rtf1\ansi\ansicpg1252{\fonttbl{\f0 Arial;}{\f1\fcharset186 Arial Baltic;}{\f2\fcharset204 Arial Cyr;}}\f0 \'e2 {\f1 \'e2} {\f2 \'e2}\par}`, "P â ā в"},
		{"unicode with uc1 fallback", `{\rtf1\ansi\uc1 \u257\'e2\u269?x\par}`, "P āčx"},
		{"uc2 skips two fallback bytes", `{\rtf1\ansi\uc2 \u12354\'82\'a0x\par}`, "P あx"},
		{"uc0 has no fallback", `{\rtf1\ansi\uc0\u8226 x\par}`, "P •x"},
		{"uc is group scoped", `{\rtf1\ansi{\uc2 \u257 ab}\u269?c\par}`, "P āčc"},
		{"negative unicode", `{\rtf1\ansi\u-3\'3f\par}`, "P \ufffd"},
		{"surrogate pair", `{\rtf1\ansi\uc1\u-10179?\u-8704?\par}`, "P 😀"},
		{"literal UTF-8 bytes", "{\\rtf1\\ansi Ā un ž\\par}", "P Ā un ž"},
		{"symbol font", `{\rtf1\ansi{\fonttbl{\f0 Times;}{\f1\fcharset2 Symbol;}}{\f1 a\'b3b}\par}`, "P α≥β"},
		{"symbol private use area", `{\rtf1\ansi{\fonttbl{\f0 Times;}{\f1\fcharset2 Symbol;}}{\f1\u-3913\'b7}\par}`, "P •"},
		{"control symbols", `{\rtf1\ansi a\~b\-c\_d \{\}\\\emdash\endash\bullet\lquote\rquote\ldblquote\rdblquote\par}`, "P a\u00a0bc\u2011d {}\\—–•‘’“”"},
		{"escaped hex in literal text", `{\rtf1\ansi\ansicpg1252 \'41\'42c\par}`, "P ABc"},
		{"shift-jis", `{\rtf1\ansi\ansicpg932 \'82\'a0\par}`, "P あ"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, tc.src)
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestInlineFormatting(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"split runs merge", `{\b a}{\b b}{\b c}\par`, "P **abc**"},
		{"nested bold italic", `{\b bold {\i both} bold}\par`, "P **bold *both* bold**"},
		{"spaces move outside wrappers", `x{\b  y }z\par`, "P x **y** z"},
		{"toggle with zero param", `\b a\b0  b\i c\i0\par`, "P **a** b*c*"},
		{"underline variants", `{\uldb a}{\ulwave b} {\ul c\ul0 d} {\ul e\ulnone f}\par`, "P _{ab} _{c}d _{e}f"},
		{"strike and double strike", `{\strike s}{\striked1 d}\par`, "P ~~sd~~"},
		{"super sub and up dn", `x{\super 2} H{\sub 2}O {\up6 a}{\dn6 b}{\super c\nosupersub d}\par`, "P x^{2} H~{2}O ^{a}~{b}^{c}d"},
		{"small caps kept, caps keeps text", `{\scaps Small} {\caps lower}\par`, "P sc{Small} lower"},
		{"plain resets formatting", `\b\i a\plain  b\par`, "P ***a*** b"},
		{"hidden text dropped", `a{\v hidden}b\par`, "P ab"},
		{"highlight", `{\highlight3 hi}\par`, "P =={hi}"},
		{"white background is no highlight", `{\cb2 white}{\cb3 yellow}\par`, "P white=={yellow}"},
		{"monospace font is code", `use {\f1 go test} now\par`, "P use `go test` now"},
		{"whitespace collapse and tabs", "a  \t b\\tab c\\par", "P a b c"},
		{"line breaks", `one\line two\par`, "P one↵two"},
		{"backslash newline ends a paragraph", "one\\\ntwo\\\n", "P one\nP two"},
		{"empty paragraphs dropped", `\par\par\par x\par\par`, "P x"},
		{"unknown ignorable destinations skipped", `{\*\madeup secret {nested} text}vis{\*\other\b hidden}ible\par`, "P visible"},
		{"unknown plain groups kept", `{\madeup shown} text\par`, "P shown text"},
		{"unicode line separator", `{\uc0 a\u8232 b}\par`, "P a↵b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, body(tc.src))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestParagraphStyles(t *testing.T) {
	sheet := `{\stylesheet{\s0 Normal;}{\s1\outlinelevel0 Virsraksts 1;}{\s2 Heading 2;}{\s3 Title;}{\s4 Subtitle;}{\s5 Quote;}{\s6 HTML Preformatted;}{\s7\sbasedon2 My Heading;}{\s8 Überschrift 3;}}`
	src := body(sheet + `\pard\s3 Main title\par\pard\s4 A subtitle\par` +
		`\pard\s1 First\par\pard\s2 Second\par\pard\s7 Custom\par\pard\s8 Third\par` +
		`\pard\s5 Quoted one\par\pard\s5 Quoted two\par` +
		`\pard\s6 code   line\par\pard\s6 \par\pard\s6 more\par` +
		`\pard\outlinelevel3 Outline four\par\pard\qc Centered\par\pard Plain\par`)
	doc, _ := read(t, src)
	if doc.Meta.Title != "Main title" || doc.Meta.Subtitle != "A subtitle" {
		t.Errorf("title/subtitle = %q / %q", doc.Meta.Title, doc.Meta.Subtitle)
	}
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 First",
		"H2 Second",
		"H2 Custom",
		"H3 Third",
		"Quote{P Quoted one; P Quoted two}",
		`Code() "code   line\n\nmore"`,
		"H4 Outline four",
		"Div.center{P Centered}",
		"P Plain",
	}, "\n"))
}

func TestLists(t *testing.T) {
	listTable := `{\*\listtable{\list{\listlevel\levelnfc0\levelstartat1}{\listlevel\levelnfc4\levelstartat1}\listid1}` +
		`{\list{\listlevel\levelnfc23}{\listlevel\levelnfc2\levelstartat1}\listid2}{\list{\listlevel\levelnfc1\levelstartat3}\listid3}{\list{\listlevel\levelnfc255}\listid4}}` +
		`{\*\listoverridetable{\listoverride\listid1\ls1}{\listoverride\listid2\ls2}{\listoverride\listid3\ls3}{\listoverride\listid4\ls4}}`
	tests := []struct {
		name, src, want string
	}{
		{"numbered with nested alpha",
			listTable + `\pard\ls1\li720 One\par\pard\ls1\ilvl1\li1440 Sub a\par\pard\ls1\ilvl1\li1440 Sub b\par\pard\ls1\li720 Two\par`,
			"OL(1,0){One; OL(1,1){Sub a | Sub b} | Two}"},
		{"bullets with nested roman",
			listTable + `\pard\ls2 A\par\pard\ls2\ilvl1 i\par\pard\ls2 B\par`,
			"UL{A; OL(1,3){i} | B}"},
		{"start at",
			listTable + `\pard\ls3 Third\par\pard\ls3 Fourth\par`,
			"OL(3,4){Third | Fourth}"},
		{"numbering continues after interruption",
			listTable + `\pard\ls1 One\par\pard Break\par\pard\ls1 Two\par`,
			"OL(1,0){One}\nP Break\nOL(2,0){Two}"},
		{"different list ids are different lists",
			listTable + `\pard\ls1 One\par\pard\ls2 Bullet\par`,
			"OL(1,0){One}\nUL{Bullet}"},
		{"level without marker is not a list",
			listTable + `\pard\ls4 Plain\par`,
			"P Plain"},
		{"continuation paragraph",
			listTable + `\pard\ls1\li720 One\par\pard\li720 More about one\par\pard\ls1\li720 Two\par`,
			"OL(1,0)loose{P One; P More about one | P Two}"},
		{"listtext number wins",
			listTable + `{\listtext 7.\tab}\pard\ls1 Seven\par`,
			"OL(7,0){Seven}"},
		{"old-style pn bullets",
			`{\pntext\f2\'b7\tab}{\*\pn\pnlvlblt\pnf2\pnindent0{\pntxtb\'b7}}\fi-720\li720 First\par{\pntext\f2\'b7\tab}Second\par\pard After\par`,
			"UL{First | Second}\nP After"},
		{"old-style pn numbers",
			`{\pntext\f0 1.\tab}{\*\pn\pnlvlbody\pnf0\pnindent0\pnstart1\pndec{\pntxta.}}\fi-360\li720 One\par{\pntext\f0 2.\tab}Two\par`,
			"OL(1,0){One | Two}"},
		{"listtext only",
			`{\listtext a)\tab}Alpha\par{\listtext b)\tab}Beta\par`,
			"OL(1,1){Alpha | Beta}"},
		{"monospace list items stay a list",
			listTable + `\pard\ls2{\f1 main.go}\par\pard\ls2{\f1 go.mod}\par`,
			"UL{`main.go` | `go.mod`}"},
		{"typed bullets",
			`\pard • Apples\par\pard • Pears\par\pard\li720 ◦ Green\par\pard • Plums\par`,
			"UL{Apples | Pears; UL{Green} | Plums}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, body(tc.src))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestTables(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"simple",
			`\trowd\cellx1000\cellx2000\pard\intbl a\cell b\cell\row\trowd\cellx1000\cellx2000\pard\intbl c\cell d\cell\row\pard after\par`,
			"Table[--]{[a|b][c|d]}\nP after"},
		{"bold first row is header",
			`\trowd\cellx1000\cellx2000\pard\intbl{\b Name}\cell{\b Qty}\cell\row\trowd\cellx1000\cellx2000\pard\intbl x\cell 1\cell\row`,
			"Table[--]{H[**Name**|**Qty**][x|1]}"},
		{"old style horizontal merge",
			`\trowd\clmgf\cellx1000\clmrg\cellx2000\cellx3000\pard\intbl wide\cell\cell c\cell\row\trowd\cellx1000\cellx2000\cellx3000\pard\intbl a\cell b\cell c\cell\row`,
			"Table[---]{[wide<c2>|c][a|b|c]}"},
		{"span from cell boundaries",
			`\trowd\cellx3000\pard\intbl all\cell\row\trowd\cellx1000\cellx2000\cellx3000\pard\intbl a\cell b\cell c\cell\row`,
			"Table[---]{[all<c3>][a|b|c]}"},
		{"vertical merge",
			`\trowd\clvmgf\cellx1000\cellx2000\pard\intbl tall\cell x\cell\row\trowd\clvmrg\cellx1000\cellx2000\pard\intbl\cell y\cell\row\trowd\clvmrg\cellx1000\cellx2000\pard\intbl\cell z\cell\row`,
			"Table[--]{[tall<r3>|x][y][z]}"},
		{"nested table",
			`\trowd\cellx4000\cellx8000\pard\intbl outer\par\pard\intbl\itap2 in1\nestcell in2\nestcell{\*\nesttableprops\trowd\cellx2000\cellx4000\nestrow}{\nonesttables\par}\pard\intbl\itap1 tail\cell right\cell\row\pard done\par`,
			"Table[--]{[P outer; Table[--]{[in1|in2]}; P tail|right]}\nP done"},
		{"missing row end is closed at end of input",
			`\trowd\cellx1000\pard\intbl only\cell`,
			"Table[-]{[only]}"},
		{"cell content more than definitions",
			`\trowd\cellx1000\pard\intbl a\cell b\cell c\cell\row`,
			"Table[---]{[a|b|c]}"},
		{"no headings or floats inside cells",
			`{\stylesheet{\s1 heading 1;}}\trowd\cellx3000\pard\intbl\s1 Head\par\pard\intbl{\pict\pngblip 89504e47}\cell\row`,
			"Table[-]{[P Head; P ![](res:rtf/image1.png)]}"},
		{"list inside cell",
			`{\*\listtable{\list{\listlevel\levelnfc23}\listid1}}{\*\listoverridetable{\listoverride\listid1\ls1}}\trowd\cellx3000\pard\intbl\ls1 one\par\pard\intbl\ls1 two\cell\row`,
			"Table[-]{[UL{one | two}]}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, body(tc.src))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestFieldsAndBookmarks(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"hyperlink", `{\field{\*\fldinst{HYPERLINK "http://a.example/x"}}{\fldrslt{\ul link}}}\par`, "P [link](http://a.example/x)"},
		{"plain inside hyperlink result", `{\field{\*\fldinst HYPERLINK "http://p.example"}{\fldrslt\plain\b bold link}}\par`, "P [**bold link**](http://p.example)"},
		{"hyperlink unquoted", `{\field{\*\fldinst HYPERLINK http://a.example }{\fldrslt site}}\par`, "P [site](http://a.example)"},
		{"internal hyperlink", `{\*\bkmkstart Target_One}{\*\bkmkend Target_One}x\par{\field{\*\fldinst HYPERLINK \\l "Target_One"}{\fldrslt go}}\par`, "P {#target_one}x\nP [go](#target_one)"},
		{"url with anchor", `{\field{\*\fldinst HYPERLINK "http://a.example/" \\l "sec"}{\fldrslt s}}\par`, "P [s](http://a.example/#sec)"},
		{"hidden bookmarks are pruned", `{\*\bkmkstart _GoBack}{\*\bkmkend _GoBack}text\par`, "P text"},
		{"referenced hidden bookmark kept", `{\*\bkmkstart _Ref1}x{\*\bkmkend _Ref1}\par{\field{\*\fldinst REF _Ref1 \\h}{\fldrslt see}}\par`, "P {#_ref1}x\nP [see](#_ref1)"},
		{"toc dropped", `{\field{\*\fldinst TOC \\o "1-3"}{\fldrslt{Intro\tab 1\par Other\tab 2\par}}}After\par`, "P After"},
		{"other fields keep result", `Page {\field{\*\fldinst PAGE}{\fldrslt 3}} of 5\par`, "P Page 3 of 5"},
		{"symbol field without result", `{\field{\*\fldinst SYMBOL 183 \\f "Symbol" \\s 10}{\fldrslt}}\par`, "P •"},
		{"linked picture url", `{\field{\*\fldinst INCLUDEPICTURE "https://img.example/p.png" \\d}{\fldrslt}}\par`, "Fig ![](https://img.example/p.png)"},
		{"field nested in instruction", `{\field{\*\fldinst HYPERLINK "{\field{\*\fldinst DOCPROPERTY Url}{\fldrslt https://n.example}}"}{\fldrslt nested}}\par`, "P [nested](https://n.example)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := read(t, body(tc.src))
			checkDump(t, doc.Blocks, tc.want)
		})
	}
}

func TestFootnotes(t *testing.T) {
	src := body(`Text{\super\chftn}{\footnote\pard\plain{\super\chftn} First para.\par Second para.}} more{\super\chftn}{\*\footnote\ftnalt\pard End note.} done.\par`)
	doc, _ := read(t, src)
	checkDump(t, doc.Blocks, "P Text^[P First para.; P Second para.] more^[P End note.] done.")
}

var pngBytes = func() []byte {
	// 1x1 transparent PNG
	b, _ := hex.DecodeString("89504e470d0a1a0a0000000d4948445200000001000000010806000000" +
		"1f15c4890000000d49444154789c6360000002000154a24f5d0000000049454e44ae426082")
	return b
}()

func TestPictures(t *testing.T) {
	hexPNG := hex.EncodeToString(pngBytes)
	t.Run("png goal size and scale", func(t *testing.T) {
		doc, _ := read(t, body(`{\pict\pngblip\picw10\pich10\picwgoal2000\pichgoal1000\picscalex50\picscaley50 `+hexPNG+`}\par`))
		checkDump(t, doc.Blocks, "Fig ![](res:rtf/image1.png 50pt×25pt)")
		r, ok := doc.Resources.Get("res:rtf/image1.png")
		if !ok || r.MediaType != "image/png" || !bytes.Equal(r.Data, pngBytes) {
			t.Errorf("resource = %+v", r)
		}
	})
	t.Run("binary data", func(t *testing.T) {
		src := body(`{\pict\pngblip\bin` + strconv.Itoa(len(pngBytes)) + ` ` + string(pngBytes) + `}\par`)
		doc, _ := read(t, src)
		r, ok := doc.Resources.Get("res:rtf/image1.png")
		if !ok || !bytes.Equal(r.Data, pngBytes) {
			t.Fatalf("binary picture not decoded")
		}
	})
	t.Run("inline picture with caption and alt", func(t *testing.T) {
		src := body(`{\stylesheet{\s0 Normal;}{\s9 caption;}}` +
			`\pard Before {\pict{\*\picprop{\sp{\sn wzDescription}{\sv A dot}}}\pngblip\picw1\pich1 ` + hexPNG + `} after\par` +
			`\pard{\pict\pngblip\picw4\pich3 ` + hexPNG + `}\par\pard\s9 Figure 2: The dot\par`)
		doc, _ := read(t, src)
		checkDump(t, doc.Blocks, "P Before ![A dot](res:rtf/image1.png 1px×1px) after\nFig ![](res:rtf/image1.png 4px×3px) cap=The dot")
	})
	t.Run("shape picture replaces fallback", func(t *testing.T) {
		src := body(`{\shp{\*\shpinst\shpleft0{\sp{\sn shapeType}{\sv 75}}{\sp{\sn wzDescription}{\sv Logo}}{\sp{\sn pib}{\sv {\pict\pngblip ` + hexPNG + `}}}}{\shprslt{\*\do\dobxcolumn}{\pict\wmetafile8 0100090000}}}\par` +
			`{\*\shppict{\pict\pngblip ` + hexPNG + `}}{\nonshppict{\pict\wmetafile8 0100}}\par`)
		doc, _ := read(t, src)
		checkDump(t, doc.Blocks, "Fig ![Logo](res:rtf/image1.png)\nFig ![](res:rtf/image1.png)")
		if doc.Resources.Len() != 1 {
			t.Errorf("duplicate image data should share one resource, got %v", doc.Resources.Names())
		}
	})
	t.Run("wmf gets placeable header", func(t *testing.T) {
		doc, _ := read(t, body(`{\pict\wmetafile8\picw2540\pich1270\picwgoal1440\pichgoal720 010009000003}\par`))
		r, ok := doc.Resources.Get("res:rtf/image1.wmf")
		if !ok || r.MediaType != "image/wmf" || binary.LittleEndian.Uint32(r.Data) != 0x9AC6CDD7 || len(r.Data) != 22+6 {
			t.Fatalf("wmf resource = %+v", r)
		}
	})
	t.Run("dib becomes bmp", func(t *testing.T) {
		dib := make([]byte, 40+8+4)
		binary.LittleEndian.PutUint32(dib, 40)
		binary.LittleEndian.PutUint32(dib[4:], 1)
		binary.LittleEndian.PutUint32(dib[8:], 1)
		binary.LittleEndian.PutUint16(dib[12:], 1)
		binary.LittleEndian.PutUint16(dib[14:], 1) // 1 bpp -> 2 palette entries
		doc, _ := read(t, body(`{\pict\dibitmap0 `+hex.EncodeToString(dib)+`}\par`))
		r, ok := doc.Resources.Get("res:rtf/image1.bmp")
		if !ok || string(r.Data[:2]) != "BM" || binary.LittleEndian.Uint32(r.Data[10:]) != 14+40+8 {
			t.Fatalf("bmp resource = %+v", r)
		}
	})
	t.Run("unsupported formats warn", func(t *testing.T) {
		doc, warns := read(t, body(`{\pict\macpict 0011}\par{{\NeXTGraphic shot.png \width100 \height100}`+"\xac"+`}\par{\object\objemb{\*\objclass Sheet}{\*\objdata 0102}}x\par`))
		checkDump(t, doc.Blocks, "P x")
		if len(warns) != 3 {
			t.Errorf("warnings = %q", warns)
		}
	})
	t.Run("ole object uses result picture", func(t *testing.T) {
		doc, warns := read(t, body(`{\object\objemb{\*\objclass Sheet.8}{\*\objdata 0102}{\result{\pict\pngblip `+hexPNG+`}}}\par`))
		checkDump(t, doc.Blocks, "Fig ![](res:rtf/image1.png)")
		if len(warns) != 0 {
			t.Errorf("warnings = %q", warns)
		}
	})
}

func TestHeadingsFromFontSize(t *testing.T) {
	// Documents from simple editors carry no styles: large or bold short
	// lines are the headings.
	src := `{\rtf1\ansi{\fonttbl{\f0 Calibri;}}\f0\fs22 ` +
		`{\fs40 Report}\par Body text one that is long enough to be body.\par ` +
		`{\fs30 Background}\par More body text here, with words.\par` +
		`{\b Details}\par Another paragraph of body text.\par Closing words of the text.\par}`
	doc, _ := read(t, src)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 Report",
		"P Body text one that is long enough to be body.",
		"H2 Background",
		"P More body text here, with words.",
		"H3 Details",
		"P Another paragraph of body text.",
		"P Closing words of the text.",
	}, "\n"))
}

func TestPageBreaks(t *testing.T) {
	doc, _ := read(t, body(`\page A\page\par\pard\pagebb B\par\pard C{\page}\page\par`))
	checkDump(t, doc.Blocks, "P A\nPB\nP B\nP C")
}

func TestTextBox(t *testing.T) {
	doc, _ := read(t, body(`Main {\shp{\*\shpinst{\sp{\sn shapeType}{\sv 202}}{\shptxt \pard Boxed text\par}}{\shprslt{\*\do\dobxcolumn{\dptxbx{\pard Boxed text\par}}}}} text\par`))
	checkDump(t, doc.Blocks, "P Main text\nP Boxed text")
}

func TestStylelessEditorDocument(t *testing.T) {
	src := `{\rtf1\ansi\ansicpg1252\cocoartf2761
\cocoatextscaling0\cocoaplatform0{\fonttbl\f0\fswiss\fcharset0 Helvetica;\f1\fswiss\fcharset0 Helvetica-Bold;\f2\fmodern\fcharset0 Menlo-Regular;}
{\colortbl;\red255\green255\blue255;\red0\green0\blue233;}
{\*\expandedcolortbl;;\cssrgb\c0\c0\c93333;}
{\*\listtable{\list\listtemplateid1\listhybrid{\listlevel\levelnfc23\levelnfcn23\leveljc0\leveljcn0\levelfollow0\levelstartat0\levelspace360\levelindent0{\*\levelmarker \{disc\}}{\leveltext\leveltemplateid1\'01\uc0\u8226 ;}{\levelnumbers;}\fi-360\li720\lin720 }{\listname ;}\listid1}}
{\*\listoverridetable{\listoverride\listid1\listoverridecount0\ls1}}
\paperw11900\paperh16840\margl1440\margr1440\vieww11520\viewh8400\viewkind0
\pard\tx566\tx1133\tx1700\pardirnatural\partightenfactor0

\f1\b\fs36 \cf0 Shopping\

\f0\b0\fs24 Things to buy:\
\pard\tx220\tx720\pardeftab720\li720\fi-720\partightenfactor0
\ls1\ilvl0\cf0 {\listtext	\uc0\u8226 	}Milk\
{\listtext	\uc0\u8226 	}Bread\
\pard\tx566\pardirnatural\partightenfactor0
\cf0 Visit {\field{\*\fldinst{HYPERLINK "https://shop.example/"}}{\fldrslt \cf2 \ul \ulc2 the shop}} or run 
\f2 open -a Notes
\f0 .\
\
{{\NeXTGraphic photo.jpeg \width2000 \height1500 \appleattachmentpadding0 \appleembedtype0 \appleaqc
}\'ac}}`
	doc, warns := read(t, src)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 Shopping",
		"P Things to buy:",
		"UL{Milk | Bread}",
		"P Visit [the shop](https://shop.example/) or run `open -a Notes`.",
	}, "\n"))
	if len(warns) != 1 || !strings.Contains(warns[0], "photo.jpeg") {
		t.Errorf("warnings = %q", warns)
	}
}

func TestMalformedInputNeverPanics(t *testing.T) {
	inputs := []string{
		`{\rtf1`,
		`{\rtf1 unbalanced}}}}} text\par`,
		`{\rtf1 {{{{{ deep`,
		`{\rtf1 truncated hex \'`,
		`{\rtf1 bad hex \'zz ok\par}`,
		`{\rtf1 \u`,
		`{\rtf1 \u99999999999999999999 x}`,
		`{\rtf1 \bin99999999 abc}`,
		`{\rtf1 \bin-5 abc}`,
		`{\rtf1 {\pict\pngblip 89504e47 0d0a1a0}}`,
		`{\rtf1 \cell\row\nestcell\nestrow\cell}`,
		`{\rtf1 \trowd\cellx-5\cellx-10\pard\intbl a\cell\row}`,
		`{\rtf1 {\field{\*\fldinst HYPERLINK "unterminated}{\fldrslt x}}}`,
		`{\rtf1 {\footnote {\footnote nested}}}`,
		`{\rtf1 \ls999\ilvl99 x\par\itap99 \intbl y\cell}`,
		`{\rtf1 {\*\shppict}{\shp{\sv}{\sn}{\sp}}}`,
		`{\rtf1 {\upr{\*\ud}}}`,
		`{\rtf1 \abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz123 x}`,
		`{\rtf1 \'e2\'e8\uc5\u257\'e2`,
		"{\\rtf1 \x00\x01\x02 binary \xff\xfe junk}",
		`{\rtf1 {\*\listtable{\list{\listlevel}{\listlevel\levelnfc999}}}\ls1 x\par}`,
		`{\rtf1 {\stylesheet{\s1\sbasedon1 Loop;}{\s2\sbasedon3 A;}{\s3\sbasedon2 B;}}\s2 x\par\s1 y\par}`,
		`{\rtf1 {\pict\dibitmap ffffffff}{\pict\wmetafile8 00}{\pict\emfblip 00}}`,
		`{\rtf1 ` + strings.Repeat("{", 100000) + `x`,
		`{\rtf1 ` + strings.Repeat(`{\b x}`, 10000) + `}`,
	}
	for i, in := range inputs {
		doc, _, err := Read(context.Background(), []byte(in), rd.Options{})
		if err != nil && !strings.Contains(in, `{\rtf`) {
			continue
		}
		if err != nil {
			t.Errorf("input %d: unexpected error %v", i, err)
			continue
		}
		if doc == nil || doc.Resources == nil {
			t.Errorf("input %d: nil document", i)
		}
	}
}

func TestNotRTF(t *testing.T) {
	if _, _, err := Read(context.Background(), []byte("plain text"), rd.Options{}); err == nil {
		t.Fatal("expected an error for non-RTF input")
	}
	// Leading garbage before the header is tolerated.
	doc, _, err := Read(context.Background(), []byte("\xef\xbb\xbf  {\\rtf1 ok\\par}"), rd.Options{})
	if err != nil || dump(doc.Blocks) != "P ok" {
		t.Fatalf("doc = %v, err = %v", doc, err)
	}
}

func TestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := `{\rtf1 ` + strings.Repeat(`x\par `, 10000) + `}`
	_, _, err := Read(ctx, []byte(src), rd.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestSanitizeID(t *testing.T) {
	tests := map[string]string{
		"Intro":           "intro",
		"_Toc12345":       "_toc12345",
		"Sec 2.1 (draft)": "sec-2.1-draft",
		"Ievads_ā":        "ievads_",
		"  ":              "",
	}
	for in, want := range tests {
		if got := sanitizeID(in); got != want {
			t.Errorf("sanitizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

// Pathological but valid inputs must stay linear (both once were not).
func TestLargeInputsStayFast(t *testing.T) {
	if testing.Short() {
		t.Skip("large inputs")
	}
	inputs := map[string]string{
		"cells in one row": `{\rtf1 \trowd\pard\intbl ` + strings.Repeat(`a\cell `, 200000) + `\row}`,
		"code lines":       `{\rtf1{\fonttbl{\f1 Courier;}}\f1 ` + strings.Repeat(`x = 1\par `, 200000) + `}`,
	}
	for name, src := range inputs {
		start := time.Now()
		read(t, src)
		if d := time.Since(start); d > 10*time.Second {
			t.Errorf("%s took %v", name, d)
		}
	}
}

func FuzzRead(f *testing.F) {
	f.Add([]byte(fullDoc))
	f.Add([]byte(`{\rtf1{\field{\*\fldinst HYPERLINK "x"}{\fldrslt y}}\trowd\cellx1\intbl a\cell\row}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, _, err := Read(context.Background(), data, rd.Options{})
		if err == nil && (doc == nil || doc.Resources == nil) {
			t.Fatal("nil document without error")
		}
	})
}

// A document in the shape office suites write: \loch/\hich noise, paragraph
// ends written after the next run group, named styles.
const suiteDoc = `{\rtf1\ansi\deff4\adeflang1025
{\fonttbl{\f0\froman\fprq2\fcharset0 Times New Roman;}{\f1\froman\fprq2\fcharset2 Symbol;}{\f4\fswiss\fprq2\fcharset0 Liberation Sans{\*\falt Arial};}{\f5\fmodern\fprq1\fcharset0 Liberation Mono{\*\falt Courier New};}{\f6\fnil\fprq2\fcharset0 Noto Sans CJK SC;}}
{\colortbl;\red0\green0\blue0;\red0\green0\blue128;}
{\stylesheet{\s0\snext0\dbch\af6\langfe2052\afs24\alang1081\loch\f4\hich\af4\fs24\lang1033 Normal;}
{\*\cs15\snext15 Footnote Characters;}
{\*\cs16\snext16\cf2\ul\ulc0\langfe255\alang255\lang255 Internet Link;}
{\s1\sbasedon17\snext18\ilvl0\outlinelevel0\sb240\sa120\keepn\dbch\af6\afs36\ab\loch\f4\fs36\b Heading 1;}
{\s17\sbasedon0\snext18\sb240\sa120\keepn\dbch\af6\afs28\loch\f4\fs28 Heading;}
{\s18\sbasedon0\snext18\sl276\slmult1\sb0\sa140 Text Body;}
{\s20\sbasedon0\snext20\sb120\sa120\noline\i\afs24\ai\fs24 Caption;}
{\s27\sbasedon0\snext27\sb0\sa0\loch\f5\fs20 Preformatted Text;}
{\s28\sbasedon0\snext28\noline Table Contents;}
{\s29\sbasedon28\snext29\qc\noline\ab\b Table Heading;}
}{\*\generator Writer/1.0}{\info{\author Jane Roe}{\creatim\yr2024\mo1\dy2\hr3\min4}{\revtim\yr0\mo0\dy0\hr0\min0}{\printim\yr0\mo0\dy0\hr0\min0}}{\*\userprops}\deftab709
\hyphauto1\viewscale100\formshade\paperh16838\paperw11906\margl1134\margr1134\margt1134\margb1134\sectd\sbknone\sftnnar\saftnnrlc\sectunlocked1\pgwsxn11906\pghsxn16838\marglsxn1134\margrsxn1134\margtsxn1134\margbsxn1134\ftnbj\ftnstart1\ftnrstcont\ftnnar\aenddoc\aftnrstcont\aftnstart1\aftnnrlc
{\*\ftnsep\chftnsep}\pgndec\pard\plain \s1\ilvl0\outlinelevel0\sb240\sa120\keepn\dbch\af6\afs36\ab\loch\f4\fs36\b\rtlch \ab \ltrch\loch
Overview
\par \pard\plain \s18\sl276\slmult1\sb0\sa140{\loch
Body with }{\b\ab\rtlch \ltrch\loch
bold}{\loch
 and }{\cs16\cf2\ul\ulc0\langfe255\alang255\lang255{\field{\*\fldinst HYPERLINK "https://example.org/" }{\fldrslt {\cs16\cf2\ul\ulc0\langfe255\alang255\lang255\loch
a link}}}}{\loch
.}{\super\chftn}{\*\footnote\pard\plain \s23\sl276\slmult1\ql\fi-339\li339 {\super\chftn}\tab {\loch
The note.}}
\par \trowd\trql\trleft0\ltrrow\trpaddft3\trpaddt0\trpaddfl3\trpaddl0\trpaddfb3\trpaddb0\trpaddfr3\trpaddr0\clbrdrt\brdrs\brdrw10\brdrcf1\clbrdrl\brdrs\brdrw10\brdrcf1\clbrdrb\brdrs\brdrw10\brdrcf1\cellx4818\clbrdrt\brdrs\brdrw10\brdrcf1\clbrdrl\brdrs\brdrw10\brdrcf1\clbrdrb\brdrs\brdrw10\brdrcf1\clbrdrr\brdrs\brdrw10\brdrcf1\cellx9637\pard\plain \s29\intbl\qc\noline\ab\b{\loch
Key}\cell\pard\plain \s29\intbl\qc\noline\ab\b{\loch
Value}\cell\row\pard\trowd\trql\trleft0\ltrrow\cellx4818\cellx9637\pard\plain \s28\intbl\noline{\loch
a}\cell\pard\plain \s28\intbl\noline{\loch
1}\cell\row\pard\pard\plain \s20\sb120\sa120\noline\i\afs24\ai\fs24{\loch
Table 1: Values}
\par \pard\plain \s27\sb0\sa0\loch\f5\fs20{\loch
if x \{}
\par \pard\plain \s27\sb0\sa0\loch\f5\fs20{\loch
    y()}
\par \pard\plain \s27\sb0\sa0\loch\f5\fs20{\loch
\}}
\par }`

func TestSuiteDocument(t *testing.T) {
	doc, warns := read(t, suiteDoc)
	checkDump(t, doc.Blocks, strings.Join([]string{
		"H1 Overview",
		"P Body with **bold** and [a link](https://example.org/).^[P The note.]",
		"Table(Values)[--]{H[**Key**|**Value**][a|1]}",
		`Code() "if x {\n    y()\n}"`,
	}, "\n"))
	if len(doc.Meta.Authors) != 1 || doc.Meta.Authors[0].Name != "Jane Roe" || doc.Meta.Date != "2024-01-02" || doc.Meta.Lang != "en-US" {
		t.Errorf("meta = %+v", doc.Meta)
	}
	if len(warns) != 0 {
		t.Errorf("warnings = %q", warns)
	}
}

func TestUnicodeAlternatives(t *testing.T) {
	src := `{\rtf1\ansi{\info{\upr{\title Ansi title}{\*\ud{\title Unicode title \u257}}}}{\upr{ansi body}{\*\ud{unicode body}}}\par}`
	doc, _ := read(t, src)
	if doc.Meta.Title != "Unicode title ā" {
		t.Errorf("title = %q", doc.Meta.Title)
	}
	checkDump(t, doc.Blocks, "P unicode body")
}

func TestUTF8CodePage(t *testing.T) {
	doc, _ := read(t, `{\rtf1\ansi\ansicpg65001 \'c4\'81\'c5\'a1\par}`)
	checkDump(t, doc.Blocks, "P āš")
}
