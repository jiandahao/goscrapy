package goscrapy

import "regexp"

// URLMatcher url matcher
type URLMatcher interface {
	// Match returns true if url is matched
	Match(url string) bool
}

// URLRegExpMatcher url regexp matcher
type URLRegExpMatcher struct {
	reg *regexp.Regexp
}

// NewRegexpMatcher new URL matcher
func NewRegexpMatcher(str string) *URLRegExpMatcher {
	return &URLRegExpMatcher{
		reg: regexp.MustCompile(str),
	}
}

// Match returns true if url is matched
func (m *URLRegExpMatcher) Match(url string) bool {
	return m.reg.Match([]byte(url))
}

// StringMatcher static string matcher
type StringMatcher struct {
	str string
}

// NewStaticStringMatcher new static string matcher
func NewStaticStringMatcher(str string) *StringMatcher {
	return &StringMatcher{
		str: str,
	}
}

// Match returns true if url is matched
func (m *StringMatcher) Match(url string) bool {
	return m.str == url
}
