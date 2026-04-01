package main

// getStyleTemplate returns the complete LaTeX template for the given style name.
// All templates use << >> as Go template delimiters to avoid collision with LaTeX braces.
func getStyleTemplate(style string) string {
	switch style {
	case "ligums":
		return ligumsTemplate
	case "legal":
		return legalTemplate
	case "technical":
		return technicalTemplate
	case "minimal":
		return minimalTemplate
	case "letter":
		return letterTemplate
	case "academic":
		return academicTemplate
	case "invoice":
		return invoiceTemplate
	case "memo":
		return memoTemplate
	default:
		return reportTemplate
	}
}

// ============================================================================
// Shared LaTeX preamble components (no template actions, pure LaTeX)
// ============================================================================

const sharedFontSetup = `%% === Engine & Font Setup (Full UTF-8 / Multilingual) ===
\usepackage{fontspec}
\usepackage{amssymb}    %% must load before unicode-math to avoid \eth clash
\usepackage{unicode-math}

%% Full Unicode support — handles Latvian (āčēģīķļņšūž), German (äöüß),
%% French (àâéèêëïôùûüÿç), Polish, Czech, Nordic, and more.
%% LuaLaTeX/XeLaTeX with fontspec provides native Unicode — no babel/inputenc needed.
\defaultfontfeatures{Ligatures=TeX, Scale=MatchLowercase}
`

const sharedSerifFonts = `%% Premium serif font stack
\IfFontExistsTF{EB Garamond}{
  \setmainfont{EB Garamond}[Numbers=OldStyle, Ligatures=TeX]
}{
  \IfFontExistsTF{Palatino}{
    \setmainfont{Palatino}[Ligatures=TeX]
  }{
    \setmainfont{Latin Modern Roman}[Ligatures=TeX]
  }
}
`

const sharedSansFonts = `%% Sans-serif font stack
\IfFontExistsTF{Inter}{
  \setsansfont{Inter}[Scale=0.92]
}{
  \IfFontExistsTF{Helvetica Neue}{
    \setsansfont{Helvetica Neue}[Scale=0.92]
  }{
    \setsansfont{Latin Modern Sans}[Scale=0.92]
  }
}
`

const sharedMonoFonts = `%% Monospace font stack
\IfFontExistsTF{JetBrains Mono}{
  \setmonofont{JetBrains Mono}[Scale=0.85]
}{
  \IfFontExistsTF{Menlo}{
    \setmonofont{Menlo}[Scale=0.85]
  }{
    \setmonofont{Latin Modern Mono}[Scale=0.85]
  }
}
`

const sharedPackages = `%% === Typography ===
\usepackage{microtype}
\usepackage{setspace}
\usepackage{parskip}

%% === Lists ===
\usepackage{enumitem}
\setlist{nosep, leftmargin=1.5em}
%% amssymb loaded in font setup (before unicode-math)

%% === Tables ===
\usepackage{tabularx}
\usepackage{booktabs}
\usepackage{longtable}

%% === Graphics ===
\usepackage{graphicx}
\usepackage[export]{adjustbox}

%% === Code Listings ===
\usepackage{listings}
\lstdefinestyle{codestyle}{
  basicstyle=\ttfamily\small,
  backgroundcolor=\color{codebg},
  frame=single,
  framerule=0pt,
  framesep=8pt,
  rulecolor=\color{codebg},
  breaklines=true,
  breakatwhitespace=false,
  tabsize=2,
  showstringspaces=false,
  numbers=left,
  numberstyle=\tiny\color{medgray},
  numbersep=8pt,
  xleftmargin=16pt,
  aboveskip=1em,
  belowskip=1em,
  keywordstyle=\color{codekey}\bfseries,
  stringstyle=\color{codestring},
  commentstyle=\color{codecomment}\itshape,
}
\lstset{style=codestyle}

%% === Blockquote Environment ===
\usepackage{mdframed}
\newmdenv[
  topline=false,
  bottomline=false,
  rightline=false,
  leftline=true,
  linewidth=3pt,
  linecolor=quotecolor,
  backgroundcolor=quotebg,
  innerleftmargin=12pt,
  innerrightmargin=12pt,
  innertopmargin=8pt,
  innerbottommargin=8pt,
  skipabove=1em,
  skipbelow=1em,
]{quotebox}

%% === Math ===
\usepackage{amsmath}

%% === Table of Contents ===
\usepackage{tocloft}
\renewcommand{\cftsecleader}{\cftdotfill{\cftdotsep}}
\setlength{\cftbeforesecskip}{0.4em}

%% === Links ===
\usepackage{url}
\usepackage{hyperref}
`

// ============================================================================
// LIGUMS STYLE — Latvian agreements, monotone, no English, unnumbered sections
// ============================================================================

const ligumsTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "3cm" >>, bottom=<< or .MarginBottom "3cm" >>,
  left=<< or .MarginLeft "3cm" >>, right=<< or .MarginRight "3cm" >>,
  headheight=14pt, headsep=1.2cm, footskip=1.2cm
]{geometry}

` + sharedPackages + `

\setstretch{1.25}

%% === Colors (Monotone — serious business) ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{111111}
\definecolor{rulecolor}{HTML}{333333}
\definecolor{accentcolor}{HTML}{444444}
\definecolor{lightgray}{HTML}{f5f5f5}
\definecolor{medgray}{HTML}{666666}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{d4a017}
\definecolor{statusblue}{HTML}{2980b9}
\definecolor{codebg}{HTML}{f8f8f8}
\definecolor{codekey}{HTML}{333333}
\definecolor{codestring}{HTML}{444444}
\definecolor{codecomment}{HTML}{888888}
\definecolor{quotecolor}{HTML}{333333}
\definecolor{quotebg}{HTML}{f5f5f5}

%% === Section Formatting (unnumbered — document uses own numbering) ===
\usepackage{titlesec}
\setcounter{secnumdepth}{0}

\titleformat{\section}
  {\Large\bfseries\sffamily\color{headingcolor}}
  {}{0em}{}
  [\vspace{-0.3em}\textcolor{rulecolor}{\rule{\textwidth}{0.5pt}}]

\titleformat{\subsection}
  {\large\bfseries\sffamily\color{headingcolor}}
  {}{0em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\sffamily\color{headingcolor}}
  {}{0em}{}

\titlespacing*{\section}{0pt}{1.8em}{0.8em}
\titlespacing*{\subsection}{0pt}{1.4em}{0.5em}
\titlespacing*{\subsubsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\sffamily\color{medgray}<< or .HeaderLeft (escapeLaTeX .Title) >>}
\fancyhead[R]{\small\sffamily\color{medgray}<< or .HeaderRight "" >>}
\fancyfoot[C]{%
  \textcolor{rulecolor}{\rule{2cm}{0.3pt}}\\[3pt]
  \small\sffamily\color{medgray}Lapa \thepage\ no \pageref{LastPage}
}

\renewcommand{\cftsecfont}{\sffamily}
\renewcommand{\cftsubsecfont}{\sffamily\small}
\renewcommand{\cftsecpagefont}{\sffamily}
\renewcommand{\cftsubsecpagefont}{\sffamily\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=accentcolor,
  pdfauthor={<< escapeLaTeX .Author >>},
  pdftitle={<< escapeLaTeX .Title >>},
}

\begin{document}

<< if not .NoTitlePage >>
%% TITLE PAGE — Clean, minimal, Latvian
\begin{titlepage}
\newgeometry{top=3cm, bottom=3cm, left=3.5cm, right=3.5cm}

\vspace*{4cm}

\begin{center}

{\fontsize{28}{34}\selectfont\bfseries\sffamily\color{headingcolor}
<< escapeLaTeX .Title >>}

<< if hasContent .Subtitle >>
\vspace{0.8cm}
{\Large\sffamily\color{accentcolor}<< escapeLaTeX .Subtitle >>}
<< end >>

\vspace{1.5cm}
{\textcolor{rulecolor}{\rule{5cm}{0.5pt}}}

<< if hasContent .Summary >>
\vspace{1.2cm}
\begin{minipage}{0.8\textwidth}
\centering
{\normalsize\color{accentcolor}\itshape
<< escapeLaTeX .Summary >>}
\end{minipage}
<< end >>

<< if hasContent .Date >>
\vspace{2cm}
{\normalsize\sffamily\color{medgray}<< escapeLaTeX .Date >>}
<< end >>

\end{center}

\vfill

<< if hasContent .Author >>
\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.3pt}}
\vspace{0.4cm}
\begin{center}
{\small\sffamily\color{medgray}<< escapeLaTeX .Author >>}
\end{center}
<< end >>

\restoregeometry
\end{titlepage}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\newpage
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// LEGAL STYLE
// ============================================================================

const legalTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "3.2cm" >>, bottom=<< or .MarginBottom "3.2cm" >>,
  left=<< or .MarginLeft "3cm" >>, right=<< or .MarginRight "3cm" >>,
  headheight=14pt, headsep=1.5cm, footskip=1.5cm
]{geometry}

` + sharedPackages + `

\setstretch{1.25}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{1a1a2e}
\definecolor{rulecolor}{HTML}{c9a84c}
\definecolor{accentcolor}{HTML}{2d3436}
\definecolor{lightgray}{HTML}{f5f5f5}
\definecolor{medgray}{HTML}{888888}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{d4a017}
\definecolor{statusblue}{HTML}{2980b9}
\definecolor{codebg}{HTML}{f8f8f8}
\definecolor{codekey}{HTML}{0550ae}
\definecolor{codestring}{HTML}{0a3069}
\definecolor{codecomment}{HTML}{6a737d}
\definecolor{quotecolor}{HTML}{c9a84c}
\definecolor{quotebg}{HTML}{fdf8ed}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\Large\bfseries\sffamily\color{headingcolor}}
  {\thesection.}{0.6em}{}
  [\vspace{-0.3em}\textcolor{rulecolor}{\rule{\textwidth}{0.6pt}}]

\titleformat{\subsection}
  {\large\bfseries\sffamily\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\sffamily\color{headingcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{1.8em}{0.8em}
\titlespacing*{\subsection}{0pt}{1.4em}{0.5em}
\titlespacing*{\subsubsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\sffamily\color{medgray}<< or .HeaderLeft (escapeLaTeX .Title) >>}
\fancyhead[R]{\small\sffamily\color{medgray}<< or .HeaderRight (classIcon .Classification) >>}
\fancyfoot[L]{\footnotesize\sffamily\color{medgray}<< or .FooterLeft (escapeLaTeX .Status) >>}
\fancyfoot[C]{%
  \textcolor{rulecolor}{\rule{2cm}{0.3pt}}\\[3pt]
  \small\sffamily\color{medgray}Page \thepage\ of \pageref{LastPage}
}
\fancyfoot[R]{\footnotesize\sffamily\color{medgray}<< or .FooterRight "" >>}

\renewcommand{\cftsecfont}{\sffamily}
\renewcommand{\cftsubsecfont}{\sffamily\small}
\renewcommand{\cftsecpagefont}{\sffamily}
\renewcommand{\cftsubsecpagefont}{\sffamily\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=accentcolor,
  pdfauthor={<< escapeLaTeX .Author >>},
  pdftitle={<< escapeLaTeX .Title >>},
}

\begin{document}

<< if not .NoTitlePage >>
%% TITLE PAGE
\begin{titlepage}
\newgeometry{top=3cm, bottom=3cm, left=3.5cm, right=3.5cm}

\noindent
\begin{minipage}[t]{0.5\textwidth}
\vspace{0pt}
<< if .Logo >>\includegraphics[height=1.2cm]{<< .Logo >>}<< else >>{\small\sffamily\color{medgray}<< if .Author >><< escapeLaTeX .Author >><< else >><< end >>}<< end >>
\end{minipage}%
\hfill
\begin{minipage}[t]{0.4\textwidth}
\vspace{0pt}
\raggedleft
{\footnotesize\sffamily
\colorbox{<< statusColor .Status >>!15}{\color{<< statusColor .Status >>}\textbf{\,<< escapeLaTeX .Status >>\,}}}
\end{minipage}

\vspace{0.5cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.8pt}}

\vspace{3cm}

\begin{center}

{\fontsize{32}{38}\selectfont\bfseries\sffamily\color{headingcolor}
<< escapeLaTeX .Title >>}

<< if hasContent .Subtitle >>
\vspace{0.8cm}
{\Large\sffamily\color{accentcolor}<< escapeLaTeX .Subtitle >>}
<< end >>

\vspace{1.5cm}
{\textcolor{rulecolor}{\rule{5cm}{0.6pt}}}

<< if hasContent .Summary >>
\vspace{1.2cm}
\begin{minipage}{0.8\textwidth}
\centering
{\normalsize\color{accentcolor}\itshape
<< escapeLaTeX .Summary >>}
\end{minipage}
<< end >>

\end{center}

\vfill

\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}
\vspace{0.6cm}

\noindent
\begin{minipage}[t]{0.48\textwidth}
{\small\sffamily
\textcolor{medgray}{Version:} \textbf{<< escapeLaTeX .Version >>}\\[0.3em]
\textcolor{medgray}{Status:} \textbf{\textcolor{<< statusColor .Status >>}{<< escapeLaTeX .Status >>}}\\[0.3em]
\textcolor{medgray}{Classification:} \textbf{<< classIcon .Classification >>}
}
\end{minipage}%
\hfill
\begin{minipage}[t]{0.48\textwidth}
\raggedleft
{\small\sffamily
<< if hasContent .Date >>
\textcolor{medgray}{Date:} \textbf{<< escapeLaTeX .Date >>}\\[0.3em]
<< end >>
\textcolor{medgray}{Generated:} \textbf{<< escapeLaTeX .GeneratedDate >>}\\[0.3em]
<< if hasContent .Author >>\textcolor{medgray}{Author:} \textbf{<< escapeLaTeX .Author >>}\\[0.3em]<< end >>
\textcolor{medgray}{Pages:} \textbf{\pageref{LastPage}}
}
\end{minipage}

\vspace{0.6cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}

\restoregeometry
\end{titlepage}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\newpage
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

<< if .HasSignatures >>
%% SIGNATURE BLOCK
\vspace{2cm}

\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}

\vspace{0.3cm}
\begin{center}
{\small\sffamily\color{medgray}\textit{IN WITNESS WHEREOF, the Parties have executed this document as of the date set forth below.}}
\end{center}
\vspace{1cm}

\noindent
\begin{minipage}[t]{0.45\textwidth}
{\sffamily\textbf{\color{headingcolor}Party A}}\\[2cm]
\rule{6cm}{0.4pt}\\[0.4em]
{\small\sffamily Name:} \rule{4cm}{0.2pt}\\[0.4em]
{\small\sffamily Title:} \rule{4.1cm}{0.2pt}\\[0.4em]
{\small\sffamily Date:} \rule{4.15cm}{0.2pt}
\end{minipage}%
\hfill
\begin{minipage}[t]{0.45\textwidth}
{\sffamily\textbf{\color{headingcolor}Party B}}\\[2cm]
\rule{6cm}{0.4pt}\\[0.4em]
{\small\sffamily Name:} \rule{4cm}{0.2pt}\\[0.4em]
{\small\sffamily Title:} \rule{4.1cm}{0.2pt}\\[0.4em]
{\small\sffamily Date:} \rule{4.15cm}{0.2pt}
\end{minipage}
<< end >>

\end{document}
`

// ============================================================================
// TECHNICAL STYLE
// ============================================================================

const technicalTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + `
%% Technical style: sans-serif primary
\IfFontExistsTF{Inter}{
  \setmainfont{Inter}[Scale=0.95, Ligatures=TeX]
}{
  \IfFontExistsTF{Helvetica Neue}{
    \setmainfont{Helvetica Neue}[Ligatures=TeX]
  }{
    \setmainfont{Latin Modern Sans}[Ligatures=TeX]
  }
}
` + sharedSansFonts + `
\IfFontExistsTF{JetBrains Mono}{
  \setmonofont{JetBrains Mono}[Scale=0.88]
}{
  \IfFontExistsTF{Menlo}{
    \setmonofont{Menlo}[Scale=0.88]
  }{
    \setmonofont{Latin Modern Mono}[Scale=0.88]
  }
}

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "2.5cm" >>, bottom=<< or .MarginBottom "2.5cm" >>,
  left=<< or .MarginLeft "2cm" >>, right=<< or .MarginRight "2cm" >>,
  headheight=14pt, headsep=1cm, footskip=1.2cm
]{geometry}

` + sharedPackages + `

\setstretch{1.2}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{24292e}
\definecolor{rulecolor}{HTML}{0366d6}
\definecolor{accentcolor}{HTML}{586069}
\definecolor{lightgray}{HTML}{f6f8fa}
\definecolor{medgray}{HTML}{6a737d}
\definecolor{statusgreen}{HTML}{28a745}
\definecolor{statusamber}{HTML}{dbab09}
\definecolor{statusblue}{HTML}{0366d6}
\definecolor{codebg}{HTML}{f6f8fa}
\definecolor{codekey}{HTML}{d73a49}
\definecolor{codestring}{HTML}{032f62}
\definecolor{codecomment}{HTML}{6a737d}
\definecolor{quotecolor}{HTML}{0366d6}
\definecolor{quotebg}{HTML}{f1f8ff}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\Large\bfseries\color{headingcolor}}
  {\thesection}{0.6em}{}
  [\vspace{-0.3em}\textcolor{rulecolor}{\rule{\textwidth}{0.5pt}}]

\titleformat{\subsection}
  {\large\bfseries\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\color{headingcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{1.5em}{0.6em}
\titlespacing*{\subsection}{0pt}{1.2em}{0.4em}
\titlespacing*{\subsubsection}{0pt}{0.8em}{0.3em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0.4pt}
\renewcommand{\headrule}{\hbox to\headwidth{\color{lightgray}\leaders\hrule height \headrulewidth\hfill}}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\color{medgray}<< or .HeaderLeft (escapeLaTeX .Title) >>}
\fancyhead[R]{\small\color{medgray}v<< escapeLaTeX .Version >>}
\fancyfoot[C]{\small\color{medgray}Page \thepage\ of \pageref{LastPage}}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsubsecfont}{\small}
\renewcommand{\cftsecpagefont}{\bfseries}
\renewcommand{\cftsubsecpagefont}{\small}

\hypersetup{
  colorlinks=true,
  linkcolor=rulecolor,
  urlcolor=rulecolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

<< if not .NoTitlePage >>
%% TITLE PAGE
\begin{titlepage}
\newgeometry{top=4cm, bottom=3cm, left=3cm, right=3cm}

<< if .Logo >>\noindent\includegraphics[height=1.5cm]{<< .Logo >>}\vspace{1cm}<< end >>

\vspace{2cm}

{\fontsize{28}{34}\selectfont\bfseries\color{headingcolor}
<< escapeLaTeX .Title >>\par}

<< if hasContent .Subtitle >>
\vspace{0.6cm}
{\Large\color{accentcolor}<< escapeLaTeX .Subtitle >>\par}
<< end >>

\vspace{1cm}
\noindent\textcolor{rulecolor}{\rule{4cm}{2pt}}

<< if hasContent .Summary >>
\vspace{1.5cm}
\begin{minipage}{0.85\textwidth}
{\color{accentcolor}<< escapeLaTeX .Summary >>}
\end{minipage}
<< end >>

\vfill

{\small\color{medgray}
<< if hasContent .Author >><< escapeLaTeX .Author >>\\[0.3em]<< end >>
<< if hasContent .Date >><< escapeLaTeX .Date >>\\[0.3em]<< end >>
Version << escapeLaTeX .Version >>
}

\restoregeometry
\end{titlepage}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\newpage
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// REPORT STYLE
// ============================================================================

const reportTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "2.8cm" >>, bottom=<< or .MarginBottom "2.8cm" >>,
  left=<< or .MarginLeft "2.5cm" >>, right=<< or .MarginRight "2.5cm" >>,
  headheight=14pt, headsep=1.2cm, footskip=1.2cm
]{geometry}

` + sharedPackages + `

\setstretch{1.3}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{2c3e50}
\definecolor{rulecolor}{HTML}{2c3e50}
\definecolor{accentcolor}{HTML}{34495e}
\definecolor{lightgray}{HTML}{ecf0f1}
\definecolor{medgray}{HTML}{7f8c8d}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{f39c12}
\definecolor{statusblue}{HTML}{2980b9}
\definecolor{codebg}{HTML}{f9f9f9}
\definecolor{codekey}{HTML}{8e44ad}
\definecolor{codestring}{HTML}{27ae60}
\definecolor{codecomment}{HTML}{95a5a6}
\definecolor{quotecolor}{HTML}{2c3e50}
\definecolor{quotebg}{HTML}{f4f6f7}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\Large\bfseries\sffamily\color{headingcolor}}
  {\thesection}{0.6em}{}

\titleformat{\subsection}
  {\large\bfseries\sffamily\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\sffamily\color{headingcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{1.8em}{0.8em}
\titlespacing*{\subsection}{0pt}{1.4em}{0.5em}
\titlespacing*{\subsubsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0.4pt}
\renewcommand{\headrule}{\hbox to\headwidth{\color{rulecolor}\leaders\hrule height \headrulewidth\hfill}}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\sffamily\color{medgray}<< or .HeaderLeft (escapeLaTeX .Title) >>}
\fancyhead[R]{\small\sffamily\color{medgray}<< or .HeaderRight (escapeLaTeX .Date) >>}
\fancyfoot[C]{\small\sffamily\color{medgray}Page \thepage\ of \pageref{LastPage}}

\renewcommand{\cftsecfont}{\sffamily\bfseries}
\renewcommand{\cftsubsecfont}{\sffamily\small}
\renewcommand{\cftsecpagefont}{\sffamily\bfseries}
\renewcommand{\cftsubsecpagefont}{\sffamily\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=rulecolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

<< if not .NoTitlePage >>
%% COVER PAGE
\begin{titlepage}
\newgeometry{top=0cm, bottom=3cm, left=0cm, right=0cm}

%% Dark header band
\noindent\colorbox{headingcolor}{%
\begin{minipage}[c][8cm][c]{\paperwidth}
\hspace{3cm}
\begin{minipage}{\dimexpr\textwidth-6cm}
\color{white}
<< if .Logo >>\includegraphics[height=1.5cm]{<< .Logo >>}\\[1cm]<< end >>
{\fontsize{30}{36}\selectfont\bfseries << escapeLaTeX .Title >>\par}
<< if hasContent .Subtitle >>
\vspace{0.6cm}
{\Large << escapeLaTeX .Subtitle >>\par}
<< end >>
\end{minipage}
\end{minipage}%
}

\vspace{2cm}

\hspace{3cm}
\begin{minipage}{\dimexpr\textwidth-6cm}

<< if hasContent .Summary >>
{\large\color{accentcolor}\itshape << escapeLaTeX .Summary >>}
\vspace{1.5cm}
<< end >>

\noindent
\begin{minipage}[t]{0.5\textwidth}
{\sffamily
<< if hasContent .Author >>\textcolor{medgray}{Prepared by}\\[0.2em]
\textbf{\large << escapeLaTeX .Author >>}\\[1em]<< end >>
<< if hasContent .Date >>\textcolor{medgray}{Date}\\[0.2em]
\textbf{<< escapeLaTeX .Date >>}<< end >>
}
\end{minipage}%
\hfill
\begin{minipage}[t]{0.4\textwidth}
\raggedleft
{\sffamily
\textcolor{medgray}{Version}\\[0.2em]
\textbf{<< escapeLaTeX .Version >>}\\[1em]
\textcolor{medgray}{Status}\\[0.2em]
\textbf{<< escapeLaTeX .Status >>}
}
\end{minipage}

\end{minipage}

\vfill

\hspace{3cm}\begin{minipage}{\dimexpr\textwidth-6cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{1pt}}
\end{minipage}

\restoregeometry
\end{titlepage}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\newpage
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// MINIMAL STYLE
// ============================================================================

const minimalTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "3cm" >>, bottom=<< or .MarginBottom "3cm" >>,
  left=<< or .MarginLeft "3cm" >>, right=<< or .MarginRight "3cm" >>,
  headheight=14pt, headsep=1cm, footskip=1cm
]{geometry}

` + sharedPackages + `

\setstretch{1.3}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{333333}
\definecolor{rulecolor}{HTML}{cccccc}
\definecolor{accentcolor}{HTML}{555555}
\definecolor{lightgray}{HTML}{f5f5f5}
\definecolor{medgray}{HTML}{999999}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{f39c12}
\definecolor{statusblue}{HTML}{3498db}
\definecolor{codebg}{HTML}{f7f7f7}
\definecolor{codekey}{HTML}{0550ae}
\definecolor{codestring}{HTML}{0a3069}
\definecolor{codecomment}{HTML}{8b949e}
\definecolor{quotecolor}{HTML}{cccccc}
\definecolor{quotebg}{HTML}{fafafa}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\Large\bfseries\color{headingcolor}}
  {\thesection}{0.6em}{}

\titleformat{\subsection}
  {\large\bfseries\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\color{headingcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{1.5em}{0.6em}
\titlespacing*{\subsection}{0pt}{1.2em}{0.4em}
\titlespacing*{\subsubsection}{0pt}{0.8em}{0.3em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyfoot[C]{\small\color{medgray}Page \thepage\ of \pageref{LastPage}}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsubsecfont}{\small}
\renewcommand{\cftsecpagefont}{\bfseries}
\renewcommand{\cftsubsecpagefont}{\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=accentcolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

<< if not .NoTitlePage >>
\begin{center}
\vspace*{1cm}
{\fontsize{24}{30}\selectfont\bfseries << escapeLaTeX .Title >>\par}
<< if hasContent .Subtitle >>
\vspace{0.5cm}
{\large\color{accentcolor}<< escapeLaTeX .Subtitle >>\par}
<< end >>
\vspace{0.8cm}
{\color{medgray}
<< if hasContent .Author >><< escapeLaTeX .Author >><< end >>
<< if hasContent .Date >>\enspace$\cdot$\enspace << escapeLaTeX .Date >><< end >>
<< if hasContent .Version >>\enspace$\cdot$\enspace v<< escapeLaTeX .Version >><< end >>
}
\vspace{0.3cm}
\noindent\textcolor{rulecolor}{\rule{0.5\textwidth}{0.4pt}}
\end{center}
\vspace{1cm}
<< end >>

<< if hasContent .Summary >>
\begin{center}
\begin{minipage}{0.85\textwidth}
\itshape\color{accentcolor}<< escapeLaTeX .Summary >>
\end{minipage}
\end{center}
\vspace{1cm}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\vspace{1cm}
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// LETTER STYLE
// ============================================================================

const letterTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "3cm" >>, bottom=<< or .MarginBottom "3cm" >>,
  left=<< or .MarginLeft "3cm" >>, right=<< or .MarginRight "3cm" >>,
  headheight=14pt, headsep=1cm, footskip=1cm
]{geometry}

` + sharedPackages + `

\setstretch{1.3}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{2c3e50}
\definecolor{rulecolor}{HTML}{bdc3c7}
\definecolor{accentcolor}{HTML}{34495e}
\definecolor{lightgray}{HTML}{ecf0f1}
\definecolor{medgray}{HTML}{7f8c8d}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{f39c12}
\definecolor{statusblue}{HTML}{2980b9}
\definecolor{codebg}{HTML}{f9f9f9}
\definecolor{codekey}{HTML}{0550ae}
\definecolor{codestring}{HTML}{0a3069}
\definecolor{codecomment}{HTML}{8b949e}
\definecolor{quotecolor}{HTML}{bdc3c7}
\definecolor{quotebg}{HTML}{f8f9fa}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\large\bfseries\color{headingcolor}}
  {}{0em}{}

\titleformat{\subsection}
  {\normalsize\bfseries\color{headingcolor}}
  {}{0em}{}

\titlespacing*{\section}{0pt}{1.2em}{0.5em}
\titlespacing*{\subsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyfoot[C]{\small\color{medgray}Page \thepage\ of \pageref{LastPage}}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsecpagefont}{\bfseries}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=accentcolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

%% SENDER BLOCK
\noindent
<< if .Logo >>\includegraphics[height=1.2cm]{<< .Logo >>}\\[0.5cm]<< end >>
\begin{minipage}[t]{0.5\textwidth}
{\sffamily\bfseries\color{headingcolor}<< if .Author >><< escapeLaTeX .Author >><< end >>}
<< if hasContent .Subtitle >>\\{\small\color{accentcolor}<< escapeLaTeX .Subtitle >>}<< end >>
\end{minipage}%
\hfill
\begin{minipage}[t]{0.4\textwidth}
\raggedleft
{\small\color{medgray}<< if hasContent .Date >><< escapeLaTeX .Date >><< else >><< escapeLaTeX .GeneratedDate >><< end >>}
\end{minipage}

\vspace{1cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}
\vspace{1cm}

%% SUBJECT
<< if hasContent .Title >>
\noindent{\large\bfseries\color{headingcolor}Re: << escapeLaTeX .Title >>}
\vspace{0.8cm}
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

<< if .HasSignatures >>
\vspace{2cm}
\noindent Yours sincerely,

\vspace{1.5cm}

\noindent\rule{6cm}{0.4pt}\\[0.3em]
\noindent << if .Author >><< escapeLaTeX .Author >><< end >>
<< end >>

\end{document}
`

// ============================================================================
// ACADEMIC STYLE
// ============================================================================

const academicTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + sharedSerifFonts + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "2.5cm" >>, bottom=<< or .MarginBottom "2.5cm" >>,
  left=<< or .MarginLeft "3cm" >>, right=<< or .MarginRight "3cm" >>,
  headheight=14pt, headsep=1cm, footskip=1.2cm
]{geometry}

` + sharedPackages + `

\setstretch{1.5}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{1a1a1a}
\definecolor{rulecolor}{HTML}{333333}
\definecolor{accentcolor}{HTML}{444444}
\definecolor{lightgray}{HTML}{f0f0f0}
\definecolor{medgray}{HTML}{777777}
\definecolor{statusgreen}{HTML}{27ae60}
\definecolor{statusamber}{HTML}{d4a017}
\definecolor{statusblue}{HTML}{2980b9}
\definecolor{codebg}{HTML}{f5f5f5}
\definecolor{codekey}{HTML}{0550ae}
\definecolor{codestring}{HTML}{0a3069}
\definecolor{codecomment}{HTML}{6a737d}
\definecolor{quotecolor}{HTML}{555555}
\definecolor{quotebg}{HTML}{f8f8f8}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\Large\bfseries\color{headingcolor}}
  {\thesection}{0.6em}{}

\titleformat{\subsection}
  {\large\bfseries\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\itshape\color{headingcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{2em}{0.8em}
\titlespacing*{\subsection}{0pt}{1.5em}{0.6em}
\titlespacing*{\subsubsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0.4pt}
\renewcommand{\headrule}{\hbox to\headwidth{\color{rulecolor}\leaders\hrule height \headrulewidth\hfill}}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\itshape\color{medgray}<< or .HeaderLeft (escapeLaTeX .Title) >>}
\fancyhead[R]{\small\color{medgray}<< or .HeaderRight (escapeLaTeX .Author) >>}
\fancyfoot[C]{\small\color{medgray}\thepage}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsubsecfont}{\small}
\renewcommand{\cftsecpagefont}{\bfseries}
\renewcommand{\cftsubsecpagefont}{\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=accentcolor,
  citecolor=accentcolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

<< if not .NoTitlePage >>
%% ACADEMIC TITLE BLOCK
\begin{center}
\vspace*{2cm}

{\fontsize{20}{26}\selectfont\bfseries << escapeLaTeX .Title >>\par}

<< if hasContent .Subtitle >>
\vspace{0.6cm}
{\large\color{accentcolor}<< escapeLaTeX .Subtitle >>\par}
<< end >>

\vspace{1.2cm}

<< if hasContent .Author >>
{\large << escapeLaTeX .Author >>}\\[0.4em]
<< end >>
<< if hasContent .Date >>
{\color{medgray}<< escapeLaTeX .Date >>}
<< end >>

\vspace{0.8cm}
\noindent\textcolor{rulecolor}{\rule{0.4\textwidth}{0.5pt}}

<< if hasContent .Summary >>
\vspace{1cm}
\begin{minipage}{0.85\textwidth}
\begin{center}
\textbf{Abstract}
\end{center}
\vspace{0.3cm}
\small << escapeLaTeX .Summary >>
\end{minipage}
<< end >>

\end{center}
\vspace{1.5cm}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\newpage
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// INVOICE STYLE
// ============================================================================

const invoiceTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + `
%% Invoice style: clean sans-serif
\IfFontExistsTF{Inter}{
  \setmainfont{Inter}[Scale=0.95, Ligatures=TeX]
}{
  \IfFontExistsTF{Helvetica Neue}{
    \setmainfont{Helvetica Neue}[Ligatures=TeX]
  }{
    \setmainfont{Latin Modern Sans}[Ligatures=TeX]
  }
}
` + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "2cm" >>, bottom=<< or .MarginBottom "2cm" >>,
  left=<< or .MarginLeft "2.5cm" >>, right=<< or .MarginRight "2.5cm" >>,
  headheight=14pt, headsep=1cm, footskip=1.2cm
]{geometry}

` + sharedPackages + `

\setstretch{1.15}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{1a1a2e}
\definecolor{rulecolor}{HTML}{2563eb}
\definecolor{accentcolor}{HTML}{374151}
\definecolor{lightgray}{HTML}{f3f4f6}
\definecolor{medgray}{HTML}{6b7280}
\definecolor{statusgreen}{HTML}{059669}
\definecolor{statusamber}{HTML}{d97706}
\definecolor{statusblue}{HTML}{2563eb}
\definecolor{codebg}{HTML}{f9fafb}
\definecolor{codekey}{HTML}{7c3aed}
\definecolor{codestring}{HTML}{059669}
\definecolor{codecomment}{HTML}{9ca3af}
\definecolor{quotecolor}{HTML}{2563eb}
\definecolor{quotebg}{HTML}{eff6ff}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\large\bfseries\color{headingcolor}}
  {}{0em}{}
  [\vspace{-0.3em}\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}]

\titleformat{\subsection}
  {\normalsize\bfseries\color{headingcolor}}
  {}{0em}{}

\titlespacing*{\section}{0pt}{1.5em}{0.6em}
\titlespacing*{\subsection}{0pt}{1em}{0.4em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyfoot[L]{\footnotesize\color{medgray}<< or .FooterLeft "" >>}
\fancyfoot[C]{\footnotesize\color{medgray}Page \thepage\ of \pageref{LastPage}}
\fancyfoot[R]{\footnotesize\color{medgray}<< or .FooterRight "" >>}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsecpagefont}{\bfseries}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=rulecolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

%% INVOICE HEADER
\noindent
\begin{minipage}[t]{0.55\textwidth}
\vspace{0pt}
<< if .Logo >>\includegraphics[height=1.5cm]{<< .Logo >>}\\[0.6cm]<< end >>
{\large\bfseries\color{headingcolor}<< if .Author >><< escapeLaTeX .Author >><< end >>}
<< if hasContent .Subtitle >>\\[0.3em]{\small\color{accentcolor}<< escapeLaTeX .Subtitle >>}<< end >>
\end{minipage}%
\hfill
\begin{minipage}[t]{0.4\textwidth}
\vspace{0pt}
\raggedleft
{\fontsize{22}{28}\selectfont\bfseries\color{rulecolor}INVOICE}\\[0.6em]
<< if hasContent .Version >>{\small\color{medgray}No.\enspace}\textbf{<< escapeLaTeX .Version >>}\\[0.3em]<< end >>
<< if hasContent .Date >>{\small\color{medgray}Date:\enspace}\textbf{<< escapeLaTeX .Date >>}\\[0.3em]<< end >>
<< if hasContent .Status >>{\small\color{medgray}Status:\enspace}\textbf{\textcolor{<< statusColor .Status >>}{<< escapeLaTeX .Status >>}}<< end >>
\end{minipage}

\vspace{0.8cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{1.5pt}}
\vspace{0.8cm}

<< if hasContent .Summary >>
\noindent{\small\color{accentcolor}<< escapeLaTeX .Summary >>}
\vspace{0.6cm}
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

<< if .HasSignatures >>
\vspace{2cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{0.4pt}}
\vspace{0.5cm}
\noindent
\begin{minipage}[t]{0.45\textwidth}
{\small\color{medgray}Authorized by:}\\[1.5cm]
\rule{6cm}{0.4pt}\\[0.3em]
{\small Name / Signature}
\end{minipage}%
\hfill
\begin{minipage}[t]{0.45\textwidth}
\raggedleft
{\small\color{medgray}Date:}\\[1.5cm]
\rule{6cm}{0.4pt}\\[0.3em]
{\small Date}
\end{minipage}
<< end >>

\end{document}
`

// ============================================================================
// MEMO STYLE
// ============================================================================

const memoTemplate = `\documentclass[<< .FontSize >>pt, a4paper]{article}

` + sharedFontSetup + `
%% Memo style: clean sans-serif
\IfFontExistsTF{Inter}{
  \setmainfont{Inter}[Scale=0.95, Ligatures=TeX]
}{
  \IfFontExistsTF{Helvetica Neue}{
    \setmainfont{Helvetica Neue}[Ligatures=TeX]
  }{
    \setmainfont{Latin Modern Sans}[Ligatures=TeX]
  }
}
` + sharedSansFonts + sharedMonoFonts + `

%% === Page Geometry ===
\usepackage[
  top=<< or .MarginTop "2.5cm" >>, bottom=<< or .MarginBottom "2.5cm" >>,
  left=<< or .MarginLeft "2.5cm" >>, right=<< or .MarginRight "2.5cm" >>,
  headheight=14pt, headsep=1cm, footskip=1cm
]{geometry}

` + sharedPackages + `

\setstretch{1.25}

%% === Colors ===
\usepackage{xcolor}
\definecolor{headingcolor}{HTML}{1e293b}
\definecolor{rulecolor}{HTML}{e11d48}
\definecolor{accentcolor}{HTML}{475569}
\definecolor{lightgray}{HTML}{f1f5f9}
\definecolor{medgray}{HTML}{94a3b8}
\definecolor{statusgreen}{HTML}{16a34a}
\definecolor{statusamber}{HTML}{ca8a04}
\definecolor{statusblue}{HTML}{2563eb}
\definecolor{codebg}{HTML}{f8fafc}
\definecolor{codekey}{HTML}{7c3aed}
\definecolor{codestring}{HTML}{059669}
\definecolor{codecomment}{HTML}{94a3b8}
\definecolor{quotecolor}{HTML}{e11d48}
\definecolor{quotebg}{HTML}{fff1f2}

%% === Section Formatting ===
\usepackage{titlesec}

\titleformat{\section}
  {\large\bfseries\color{headingcolor}}
  {\thesection}{0.6em}{}

\titleformat{\subsection}
  {\normalsize\bfseries\color{headingcolor}}
  {\thesubsection}{0.5em}{}

\titleformat{\subsubsection}
  {\normalsize\bfseries\color{accentcolor}}
  {\thesubsubsection}{0.5em}{}

\titlespacing*{\section}{0pt}{1.5em}{0.5em}
\titlespacing*{\subsection}{0pt}{1.2em}{0.4em}
\titlespacing*{\subsubsection}{0pt}{0.8em}{0.3em}

%% === Header & Footer ===
\usepackage{fancyhdr}
\usepackage{lastpage}
\pagestyle{fancy}
\fancyhf{}
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

\fancyhead[L]{\small\color{rulecolor}\textbf{MEMO}}
\fancyhead[R]{\small\color{medgray}<< or .HeaderRight (escapeLaTeX .Date) >>}
\fancyfoot[C]{\small\color{medgray}Page \thepage\ of \pageref{LastPage}}

\renewcommand{\cftsecfont}{\bfseries}
\renewcommand{\cftsubsecfont}{\small}
\renewcommand{\cftsecpagefont}{\bfseries}
\renewcommand{\cftsubsecpagefont}{\small}

\hypersetup{
  colorlinks=true,
  linkcolor=headingcolor,
  urlcolor=rulecolor,
  pdftitle={<< escapeLaTeX .Title >>},
  pdfauthor={<< escapeLaTeX .Author >>},
}

\begin{document}

%% MEMO HEADER BLOCK
\noindent
\begin{minipage}{\textwidth}
<< if .Logo >>\includegraphics[height=1.2cm]{<< .Logo >>}\\[0.5cm]<< end >>
\colorbox{rulecolor!10}{%
\begin{minipage}{\dimexpr\textwidth-2\fboxsep}
\vspace{0.5cm}
\begin{tabular}{@{}l@{\hspace{0.8em}}l}
\textcolor{rulecolor}{\textbf{TO:}} & << if hasContent .Subtitle >><< escapeLaTeX .Subtitle >><< else >>---<< end >> \\[0.4em]
\textcolor{rulecolor}{\textbf{FROM:}} & << if hasContent .Author >><< escapeLaTeX .Author >><< else >>---<< end >> \\[0.4em]
\textcolor{rulecolor}{\textbf{DATE:}} & << if hasContent .Date >><< escapeLaTeX .Date >><< else >><< escapeLaTeX .GeneratedDate >><< end >> \\[0.4em]
\textcolor{rulecolor}{\textbf{RE:}} & \textbf{<< escapeLaTeX .Title >>} \\
\end{tabular}
\vspace{0.5cm}
\end{minipage}%
}
\end{minipage}

\vspace{0.3cm}
\noindent\textcolor{rulecolor}{\rule{\textwidth}{1.5pt}}
\vspace{0.6cm}

<< if hasContent .Summary >>
\noindent\textbf{Summary:} << escapeLaTeX .Summary >>
\vspace{0.6cm}
<< end >>

<< if .ShouldShowTOC >>
\tableofcontents
\vspace{1cm}
<< end >>

<< if hasContent .RawPreamble >>
<< mdToLaTeX .RawPreamble >>
<< end >>

<< range .Sections >>
\<< sectionCmd .Level >>{<< escapeLaTeX .Title >>}
<< mdToLaTeX .Content >>
<< end >>

\end{document}
`

// ============================================================================
// Template helper functions
// ============================================================================

func statusColor(status string) string {
	switch status {
	case "FINAL", "ACTIVE":
		return "statusgreen"
	case "DRAFT":
		return "statusamber"
	case "TEMPLATE":
		return "statusblue"
	default:
		return "medgray"
	}
}

func classIcon(class string) string {
	switch class {
	case "CONFIDENTIAL":
		return "CONFIDENTIAL"
	case "INTERNAL":
		return "INTERNAL USE ONLY"
	case "PUBLIC":
		return "PUBLIC"
	default:
		return ""
	}
}
