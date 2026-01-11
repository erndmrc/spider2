// Package testing provides test utilities for the spider crawler.
package testing

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

// Assertion provides assertion helpers for testing.
type Assertion struct {
	t       TestingT
	subject interface{}
	name    string
}

// TestingT is the interface for testing.T.
type TestingT interface {
	Errorf(format string, args ...interface{})
	FailNow()
	Helper()
}

// Assert creates a new assertion.
func Assert(t TestingT, subject interface{}) *Assertion {
	return &Assertion{t: t, subject: subject}
}

// Named sets a name for the assertion.
func (a *Assertion) Named(name string) *Assertion {
	a.name = name
	return a
}

// fail reports a failure.
func (a *Assertion) fail(msg string, args ...interface{}) {
	a.t.Helper()
	prefix := ""
	if a.name != "" {
		prefix = a.name + ": "
	}
	a.t.Errorf(prefix+msg, args...)
}

// IsNil asserts that the subject is nil.
func (a *Assertion) IsNil() *Assertion {
	a.t.Helper()
	if a.subject != nil {
		val := reflect.ValueOf(a.subject)
		if !val.IsNil() {
			a.fail("expected nil, got %v", a.subject)
		}
	}
	return a
}

// IsNotNil asserts that the subject is not nil.
func (a *Assertion) IsNotNil() *Assertion {
	a.t.Helper()
	if a.subject == nil {
		a.fail("expected non-nil value")
		return a
	}
	val := reflect.ValueOf(a.subject)
	if val.Kind() == reflect.Ptr && val.IsNil() {
		a.fail("expected non-nil value")
	}
	return a
}

// Equals asserts that the subject equals expected.
func (a *Assertion) Equals(expected interface{}) *Assertion {
	a.t.Helper()
	if !reflect.DeepEqual(a.subject, expected) {
		a.fail("expected %v, got %v", expected, a.subject)
	}
	return a
}

// NotEquals asserts that the subject does not equal expected.
func (a *Assertion) NotEquals(expected interface{}) *Assertion {
	a.t.Helper()
	if reflect.DeepEqual(a.subject, expected) {
		a.fail("expected value different from %v", expected)
	}
	return a
}

// IsTrue asserts that the subject is true.
func (a *Assertion) IsTrue() *Assertion {
	a.t.Helper()
	if b, ok := a.subject.(bool); !ok || !b {
		a.fail("expected true, got %v", a.subject)
	}
	return a
}

// IsFalse asserts that the subject is false.
func (a *Assertion) IsFalse() *Assertion {
	a.t.Helper()
	if b, ok := a.subject.(bool); !ok || b {
		a.fail("expected false, got %v", a.subject)
	}
	return a
}

// Contains asserts that the subject contains the substring.
func (a *Assertion) Contains(substr string) *Assertion {
	a.t.Helper()
	s, ok := a.subject.(string)
	if !ok {
		a.fail("expected string, got %T", a.subject)
		return a
	}
	if !strings.Contains(s, substr) {
		a.fail("expected '%s' to contain '%s'", s, substr)
	}
	return a
}

// NotContains asserts that the subject does not contain the substring.
func (a *Assertion) NotContains(substr string) *Assertion {
	a.t.Helper()
	s, ok := a.subject.(string)
	if !ok {
		a.fail("expected string, got %T", a.subject)
		return a
	}
	if strings.Contains(s, substr) {
		a.fail("expected '%s' to not contain '%s'", s, substr)
	}
	return a
}

// StartsWith asserts that the subject starts with prefix.
func (a *Assertion) StartsWith(prefix string) *Assertion {
	a.t.Helper()
	s, ok := a.subject.(string)
	if !ok {
		a.fail("expected string, got %T", a.subject)
		return a
	}
	if !strings.HasPrefix(s, prefix) {
		a.fail("expected '%s' to start with '%s'", s, prefix)
	}
	return a
}

// EndsWith asserts that the subject ends with suffix.
func (a *Assertion) EndsWith(suffix string) *Assertion {
	a.t.Helper()
	s, ok := a.subject.(string)
	if !ok {
		a.fail("expected string, got %T", a.subject)
		return a
	}
	if !strings.HasSuffix(s, suffix) {
		a.fail("expected '%s' to end with '%s'", s, suffix)
	}
	return a
}

// Matches asserts that the subject matches the regex pattern.
func (a *Assertion) Matches(pattern string) *Assertion {
	a.t.Helper()
	s, ok := a.subject.(string)
	if !ok {
		a.fail("expected string, got %T", a.subject)
		return a
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		a.fail("invalid regex pattern: %v", err)
		return a
	}
	if !re.MatchString(s) {
		a.fail("expected '%s' to match pattern '%s'", s, pattern)
	}
	return a
}

// HasLength asserts that the subject has the expected length.
func (a *Assertion) HasLength(expected int) *Assertion {
	a.t.Helper()
	val := reflect.ValueOf(a.subject)
	switch val.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		if val.Len() != expected {
			a.fail("expected length %d, got %d", expected, val.Len())
		}
	default:
		a.fail("cannot get length of %T", a.subject)
	}
	return a
}

// IsEmpty asserts that the subject is empty.
func (a *Assertion) IsEmpty() *Assertion {
	a.t.Helper()
	val := reflect.ValueOf(a.subject)
	switch val.Kind() {
	case reflect.String:
		if val.Len() != 0 {
			a.fail("expected empty string, got '%s'", a.subject)
		}
	case reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		if val.Len() != 0 {
			a.fail("expected empty %T, got length %d", a.subject, val.Len())
		}
	default:
		a.fail("cannot check emptiness of %T", a.subject)
	}
	return a
}

// IsNotEmpty asserts that the subject is not empty.
func (a *Assertion) IsNotEmpty() *Assertion {
	a.t.Helper()
	val := reflect.ValueOf(a.subject)
	switch val.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map, reflect.Chan:
		if val.Len() == 0 {
			a.fail("expected non-empty %T", a.subject)
		}
	default:
		a.fail("cannot check emptiness of %T", a.subject)
	}
	return a
}

// IsGreaterThan asserts that the subject is greater than expected.
func (a *Assertion) IsGreaterThan(expected int) *Assertion {
	a.t.Helper()
	val, ok := toInt(a.subject)
	if !ok {
		a.fail("expected numeric type, got %T", a.subject)
		return a
	}
	if val <= expected {
		a.fail("expected %d to be greater than %d", val, expected)
	}
	return a
}

// IsLessThan asserts that the subject is less than expected.
func (a *Assertion) IsLessThan(expected int) *Assertion {
	a.t.Helper()
	val, ok := toInt(a.subject)
	if !ok {
		a.fail("expected numeric type, got %T", a.subject)
		return a
	}
	if val >= expected {
		a.fail("expected %d to be less than %d", val, expected)
	}
	return a
}

// IsBetween asserts that the subject is between min and max (inclusive).
func (a *Assertion) IsBetween(min, max int) *Assertion {
	a.t.Helper()
	val, ok := toInt(a.subject)
	if !ok {
		a.fail("expected numeric type, got %T", a.subject)
		return a
	}
	if val < min || val > max {
		a.fail("expected %d to be between %d and %d", val, min, max)
	}
	return a
}

// toInt converts a value to int.
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	default:
		return 0, false
	}
}

// URLAssertion provides URL-specific assertions.
type URLAssertion struct {
	*Assertion
	parsed *url.URL
}

// AssertURL creates a URL assertion.
func AssertURL(t TestingT, urlStr string) *URLAssertion {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		t.Errorf("invalid URL: %s - %v", urlStr, err)
	}
	return &URLAssertion{
		Assertion: Assert(t, urlStr),
		parsed:    parsed,
	}
}

// HasScheme asserts the URL has the expected scheme.
func (u *URLAssertion) HasScheme(expected string) *URLAssertion {
	u.t.Helper()
	if u.parsed.Scheme != expected {
		u.fail("expected scheme '%s', got '%s'", expected, u.parsed.Scheme)
	}
	return u
}

// HasHost asserts the URL has the expected host.
func (u *URLAssertion) HasHost(expected string) *URLAssertion {
	u.t.Helper()
	if u.parsed.Host != expected {
		u.fail("expected host '%s', got '%s'", expected, u.parsed.Host)
	}
	return u
}

// HasPath asserts the URL has the expected path.
func (u *URLAssertion) HasPath(expected string) *URLAssertion {
	u.t.Helper()
	if u.parsed.Path != expected {
		u.fail("expected path '%s', got '%s'", expected, u.parsed.Path)
	}
	return u
}

// HasQuery asserts the URL has the expected query parameter.
func (u *URLAssertion) HasQuery(key, value string) *URLAssertion {
	u.t.Helper()
	actual := u.parsed.Query().Get(key)
	if actual != value {
		u.fail("expected query param '%s'='%s', got '%s'", key, value, actual)
	}
	return u
}

// IsAbsolute asserts the URL is absolute.
func (u *URLAssertion) IsAbsolute() *URLAssertion {
	u.t.Helper()
	if !u.parsed.IsAbs() {
		u.fail("expected absolute URL, got '%s'", u.subject)
	}
	return u
}

// IsRelative asserts the URL is relative.
func (u *URLAssertion) IsRelative() *URLAssertion {
	u.t.Helper()
	if u.parsed.IsAbs() {
		u.fail("expected relative URL, got '%s'", u.subject)
	}
	return u
}

// SliceAssertion provides slice-specific assertions.
type SliceAssertion struct {
	*Assertion
	slice reflect.Value
}

// AssertSlice creates a slice assertion.
func AssertSlice[T any](t TestingT, slice []T) *SliceAssertion {
	return &SliceAssertion{
		Assertion: Assert(t, slice),
		slice:     reflect.ValueOf(slice),
	}
}

// ContainsElement asserts the slice contains the element.
func (s *SliceAssertion) ContainsElement(element interface{}) *SliceAssertion {
	s.t.Helper()
	for i := 0; i < s.slice.Len(); i++ {
		if reflect.DeepEqual(s.slice.Index(i).Interface(), element) {
			return s
		}
	}
	s.fail("expected slice to contain %v", element)
	return s
}

// DoesNotContain asserts the slice does not contain the element.
func (s *SliceAssertion) DoesNotContain(element interface{}) *SliceAssertion {
	s.t.Helper()
	for i := 0; i < s.slice.Len(); i++ {
		if reflect.DeepEqual(s.slice.Index(i).Interface(), element) {
			s.fail("expected slice to not contain %v", element)
			return s
		}
	}
	return s
}

// MapAssertion provides map-specific assertions.
type MapAssertion struct {
	*Assertion
	mapVal reflect.Value
}

// AssertMap creates a map assertion.
func AssertMap[K comparable, V any](t TestingT, m map[K]V) *MapAssertion {
	return &MapAssertion{
		Assertion: Assert(t, m),
		mapVal:    reflect.ValueOf(m),
	}
}

// HasKey asserts the map has the key.
func (m *MapAssertion) HasKey(key interface{}) *MapAssertion {
	m.t.Helper()
	val := m.mapVal.MapIndex(reflect.ValueOf(key))
	if !val.IsValid() {
		m.fail("expected map to have key %v", key)
	}
	return m
}

// HasValue asserts the map has the key with the expected value.
func (m *MapAssertion) HasValue(key, expected interface{}) *MapAssertion {
	m.t.Helper()
	val := m.mapVal.MapIndex(reflect.ValueOf(key))
	if !val.IsValid() {
		m.fail("expected map to have key %v", key)
		return m
	}
	if !reflect.DeepEqual(val.Interface(), expected) {
		m.fail("expected map[%v] = %v, got %v", key, expected, val.Interface())
	}
	return m
}

// ErrorAssertion provides error-specific assertions.
type ErrorAssertion struct {
	*Assertion
	err error
}

// AssertError creates an error assertion.
func AssertError(t TestingT, err error) *ErrorAssertion {
	return &ErrorAssertion{
		Assertion: Assert(t, err),
		err:       err,
	}
}

// IsNoError asserts there is no error.
func (e *ErrorAssertion) IsNoError() *ErrorAssertion {
	e.t.Helper()
	if e.err != nil {
		e.fail("expected no error, got %v", e.err)
	}
	return e
}

// HasError asserts there is an error.
func (e *ErrorAssertion) HasError() *ErrorAssertion {
	e.t.Helper()
	if e.err == nil {
		e.fail("expected an error")
	}
	return e
}

// HasMessage asserts the error has the expected message.
func (e *ErrorAssertion) HasMessage(msg string) *ErrorAssertion {
	e.t.Helper()
	if e.err == nil {
		e.fail("expected an error with message '%s'", msg)
		return e
	}
	if e.err.Error() != msg {
		e.fail("expected error message '%s', got '%s'", msg, e.err.Error())
	}
	return e
}

// ContainsMessage asserts the error message contains the substring.
func (e *ErrorAssertion) ContainsMessage(substr string) *ErrorAssertion {
	e.t.Helper()
	if e.err == nil {
		e.fail("expected an error containing '%s'", substr)
		return e
	}
	if !strings.Contains(e.err.Error(), substr) {
		e.fail("expected error to contain '%s', got '%s'", substr, e.err.Error())
	}
	return e
}

// MustNotFail fails the test if there's an error.
func MustNotFail(t TestingT, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		t.FailNow()
	}
}

// MustFail fails the test if there's no error.
func MustFail(t TestingT, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("expected error but got none")
		t.FailNow()
	}
}

// Describe formats a test description.
func Describe(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
