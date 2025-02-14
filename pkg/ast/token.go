package ast

type Token int

const (
	// Special Tokens
	ILLEGAL Token = iota
	EOF
	WS
	COMMENT

	AS
	// Literals
	IDENT // main

	INTEGER   // 12345
	NUMBER    //12345.67
	STRING    // "abc"
	BADSTRING // "abc

	operatorBeg
	// ADD and the following are InfluxQL Operators
	ADD         // +
	SUB         // -
	MUL         // *
	DIV         // /
	MOD         // %
	BITWISE_AND // &
	BITWISE_OR  // |
	BITWISE_XOR // ^

	AND // AND
	OR  // OR

	EQ  // =
	NEQ // !=
	LT  // <
	LTE // <=
	GT  // >
	GTE // >=

	SUBSET //[
	ARROW  //->

	operatorEnd

	// Misc characters
	ASTERISK  // *
	COMMA     // ,
	LPAREN    // (
	RPAREN    // )
	LBRACKET  //[
	RBRACKET  //]
	HASH      // #
	DOT       // .
	COLON     //:
	SEMICOLON //;
	COLSEP    //\007

	// Keywords
	SELECT
	FROM
	JOIN
	INNER
	LEFT
	RIGHT
	FULL
	CROSS
	ON
	WHERE
	GROUP
	ORDER
	HAVING
	BY
	ASC
	DESC
	FILTER
	CASE
	WHEN
	THEN
	ELSE
	END

	TRUE
	FALSE

	CREATE
	DROP
	EXPLAIN
	DESCRIBE
	SHOW
	STREAM
	TABLE
	STREAMS
	TABLES
	WITH

	XBIGINT
	XFLOAT
	XSTRING
	XBYTEA
	XDATETIME
	XBOOLEAN
	XARRAY
	XSTRUCT

	DATASOURCE
	KEY
	FORMAT
	CONF_KEY
	TYPE
	STRICT_VALIDATION
	TIMESTAMP
	TIMESTAMP_FORMAT
	RETAIN_SIZE
	SHARED

	DD
	HH
	MI
	SS
	MS
)

var Tokens = []string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	AS:      "AS",
	WS:      "WS",
	IDENT:   "IDENT",
	INTEGER: "INTEGER",
	NUMBER:  "NUMBER",
	STRING:  "STRING",

	ADD:         "+",
	SUB:         "-",
	MUL:         "*",
	DIV:         "/",
	MOD:         "%",
	BITWISE_AND: "&",
	BITWISE_OR:  "|",
	BITWISE_XOR: "^",

	EQ:  "=",
	NEQ: "!=",
	LT:  "<",
	LTE: "<=",
	GT:  ">",
	GTE: ">=",

	SUBSET: "[]",
	ARROW:  "->",

	ASTERISK: "*",
	COMMA:    ",",

	LPAREN:    "(",
	RPAREN:    ")",
	LBRACKET:  "[",
	RBRACKET:  "]",
	HASH:      "#",
	DOT:       ".",
	SEMICOLON: ";",
	COLON:     ":",
	COLSEP:    "\007",

	SELECT: "SELECT",
	FROM:   "FROM",
	JOIN:   "JOIN",
	LEFT:   "LEFT",
	INNER:  "INNER",
	ON:     "ON",
	WHERE:  "WHERE",
	GROUP:  "GROUP",
	ORDER:  "ORDER",
	HAVING: "HAVING",
	BY:     "BY",
	ASC:    "ASC",
	DESC:   "DESC",

	CREATE:   "CREATE",
	DROP:     "RROP",
	EXPLAIN:  "EXPLAIN",
	DESCRIBE: "DESCRIBE",
	SHOW:     "SHOW",
	STREAM:   "STREAM",
	TABLE:    "TABLE",
	STREAMS:  "STREAMS",
	TABLES:   "TABLES",
	WITH:     "WITH",

	XBIGINT:   "BIGINT",
	XFLOAT:    "FLOAT",
	XSTRING:   "STRING",
	XBYTEA:    "BYTEA",
	XDATETIME: "DATETIME",
	XBOOLEAN:  "BOOLEAN",
	XARRAY:    "ARRAY",
	XSTRUCT:   "STRUCT",

	DATASOURCE:        "DATASOURCE",
	KEY:               "KEY",
	FORMAT:            "FORMAT",
	CONF_KEY:          "CONF_KEY",
	TYPE:              "TYPE",
	STRICT_VALIDATION: "STRICT_VALIDATION",
	TIMESTAMP:         "TIMESTAMP",
	TIMESTAMP_FORMAT:  "TIMESTAMP_FORMAT",
	RETAIN_SIZE:       "RETAIN_SIZE",
	SHARED:            "SHARED",

	AND:   "AND",
	OR:    "OR",
	TRUE:  "TRUE",
	FALSE: "FALSE",

	DD: "DD",
	HH: "HH",
	MI: "MI",
	SS: "SS",
	MS: "MS",
}

var COLUMN_SEPARATOR = Tokens[COLSEP]

func (tok Token) String() string {
	if tok >= 0 && tok < Token(len(Tokens)) {
		return Tokens[tok]
	}
	return ""
}

func (tok Token) IsOperator() bool {
	return (tok > operatorBeg && tok < operatorEnd) || tok == ASTERISK || tok == LBRACKET
}

func (tok Token) IsTimeLiteral() bool { return tok >= DD && tok <= MS }

func (tok Token) AllowedSourceToken() bool {
	return tok == IDENT || tok == DIV || tok == HASH || tok == ADD
}

//Allowed special field name token
func (tok Token) AllowedSFNToken() bool { return tok == DOT }

func (tok Token) Precedence() int {
	switch tok {
	case OR:
		return 1
	case AND:
		return 2
	case EQ, NEQ, LT, LTE, GT, GTE:
		return 3
	case ADD, SUB, BITWISE_OR, BITWISE_XOR:
		return 4
	case MUL, DIV, MOD, BITWISE_AND, SUBSET, ARROW:
		return 5
	}
	return 0
}

type DataType int

const (
	UNKNOWN DataType = iota
	BIGINT
	FLOAT
	STRINGS
	BYTEA
	DATETIME
	BOOLEAN
	ARRAY
	STRUCT
)

var dataTypes = []string{
	BIGINT:   "bigint",
	FLOAT:    "float",
	STRINGS:  "string",
	BYTEA:    "bytea",
	DATETIME: "datetime",
	BOOLEAN:  "boolean",
	ARRAY:    "array",
	STRUCT:   "struct",
}

func (d DataType) IsSimpleType() bool {
	return d >= BIGINT && d <= BOOLEAN
}

func (d DataType) String() string {
	if d >= 0 && d < DataType(len(dataTypes)) {
		return dataTypes[d]
	}
	return ""
}

func GetDataType(tok Token) DataType {
	switch tok {
	case XBIGINT:
		return BIGINT
	case XFLOAT:
		return FLOAT
	case XSTRING:
		return STRINGS
	case XBYTEA:
		return BYTEA
	case XDATETIME:
		return DATETIME
	case XBOOLEAN:
		return BOOLEAN
	case XARRAY:
		return ARRAY
	case XSTRUCT:
		return STRUCT
	}
	return UNKNOWN
}
