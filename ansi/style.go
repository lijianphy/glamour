package ansi

// Chroma holds all the chroma settings.
type Chroma struct {
	Text                StylePrimitive `json:"text"`
	Error               StylePrimitive `json:"error"`
	Comment             StylePrimitive `json:"comment"`
	CommentPreproc      StylePrimitive `json:"comment_preproc"`
	Keyword             StylePrimitive `json:"keyword"`
	KeywordReserved     StylePrimitive `json:"keyword_reserved"`
	KeywordNamespace    StylePrimitive `json:"keyword_namespace"`
	KeywordType         StylePrimitive `json:"keyword_type"`
	Operator            StylePrimitive `json:"operator"`
	Punctuation         StylePrimitive `json:"punctuation"`
	Name                StylePrimitive `json:"name"`
	NameBuiltin         StylePrimitive `json:"name_builtin"`
	NameTag             StylePrimitive `json:"name_tag"`
	NameAttribute       StylePrimitive `json:"name_attribute"`
	NameClass           StylePrimitive `json:"name_class"`
	NameConstant        StylePrimitive `json:"name_constant"`
	NameDecorator       StylePrimitive `json:"name_decorator"`
	NameException       StylePrimitive `json:"name_exception"`
	NameFunction        StylePrimitive `json:"name_function"`
	NameOther           StylePrimitive `json:"name_other"`
	Literal             StylePrimitive `json:"literal"`
	LiteralNumber       StylePrimitive `json:"literal_number"`
	LiteralDate         StylePrimitive `json:"literal_date"`
	LiteralString       StylePrimitive `json:"literal_string"`
	LiteralStringEscape StylePrimitive `json:"literal_string_escape"`
	GenericDeleted      StylePrimitive `json:"generic_deleted"`
	GenericEmph         StylePrimitive `json:"generic_emph"`
	GenericInserted     StylePrimitive `json:"generic_inserted"`
	GenericStrong       StylePrimitive `json:"generic_strong"`
	GenericSubheading   StylePrimitive `json:"generic_subheading"`
	Background          StylePrimitive `json:"background"`
}

// StylePrimitive holds all the basic style settings.
type StylePrimitive struct {
	BlockPrefix     string  `json:"block_prefix,omitempty"`
	BlockSuffix     string  `json:"block_suffix,omitempty"`
	Prefix          string  `json:"prefix,omitempty"`
	Suffix          string  `json:"suffix,omitempty"`
	Color           *string `json:"color,omitempty"`
	BackgroundColor *string `json:"background_color,omitempty"`
	Underline       *bool   `json:"underline,omitempty"`
	Bold            *bool   `json:"bold,omitempty"`
	Upper           *bool   `json:"upper,omitempty"`
	Lower           *bool   `json:"lower,omitempty"`
	Title           *bool   `json:"title,omitempty"`
	Italic          *bool   `json:"italic,omitempty"`
	CrossedOut      *bool   `json:"crossed_out,omitempty"`
	Faint           *bool   `json:"faint,omitempty"`
	Conceal         *bool   `json:"conceal,omitempty"`
	Inverse         *bool   `json:"inverse,omitempty"`
	Blink           *bool   `json:"blink,omitempty"`
	Format          string  `json:"format,omitempty"`
}

// StyleTask holds the style settings for a task item.
type StyleTask struct {
	StylePrimitive
	Ticked   string `json:"ticked,omitempty"`
	Unticked string `json:"unticked,omitempty"`
}

// StyleBlock holds the basic style settings for block elements.
type StyleBlock struct {
	StylePrimitive
	Indent      *uint   `json:"indent,omitempty"`
	IndentToken *string `json:"indent_token,omitempty"`
	Margin      *uint   `json:"margin,omitempty"`
}

// StyleCodeBlock holds the style settings for a code block.
type StyleCodeBlock struct {
	StyleBlock
	Theme  string  `json:"theme,omitempty"`
	Chroma *Chroma `json:"chroma,omitempty"`
}

// StyleList holds the style settings for a list.
type StyleList struct {
	StyleBlock
	LevelIndent uint `json:"level_indent,omitempty"`
}

// StyleTable holds the style settings for a table.
type StyleTable struct {
	StyleBlock
	CenterSeparator *string        `json:"center_separator,omitempty"`
	ColumnSeparator *string        `json:"column_separator,omitempty"`
	RowSeparator    *string        `json:"row_separator,omitempty"`
	Border          StylePrimitive `json:"border"`
	Header          StylePrimitive `json:"header"`
	Cell            StylePrimitive `json:"cell"`
}

// StyleConfig is used to configure the styling behavior of an ANSIRenderer.
type StyleConfig struct {
	Document   StyleBlock `json:"document"`
	BlockQuote StyleBlock `json:"block_quote"`
	Paragraph  StyleBlock `json:"paragraph"`
	List       StyleList  `json:"list"`

	Heading StyleBlock `json:"heading"`
	H1      StyleBlock `json:"h1"`
	H2      StyleBlock `json:"h2"`
	H3      StyleBlock `json:"h3"`
	H4      StyleBlock `json:"h4"`
	H5      StyleBlock `json:"h5"`
	H6      StyleBlock `json:"h6"`

	Text           StylePrimitive `json:"text"`
	Strikethrough  StylePrimitive `json:"strikethrough"`
	Emph           StylePrimitive `json:"emph"`
	Strong         StylePrimitive `json:"strong"`
	HorizontalRule StylePrimitive `json:"hr"`

	Item        StylePrimitive `json:"item"`
	Enumeration StylePrimitive `json:"enumeration"`
	Task        StyleTask      `json:"task"`

	Link     StylePrimitive `json:"link"`
	LinkText StylePrimitive `json:"link_text"`

	Image     StylePrimitive `json:"image"`
	ImageText StylePrimitive `json:"image_text"`

	Code      StyleBlock     `json:"code"`
	CodeBlock StyleCodeBlock `json:"code_block"`

	Table StyleTable `json:"table"`

	DefinitionList        StyleBlock     `json:"definition_list"`
	DefinitionTerm        StylePrimitive `json:"definition_term"`
	DefinitionDescription StylePrimitive `json:"definition_description"`

	HTMLBlock StyleBlock `json:"html_block"`
	HTMLSpan  StyleBlock `json:"html_span"`
}

func cascadeStyles(s ...StyleBlock) StyleBlock {
	var r StyleBlock
	for _, v := range s {
		r = cascadeStyle(r, v, true)
	}
	return r
}

func cascadeStylePrimitives(s ...StylePrimitive) StylePrimitive {
	var r StylePrimitive
	for _, v := range s {
		r = cascadeStylePrimitive(r, v, true)
	}
	return r
}

func cascadeStylePrimitive(parent, child StylePrimitive, toBlock bool) StylePrimitive {
	s := child

	s.Color = parent.Color
	s.BackgroundColor = parent.BackgroundColor
	s.Underline = parent.Underline
	s.Bold = parent.Bold
	s.Upper = parent.Upper
	s.Title = parent.Title
	s.Lower = parent.Lower
	s.Italic = parent.Italic
	s.CrossedOut = parent.CrossedOut
	s.Faint = parent.Faint
	s.Conceal = parent.Conceal
	s.Inverse = parent.Inverse
	s.Blink = parent.Blink

	if toBlock {
		s.BlockPrefix = parent.BlockPrefix
		s.BlockSuffix = parent.BlockSuffix
		s.Prefix = parent.Prefix
		s.Suffix = parent.Suffix
	}

	if child.Color != nil {
		s.Color = child.Color
	}
	if child.BackgroundColor != nil {
		s.BackgroundColor = child.BackgroundColor
	}
	if child.Underline != nil {
		s.Underline = child.Underline
	}
	if child.Bold != nil {
		s.Bold = child.Bold
	}
	if child.Upper != nil {
		s.Upper = child.Upper
	}
	if child.Lower != nil {
		s.Lower = child.Lower
	}
	if child.Title != nil {
		s.Title = child.Title
	}
	if child.Italic != nil {
		s.Italic = child.Italic
	}
	if child.CrossedOut != nil {
		s.CrossedOut = child.CrossedOut
	}
	if child.Faint != nil {
		s.Faint = child.Faint
	}
	if child.Conceal != nil {
		s.Conceal = child.Conceal
	}
	if child.Inverse != nil {
		s.Inverse = child.Inverse
	}
	if child.Blink != nil {
		s.Blink = child.Blink
	}
	if child.BlockPrefix != "" {
		s.BlockPrefix = child.BlockPrefix
	}
	if child.BlockSuffix != "" {
		s.BlockSuffix = child.BlockSuffix
	}
	if child.Prefix != "" {
		s.Prefix = child.Prefix
	}
	if child.Suffix != "" {
		s.Suffix = child.Suffix
	}
	if child.Format != "" {
		s.Format = child.Format
	}

	return s
}

func cascadeStyle(parent StyleBlock, child StyleBlock, toBlock bool) StyleBlock {
	s := child
	s.StylePrimitive = cascadeStylePrimitive(parent.StylePrimitive, child.StylePrimitive, toBlock)

	if toBlock {
		s.Indent = parent.Indent
		s.Margin = parent.Margin
	}

	if child.Indent != nil {
		s.Indent = child.Indent
	}

	return s
}
