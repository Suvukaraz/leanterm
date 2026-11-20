package completion

import "strings"

// The keywords of the common languages, by LeanEdit language id (the ids
// that its language detection gives; see leanedit/languages.go). Languages
// without a list complete document words only.
var keywords = map[string]string{
	"go": `break case chan const continue default defer else fallthrough for func go goto if import
		interface map package range return select struct switch type var
		bool byte complex64 complex128 error float32 float64 int int8 int16 int32 int64 rune string
		uint uint8 uint16 uint32 uint64 uintptr any comparable true false nil iota
		append cap clear close complex copy delete imag len make max min new panic print println real recover`,
	"python": `False None True and as assert async await break class continue def del elif else except
		finally for from global if import in is lambda nonlocal not or pass raise return try while with yield
		match case self print len range list dict set tuple str int float bool bytes object type isinstance
		enumerate zip sorted reversed open super property staticmethod classmethod Exception ValueError
		TypeError KeyError IndexError RuntimeError`,
	"javascript": jsKeywords,
	"jsx":        jsKeywords,
	"typescript": jsKeywords + tsKeywords,
	"tsx":        jsKeywords + tsKeywords,
	"svelte":     jsKeywords,
	"rust": `as async await break const continue crate dyn else enum extern false fn for if impl in let loop
		match mod move mut pub ref return self Self static struct super trait true type unsafe use where while
		i8 i16 i32 i64 i128 isize u8 u16 u32 u64 u128 usize f32 f64 bool char str String Vec Option Some None
		Result Ok Err Box println format macro_rules derive`,
	"c":       cKeywords,
	"arduino": cKeywords + cppKeywords + ` setup loop pinMode digitalWrite digitalRead analogRead analogWrite delay millis Serial HIGH LOW INPUT OUTPUT`,
	"cpp":     cKeywords + cppKeywords,
	"java": `abstract assert boolean break byte case catch char class const continue default do double else
		enum extends final finally float for goto if implements import instanceof int interface long native new
		package private protected public return short static strictfp super switch synchronized this throw
		throws transient try void volatile while var record sealed permits yield true false null String Object
		Override System`,
	"kotlin": `as break class continue do else false for fun if in interface is null object package return super
		this throw true try typealias typeof val var when while by catch constructor delegate dynamic field file
		finally get import init param property receiver set setparam where actual abstract annotation companion
		const crossinline data enum expect external final infix inline inner internal lateinit noinline open
		operator out override private protected public reified sealed suspend tailrec vararg`,
	"csharp": `abstract as base bool break byte case catch char checked class const continue decimal default
		delegate do double else enum event explicit extern false finally fixed float for foreach goto if implicit
		in int interface internal is lock long namespace new null object operator out override params private
		protected public readonly ref return sbyte sealed short sizeof stackalloc static string struct switch
		this throw true try typeof uint ulong unchecked unsafe ushort using virtual void volatile while async
		await var record dynamic get set yield`,
	"dart": `abstract as assert async await break case catch class const continue covariant default deferred do
		dynamic else enum export extends extension external factory false final finally for Function get hide if
		implements import in interface is late library mixin new null on operator part required rethrow return
		set show static super switch sync this throw true try typedef var void while with yield Widget`,
	"bash": `if then else elif fi case esac for select while until do done in function time coproc return exit
		break continue declare export local readonly typeset unset shift source alias echo printf read test
		true false eval exec trap wait`,
	"powershell": `begin break catch class continue data define do dynamicparam else elseif end enum exit filter
		finally for foreach from function hidden if in param process return static switch throw trap try until
		using var while Write-Host Write-Output Get-Item Set-Item Get-ChildItem`,
	"lua": `and break do else elseif end false for function goto if in local nil not or repeat return then true
		until while require pairs ipairs print table string`,
	"sql": `SELECT FROM WHERE INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE ALTER DROP INDEX VIEW JOIN INNER
		LEFT RIGHT FULL OUTER CROSS ON AND OR NOT NULL IS IN EXISTS BETWEEN LIKE AS DISTINCT GROUP BY ORDER HAVING
		LIMIT OFFSET UNION ALL CASE WHEN THEN ELSE END PRIMARY KEY FOREIGN REFERENCES DEFAULT UNIQUE CHECK
		CONSTRAINT INTEGER VARCHAR TEXT BOOLEAN DATE TIMESTAMP COUNT SUM AVG MIN MAX COALESCE ASC DESC
		BEGIN COMMIT ROLLBACK TRANSACTION`,
	"html": `html head body title meta link script style div span section article header footer nav main aside
		form input button select option textarea label table thead tbody tr td th img href src class
		width height type value name placeholder`,
	"css":   cssKeywords,
	"scss":  cssKeywords + ` mixin include extend import use forward function return each else`,
	"less":  cssKeywords,
	"yaml":  `true false null yes no`,
	"json":  `true false null`,
	"jsonc": `true false null`,
	"toml":  `true false`,
	"dockerfile": `FROM RUN CMD LABEL EXPOSE ENV ADD COPY ENTRYPOINT VOLUME USER WORKDIR ARG ONBUILD STOPSIGNAL
		HEALTHCHECK SHELL`,
	"makefile": `ifeq ifneq ifdef ifndef else endif include define endef export override PHONY`,
	"nginx": `server location listen server_name root index proxy_pass proxy_set_header return rewrite include
		upstream http events worker_processes access_log error_log ssl_certificate ssl_certificate_key`,
}

const jsKeywords = `await break case catch class const continue debugger default delete do else export extends
	false finally for function if import in instanceof let new null return static super switch this throw true
	try typeof undefined var void while with yield async of console document window Promise Array Object
	String Number Boolean JSON Math`

const tsKeywords = ` abstract any as asserts bigint boolean declare enum implements infer interface is keyof
	namespace never number object private protected public readonly string symbol type unknown`

const cKeywords = `auto break case char const continue default do double else enum extern float for goto if
	inline int long register restrict return short signed sizeof static struct switch typedef union unsigned
	void volatile while bool true false NULL size_t uint8_t uint16_t uint32_t uint64_t int8_t int16_t int32_t
	int64_t include define ifdef ifndef endif pragma printf malloc free`

const cppKeywords = ` alignas alignof catch class constexpr consteval decltype delete explicit export friend
	mutable namespace new noexcept nullptr operator override private protected public template this throw try
	typename using virtual std string vector cout endl`

const cssKeywords = `color background border margin padding display position width height font font-size
	font-weight flex grid align-items justify-content important none block inline relative absolute fixed
	hidden visible auto inherit transparent solid`

var keywordLists = map[string][]string{}

// Keywords are the keywords of a language (nil for unknown languages and
// plain text).
func Keywords(langID string) []string {
	if l, ok := keywordLists[langID]; ok {
		return l
	}
	return nil
}

func init() {
	for id, s := range keywords {
		seen := map[string]bool{}
		var l []string
		for _, w := range strings.Fields(s) {
			if !seen[w] {
				seen[w] = true
				l = append(l, w)
			}
		}
		keywordLists[id] = l
	}
}
