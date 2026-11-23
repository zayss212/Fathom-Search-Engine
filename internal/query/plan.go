package query

import (
	"regexp/syntax"

	"fathom/internal/trigram"
)

func RequiredTrigrams(pattern string, isRegex bool, ignoreCase bool) []uint32 {
	if ignoreCase {
		return nil
	}
	if !isRegex {
		return trigram.OfString(pattern)
	}

	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	re = re.Simplify()

	if hasAlternate(re) {
		return nil
	}

	var literals []string
	collectLiterals(re, &literals)

	var required []uint32
	for _, lit := range literals {
		required = append(required, trigram.OfString(lit)...)
	}
	return required
}

func hasAlternate(re *syntax.Regexp) bool {
	if re.Op == syntax.OpAlternate {
		return true
	}
	for _, sub := range re.Sub {
		if hasAlternate(sub) {
			return true
		}
	}
	return false
}

func collectLiterals(re *syntax.Regexp, out *[]string) {
	switch re.Op {
	case syntax.OpLiteral:
		*out = append(*out, string(re.Rune))
	case syntax.OpConcat, syntax.OpCapture:
		for _, sub := range re.Sub {
			collectLiterals(sub, out)
		}
	case syntax.OpPlus:
		collectLiterals(re.Sub[0], out)
	case syntax.OpRepeat:
		if re.Min >= 1 {
			collectLiterals(re.Sub[0], out)
		}
	default:
	}
}
